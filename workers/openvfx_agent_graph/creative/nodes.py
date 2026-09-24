"""Node functions for the Creative Director graph.

Each node does one narrow job, tries the configured model, and falls back to the
deterministic equivalent when the model is absent or unusable. Every fallback is
recorded: a reader of the trace can always see which nodes actually reasoned.

Nodes return partial state dicts; LangGraph merges them.
"""

from __future__ import annotations

import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable

from ..io import read_json
from . import deterministic as det
from . import prompts
from .llm import LLMError, build_client, extract_json
from .schemas import (
    MODE_DETERMINISTIC,
    MODE_LLM,
    TREATMENT_SCHEMA,
    VALID_CONFIDENCE,
    VALID_ROLES,
)


def _iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _trace(node: str, started_ms: float, status: str, detail: str = "") -> dict[str, Any]:
    return {
        "node": node,
        "started_at": datetime.fromtimestamp(started_ms / 1000, tz=timezone.utc).isoformat(),
        "completed_at": _iso(),
        "status": status,
        "duration_ms": round(time.monotonic() * 1000 - started_ms, 1),
        "detail": detail,
    }


def _append(state: dict[str, Any], event: dict[str, Any]) -> list[dict[str, Any]]:
    return list(state.get("trace_events", [])) + [event]


def _try_llm(
    state: dict[str, Any],
    node: str,
    system: str,
    user: str,
    validate: Callable[[dict[str, Any]], dict[str, Any]],
) -> tuple[dict[str, Any] | None, str]:
    """Attempt one model call.

    Returns (result, failure_reason). A non-empty failure reason means the caller
    must fall back deterministically - it never means "use a partial result".
    """
    client = build_client(state.get("director_config", {}) or {})
    if not client.available():
        reason = getattr(client, "reason", "no model available")
        return None, reason
    try:
        raw = client.complete(system, user)
        parsed = extract_json(raw)
        return validate(parsed), ""
    except LLMError as exc:
        return None, str(exc)
    except (KeyError, TypeError, ValueError) as exc:
        # Malformed-but-parseable output: the model returned JSON of the wrong shape.
        return None, f"model output failed validation: {exc}"


def _note_fallback(state: dict[str, Any], node: str, reason: str) -> dict[str, Any]:
    """Record that a node could not reason and why."""
    warnings = list(state.get("warnings", []))
    message = f"{node}: fell back to deterministic reasoning ({reason})"
    if message not in warnings:
        warnings.append(message)
    return {
        "warnings": warnings,
        "fallback_used": True,
        "fallback_reason": state.get("fallback_reason") or reason,
    }


# ---- Node: observe ----


def observe_node(state: dict[str, Any]) -> dict[str, Any]:
    """Load the asset observations and probed capabilities from disk."""
    t0 = time.monotonic() * 1000
    root = Path(state["production_dir"])

    observations = read_json(root / "asset_observations.json")
    capabilities = read_json(root / "capabilities.json")

    warnings = list(state.get("warnings", []))
    if observations is None:
        status = "missing_observations"
        detail = "asset_observations.json not found"
        warnings.append(detail)
    elif not observations.get("assets"):
        status = "empty"
        detail = "asset_observations.json contains no assets"
        warnings.append(detail)
    else:
        status = "ok"
        detail = f"{len(observations.get('assets', []))} asset(s) loaded"

    if capabilities is None:
        warnings.append("capabilities.json not found; gap detection will be limited")

    return {
        "observations": observations,
        "capabilities": capabilities,
        "observe_status": status,
        "warnings": warnings,
        "trace_events": _append(state, _trace("observe", t0, status, detail)),
    }


# ---- Node: interpret_brief ----


def _validate_interpretation(parsed: dict[str, Any]) -> dict[str, Any]:
    if not str(parsed.get("objective", "")).strip():
        raise ValueError("objective is empty")
    duration = parsed.get("target_duration_seconds")
    if duration is not None:
        parsed["target_duration_seconds"] = float(duration)
    for key in ("tone", "visual_language", "constraints", "assumptions", "uncertainties"):
        value = parsed.get(key)
        if value is None:
            parsed[key] = []
        elif not isinstance(value, list):
            raise ValueError(f"{key} must be a list")
    for key in ("caption_strategy", "audio_strategy"):
        if not isinstance(parsed.get(key), dict):
            parsed[key] = {}
    return parsed


def interpret_brief_node(state: dict[str, Any]) -> dict[str, Any]:
    t0 = time.monotonic() * 1000
    observations = state.get("observations") or {}
    brief = state.get("brief", "")

    result, reason = _try_llm(
        state,
        "interpret_brief",
        prompts.INTERPRET_SYSTEM,
        prompts.interpret_user_prompt(brief, observations),
        _validate_interpretation,
    )

    if result is not None:
        return {
            "interpretation": result,
            "llm_calls": int(state.get("llm_calls", 0)) + 1,
            "trace_events": _append(state, _trace("interpret_brief", t0, "llm", "model interpreted the brief")),
        }

    out = _note_fallback(state, "interpret_brief", reason)
    out["interpretation"] = det.interpret(brief, observations)
    out["trace_events"] = _append(state, _trace("interpret_brief", t0, "deterministic", reason))
    return out


# ---- Node: assign_asset_roles ----


def _role_validator(observations: dict[str, Any]) -> Callable[[dict[str, Any]], dict[str, Any]]:
    known = {a.get("id"): a for a in observations.get("assets", []) or []}

    def validate(parsed: dict[str, Any]) -> dict[str, Any]:
        roles = parsed.get("asset_roles")
        if not isinstance(roles, list) or not roles:
            raise ValueError("asset_roles must be a non-empty list")
        seen = set()
        for entry in roles:
            asset_id = entry.get("asset_id")
            if asset_id not in known:
                raise ValueError(f"unknown asset_id {asset_id!r}")
            if asset_id in seen:
                raise ValueError(f"duplicate asset_id {asset_id!r}")
            seen.add(asset_id)
            if entry.get("role") not in VALID_ROLES:
                raise ValueError(f"invalid role {entry.get('role')!r}")
            if entry.get("confidence") not in VALID_CONFIDENCE:
                entry["confidence"] = "uncertain"
            entry["path"] = known[asset_id].get("path")
        missing = set(known) - seen
        if missing:
            raise ValueError(f"model omitted assets: {sorted(missing)}")
        return parsed

    return validate


def assign_asset_roles_node(state: dict[str, Any]) -> dict[str, Any]:
    t0 = time.monotonic() * 1000
    observations = state.get("observations") or {}

    if not observations.get("assets"):
        return {
            "asset_roles": [],
            "trace_events": _append(state, _trace("assign_asset_roles", t0, "skipped", "no assets")),
        }

    result, reason = _try_llm(
        state,
        "assign_asset_roles",
        prompts.ROLES_SYSTEM,
        prompts.roles_user_prompt(state.get("brief", ""), observations, state.get("interpretation", {})),
        _role_validator(observations),
    )

    if result is not None:
        return {
            "asset_roles": result["asset_roles"],
            "llm_calls": int(state.get("llm_calls", 0)) + 1,
            "trace_events": _append(
                state, _trace("assign_asset_roles", t0, "llm", f"{len(result['asset_roles'])} role(s) assigned")
            ),
        }

    out = _note_fallback(state, "assign_asset_roles", reason)
    out["asset_roles"] = det.assign_roles(observations)
    out["trace_events"] = _append(state, _trace("assign_asset_roles", t0, "deterministic", reason))
    return out


# ---- Node: design_story ----


def _story_validator(observations: dict[str, Any]) -> Callable[[dict[str, Any]], dict[str, Any]]:
    known = {a.get("id"): a for a in observations.get("assets", []) or []}

    def validate(parsed: dict[str, Any]) -> dict[str, Any]:
        segments = parsed.get("segments")
        if not isinstance(segments, list) or not segments:
            raise ValueError("segments must be a non-empty list")
        for index, segment in enumerate(segments):
            if not segment.get("id"):
                segment["id"] = f"seg_{index + 1:04d}"
            segment["order"] = int(segment.get("order") or index + 1)
            if segment.get("needs_generation"):
                continue
            asset_id = segment.get("asset_id")
            if asset_id not in known:
                raise ValueError(f"segment {segment['id']} references unknown asset {asset_id!r}")
            source_in = float(segment.get("source_in") or 0)
            source_out = float(segment.get("source_out") or 0)
            duration = float(known[asset_id].get("duration_seconds") or 0)
            if source_out <= source_in:
                raise ValueError(f"segment {segment['id']} has source_out <= source_in")
            # Models routinely overrun clip ends; clamp rather than discard the
            # whole story, but only within a small tolerance.
            if source_out > duration:
                if source_out - duration > max(1.0, duration * 0.25):
                    raise ValueError(
                        f"segment {segment['id']} runs {source_out}s past a {duration}s asset"
                    )
                segment["source_out"] = round(duration, 3)
            segment["source_in"] = round(max(0.0, source_in), 3)
        for key in ("narrative_structure", "pacing_strategy", "decisions"):
            if not isinstance(parsed.get(key), list):
                parsed[key] = []
        return parsed

    return validate


def design_story_node(state: dict[str, Any]) -> dict[str, Any]:
    t0 = time.monotonic() * 1000
    observations = state.get("observations") or {}
    interpretation = state.get("interpretation", {})
    roles = state.get("asset_roles", [])

    result, reason = _try_llm(
        state,
        "design_story",
        prompts.STORY_SYSTEM,
        prompts.story_user_prompt(state.get("brief", ""), observations, interpretation, roles),
        _story_validator(observations),
    )

    if result is not None:
        covered = sum(
            float(s.get("source_out", 0)) - float(s.get("source_in", 0))
            for s in result["segments"]
            if not s.get("needs_generation")
        )
        result["covered_seconds"] = round(covered, 3)
        return {
            "story": result,
            "llm_calls": int(state.get("llm_calls", 0)) + 1,
            "trace_events": _append(
                state, _trace("design_story", t0, "llm", f"{len(result['segments'])} segment(s) designed")
            ),
        }

    out = _note_fallback(state, "design_story", reason)
    out["story"] = det.design_story(observations, interpretation, roles)
    out["trace_events"] = _append(state, _trace("design_story", t0, "deterministic", reason))
    return out


# ---- Node: identify_gaps ----


def _validate_gaps(parsed: dict[str, Any]) -> dict[str, Any]:
    expected = ("generated_asset_needs", "transformation_requests", "degraded_alternatives")
    # A response carrying none of the expected keys is not an answer to the
    # question - it is unrelated JSON. Accepting it would let garbage output
    # count as semantic reasoning, which is exactly the failure mode the
    # reasoning-mode contract exists to prevent.
    if not any(key in parsed for key in expected):
        raise ValueError("response contained none of the expected gap fields")
    for key in expected:
        value = parsed.get(key)
        if value is None:
            parsed[key] = []
        elif not isinstance(value, list):
            raise ValueError(f"{key} must be a list")
    for index, need in enumerate(parsed["generated_asset_needs"]):
        if not need.get("id"):
            need["id"] = f"need_{index + 1:04d}"
    return parsed


def identify_gaps_node(state: dict[str, Any]) -> dict[str, Any]:
    t0 = time.monotonic() * 1000
    observations = state.get("observations") or {}
    interpretation = state.get("interpretation", {})
    story = state.get("story", {})
    capabilities = state.get("capabilities") or {}

    result, reason = _try_llm(
        state,
        "identify_gaps",
        prompts.GAPS_SYSTEM,
        prompts.gaps_user_prompt(state.get("brief", ""), observations, interpretation, story, capabilities),
        _validate_gaps,
    )

    if result is not None:
        # Capability gaps are facts, not opinions: merge the probed ones in
        # regardless of what the model said.
        probed = det.identify_gaps(observations, interpretation, story, capabilities)
        existing = {a.get("for_need") for a in result["degraded_alternatives"]}
        for alternative in probed["degraded_alternatives"]:
            if alternative.get("for_need") not in existing:
                result["degraded_alternatives"].append(alternative)
        return {
            "gaps": result,
            "llm_calls": int(state.get("llm_calls", 0)) + 1,
            "trace_events": _append(
                state,
                _trace("identify_gaps", t0, "llm", f"{len(result['generated_asset_needs'])} gap(s) identified"),
            ),
        }

    out = _note_fallback(state, "identify_gaps", reason)
    out["gaps"] = det.identify_gaps(observations, interpretation, story, capabilities)
    out["trace_events"] = _append(state, _trace("identify_gaps", t0, "deterministic", reason))
    return out


# ---- Node: design_treatment ----


def design_treatment_node(state: dict[str, Any]) -> dict[str, Any]:
    """Assemble the node outputs into one treatment document.

    This node performs no model call: it is pure composition, which keeps the
    artifact's shape independent of whichever nodes reasoned and which fell back.
    """
    t0 = time.monotonic() * 1000
    interpretation = state.get("interpretation", {}) or {}
    story = state.get("story", {}) or {}
    gaps = state.get("gaps", {}) or {}

    treatment = {
        "schema_version": TREATMENT_SCHEMA,
        "brief": state.get("brief", ""),
        "objective": interpretation.get("objective", ""),
        "audience": interpretation.get("audience", "") or "",
        "platform": interpretation.get("platform", "") or "",
        "target_duration_seconds": interpretation.get("target_duration_seconds") or 0,
        "aspect_ratio": interpretation.get("aspect_ratio", "") or "",
        "tone": interpretation.get("tone", []) or [],
        "visual_language": interpretation.get("visual_language", []) or [],
        "opening_strategy": interpretation.get("opening_strategy", "") or "",
        "narrative_structure": story.get("narrative_structure", []) or [],
        "pacing_strategy": story.get("pacing_strategy", []) or [],
        "asset_roles": state.get("asset_roles", []) or [],
        "segments": story.get("segments", []) or [],
        "audio_strategy": interpretation.get("audio_strategy", {}) or {},
        "caption_strategy": interpretation.get("caption_strategy", {}) or {},
        "generated_asset_needs": gaps.get("generated_asset_needs", []) or [],
        "transformation_requests": gaps.get("transformation_requests", []) or [],
        "degraded_alternatives": gaps.get("degraded_alternatives", []) or [],
        "constraints": interpretation.get("constraints", []) or [],
        "success_criteria": _success_criteria(interpretation, story),
        "uncertainties": interpretation.get("uncertainties", []) or [],
        "assumptions": interpretation.get("assumptions", []) or [],
        "decisions": story.get("decisions", []) or [],
    }

    return {
        "treatment": treatment,
        "trace_events": _append(
            state,
            _trace("design_treatment", t0, "ok", f"{len(treatment['segments'])} segment(s) composed"),
        ),
    }


def _success_criteria(interpretation: dict[str, Any], story: dict[str, Any]) -> list[dict[str, Any]]:
    """Turn the treatment's own goals into checkable criteria.

    These become real assertions in the Go validator, which is what stops
    "success" from being a matter of opinion.
    """
    criteria: list[dict[str, Any]] = []
    duration = interpretation.get("target_duration_seconds")
    if duration:
        criteria.append(
            {
                "id": "crit_0001",
                "assertion": "duration_within_tolerance",
                "target": f"{float(duration):g}s +/-10%",
                "seconds": float(duration),
            }
        )
    aspect = interpretation.get("aspect_ratio")
    if aspect and aspect != "unspecified":
        criteria.append({"id": "crit_0002", "assertion": "aspect_ratio_matches", "target": aspect})
    captions = interpretation.get("caption_strategy", {}) or {}
    if captions.get("required"):
        criteria.append(
            {
                "id": "crit_0003",
                "assertion": "captions_delivered",
                "target": "burned in" if captions.get("burn_in") else "delivered",
            }
        )
    audio = interpretation.get("audio_strategy", {}) or {}
    if audio.get("use_source_audio"):
        criteria.append({"id": "crit_0004", "assertion": "has_audio_stream", "target": "present"})
    return criteria


# ---- Node: critique_treatment ----


def _validate_critique(parsed: dict[str, Any]) -> dict[str, Any]:
    expected = ("findings", "revised_segments", "revision_notes")
    if not any(key in parsed for key in expected):
        raise ValueError("response contained none of the expected critique fields")
    for key in expected:
        value = parsed.get(key)
        if value is None:
            parsed[key] = []
        elif not isinstance(value, list):
            raise ValueError(f"{key} must be a list")
    return parsed


def critique_treatment_node(state: dict[str, Any]) -> dict[str, Any]:
    """One bounded critique pass. Never more than one.

    The bound is structural rather than conditional: this node runs exactly once
    in the graph and has no edge back to itself.
    """
    t0 = time.monotonic() * 1000
    observations = state.get("observations") or {}
    treatment = dict(state.get("treatment", {}) or {})

    result, reason = _try_llm(
        state,
        "critique_treatment",
        prompts.CRITIQUE_SYSTEM,
        prompts.critique_user_prompt(state.get("brief", ""), observations, treatment),
        _validate_critique,
    )

    used_llm = result is not None
    if result is None:
        result = det.critique(observations, treatment)

    findings = [str(f) for f in result.get("findings", [])]
    revisions: list[str] = [str(n) for n in result.get("revision_notes", [])]

    # Apply proposed segment replacements only if they survive the same
    # validation the original story had to pass.
    proposed = result.get("revised_segments") or []
    if proposed:
        try:
            validated = _story_validator(observations)({"segments": proposed})
            treatment["segments"] = validated["segments"]
            if not revisions:
                revisions.append("segments replaced by critique pass")
        except (KeyError, TypeError, ValueError) as exc:
            findings.append(f"critique proposed unusable segments; kept the original ({exc})")

    critique_record = {
        "performed": True,
        "findings": findings,
        "revisions": revisions,
        "passes": 1,
    }
    treatment["critique"] = critique_record

    out: dict[str, Any] = {
        "treatment": treatment,
        "critique": critique_record,
        "trace_events": _append(
            state,
            _trace(
                "critique_treatment",
                t0,
                "llm" if used_llm else "deterministic",
                f"{len(findings)} finding(s), {len(revisions)} revision(s)",
            ),
        ),
    }
    if used_llm:
        out["llm_calls"] = int(state.get("llm_calls", 0)) + 1
    else:
        out.update(_note_fallback(state, "critique_treatment", reason))
        # _note_fallback rebuilds warnings; keep the trace we just built.
        out["trace_events"] = _append(
            state, _trace("critique_treatment", t0, "deterministic", reason)
        )
    return out


# ---- Node: finalize ----


def finalize_node(state: dict[str, Any]) -> dict[str, Any]:
    """Stamp reasoning provenance onto the treatment.

    This is where the honesty contract is enforced: semantic_reasoning is true
    only when at least one node actually completed a model call, and the
    effective mode reflects what happened rather than what was requested.
    """
    t0 = time.monotonic() * 1000
    treatment = dict(state.get("treatment", {}) or {})

    requested = state.get("requested_mode") or MODE_DETERMINISTIC
    llm_calls = int(state.get("llm_calls", 0))
    effective = MODE_LLM if llm_calls > 0 else MODE_DETERMINISTIC

    config = state.get("director_config", {}) or {}
    client = build_client(config)

    treatment["reasoning"] = {
        "requested_mode": requested,
        "effective_mode": effective,
        "graph": "creative_director.v1",
        "provider": getattr(client, "provider", "") if llm_calls else "",
        "model": getattr(client, "model", "") if llm_calls else "",
        "backend": getattr(client, "backend", "") if llm_calls else "",
        "route": str(config.get("route", "")),
        "fallback_used": bool(state.get("fallback_used")),
        "fallback_reason": state.get("fallback_reason", "") or "",
        "semantic_reasoning": effective == MODE_LLM,
    }
    treatment["warnings"] = list(state.get("warnings", []))

    return {
        "treatment": treatment,
        "effective_mode": effective,
        "trace_events": _append(
            state,
            _trace("finalize", t0, effective, f"{llm_calls} model call(s) completed"),
        ),
    }

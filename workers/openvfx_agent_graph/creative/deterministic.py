"""Deterministic creative reasoning.

This is the degraded path taken when no language model is configured or when the
configured one fails. It produces a genuine, useful treatment from observable
evidence - durations, audio presence, transcript presence, dimensions, and
literal phrases in the brief.

What it must never do is pretend to be semantic reasoning. Every treatment it
produces carries effective_mode "deterministic" and semantic_reasoning false,
and it records its own limits in `uncertainties` so the creator can see what was
inferred by rule rather than understood.
"""

from __future__ import annotations

import re
from typing import Any, Optional

_DURATION_SECONDS = re.compile(r"(\d+)[\s-]*(?:second|sec|s)\b", re.IGNORECASE)
_DURATION_MINUTES = re.compile(r"(\d+)[\s-]*(?:minute|min)\b", re.IGNORECASE)

_TONE_WORDS = (
    "cinematic", "premium", "dramatic", "aggressive", "energetic", "calm",
    "moody", "playful", "serious", "luxury", "gritty", "clean", "warm", "cold",
    "intense", "intimate", "epic", "minimal",
)

_VISUAL_WORDS = (
    "handheld", "static", "slow motion", "close-up", "wide", "macro",
    "high contrast", "desaturated", "saturated", "grain", "vignette",
    "shallow depth", "neon", "natural light", "silhouette",
)


def parse_duration(brief: str) -> Optional[float]:
    lowered = brief.lower()
    minutes = _DURATION_MINUTES.search(lowered)
    seconds = _DURATION_SECONDS.search(lowered)
    if seconds:
        return float(seconds.group(1))
    if minutes:
        return float(minutes.group(1)) * 60
    return None


def parse_platform(brief: str) -> str:
    lowered = brief.lower()
    if "instagram" in lowered or "reel" in lowered:
        return "instagram_reel"
    if "tiktok" in lowered:
        return "tiktok"
    if "short" in lowered and "youtube" in lowered:
        return "youtube_short"
    if "youtube" in lowered:
        return "youtube"
    return "unspecified"


def parse_aspect(brief: str) -> str:
    lowered = brief.lower()
    if any(w in lowered for w in ("vertical", "9:16", "reel", "tiktok", "short")):
        return "9:16"
    if "square" in lowered or "1:1" in lowered:
        return "1:1"
    if any(w in lowered for w in ("widescreen", "16:9", "landscape")):
        return "16:9"
    return "unspecified"


def _words_present(brief: str, vocabulary: tuple[str, ...], limit: int = 4) -> list[str]:
    lowered = brief.lower()
    return [w for w in vocabulary if w in lowered][:limit]


def interpret(brief: str, observations: dict[str, Any]) -> dict[str, Any]:
    """Rule-based reading of the brief."""
    lowered = brief.lower()
    wants_captions = "caption" in lowered or "subtitle" in lowered
    burn_in = wants_captions and not ("sidecar" in lowered or "srt file" in lowered)

    style = ""
    if "bold" in lowered:
        style = "bold"
    elif "boxed" in lowered:
        style = "boxed"

    position = ""
    if "low" in lowered or "bottom" in lowered:
        position = "bottom"
    elif "center" in lowered or "centre" in lowered:
        position = "center"
    elif "top" in lowered:
        position = "top"

    has_audio = int(observations.get("totals", {}).get("with_audio", 0) or 0) > 0

    uncertainties = [
        "Creative intent was derived by keyword rules, not semantic understanding; "
        "nuance in the request may have been missed.",
    ]
    if not _words_present(brief, _TONE_WORDS):
        uncertainties.append("No recognised tone vocabulary found in the brief.")

    return {
        "objective": _objective_sentence(brief, observations),
        "audience": "",
        "platform": parse_platform(brief),
        "target_duration_seconds": parse_duration(brief),
        "aspect_ratio": parse_aspect(brief),
        "tone": _words_present(brief, _TONE_WORDS),
        "visual_language": _words_present(brief, _VISUAL_WORDS),
        "opening_strategy": (
            "Open on the first available asset; the brief was not parsed for a "
            "specific opening beat."
        ),
        "caption_strategy": {
            "required": wants_captions,
            "burn_in": burn_in,
            "style": style,
            "position": position,
            "notes": "derived from keyword rules",
        },
        "audio_strategy": {
            "use_source_audio": has_audio,
            "narration_plan": "",
            "music_plan": "",
            "notes": ["source audio retained where present"] if has_audio else [],
        },
        "constraints": [],
        "assumptions": [
            "Assets are used in the order they were supplied.",
        ],
        "uncertainties": uncertainties,
    }


def _objective_sentence(brief: str, observations: dict[str, Any]) -> str:
    count = observations.get("totals", {}).get("asset_count", 0)
    duration = parse_duration(brief)
    platform = parse_platform(brief)
    parts = ["Assemble"]
    if duration:
        parts.append(f"a {duration:g}-second edit")
    else:
        parts.append("an edit")
    parts.append(f"from {count} supplied asset(s)")
    if platform != "unspecified":
        parts.append(f"for {platform.replace('_', ' ')}")
    return " ".join(parts) + "."


def assign_roles(observations: dict[str, Any]) -> list[dict[str, Any]]:
    """Assign roles from observable evidence only.

    The confidence field carries the honesty: a role inferred from "this clip
    has audio and a transcript" is medium confidence at best, and a clip we know
    nothing distinguishing about is explicitly unknown/uncertain.
    """
    assets = observations.get("assets", []) or []
    if not assets:
        return []

    roles: list[dict[str, Any]] = []
    longest = max(assets, key=lambda a: float(a.get("duration_seconds") or 0))

    for index, asset in enumerate(assets):
        asset_id = asset.get("id")
        has_audio = bool(asset.get("has_audio"))
        has_transcript = bool(asset.get("has_transcript"))
        is_audio_only = asset.get("media_type") == "audio_only"

        if is_audio_only:
            role, confidence, why = (
                "audio_bed",
                "high",
                "asset carries audio with no video stream",
            )
        elif has_transcript and has_audio:
            role, confidence, why = (
                "primary_narration",
                "medium",
                "asset has audio and a transcript, so it can carry spoken content",
            )
        elif index == 0:
            role, confidence, why = (
                "hook",
                "low",
                "first supplied asset; opening position inferred from order, not content",
            )
        elif asset_id == longest.get("id"):
            role, confidence, why = (
                "hero_shot",
                "low",
                "longest asset by duration",
            )
        elif not has_audio:
            role, confidence, why = (
                "b_roll",
                "low",
                "no audio stream, so usable as silent coverage",
            )
        else:
            role, confidence, why = (
                "unknown",
                "uncertain",
                "no distinguishing evidence available without content analysis",
            )

        roles.append(
            {
                "asset_id": asset_id,
                "path": asset.get("path"),
                "role": role,
                "confidence": confidence,
                "rationale": why,
            }
        )
    return roles


def design_story(
    observations: dict[str, Any],
    interpretation: dict[str, Any],
    roles: list[dict[str, Any]],
) -> dict[str, Any]:
    """Build beats, pacing and segments by arithmetic over real durations."""
    assets = [a for a in observations.get("assets", []) or [] if a.get("has_video")]
    target = interpretation.get("target_duration_seconds") or 0.0

    segments: list[dict[str, Any]] = []
    timeline = 0.0
    for asset in assets:
        if target and timeline >= target - 0.05:
            break
        duration = float(asset.get("duration_seconds") or 0)
        take = duration
        if target and timeline + take > target:
            take = target - timeline
        if take <= 0.05:
            continue
        segments.append(
            {
                "id": f"seg_{len(segments) + 1:04d}",
                "order": len(segments) + 1,
                "asset_id": asset.get("id"),
                "source_in": 0.0,
                "source_out": round(take, 3),
                "purpose": "sequential coverage selected by duration arithmetic",
                "beat_ref": "beat_0001",
                "decision_id": "dec_0001",
                "needs_generation": False,
            }
        )
        timeline += take

    beats = [
        {
            "id": "beat_0001",
            "name": "assembly",
            "purpose": "Play the supplied coverage in the order it was provided.",
            "approx_seconds": round(timeline, 3),
        }
    ]
    pacing = [
        {
            "id": "phase_0001",
            "from_seconds": 0.0,
            "to_seconds": round(timeline, 3),
            "intent": "Even pacing; no phase structure was derived without semantic reasoning.",
            "cut_style": "medium",
        }
    ]
    decisions = [
        {
            "id": "dec_0001",
            "summary": "Use assets in supplied order, trimming the last to hit the target.",
            "rationale": "Deterministic fallback: no model was available to reason about content.",
            "evidence": "asset_observations.json durations",
        }
    ]
    return {
        "narrative_structure": beats,
        "pacing_strategy": pacing,
        "segments": segments,
        "decisions": decisions,
        "covered_seconds": round(timeline, 3),
    }


def identify_gaps(
    observations: dict[str, Any],
    interpretation: dict[str, Any],
    story: dict[str, Any],
    capabilities: dict[str, Any],
) -> dict[str, Any]:
    """Report only gaps that arithmetic or capability probing can prove."""
    needs: list[dict[str, Any]] = []
    alternatives: list[dict[str, Any]] = []

    target = float(interpretation.get("target_duration_seconds") or 0)
    covered = float(story.get("covered_seconds") or 0)

    if target and covered < target - 0.05:
        shortfall = round(target - covered, 2)
        needs.append(
            {
                "id": "need_0001",
                "kind": "generated_broll",
                "description": (
                    f"{shortfall}s of additional coverage to reach the {target:g}s target"
                ),
                "required": False,
                "reason": (
                    f"supplied assets total {covered:g}s of usable coverage, "
                    f"short of the {target:g}s target"
                ),
            }
        )
        alternatives.append(
            {
                "id": "alt_0001",
                "for_need": "need_0001",
                "approach": "Extend the edit by repeating the tail of the timeline.",
                "impact": "Repeated footage rather than new coverage.",
            }
        )

    captions = interpretation.get("caption_strategy", {}) or {}
    if captions.get("required") and not int(
        observations.get("totals", {}).get("with_audio", 0) or 0
    ):
        needs.append(
            {
                "id": f"need_{len(needs) + 1:04d}",
                "kind": "narration_audio",
                "description": "Spoken audio to transcribe into captions",
                "required": True,
                "reason": "captions were requested but no asset carries an audio stream",
            }
        )

    # A requested burn-in this machine cannot perform is a real, provable gap.
    if captions.get("burn_in") and capabilities:
        unavailable = {
            c.get("id")
            for c in capabilities.get("capabilities", [])
            if c.get("status") != "available"
        }
        if "ffmpeg.filter.subtitles" in unavailable:
            alternatives.append(
                {
                    "id": f"alt_{len(alternatives) + 1:04d}",
                    "for_need": "caption_burn_in",
                    "approach": "Deliver captions as a sidecar SRT alongside the video.",
                    "impact": "Captions are not baked into the picture.",
                }
            )

    return {
        "generated_asset_needs": needs,
        "transformation_requests": [],
        "degraded_alternatives": alternatives,
    }


def critique(
    observations: dict[str, Any], treatment: dict[str, Any]
) -> dict[str, Any]:
    """Deterministic consistency checks over the assembled treatment."""
    findings: list[str] = []
    assets = {a.get("id"): a for a in observations.get("assets", []) or []}

    target = float(treatment.get("target_duration_seconds") or 0)
    total = 0.0
    for segment in treatment.get("segments", []) or []:
        if segment.get("needs_generation"):
            continue
        asset = assets.get(segment.get("asset_id"))
        if asset is None:
            findings.append(
                f"{segment.get('id')} references unknown asset {segment.get('asset_id')!r}"
            )
            continue
        out = float(segment.get("source_out") or 0)
        duration = float(asset.get("duration_seconds") or 0)
        if out > duration + 0.05:
            findings.append(
                f"{segment.get('id')} ends at {out:g}s but "
                f"{segment.get('asset_id')} is only {duration:g}s long"
            )
        total += out - float(segment.get("source_in") or 0)

    if target and abs(total - target) > target * 0.1:
        findings.append(
            f"planned segments total {round(total, 2)}s against a {target:g}s target"
        )

    if not treatment.get("segments"):
        findings.append("treatment contains no segments")

    return {"findings": findings, "revised_segments": [], "revision_notes": []}

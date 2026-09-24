"""Tests for the Creative Director graph.

Every model interaction here is stubbed. No test requires a running Ollama, and
none makes a network call - the LLM client is replaced at the seam.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from openvfx_agent_graph.creative import deterministic as det
from openvfx_agent_graph.creative import nodes as creative_nodes
from openvfx_agent_graph.creative.graph import build_creative_graph, initial_state
from openvfx_agent_graph.creative.llm import (
    LLMClient,
    LLMError,
    NullClient,
    build_client,
    extract_json,
)

RICH_BRIEF = (
    "Make a 25-second cinematic Instagram reel from these clips. Open aggressively "
    "on the gym footage. Use the talking clip only where the delivery is strongest. "
    "Build tension for roughly the first 15 seconds and then slow down for the final "
    "line. Keep captions bold and low. If there isn't enough atmospheric coverage, "
    "request generated inserts. I want it to feel premium and dramatic rather than "
    "like a generic social edit."
)


# ---- fixtures ----


def _observations() -> dict:
    return {
        "schema_version": "openvfx_asset_observations.v1",
        "input_path": "/tmp/assets",
        "assets": [
            {
                "id": "asset_0001",
                "path": "/tmp/assets/gym.mp4",
                "media_type": "video_only",
                "duration_seconds": 12.0,
                "width": 1920,
                "height": 1080,
                "aspect_ratio": "16:9",
                "has_video": True,
                "has_audio": False,
                "has_transcript": False,
                "origin": "source",
            },
            {
                "id": "asset_0002",
                "path": "/tmp/assets/talking.mp4",
                "media_type": "audiovisual",
                "duration_seconds": 20.0,
                "width": 1920,
                "height": 1080,
                "aspect_ratio": "16:9",
                "has_video": True,
                "has_audio": True,
                "has_transcript": True,
                "transcript_chars": 420,
                "origin": "source",
            },
        ],
        "totals": {
            "asset_count": 2,
            "duration_seconds": 32.0,
            "with_audio": 1,
            "with_video": 2,
        },
    }


def _capabilities() -> dict:
    return {
        "schema_version": "openvfx_capabilities.v1",
        "capabilities": [
            {"id": "ffmpeg.binary", "status": "available"},
            {"id": "ffmpeg.filter.scale", "status": "available"},
            {
                "id": "ffmpeg.filter.subtitles",
                "status": "unavailable",
                "detail": "libass not compiled in; caption burn-in unavailable",
            },
            {"id": "asr.faster_whisper", "status": "available"},
        ],
    }


def _write_production(tmp_path: Path) -> Path:
    root = tmp_path / "prod"
    root.mkdir()
    (root / "asset_observations.json").write_text(json.dumps(_observations()))
    (root / "capabilities.json").write_text(json.dumps(_capabilities()))
    return root


class StubClient(LLMClient):
    """A scripted model. Each call pops the next canned response."""

    name = "stub"
    provider = "stub-provider"

    def __init__(self, responses: list[str], model: str = "stub-model") -> None:
        self.responses = list(responses)
        self.model = model
        self.backend = "stub://local"
        self.calls: list[tuple[str, str]] = []

    def available(self) -> bool:
        return True

    def complete(self, system: str, user: str) -> str:
        self.calls.append((system, user))
        if not self.responses:
            raise LLMError("stub exhausted")
        return self.responses.pop(0)


def _rich_responses() -> list[str]:
    """What a competent model would return for the rich brief."""
    interpret = {
        "objective": "A premium, dramatic 25-second vertical teaser that opens hard on gym footage and lands on a single spoken line.",
        "audience": "Fitness-adjacent social audience expecting cinematic production value",
        "platform": "instagram_reel",
        "target_duration_seconds": 25,
        "aspect_ratio": "9:16",
        "tone": ["cinematic", "premium", "dramatic", "tense"],
        "visual_language": ["high contrast", "shallow depth", "controlled motion"],
        "opening_strategy": "Open on the gym footage with no build - the first frame should already be at intensity.",
        "caption_strategy": {
            "required": True,
            "burn_in": True,
            "style": "bold",
            "position": "bottom",
            "notes": "Bold lower-third captions, kept low so they never cross the subject.",
        },
        "audio_strategy": {
            "use_source_audio": True,
            "narration_plan": "Use the talking clip only for the final line.",
            "music_plan": "",
            "notes": ["Source audio carries the spoken line"],
        },
        "constraints": ["Must not read as a generic social edit"],
        "assumptions": ["The gym footage is the visually strongest material"],
        "uncertainties": ["Which part of the talking clip has the strongest delivery"],
    }
    roles = {
        "asset_roles": [
            {
                "asset_id": "asset_0001",
                "role": "hook",
                "confidence": "high",
                "rationale": "Silent, visually driven footage suits an aggressive cold open.",
            },
            {
                "asset_id": "asset_0002",
                "role": "primary_narration",
                "confidence": "high",
                "rationale": "Only asset with audio and a transcript, so it carries the spoken line.",
            },
        ]
    }
    story = {
        "narrative_structure": [
            {"id": "beat_0001", "name": "cold open", "purpose": "Start at peak intensity", "approx_seconds": 6},
            {"id": "beat_0002", "name": "build", "purpose": "Escalate tension", "approx_seconds": 9},
            {"id": "beat_0003", "name": "release", "purpose": "Slow down for the final line", "approx_seconds": 10},
        ],
        "pacing_strategy": [
            {"id": "phase_0001", "from_seconds": 0, "to_seconds": 15, "intent": "Accelerating tension", "cut_style": "fast"},
            {"id": "phase_0002", "from_seconds": 15, "to_seconds": 25, "intent": "Settle for the spoken line", "cut_style": "slow"},
        ],
        "segments": [
            {"id": "seg_0001", "order": 1, "asset_id": "asset_0001", "source_in": 0, "source_out": 6,
             "purpose": "Aggressive cold open", "beat_ref": "beat_0001", "decision_id": "dec_0001"},
            {"id": "seg_0002", "order": 2, "asset_id": "asset_0001", "source_in": 6, "source_out": 12,
             "purpose": "Continue the build", "beat_ref": "beat_0002", "decision_id": "dec_0001"},
            {"id": "seg_0003", "order": 3, "asset_id": "asset_0002", "source_in": 8, "source_out": 15,
             "purpose": "Strongest delivery of the spoken line", "beat_ref": "beat_0003", "decision_id": "dec_0002"},
        ],
        "decisions": [
            {"id": "dec_0001", "summary": "Open on gym footage and hold it for the whole build.",
             "rationale": "The creator asked to open aggressively.", "evidence": "brief: 'Open aggressively on the gym footage'"},
            {"id": "dec_0002", "summary": "Use the talking clip only at the end.",
             "rationale": "The creator wants it only where delivery is strongest.", "evidence": "brief: 'only where the delivery is strongest'"},
        ],
    }
    gaps = {
        "generated_asset_needs": [
            {"id": "need_0001", "kind": "generated_broll",
             "description": "Atmospheric insert to cover the transition into the final line",
             "required": False,
             "reason": "No atmospheric coverage exists in the supplied assets."}
        ],
        "transformation_requests": [],
        "degraded_alternatives": [
            {"id": "alt_0001", "for_need": "need_0001",
             "approach": "Hold the gym footage longer instead of cutting to an insert.",
             "impact": "Less visual variety through the middle."}
        ],
    }
    critique = {"findings": [], "revised_segments": [], "revision_notes": []}
    return [json.dumps(x) for x in (interpret, roles, story, gaps, critique)]


def _run_graph(monkeypatch, root: Path, client: LLMClient, mode: str = "llm") -> dict:
    monkeypatch.setattr(creative_nodes, "build_client", lambda cfg: client)
    graph = build_creative_graph()
    state = initial_state(
        production_id="prod_test",
        production_dir=str(root),
        brief=RICH_BRIEF,
        director_config={"mode": mode, "provider": "stub", "model": "stub-model"},
    )
    return graph.invoke(state)


# ---- multi-asset observation ----


def test_observe_loads_multiple_assets(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    assert final["observe_status"] == "ok"
    assert len(final["observations"]["assets"]) == 2
    assert final["capabilities"] is not None


def test_observe_reports_missing_observations(tmp_path, monkeypatch):
    root = tmp_path / "empty"
    root.mkdir()
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    assert final["observe_status"] == "missing_observations"
    assert any("asset_observations.json" in w for w in final["warnings"])


# ---- rich treatment generation ----


def test_rich_brief_produces_a_real_treatment(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    client = StubClient(_rich_responses())
    final = _run_graph(monkeypatch, root, client)
    treatment = final["treatment"]

    # The point of the milestone: the request is not flattened to a keyword.
    assert treatment["target_duration_seconds"] == 25
    assert treatment["platform"] == "instagram_reel"
    assert "dramatic" in treatment["tone"]
    assert len(treatment["narrative_structure"]) == 3
    assert len(treatment["pacing_strategy"]) == 2
    assert treatment["pacing_strategy"][0]["cut_style"] == "fast"
    assert treatment["pacing_strategy"][1]["cut_style"] == "slow"
    assert len(treatment["segments"]) == 3
    assert treatment["caption_strategy"]["style"] == "bold"
    assert treatment["caption_strategy"]["position"] == "bottom"
    assert treatment["opening_strategy"]
    assert len(treatment["decisions"]) == 2


def test_every_node_receives_its_own_prompt(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    client = StubClient(_rich_responses())
    _run_graph(monkeypatch, root, client)
    # interpret, roles, story, gaps, critique
    assert len(client.calls) == 5
    # Each node must carry its own task instructions, not one mega-prompt.
    systems = [call[0] for call in client.calls]
    assert len(set(systems)) == 5


# ---- reasoning mode honesty ----


def test_effective_mode_is_llm_when_a_model_reasoned(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    reasoning = final["treatment"]["reasoning"]
    assert reasoning["requested_mode"] == "llm"
    assert reasoning["effective_mode"] == "llm"
    assert reasoning["semantic_reasoning"] is True
    assert reasoning["fallback_used"] is False


def test_deterministic_path_never_claims_semantic_reasoning(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(
        monkeypatch, root, NullClient("no model configured"), mode="deterministic"
    )
    reasoning = final["treatment"]["reasoning"]
    assert reasoning["requested_mode"] == "deterministic"
    assert reasoning["effective_mode"] == "deterministic"
    assert reasoning["semantic_reasoning"] is False
    assert reasoning["model"] == ""
    # It must still produce a usable treatment.
    assert final["treatment"]["segments"]


def test_missing_model_falls_back_and_records_the_reason(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(
        monkeypatch, root, NullClient("provider 'ollama' selected but no model configured"), mode="llm"
    )
    reasoning = final["treatment"]["reasoning"]
    assert reasoning["requested_mode"] == "llm"
    assert reasoning["effective_mode"] == "deterministic"
    assert reasoning["semantic_reasoning"] is False
    assert reasoning["fallback_used"] is True
    assert "no model configured" in reasoning["fallback_reason"]


def test_malformed_model_output_falls_back_rather_than_crashing(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    client = StubClient(["this is not JSON at all"] * 5)
    final = _run_graph(monkeypatch, root, client)
    reasoning = final["treatment"]["reasoning"]
    assert reasoning["effective_mode"] == "deterministic"
    assert reasoning["fallback_used"] is True
    # A usable treatment still comes out the other side.
    assert final["treatment"]["segments"]
    assert any("interpret_brief" in w for w in final["warnings"])


def test_wrong_shaped_model_output_is_rejected(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    # Valid JSON, wrong shape: objective missing.
    client = StubClient([json.dumps({"platform": "tiktok"})] * 5)
    final = _run_graph(monkeypatch, root, client)
    assert final["treatment"]["reasoning"]["effective_mode"] == "deterministic"
    assert final["treatment"]["objective"]  # deterministic filled it in


def test_hallucinated_asset_id_is_rejected(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    responses[1] = json.dumps(
        {"asset_roles": [{"asset_id": "asset_9999", "role": "hook", "confidence": "high"}]}
    )
    client = StubClient(responses)
    final = _run_graph(monkeypatch, root, client)
    roles = {r["asset_id"] for r in final["treatment"]["asset_roles"]}
    assert roles == {"asset_0001", "asset_0002"}
    assert any("assign_asset_roles" in w for w in final["warnings"])


def test_segment_overrunning_asset_is_clamped_or_rejected(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    story = json.loads(responses[2])
    # asset_0001 is 12s; ask for 400s.
    story["segments"][0]["source_out"] = 400.0
    responses[2] = json.dumps(story)
    final = _run_graph(monkeypatch, root, StubClient(responses))
    for segment in final["treatment"]["segments"]:
        if segment.get("asset_id") == "asset_0001":
            assert segment["source_out"] <= 12.05


def test_small_overrun_is_clamped_not_discarded(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    story = json.loads(responses[2])
    story["segments"][1]["source_out"] = 12.4  # asset_0001 is 12.0s
    responses[2] = json.dumps(story)
    final = _run_graph(monkeypatch, root, StubClient(responses))
    assert final["treatment"]["reasoning"]["effective_mode"] == "llm"
    assert final["treatment"]["segments"][1]["source_out"] == 12.0


# ---- asset roles ----


def test_roles_cover_every_asset_exactly_once(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    roles = final["treatment"]["asset_roles"]
    assert [r["asset_id"] for r in roles] == ["asset_0001", "asset_0002"]
    assert roles[1]["role"] == "primary_narration"


def test_deterministic_roles_admit_uncertainty():
    observations = _observations()
    # Strip the distinguishing evidence from the second asset.
    observations["assets"][1]["has_transcript"] = False
    roles = det.assign_roles(observations)
    assert len(roles) == 2
    assert all(r["confidence"] in {"high", "medium", "low", "uncertain"} for r in roles)
    # Without evidence, the rules must not claim high confidence.
    assert all(r["confidence"] != "high" or r["role"] == "audio_bed" for r in roles)


# ---- gap detection ----


def test_gaps_are_reported_with_a_degraded_alternative(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    treatment = final["treatment"]
    assert len(treatment["generated_asset_needs"]) == 1
    assert treatment["generated_asset_needs"][0]["kind"] == "generated_broll"
    approaches = [a["approach"] for a in treatment["degraded_alternatives"]]
    assert any("Hold the gym footage" in a for a in approaches)


def test_probed_capability_gap_is_merged_even_when_the_model_misses_it(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    responses[3] = json.dumps(
        {"generated_asset_needs": [], "transformation_requests": [], "degraded_alternatives": []}
    )
    final = _run_graph(monkeypatch, root, StubClient(responses))
    # subtitles is unavailable in _capabilities(); the sidecar fallback is a fact.
    fors = [a["for_need"] for a in final["treatment"]["degraded_alternatives"]]
    assert "caption_burn_in" in fors


def test_deterministic_gap_detection_finds_coverage_shortfall():
    observations = _observations()
    interpretation = {"target_duration_seconds": 60, "caption_strategy": {}}
    story = {"covered_seconds": 32.0}
    gaps = det.identify_gaps(observations, interpretation, story, {})
    assert gaps["generated_asset_needs"]
    assert "short of the 60s target" in gaps["generated_asset_needs"][0]["reason"]


# ---- critique ----


def test_critique_runs_exactly_one_pass(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    critique = final["treatment"]["critique"]
    assert critique["performed"] is True
    assert critique["passes"] == 1


def test_critique_may_replace_segments_once(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    responses[4] = json.dumps(
        {
            "findings": ["The middle drags against the stated build."],
            "revised_segments": [
                {"id": "seg_0001", "order": 1, "asset_id": "asset_0001", "source_in": 0, "source_out": 4},
                {"id": "seg_0002", "order": 2, "asset_id": "asset_0002", "source_in": 8, "source_out": 12},
            ],
            "revision_notes": ["Tightened the cold open to 4s."],
        }
    )
    final = _run_graph(monkeypatch, root, StubClient(responses))
    treatment = final["treatment"]
    assert len(treatment["segments"]) == 2
    assert treatment["segments"][0]["source_out"] == 4
    assert treatment["critique"]["findings"]
    assert treatment["critique"]["revisions"]


def test_critique_rejects_unusable_replacement_segments(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    responses = _rich_responses()
    responses[4] = json.dumps(
        {
            "findings": [],
            "revised_segments": [
                {"id": "seg_x", "order": 1, "asset_id": "nonexistent", "source_in": 0, "source_out": 3}
            ],
            "revision_notes": [],
        }
    )
    final = _run_graph(monkeypatch, root, StubClient(responses))
    # Original segments survive; the failure is recorded as a finding.
    assert len(final["treatment"]["segments"]) == 3
    assert any("unusable" in f for f in final["treatment"]["critique"]["findings"])


def test_deterministic_critique_flags_duration_mismatch():
    observations = _observations()
    treatment = {
        "target_duration_seconds": 25,
        "segments": [
            {"id": "seg_0001", "asset_id": "asset_0001", "source_in": 0, "source_out": 3}
        ],
    }
    result = det.critique(observations, treatment)
    assert any("against a 25s target" in f for f in result["findings"])


# ---- success criteria ----


def test_success_criteria_are_derived_from_the_treatment(tmp_path, monkeypatch):
    root = _write_production(tmp_path)
    final = _run_graph(monkeypatch, root, StubClient(_rich_responses()))
    assertions = {c["assertion"] for c in final["treatment"]["success_criteria"]}
    assert "duration_within_tolerance" in assertions
    assert "aspect_ratio_matches" in assertions
    assert "captions_delivered" in assertions


# ---- llm client plumbing ----


def test_build_client_returns_null_without_a_model():
    client = build_client({"mode": "llm", "provider": "ollama"})
    assert not client.available()
    assert "no model configured" in client.reason


def test_build_client_returns_null_for_deterministic_mode():
    client = build_client({"mode": "deterministic", "model": "llama3"})
    assert not client.available()


def test_build_client_supports_swapping_provider_without_changing_contract():
    ollama = build_client({"mode": "llm", "provider": "ollama", "model": "llama3"})
    openai = build_client(
        {"mode": "llm", "provider": "openai-compatible", "model": "gpt-x", "base_url": "https://x.test/v1"}
    )
    assert ollama.available() and openai.available()
    assert ollama.provider != openai.provider
    # Both satisfy the same interface the nodes depend on.
    assert hasattr(ollama, "complete") and hasattr(openai, "complete")


def test_null_client_never_fabricates_output():
    with pytest.raises(LLMError):
        NullClient("nothing configured").complete("s", "u")


def test_extract_json_strips_code_fences():
    assert extract_json('```json\n{"a": 1}\n```') == {"a": 1}


def test_extract_json_recovers_from_surrounding_prose():
    assert extract_json('Sure! Here you go:\n{"a": 2}\nHope that helps.') == {"a": 2}


def test_extract_json_raises_on_garbage():
    with pytest.raises(LLMError):
        extract_json("no json here")


def test_extract_json_rejects_non_objects():
    with pytest.raises(LLMError):
        extract_json("[1, 2, 3]")


# ---- no silent network ----


def test_deterministic_mode_builds_no_network_client(monkeypatch):
    """The deterministic path must not construct a client that could call out."""
    client = build_client({"mode": "deterministic", "provider": "ollama", "model": "llama3"})
    assert isinstance(client, NullClient)

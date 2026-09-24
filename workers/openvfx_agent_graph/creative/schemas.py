"""State and contract constants for the OpenVFX Creative Director graph.

The treatment schema is shared with the Go runtime
(`internal/production/treatment.go`). Field names must match exactly: Go
validates whatever this graph emits and discards anything malformed.
"""

from __future__ import annotations

from typing import Any, Optional, TypedDict

TREATMENT_SCHEMA = "openvfx_creative_treatment.v1"
DIRECTOR_TRACE_SCHEMA = "openvfx_director_trace.v1"

# Reasoning modes. These mirror the Go constants and carry the honesty contract:
# only "llm" may set semantic_reasoning.
MODE_LLM = "llm"
MODE_DETERMINISTIC = "deterministic"

VALID_ROLES = frozenset(
    {
        "primary_narration",
        "hero_shot",
        "hook",
        "b_roll",
        "atmospheric_insert",
        "reaction",
        "transition_material",
        "outro",
        "audio_bed",
        "reference_asset",
        "generated_asset_candidate",
        "unknown",
    }
)

VALID_CONFIDENCE = frozenset({"high", "medium", "low", "uncertain"})

# Gap kinds reuse the existing asset-requirement vocabulary rather than
# inventing a second generation-request system.
GAP_KINDS = frozenset(
    {
        "generated_broll",
        "generated_image",
        "establishing_shot",
        "narration_audio",
        "music_bed",
        "visual_transform",
        "transition_material",
    }
)


class CreativeState(TypedDict, total=False):
    """Mutable state threaded through the creative director graph."""

    # Input
    production_id: str
    production_dir: str
    brief: str
    director_config: dict[str, Any]

    # Loaded artifacts
    observations: Optional[dict[str, Any]]
    capabilities: Optional[dict[str, Any]]

    # Node outputs
    observe_status: str
    interpretation: dict[str, Any]
    asset_roles: list[dict[str, Any]]
    story: dict[str, Any]
    gaps: dict[str, Any]
    treatment: dict[str, Any]
    critique: dict[str, Any]

    # Reasoning provenance
    requested_mode: str
    effective_mode: str
    fallback_used: bool
    fallback_reason: str
    llm_calls: int

    # Graph metadata
    trace_events: list[dict[str, Any]]
    warnings: list[str]
    error: Optional[str]

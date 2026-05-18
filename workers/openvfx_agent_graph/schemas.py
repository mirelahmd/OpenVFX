"""State and artifact schemas for the OpenVFX agent graph."""

from __future__ import annotations

from typing import Any, Optional, TypedDict


# ---- LangGraph state ----

class AgentGraphState(TypedDict):
    """Mutable state threaded through the LangGraph graph."""

    # Input
    plan_id: str
    plan_dir: str
    run_id: str

    # Loaded artifacts (None if not found on disk)
    agent_plan: Optional[dict[str, Any]]
    context_snapshot: Optional[dict[str, Any]]
    policy_review: Optional[dict[str, Any]]
    creative_brief: Optional[dict[str, Any]]
    deliverables: Optional[dict[str, Any]]
    asset_requirements: Optional[dict[str, Any]]

    # Node outputs
    observe_status: str          # "ok" | "partial" | "missing_plan" | "error"
    plan_issues: list[str]       # structural issues in the plan
    policy_status: str           # "allowed" | "warning" | "blocked" | "unknown"
    policy_blocks: list[str]     # specific block reasons

    # Decision
    decision: str                # "approve" | "flag" | "reject" | "repair"
    decision_reason: str
    repair_suggestions: list[str]

    # Graph metadata
    trace_events: list[dict[str, Any]]
    warnings: list[str]
    error: Optional[str]


# ---- Artifact schemas (written to disk) ----

GRAPH_TRACE_SCHEMA = "openvfx_graph_trace.v1"
AGENT_DECISION_SCHEMA = "openvfx_agent_decision.v1"

VALID_ACTION_TYPES = frozenset(
    {"make", "revise_make", "validate_creative_assemble", "queue_health"}
)
VALID_DECISIONS = frozenset({"approve", "flag", "reject", "repair"})
VALID_POLICY_STATUSES = frozenset({"allowed", "warning", "blocked", "unknown"})

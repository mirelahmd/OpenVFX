"""LangGraph node functions for the OpenVFX agent graph.

Each node receives the full AgentGraphState and returns a partial dict
of fields to update (LangGraph merges it into the state).
"""

from __future__ import annotations

import re
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .io import read_json, read_plan_artifacts
from .policy import derive_repair_suggestions, evaluate_policy
from .schemas import VALID_ACTION_TYPES, AgentGraphState

_ACTION_ID_RE = re.compile(r"^action_\d+$")


def _iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def _trace_event(
    node: str,
    started_ms: float,
    status: str,
    detail: str = "",
) -> dict[str, Any]:
    ended_ms = time.monotonic() * 1000
    return {
        "node": node,
        "started_at": datetime.fromtimestamp(
            started_ms / 1000, tz=timezone.utc
        ).isoformat(),
        "completed_at": _iso(),
        "status": status,
        "duration_ms": round(ended_ms - started_ms, 1),
        "detail": detail,
    }


# ---- Node: observe ----


def observe_node(state: AgentGraphState) -> dict[str, Any]:
    """Read agent plan, context snapshot, and policy review from disk."""
    t0 = time.monotonic() * 1000
    plan_dir = state["plan_dir"]

    agent_plan, context_snapshot, policy_review = read_plan_artifacts(plan_dir)
    plan_path = Path(plan_dir)
    creative_brief = read_json(plan_path / "creative_brief.json")
    deliverables = read_json(plan_path / "deliverables.json")
    asset_requirements = read_json(plan_path / "asset_requirements.json")

    if agent_plan is None:
        status = "missing_plan"
        detail = f"agent_plan.json not found in {plan_dir}"
    elif context_snapshot is None and policy_review is None:
        status = "partial"
        detail = "agent_plan.json found; context_snapshot and policy_review missing"
    else:
        status = "ok"
        detail = "all artifacts loaded"

    event = _trace_event("observe", t0, "ok" if status != "error" else "error", detail)
    existing_events = state.get("trace_events", [])

    return {
        "agent_plan": agent_plan,
        "context_snapshot": context_snapshot,
        "policy_review": policy_review,
        "creative_brief": creative_brief,
        "deliverables": deliverables,
        "asset_requirements": asset_requirements,
        "observe_status": status,
        "trace_events": existing_events + [event],
    }


# ---- Node: plan_review ----


def plan_review_node(state: AgentGraphState) -> dict[str, Any]:
    """Validate the loaded agent plan schema."""
    t0 = time.monotonic() * 1000
    plan = state.get("agent_plan")
    issues: list[str] = []
    warnings: list[str] = []

    if plan is None:
        event = _trace_event("plan_review", t0, "skipped", "no plan to review")
        return {
            "plan_issues": ["no agent plan loaded"],
            "trace_events": state.get("trace_events", []) + [event],
        }

    actions = plan.get("actions", [])
    if not actions:
        issues.append("plan has no actions")

    seen_ids: set[str] = set()
    for i, action in enumerate(actions):
        aid = action.get("id", "")
        if not _ACTION_ID_RE.match(aid):
            issues.append(f"action[{i}].id {aid!r} does not match action_NNNN format")
        if aid in seen_ids:
            issues.append(f"action[{i}].id {aid!r} is duplicate")
        seen_ids.add(aid)
        atype = action.get("type", "")
        if atype not in VALID_ACTION_TYPES:
            issues.append(f"action[{i}].type {atype!r} is unknown")
        if not str(action.get("description", "")).strip():
            issues.append(f"action[{i}].id {aid}: description is empty")

    # Warn on large plans.
    if len(actions) > 5:
        warnings.append(f"plan has {len(actions)} actions; consider splitting")

    planner = plan.get("planner", {})
    if planner.get("fallback_used"):
        warnings.append(
            f"plan used fallback planner: {planner.get('fallback_reason', 'unknown reason')}"
        )

    creative_brief = state.get("creative_brief") or {}
    if creative_brief:
        raw_prompt = str(creative_brief.get("raw_prompt", ""))
        summary = str(creative_brief.get("summary", ""))
        if len(raw_prompt) > 80 and not summary:
            warnings.append("rich creative brief has no summary")

    assets = state.get("asset_requirements") or {}
    missing_assets: list[str] = []
    if isinstance(assets, dict):
        for req in assets.get("requirements", []):
            status = req.get("status")
            capability = req.get("capability", "unknown")
            if status in ("missing", "missing_env"):
                missing_assets.append(f"{capability} is {status}")
    if missing_assets:
        warnings.append("missing creative capabilities: " + "; ".join(missing_assets))

    status = "error" if issues else "ok"
    detail = "; ".join(issues[:3]) if issues else f"{len(actions)} action(s) validated"
    event = _trace_event("plan_review", t0, status, detail)

    existing_warnings = state.get("warnings", [])
    return {
        "plan_issues": issues,
        "warnings": existing_warnings + warnings,
        "trace_events": state.get("trace_events", []) + [event],
    }


# ---- Node: policy_check ----


def policy_check_node(state: AgentGraphState) -> dict[str, Any]:
    """Evaluate the policy status for the plan."""
    t0 = time.monotonic() * 1000

    policy_status, policy_blocks = evaluate_policy(
        state.get("agent_plan"),
        state.get("policy_review"),
    )

    detail = f"status={policy_status}"
    if policy_blocks:
        detail += f"; {len(policy_blocks)} block(s)"
    event = _trace_event("policy_check", t0, "ok", detail)

    return {
        "policy_status": policy_status,
        "policy_blocks": policy_blocks,
        "trace_events": state.get("trace_events", []) + [event],
    }


# ---- Node: decide ----


def decide_node(state: AgentGraphState) -> dict[str, Any]:
    """Produce a decision based on policy status and plan review."""
    t0 = time.monotonic() * 1000

    observe_status = state.get("observe_status", "error")
    policy_status = state.get("policy_status", "unknown")
    plan_issues = state.get("plan_issues", [])
    policy_blocks = state.get("policy_blocks", [])

    decision: str
    reason: str

    if observe_status == "missing_plan":
        decision = "reject"
        reason = "No agent plan found on disk. Run agent-plan first."
    elif plan_issues:
        decision = "reject"
        reason = f"Plan has {len(plan_issues)} structural issue(s): {'; '.join(plan_issues[:2])}"
    elif policy_status == "blocked":
        if policy_blocks:
            decision = "repair"
            reason = f"Plan is blocked by policy ({len(policy_blocks)} block(s)); repair suggestions generated"
        else:
            decision = "reject"
            reason = "Plan is blocked by policy"
    elif policy_status == "warning":
        decision = "flag"
        reason = "Plan has policy warnings; human review recommended before approval"
    elif policy_status in ("allowed", "unknown"):
        decision = "approve"
        reason = (
            "Plan is valid and policy allows execution."
            if policy_status == "allowed"
            else "Plan is valid; policy status unknown (no policy_review.json); proceeding with approval recommendation"
        )
    else:
        decision = "flag"
        reason = f"Unexpected policy_status {policy_status!r}"

    event = _trace_event("decide", t0, "ok", f"decision={decision}")
    return {
        "decision": decision,
        "decision_reason": reason,
        "trace_events": state.get("trace_events", []) + [event],
    }


# ---- Node: repair ----


def repair_node(state: AgentGraphState) -> dict[str, Any]:
    """Generate concrete repair suggestions when a plan is blocked."""
    t0 = time.monotonic() * 1000

    suggestions = derive_repair_suggestions(
        policy_blocks=state.get("policy_blocks", []),
        plan_issues=state.get("plan_issues", []),
        agent_plan=state.get("agent_plan"),
        plan_id=state.get("plan_id", ""),
    )

    event = _trace_event(
        "repair",
        t0,
        "ok",
        f"{len(suggestions)} suggestion(s)",
    )
    return {
        "repair_suggestions": suggestions,
        "trace_events": state.get("trace_events", []) + [event],
    }

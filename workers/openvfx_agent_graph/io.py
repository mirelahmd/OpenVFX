"""Artifact I/O helpers for the OpenVFX agent graph."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional


def _now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def read_json(path: Path) -> Optional[dict[str, Any]]:
    """Read a JSON file; returns None if the file does not exist."""
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return None


def write_json(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2, default=str) + "\n", encoding="utf-8")


def read_plan_artifacts(
    plan_dir: str,
) -> tuple[
    Optional[dict[str, Any]],
    Optional[dict[str, Any]],
    Optional[dict[str, Any]],
]:
    """Return (agent_plan, context_snapshot, policy_review) from plan_dir."""
    d = Path(plan_dir)
    return (
        read_json(d / "agent_plan.json"),
        read_json(d / "context_snapshot.json"),
        read_json(d / "policy_review.json"),
    )


def write_graph_trace(
    plan_dir: str,
    run_id: str,
    plan_id: str,
    trace_events: list[dict[str, Any]],
    started_at: str,
) -> Path:
    now = _now_iso()
    total_ms = 0
    for ev in trace_events:
        total_ms += ev.get("duration_ms", 0)

    edges: list[str] = []
    nodes = [ev["node"] for ev in trace_events]
    if nodes:
        prev = "__start__"
        for n in nodes:
            edges.append(f"{prev} -> {n}")
            prev = n
        edges.append(f"{prev} -> __end__")

    data: dict[str, Any] = {
        "schema_version": "openvfx_graph_trace.v1",
        "run_id": run_id,
        "plan_id": plan_id,
        "created_at": now,
        "started_at": started_at,
        "graph_version": "1",
        "nodes_executed": trace_events,
        "edges_traversed": edges,
        "total_duration_ms": total_ms,
    }
    out = Path(plan_dir) / "graph_trace.json"
    write_json(out, data)
    return out


def write_agent_decision(
    plan_dir: str,
    run_id: str,
    plan_id: str,
    decision: str,
    decision_reason: str,
    policy_status: str,
    plan_issues: list[str],
    repair_suggestions: list[str],
    warnings: list[str],
) -> Path:
    next_cmds: list[str] = []
    if decision == "approve":
        next_cmds = [
            f"byom-video approve-agent-plan {plan_id}",
            f"byom-video agent-plan-to-job {plan_id} --dry-run",
        ]
    elif decision == "flag":
        next_cmds = [
            f"byom-video review-agent-plan {plan_id}",
            f"byom-video agent-policy {plan_id}",
        ]
    elif decision == "repair":
        next_cmds = [
            f"byom-video inspect-agent-plan {plan_id}",
            f"byom-video agent-plan --goal \"<revised goal>\"",
        ]
    elif decision == "reject":
        next_cmds = [
            f"byom-video reject-agent-plan {plan_id} --reason \"blocked by policy\"",
        ]

    data: dict[str, Any] = {
        "schema_version": "openvfx_agent_decision.v1",
        "run_id": run_id,
        "plan_id": plan_id,
        "created_at": _now_iso(),
        "decision": decision,
        "decision_reason": decision_reason,
        "policy_status": policy_status,
        "plan_issues": plan_issues,
        "repair_suggestions": repair_suggestions,
        "warnings": warnings,
        "next_commands": next_cmds,
    }
    out = Path(plan_dir) / "agent_decision.json"
    write_json(out, data)
    return out

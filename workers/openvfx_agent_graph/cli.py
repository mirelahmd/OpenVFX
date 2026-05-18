"""CLI entry point for the OpenVFX agent graph sidecar."""

from __future__ import annotations

import argparse
import json
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


def _iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def cmd_run(args: argparse.Namespace) -> int:
    from .graph import build_agent_graph
    from .io import write_agent_decision, write_graph_trace
    from .schemas import AgentGraphState

    plan_dir = args.plan_dir
    plan_id = args.plan_id
    run_id = args.run_id

    if not Path(plan_dir).exists():
        print(f"error: plan_dir does not exist: {plan_dir}", file=sys.stderr)
        return 1

    started_at = _iso()
    initial_state: AgentGraphState = {
        "plan_id": plan_id,
        "plan_dir": plan_dir,
        "run_id": run_id,
        "agent_plan": None,
        "context_snapshot": None,
        "policy_review": None,
        "observe_status": "",
        "plan_issues": [],
        "policy_status": "unknown",
        "policy_blocks": [],
        "decision": "",
        "decision_reason": "",
        "repair_suggestions": [],
        "trace_events": [],
        "warnings": [],
        "error": None,
    }

    graph = build_agent_graph()
    try:
        final_state: AgentGraphState = graph.invoke(initial_state)
    except Exception as exc:
        print(f"error: graph execution failed: {exc}", file=sys.stderr)
        return 1

    # Write artifacts.
    trace_path = write_graph_trace(
        plan_dir=plan_dir,
        run_id=run_id,
        plan_id=plan_id,
        trace_events=final_state.get("trace_events", []),
        started_at=started_at,
    )
    decision_path = write_agent_decision(
        plan_dir=plan_dir,
        run_id=run_id,
        plan_id=plan_id,
        decision=final_state.get("decision", "unknown"),
        decision_reason=final_state.get("decision_reason", ""),
        policy_status=final_state.get("policy_status", "unknown"),
        plan_issues=final_state.get("plan_issues", []),
        repair_suggestions=final_state.get("repair_suggestions", []),
        warnings=final_state.get("warnings", []),
    )

    result = {
        "run_id": run_id,
        "plan_id": plan_id,
        "decision": final_state.get("decision"),
        "decision_reason": final_state.get("decision_reason"),
        "policy_status": final_state.get("policy_status"),
        "plan_issues": final_state.get("plan_issues"),
        "repair_suggestions": final_state.get("repair_suggestions"),
        "warnings": final_state.get("warnings"),
        "graph_trace": str(trace_path),
        "agent_decision": str(decision_path),
    }
    print(json.dumps(result))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="openvfx-agent-graph")
    subparsers = parser.add_subparsers(dest="command", required=True)

    run_parser = subparsers.add_parser(
        "run", help="Run the agent graph against an existing agent plan"
    )
    run_parser.add_argument("--plan-id", required=True, help="Agent plan ID")
    run_parser.add_argument("--plan-dir", required=True, help="Path to plan directory")
    run_parser.add_argument("--run-id", required=True, help="Graph run ID")

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        if args.command == "run":
            return cmd_run(args)
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    parser.error(f"unknown command: {args.command}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

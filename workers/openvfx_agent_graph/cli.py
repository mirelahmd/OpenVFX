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


def cmd_creative_treatment(args: argparse.Namespace) -> int:
    """Run the Creative Director graph and write creative_treatment.json.

    The Go runtime resolves configuration and passes it in as JSON, so this
    sidecar never parses byom-video.yaml and never picks a provider on its own.
    """
    from .creative.graph import build_creative_graph, initial_state
    from .creative.schemas import DIRECTOR_TRACE_SCHEMA
    from .io import write_json

    production_dir = Path(args.production_dir)
    if not production_dir.exists():
        print(f"error: production_dir does not exist: {production_dir}", file=sys.stderr)
        return 1

    director_config: dict = {}
    if args.config:
        config_path = Path(args.config)
        if not config_path.exists():
            print(f"error: director config not found: {config_path}", file=sys.stderr)
            return 1
        try:
            director_config = json.loads(config_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            print(f"error: director config is not valid JSON: {exc}", file=sys.stderr)
            return 1

    brief = args.brief
    if args.brief_file:
        try:
            brief = Path(args.brief_file).read_text(encoding="utf-8")
        except OSError as exc:
            print(f"error: could not read brief file: {exc}", file=sys.stderr)
            return 1
    if not (brief or "").strip():
        print("error: a brief is required", file=sys.stderr)
        return 1

    started_at = _iso()
    state = initial_state(
        production_id=args.production_id,
        production_dir=str(production_dir),
        brief=brief,
        director_config=director_config,
    )

    graph = build_creative_graph()
    try:
        final_state = graph.invoke(state)
    except Exception as exc:
        print(f"error: creative graph execution failed: {exc}", file=sys.stderr)
        return 1

    treatment = final_state.get("treatment") or {}
    if not treatment:
        print("error: creative graph produced no treatment", file=sys.stderr)
        return 1

    treatment["treatment_id"] = args.production_id + "-treatment"
    treatment["production_id"] = args.production_id
    treatment["created_at"] = started_at

    treatment_path = production_dir / "creative_treatment.json"
    write_json(treatment_path, treatment)

    trace_path = production_dir / "director" / "graph_trace.json"
    write_json(
        trace_path,
        {
            "schema_version": DIRECTOR_TRACE_SCHEMA,
            "production_id": args.production_id,
            "started_at": started_at,
            "completed_at": _iso(),
            "graph": "creative_director.v1",
            "requested_mode": final_state.get("requested_mode"),
            "effective_mode": final_state.get("effective_mode"),
            "llm_calls": final_state.get("llm_calls", 0),
            "nodes": final_state.get("trace_events", []),
            "warnings": final_state.get("warnings", []),
        },
    )

    print(
        json.dumps(
            {
                "production_id": args.production_id,
                "treatment": str(treatment_path),
                "graph_trace": str(trace_path),
                "requested_mode": final_state.get("requested_mode"),
                "effective_mode": final_state.get("effective_mode"),
                "llm_calls": final_state.get("llm_calls", 0),
                "fallback_used": bool(final_state.get("fallback_used")),
                "fallback_reason": final_state.get("fallback_reason", ""),
                "segments": len(treatment.get("segments", [])),
                "gaps": len(treatment.get("generated_asset_needs", [])),
                "warnings": final_state.get("warnings", []),
            }
        )
    )
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

    treatment_parser = subparsers.add_parser(
        "creative-treatment",
        help="Run the Creative Director graph and write creative_treatment.json",
    )
    treatment_parser.add_argument("--production-id", required=True)
    treatment_parser.add_argument("--production-dir", required=True)
    treatment_parser.add_argument("--brief", default="")
    treatment_parser.add_argument(
        "--brief-file", default="", help="Read the brief from a file instead of argv"
    )
    treatment_parser.add_argument(
        "--config", default="", help="Path to JSON director config resolved by the Go runtime"
    )

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        if args.command == "run":
            return cmd_run(args)
        if args.command == "creative-treatment":
            return cmd_creative_treatment(args)
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    parser.error(f"unknown command: {args.command}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

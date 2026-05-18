"""End-to-end tests for the agent graph."""

import json
import tempfile
from pathlib import Path

from openvfx_agent_graph.graph import build_agent_graph
from openvfx_agent_graph.schemas import AgentGraphState


def _base_state(plan_dir: str, plan_id: str = "plan-1") -> AgentGraphState:
    return {
        "plan_id": plan_id,
        "plan_dir": plan_dir,
        "run_id": "run-test",
        "agent_plan": None,
        "context_snapshot": None,
        "policy_review": None,
        "creative_brief": None,
        "deliverables": None,
        "asset_requirements": None,
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


def _write(d: Path, name: str, data: dict) -> None:
    (d / name).write_text(json.dumps(data))


def test_graph_approve_allowed_plan():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {
                "id": "action_0001",
                "type": "queue_health",
                "description": "Check queue status",
                "requires_provider": False,
                "requires_overwrite": False,
            }
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {"status": "allowed", "blocks": []})
    _write(d, "context_snapshot.json", {"schema_version": "v1"})

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert state["observe_status"] == "ok"
    assert state["plan_issues"] == []
    assert state["policy_status"] == "allowed"
    assert state["decision"] == "approve"
    assert "valid" in state["decision_reason"].lower() or "policy" in state["decision_reason"].lower()
    assert state["repair_suggestions"] == []
    assert len(state["trace_events"]) >= 4


def test_graph_flag_on_warning():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "make", "description": "Make a video",
             "requires_provider": False, "requires_overwrite": False}
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {"status": "warning", "blocks": []})

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert state["decision"] == "flag"
    assert "warning" in state["decision_reason"].lower()


def test_graph_repair_on_blocked():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "make", "description": "Make a video",
             "requires_provider": True, "requires_overwrite": False}
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {
        "status": "blocked",
        "blocks": ["action_0001 requires provider calls"]
    })

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert state["decision"] == "repair"
    assert len(state["repair_suggestions"]) >= 1
    # repair node should have been in the trace
    node_names = [ev["node"] for ev in state["trace_events"]]
    assert "repair" in node_names


def test_graph_reject_missing_plan():
    d = Path(tempfile.mkdtemp())
    # No agent_plan.json

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert state["observe_status"] == "missing_plan"
    assert state["decision"] == "reject"
    assert "repair" not in [ev["node"] for ev in state["trace_events"]]


def test_graph_reject_invalid_action_type():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "launch_rockets", "description": "go boom",
             "requires_provider": False}
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {"status": "allowed", "blocks": []})

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert len(state["plan_issues"]) > 0
    assert state["decision"] == "reject"


def test_graph_trace_contains_all_nodes():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "queue_health", "description": "check",
             "requires_provider": False}
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {"status": "allowed", "blocks": []})

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    node_names = {ev["node"] for ev in state["trace_events"]}
    assert "observe" in node_names
    assert "plan_review" in node_names
    assert "policy_check" in node_names
    assert "decide" in node_names


def test_graph_warnings_from_fallback_planner():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "queue_health", "description": "check",
             "requires_provider": False}
        ],
        "planner": {
            "fallback_used": True,
            "fallback_reason": "ollama failed: connection refused",
        },
    })
    _write(d, "policy_review.json", {"status": "allowed", "blocks": []})

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert any("fallback" in w for w in state["warnings"])
    assert state["decision"] == "approve"


def test_graph_warns_on_missing_asset_requirements():
    d = Path(tempfile.mkdtemp())
    _write(d, "agent_plan.json", {
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "make", "description": "make",
             "requires_provider": False}
        ],
        "planner": {"fallback_used": False},
    })
    _write(d, "policy_review.json", {"status": "warning", "blocks": []})
    _write(d, "creative_brief.json", {"raw_prompt": "make generated b-roll", "summary": "brief"})
    _write(d, "asset_requirements.json", {
        "requirements": [
            {"capability": "video_generation_or_image_generation", "status": "missing"}
        ]
    })

    graph = build_agent_graph()
    state = graph.invoke(_base_state(str(d)))

    assert state["decision"] == "flag"
    assert any("missing creative capabilities" in w for w in state["warnings"])


def test_cli_run(tmp_path):
    """Test the CLI entry point end-to-end."""
    plan_dir = tmp_path / "plans" / "plan-1"
    plan_dir.mkdir(parents=True)
    (plan_dir / "agent_plan.json").write_text(json.dumps({
        "plan_id": "plan-1",
        "actions": [
            {"id": "action_0001", "type": "queue_health", "description": "check",
             "requires_provider": False}
        ],
        "planner": {"fallback_used": False},
    }))
    (plan_dir / "policy_review.json").write_text(
        json.dumps({"status": "allowed", "blocks": []})
    )

    from openvfx_agent_graph.cli import main
    import io
    from unittest.mock import patch

    out = io.StringIO()
    with patch("sys.stdout", out):
        rc = main([
            "run",
            "--plan-id", "plan-1",
            "--plan-dir", str(plan_dir),
            "--run-id", "graphrun-test",
        ])

    assert rc == 0
    result = json.loads(out.getvalue())
    assert result["decision"] == "approve"
    assert result["plan_id"] == "plan-1"
    assert (plan_dir / "graph_trace.json").exists()
    assert (plan_dir / "agent_decision.json").exists()

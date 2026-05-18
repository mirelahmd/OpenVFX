"""Tests for I/O helpers."""

import json
import tempfile
from pathlib import Path

from openvfx_agent_graph.io import (
    read_json,
    read_plan_artifacts,
    write_agent_decision,
    write_graph_trace,
)


def make_plan_dir(plan: dict | None = None, policy: dict | None = None) -> Path:
    d = Path(tempfile.mkdtemp())
    if plan is not None:
        (d / "agent_plan.json").write_text(json.dumps(plan))
    if policy is not None:
        (d / "policy_review.json").write_text(json.dumps(policy))
    return d


def test_read_json_missing():
    d = Path(tempfile.mkdtemp())
    assert read_json(d / "nonexistent.json") is None


def test_read_json_present():
    d = Path(tempfile.mkdtemp())
    (d / "data.json").write_text('{"x": 1}')
    assert read_json(d / "data.json") == {"x": 1}


def test_read_plan_artifacts_all_present():
    d = make_plan_dir(plan={"plan_id": "p1"}, policy={"status": "allowed"})
    (d / "context_snapshot.json").write_text('{"schema_version": "v1"}')
    plan, ctx, policy = read_plan_artifacts(str(d))
    assert plan == {"plan_id": "p1"}
    assert ctx == {"schema_version": "v1"}
    assert policy == {"status": "allowed"}


def test_read_plan_artifacts_missing_plan():
    d = Path(tempfile.mkdtemp())
    plan, ctx, policy = read_plan_artifacts(str(d))
    assert plan is None
    assert ctx is None
    assert policy is None


def test_write_graph_trace():
    d = Path(tempfile.mkdtemp())
    events = [
        {"node": "observe", "duration_ms": 10, "status": "ok"},
        {"node": "decide", "duration_ms": 5, "status": "ok"},
    ]
    out = write_graph_trace(str(d), "run-1", "plan-1", events, "2026-01-01T00:00:00Z")
    assert out.exists()
    data = json.loads(out.read_text())
    assert data["schema_version"] == "openvfx_graph_trace.v1"
    assert data["run_id"] == "run-1"
    assert data["plan_id"] == "plan-1"
    assert len(data["nodes_executed"]) == 2
    assert data["total_duration_ms"] == 15
    assert "__start__ -> observe" in data["edges_traversed"]


def test_write_agent_decision_approve():
    d = Path(tempfile.mkdtemp())
    out = write_agent_decision(
        str(d), "run-1", "plan-1",
        decision="approve",
        decision_reason="looks good",
        policy_status="allowed",
        plan_issues=[],
        repair_suggestions=[],
        warnings=[],
    )
    assert out.exists()
    data = json.loads(out.read_text())
    assert data["schema_version"] == "openvfx_agent_decision.v1"
    assert data["decision"] == "approve"
    assert any("approve-agent-plan" in c for c in data["next_commands"])


def test_write_agent_decision_repair():
    d = Path(tempfile.mkdtemp())
    out = write_agent_decision(
        str(d), "run-1", "plan-1",
        decision="repair",
        decision_reason="blocked",
        policy_status="blocked",
        plan_issues=[],
        repair_suggestions=["add --allow-provider-calls"],
        warnings=[],
    )
    data = json.loads(out.read_text())
    assert data["decision"] == "repair"
    assert data["repair_suggestions"] == ["add --allow-provider-calls"]

"""Tests for schemas module."""

from openvfx_agent_graph.schemas import (
    AGENT_DECISION_SCHEMA,
    GRAPH_TRACE_SCHEMA,
    VALID_ACTION_TYPES,
    VALID_DECISIONS,
    VALID_POLICY_STATUSES,
)


def test_action_types_complete():
    assert "make" in VALID_ACTION_TYPES
    assert "revise_make" in VALID_ACTION_TYPES
    assert "validate_creative_assemble" in VALID_ACTION_TYPES
    assert "queue_health" in VALID_ACTION_TYPES
    assert "launch_rockets" not in VALID_ACTION_TYPES


def test_decisions_complete():
    assert VALID_DECISIONS == {"approve", "flag", "reject", "repair"}


def test_policy_statuses_complete():
    assert "allowed" in VALID_POLICY_STATUSES
    assert "blocked" in VALID_POLICY_STATUSES
    assert "warning" in VALID_POLICY_STATUSES
    assert "unknown" in VALID_POLICY_STATUSES


def test_schema_version_strings():
    assert GRAPH_TRACE_SCHEMA == "openvfx_graph_trace.v1"
    assert AGENT_DECISION_SCHEMA == "openvfx_agent_decision.v1"

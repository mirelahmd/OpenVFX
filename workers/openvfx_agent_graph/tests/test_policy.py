"""Tests for policy evaluation."""

from openvfx_agent_graph.policy import derive_repair_suggestions, evaluate_policy


def test_evaluate_policy_from_review_allowed():
    policy_review = {"status": "allowed", "blocks": []}
    status, blocks = evaluate_policy(None, policy_review)
    assert status == "allowed"
    assert blocks == []


def test_evaluate_policy_from_review_blocked():
    policy_review = {"status": "blocked", "blocks": ["requires provider calls"]}
    status, blocks = evaluate_policy(None, policy_review)
    assert status == "blocked"
    assert len(blocks) == 1


def test_evaluate_policy_from_review_warning():
    policy_review = {"status": "warning", "blocks": []}
    status, blocks = evaluate_policy(None, policy_review)
    assert status == "warning"


def test_evaluate_policy_no_review_no_plan():
    status, blocks = evaluate_policy(None, None)
    assert status == "unknown"
    assert len(blocks) > 0


def test_evaluate_policy_no_review_clean_plan():
    plan = {"actions": [
        {"id": "action_0001", "type": "queue_health", "requires_provider": False, "requires_overwrite": False}
    ]}
    status, blocks = evaluate_policy(plan, None)
    assert status == "unknown"
    assert blocks == []


def test_evaluate_policy_no_review_provider_blocked():
    plan = {"actions": [
        {"id": "action_0001", "type": "make", "requires_provider": True}
    ]}
    status, blocks = evaluate_policy(plan, None)
    assert status == "blocked"
    assert any("provider" in b for b in blocks)


def test_derive_repair_suggestions_provider():
    suggestions = derive_repair_suggestions(
        policy_blocks=["action_0001 requires provider calls (type: make)"],
        plan_issues=[],
        agent_plan=None,
        plan_id="plan-1",
    )
    assert any("allow-provider-calls" in s for s in suggestions)


def test_derive_repair_suggestions_overwrite():
    suggestions = derive_repair_suggestions(
        policy_blocks=["action_0001 requires overwrite permission"],
        plan_issues=[],
        agent_plan=None,
        plan_id="plan-1",
    )
    assert any("allow-overwrite" in s for s in suggestions)


def test_derive_repair_suggestions_fallback():
    suggestions = derive_repair_suggestions(
        policy_blocks=[],
        plan_issues=[],
        agent_plan=None,
        plan_id="plan-abc",
    )
    assert len(suggestions) >= 1
    assert any("inspect-agent-plan" in s for s in suggestions)


def test_derive_repair_deduplicate():
    suggestions = derive_repair_suggestions(
        policy_blocks=[
            "requires provider calls",
            "requires provider calls again",
        ],
        plan_issues=[],
        agent_plan=None,
        plan_id="plan-1",
    )
    # Both trigger the same suggestion — should be deduplicated
    provider_suggestions = [s for s in suggestions if "allow-provider-calls" in s]
    assert len(provider_suggestions) == 1

"""Policy evaluation helpers — mirrors the Go policy review contract."""

from __future__ import annotations

from typing import Any, Optional


def evaluate_policy(
    agent_plan: Optional[dict[str, Any]],
    policy_review: Optional[dict[str, Any]],
) -> tuple[str, list[str]]:
    """Return (policy_status, blocks) by reading the existing policy_review artifact.

    Falls back to basic structural evaluation if policy_review is absent.
    """
    if policy_review is not None:
        status = policy_review.get("status", "unknown")
        blocks = policy_review.get("blocks", [])
        blocks.extend(policy_review.get("errors", []))
        for check in policy_review.get("checks", []):
            if check.get("status") == "blocked":
                blocks.append(check.get("message", "blocked policy check"))
        # Normalise Go-side status names to our set.
        if status == "allowed":
            return "allowed", blocks
        if status == "approval_required":
            return "warning", blocks
        if status in ("warning", "warnings"):
            return "warning", blocks
        if status in ("blocked", "block"):
            return "blocked", blocks
        return "unknown", blocks

    # No policy_review artifact — fall back to reading action flags.
    if agent_plan is None:
        return "unknown", ["no agent plan loaded"]

    blocks: list[str] = []
    for action in agent_plan.get("actions", []):
        if action.get("requires_provider") and not agent_plan.get(
            "allow_provider_calls", False
        ):
            blocks.append(
                f"action {action.get('id')} requires provider calls "
                f"(type: {action.get('type')})"
            )
        if action.get("requires_overwrite") and not agent_plan.get(
            "allow_overwrite", False
        ):
            blocks.append(
                f"action {action.get('id')} requires overwrite permission "
                f"(type: {action.get('type')})"
            )

    if blocks:
        return "blocked", blocks
    return "unknown", []


def derive_repair_suggestions(
    policy_blocks: list[str],
    plan_issues: list[str],
    agent_plan: Optional[dict[str, Any]],
    plan_id: str,
) -> list[str]:
    """Generate concrete repair suggestions based on blocks and plan issues."""
    suggestions: list[str] = []

    for block in policy_blocks:
        b = block.lower()
        if "provider" in b:
            suggestions.append(
                f"Re-run agent-plan with --allow-provider-calls: "
                f"byom-video agent-plan --goal \"<goal>\" --allow-provider-calls"
            )
        if "overwrite" in b:
            suggestions.append(
                f"Re-run agent-plan with --allow-overwrite: "
                f"byom-video agent-plan --goal \"<goal>\" --allow-overwrite"
            )
        if "input media" in b or "missing" in b:
            suggestions.append(
                "Provide an input media file with --input: "
                "byom-video agent-plan --goal \"<goal>\" --input <video_path>"
            )

    for issue in plan_issues:
        i = issue.lower()
        if "action" in i and "type" in i:
            suggestions.append(
                f"Review action types — only make, revise_make, "
                f"validate_creative_assemble, queue_health are allowed"
            )
        if "description" in i and "empty" in i:
            suggestions.append(
                "Plan actions require non-empty descriptions — try a different --planner or --goal"
            )

    if not suggestions:
        suggestions.append(
            f"Inspect the plan and policy: byom-video inspect-agent-plan {plan_id}"
        )

    return list(dict.fromkeys(suggestions))  # deduplicate, preserve order

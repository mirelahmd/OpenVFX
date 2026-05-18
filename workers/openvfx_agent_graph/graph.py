"""Build and compile the OpenVFX agent StateGraph."""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from .nodes import (
    decide_node,
    observe_node,
    plan_review_node,
    policy_check_node,
    repair_node,
)
from .schemas import AgentGraphState


def _should_repair(state: AgentGraphState) -> str:
    """Route to repair if decision says repair, otherwise end."""
    return "repair" if state.get("decision") == "repair" else END


def build_agent_graph() -> object:
    """Build and compile the OpenVFX agent planning graph.

    Graph flow:
        START → observe → plan_review → policy_check → decide
                                                          ↓ (if repair)
                                                        repair → END
                                                          ↓ (otherwise)
                                                         END
    """
    g: StateGraph = StateGraph(AgentGraphState)

    g.add_node("observe", observe_node)
    g.add_node("plan_review", plan_review_node)
    g.add_node("policy_check", policy_check_node)
    g.add_node("decide", decide_node)
    g.add_node("repair", repair_node)

    g.add_edge(START, "observe")
    g.add_edge("observe", "plan_review")
    g.add_edge("plan_review", "policy_check")
    g.add_edge("policy_check", "decide")
    g.add_conditional_edges(
        "decide",
        _should_repair,
        {"repair": "repair", END: END},
    )
    g.add_edge("repair", END)

    return g.compile()

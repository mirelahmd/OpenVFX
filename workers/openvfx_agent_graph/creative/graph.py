"""Build and compile the OpenVFX Creative Director graph."""

from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from .nodes import (
    assign_asset_roles_node,
    critique_treatment_node,
    design_story_node,
    design_treatment_node,
    finalize_node,
    identify_gaps_node,
    interpret_brief_node,
    observe_node,
)
from .schemas import CreativeState


def build_creative_graph() -> object:
    """Compile the creative director graph.

    Flow:
        START -> observe -> interpret_brief -> assign_asset_roles -> design_story
              -> identify_gaps -> design_treatment -> critique_treatment
              -> finalize -> END

    The path is deliberately linear with exactly one critique node and no edge
    back into the graph. The single-pass bound is therefore structural: there is
    no configuration under which this graph can loop.
    """
    graph: StateGraph = StateGraph(CreativeState)

    graph.add_node("observe", observe_node)
    graph.add_node("interpret_brief", interpret_brief_node)
    graph.add_node("assign_asset_roles", assign_asset_roles_node)
    graph.add_node("design_story", design_story_node)
    graph.add_node("identify_gaps", identify_gaps_node)
    graph.add_node("design_treatment", design_treatment_node)
    graph.add_node("critique_treatment", critique_treatment_node)
    graph.add_node("finalize", finalize_node)

    graph.add_edge(START, "observe")
    graph.add_edge("observe", "interpret_brief")
    graph.add_edge("interpret_brief", "assign_asset_roles")
    graph.add_edge("assign_asset_roles", "design_story")
    graph.add_edge("design_story", "identify_gaps")
    graph.add_edge("identify_gaps", "design_treatment")
    graph.add_edge("design_treatment", "critique_treatment")
    graph.add_edge("critique_treatment", "finalize")
    graph.add_edge("finalize", END)

    return graph.compile()


def initial_state(
    production_id: str,
    production_dir: str,
    brief: str,
    director_config: dict | None = None,
) -> CreativeState:
    config = director_config or {}
    return {
        "production_id": production_id,
        "production_dir": production_dir,
        "brief": brief,
        "director_config": config,
        "observations": None,
        "capabilities": None,
        "observe_status": "",
        "interpretation": {},
        "asset_roles": [],
        "story": {},
        "gaps": {},
        "treatment": {},
        "critique": {},
        "requested_mode": str(config.get("mode") or "deterministic"),
        "effective_mode": "",
        "fallback_used": False,
        "fallback_reason": "",
        "llm_calls": 0,
        "trace_events": [],
        "warnings": [],
        "error": None,
    }

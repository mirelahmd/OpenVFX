# Agent Plan Artifact

`agent_plan.json` is the compact contract for proposed OpenVFX actions.

Schema version:

- `openvfx_agent_plan.v1`

It stores:

- intent
- compact input references
- planner metadata
- typed actions
- references to `context_snapshot.json`, `policy_review.json`, and `plan_review.md`

It does not embed:

- raw transcripts
- queue dumps
- provider secrets
- logs
- media payloads

Prompt 061 adds approval metadata:

- `approved_at`
- `approval_mode`
- `rejected_at`
- `rejection_reason`

Converted plans use status `converted` and have linked jobs recorded in `linked_jobs.json`.

Prompt 062 adds bridge/result references that may exist alongside the core plan:

- `agent_result.md`
- `agent_run_summary.json`

These are derived summaries. The plan contract remains compact and still references rather than embeds large runtime details.

Prompt 066 adds creative planning references:

- `creative_brief.json`
- `deliverables.json`
- `asset_requirements.json`

These artifacts hold richer creator intent and capability needs so `agent_plan.json` can stay compact.

Prompt 069 adds:

- `visual_requests.dryrun.json`

This artifact previews provider-agnostic visual generation requests derived from asset requirements. It is referenced by the plan but kept separate so prompts, backend metadata, and output contracts do not bloat `agent_plan.json`.

# Agent Safety

BYOM Video agent planning is deterministic and local.

Current safety rules:

- Plans are artifacts before execution.
- Every action includes a `command_preview`.
- Custom file plans preview exact `byom-video run` flags for transcript, captions, chunks, highlights, roughcut, FFmpeg script, report, and roughcut clip count.
- Plan execution validates the plan before running actions.
- Saved plans require approval before `execute-plan`.
- `execute-plan --yes` records `approval_mode: yes_flag`.
- Input files are not modified.
- Exports require explicit `--with-export`.
- Watch plan execution requires `--once` in this version.
- `review-plan --write-artifact` and diff artifact flags write local markdown files for audit/review.
- No LLM or provider clients are called.

Validation checks include schema version, plan id, input path, goal, actions, supported action types, action statuses, and safety fields.

Unsupported or invalid plans are marked failed and are not executed.

Plan revisions create snapshots before changing `agent_plan.json`. Revisions that change executable actions or options reset approval to pending.

Approval metadata is stored in `agent_plan.json`:

- `approval_status`
- `approved_at`
- `approval_mode`

Inline `plan --execute` is still supported and records `approval_mode: inline_execute`, but the safer saved-plan workflow is review, approve, then execute.

Artifact navigation:

```sh
./byom-video plan-artifacts <plan_id>
./byom-video inspect-plan <plan_id>
```

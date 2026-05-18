# Agent Run Summary Artifact

`agent_run_summary.json` records what the `agent-run` bridge command did.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/agent_run_summary.json
```

Schema version:

- `openvfx_agent_run_summary.v1`

Shape:

```json
{
  "schema_version": "openvfx_agent_run_summary.v1",
  "created_at": "...",
  "agent_plan_id": "...",
  "status": "planned|completed|failed",
  "steps": [
    {
      "id": "step_0001",
      "type": "approve|convert|run_job|worker_once|start_daemon",
      "status": "completed|failed|skipped",
      "message": ""
    }
  ],
  "linked_jobs": [],
  "warnings": [],
  "errors": [],
  "next_commands": []
}
```

The summary is written when `agent-run` mutates state or when `--write-summary` is passed.

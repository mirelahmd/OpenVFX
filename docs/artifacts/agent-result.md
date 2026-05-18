# Agent Result Artifact

`agent_result.md` is a readable summary of a new-contract agent plan and its linked jobs.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/agent_result.md
```

It is written by:

```sh
./byom-video agent-result <agent_plan_id> --write-artifact
```

The artifact includes:

- agent plan id
- plan status and intent
- policy status
- conversion status
- linked job statuses and approval statuses
- next recommended commands

It does not execute jobs, call providers, or mutate job state.

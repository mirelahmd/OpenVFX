# Linked Jobs Artifact

`linked_jobs.json` connects an approved agent plan to durable job queue artifacts.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/linked_jobs.json
```

Schema version:

- `openvfx_agent_linked_jobs.v1`

It records created job ids, source action ids, job types, job statuses, approval statuses, job paths, outputs, warnings, and errors.

Conversion does not run jobs. Jobs remain queued until `job-run`, `job-worker`, or `daemon` processes them later.

Prompt 062 refreshes linked job status from live `job.json` artifacts when commands such as `agent-plan-jobs`, `agent-result`, `inspect-agent-plan`, and `review-agent-plan` read this artifact.

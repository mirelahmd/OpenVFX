# Queue Runtime Health

`byom-video queue` is the single runtime summary for daemon, worker, and job queue state.

## Commands

```sh
./byom-video queue
./byom-video queue --json
./byom-video queue --limit 20
./byom-video queue --failed
./byom-video queue --approval-needed
./byom-video queue --running

./byom-video queue health
./byom-video queue health --json
./byom-video queue health --strict
./byom-video queue health --stale-after 30m
./byom-video queue health --write-report
```

## What It Aggregates

- daemon state and PID liveness
- worker state and worker lock presence
- job counts by execution status
- job counts by approval status
- jobs needing approval
- failed jobs
- running jobs
- stale running jobs
- recommended next commands

## Health Statuses

- `ok`: runtime artifacts are readable and no attention items were found
- `warning`: failed jobs, pending approvals, stale runtime state, or stale running jobs were found
- `failed`: a hard runtime check failed, or `--strict` promoted warnings to failure

## Reports

`queue health --write-report` writes:

- `.byom-video/queue/queue_summary.json`
- `.byom-video/queue/queue_health.md`

The queue commands are read-only except for optional report writing.

Prompt 060 builds on this summary as an input to deterministic `agent-plan` context snapshots.

Prompt 061 can convert approved agent plans into jobs. Use `queue` afterward to see pending approvals and queued work.

Prompt 062 preserves source metadata on jobs created from agent plans. Queue JSON includes the source agent plan id when a job records it, so downstream tools can connect queue state back to the originating plan.

Prompt 067 create sessions can convert approved agent plans into jobs; use `queue` after conversion to see the resulting pending/approved work.

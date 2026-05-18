# Job Worker

`job-worker` is a foreground polling worker for the local job queue.

It scans `.byom-video/jobs/`, selects eligible jobs, and runs them sequentially through the existing `job-run` path.

Prompt 058 adds `byom-video daemon` as a background lifecycle wrapper around this worker. The worker behavior itself remains foreground and sequential.

Prompt 059 adds `byom-video queue` as the aggregated runtime health view over daemon, worker, and jobs.

## Eligibility

The worker only runs jobs where:

- `status: pending`
- `approval_status: approved` or `approval_status: not_required`

Jobs waiting for approval, rejected jobs, cancelled jobs, completed jobs, failed jobs, and already running jobs are skipped.

Selection order is oldest eligible job first by `created_at`.

## Commands

```sh
./byom-video job-worker --status
./byom-video job-worker --once
./byom-video job-worker --once --dry-run
./byom-video job-worker --loop --interval 10s --max-jobs 5
./byom-video job-worker --once --force-lock
```

## Safety

- `job-worker` requires one of `--once`, `--loop`, or `--status`
- `--status` does not acquire the worker lock
- one worker process at a time
- no parallel execution in v1
- no arbitrary shell execution

## Overrides

These runtime overrides are passed into `job-run` when selected jobs execute:

- `--allow-provider-calls`
- `--allow-overwrite`

They do not mutate the stored job policy.

# Worker Artifacts

Worker state is stored under:

```text
.byom-video/worker/
```

Files:

- `worker_state.json`
- `worker_events.jsonl`
- `worker.lock`

## `worker_state.json`

```json
{
  "schema_version": "openvfx_worker.v1",
  "worker_id": "20260509T210000Z-worker",
  "status": "idle|running|stopped|failed",
  "started_at": "2026-05-09T21:00:00Z",
  "updated_at": "2026-05-09T21:00:10Z",
  "last_scan_at": "2026-05-09T21:00:10Z",
  "last_job_id": "20260509T205000Z-make",
  "jobs_seen": 3,
  "jobs_run": 1,
  "jobs_succeeded": 1,
  "jobs_failed": 0,
  "mode": "once|loop",
  "interval_seconds": 10,
  "max_jobs": 1,
  "warnings": [],
  "errors": []
}
```

## `worker_events.jsonl`

Event types:

- `WORKER_STARTED`
- `WORKER_STOPPED`
- `WORKER_FAILED`
- `WORKER_SCAN_STARTED`
- `WORKER_SCAN_COMPLETED`
- `WORKER_JOB_SELECTED`
- `WORKER_JOB_SKIPPED`
- `WORKER_JOB_STARTED`
- `WORKER_JOB_COMPLETED`
- `WORKER_JOB_FAILED`
- `WORKER_LOCK_ACQUIRED`
- `WORKER_LOCK_RELEASED`
- `WORKER_LOCK_BUSY`

## `worker.lock`

```json
{
  "worker_id": "20260509T210000Z-worker",
  "pid": 12345,
  "created_at": "2026-05-09T21:00:00Z"
}
```

The lock prevents duplicate workers. If the process exits uncleanly, rerun with `job-worker --force-lock`.

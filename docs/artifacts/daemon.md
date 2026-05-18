# Daemon Artifacts

Daemon state is stored under:

```text
.byom-video/daemon/
```

Files:

- `daemon_state.json`
- `daemon_events.jsonl`
- `daemon.log`
- `daemon.pid`

## `daemon_state.json`

```json
{
  "schema_version": "openvfx_daemon.v1",
  "daemon_id": "20260509T220000Z-daemon",
  "status": "starting|running|stopped|failed|unknown",
  "pid": 12345,
  "started_at": "2026-05-09T22:00:00Z",
  "stopped_at": "2026-05-09T22:15:00Z",
  "updated_at": "2026-05-09T22:15:00Z",
  "worker_mode": "loop",
  "worker_interval_seconds": 10,
  "worker_max_jobs": 0,
  "allow_provider_calls": false,
  "allow_overwrite": false,
  "last_error": "",
  "warnings": [],
  "errors": []
}
```

## `daemon_events.jsonl`

Event types:

- `DAEMON_STARTED`
- `DAEMON_START_FAILED`
- `DAEMON_STOPPED`
- `DAEMON_STOP_FAILED`
- `DAEMON_STATUS_CHECKED`
- `DAEMON_ALREADY_RUNNING`
- `DAEMON_STALE_PID`
- `DAEMON_LOG_READ`

## `daemon.pid`

Stores the worker wrapper PID as plain text.

## `daemon.log`

Appended worker stdout/stderr from the background `job-worker --loop` process.

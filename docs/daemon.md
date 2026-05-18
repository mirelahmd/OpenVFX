# Daemon

`byom-video daemon` is a local background-process wrapper around `job-worker --loop`.

It does not add planning, scheduling intelligence, or a second execution path. It only manages the worker lifecycle.

## Commands

```sh
./byom-video daemon start --interval 10s
./byom-video daemon status
./byom-video daemon logs --lines 80
./byom-video daemon stop
```

## Behavior

- starts a background `job-worker --loop`
- writes PID, state, log, and daemon event artifacts under `.byom-video/daemon/`
- refuses duplicate starts when the PID is still alive
- reports stale PID state clearly
- with `--force`, clears stale PID state and passes `--force-lock` through to `job-worker`
- approved/not-required job filtering still happens inside `job-worker`

Use `byom-video queue` when you want daemon, worker, lock, and job queue state in one view.

Prompt 060 uses the queue/runtime summary as planner context only. It does not make the daemon autonomous.

## Safety

- approval gates still apply
- no arbitrary shell execution
- no new providers
- no planner or autonomous decision layer
- no system service registration yet

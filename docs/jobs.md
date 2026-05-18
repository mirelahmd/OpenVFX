# Job Queue Foundation

`byom-video` includes a local durable job system for queuing, approving, and running operations asynchronously. Jobs are filesystem artifacts — no daemon, no database.

---

## Overview

A job is a typed action envelope with a lifecycle:

```
created → pending → approved/rejected → running → completed/failed
                  ↘ not_required ↗
cancelled (any pending/running state)
```

Each job has:
- A **policy** controlling what the run is allowed to do
- An **approval gate** (bypassed with `--yes` or for `not_required` types)
- A **JSONL event log** with the full lifecycle history

---

## Job Locations

```
.byom-video/jobs/<job_id>/job.json       # job artifact
.byom-video/jobs/<job_id>/events.jsonl   # event log
```

Job IDs are timestamp-prefixed: `20260509T140000Z-make`, `20260509T140001Z-revise-make`, etc.

---

## Action Types (v1)

| Type | Approval Required | Description |
|---|---|---|
| `make` | Yes (pending) | Run `byom-video make` with a goal |
| `revise_make` | Yes (pending) | Run `byom-video revise-make` on an existing make |
| `validate_creative_assemble` | No (not_required) | Run `byom-video validate-creative-assemble` |

---

## Commands

### job-create

Create a new job. Does not execute it.

```
byom-video job-create --type <action_type> [flags]
```

**Common flags:**
- `--type <make|revise_make|validate_creative_assemble>` — required
- `--allow-provider-calls` — allow LLM/TTS provider calls during run
- `--allow-overwrite` — allow overwriting existing artifacts
- `--allow-external-network` — allow external network access
- `--json` — output JSON

**make-specific flags:**
- `--goal <text>` — required; the video goal
- `--preset <shorts|metadata>` — optional

**revise_make-specific flags:**
- `--make-id <id>` — required; the make to revise
- `--request <text>` — required; the revision instruction
- `--reassemble` — append a reassemble action

**validate_creative_assemble-specific flags:**
- `--plan-id <id>` — required; the creative plan to validate

**Examples:**

```bash
# Queue a make
byom-video job-create --type make --goal "product launch short" --preset shorts

# Queue a revision with provider access
byom-video job-create --type revise_make \
  --make-id 20260509T140000Z-product-launch \
  --request "switch to tiktok" \
  --allow-provider-calls

# Queue validation (no approval needed)
byom-video job-create --type validate_creative_assemble --plan-id 20260509T140100Z-product-launch
```

---

### jobs

List all jobs.

```
byom-video jobs [--filter <status>] [--limit <n>] [--json]
```

---

### job-inspect

Inspect a specific job.

```
byom-video job-inspect <job_id> [--json]
```

---

### job-events

Show the event log for a job.

```
byom-video job-events <job_id> [--limit <n>] [--json]
```

---

### job-approve

Approve a pending job (allows it to run without `--yes`).

```
byom-video job-approve <job_id> [--json]
```

---

### job-reject

Reject a pending job (blocks it from running).

```
byom-video job-reject <job_id> [--reason <text>] [--json]
```

---

### job-cancel

Cancel a job (pending or running).

```
byom-video job-cancel <job_id> [--reason <text>] [--json]
```

---

### job-run

Execute a job's action.

```
byom-video job-run <job_id> [--yes] [--json]
```

- `--yes` — bypass the approval gate (runs even if `approval_status: pending`)
- Without `--yes`, the job must be approved first

The handler reads the job's `input` and `policy` fields to dispatch the action.

---

### job-result

Show the result of a completed job.

```
byom-video job-result <job_id> [--json]
```

If the job was created from an agent plan, `job-result` also shows the source agent plan id, source action id, and an `inspect-agent-plan` follow-up command.

---

### job-validate

Validate a job artifact.

```
byom-video job-validate <job_id> [--json]
```

---

### job-worker

Run the foreground worker.

```
byom-video job-worker --status
byom-video job-worker --once
byom-video job-worker --once --dry-run
byom-video job-worker --loop --interval 10s --max-jobs 5
```

- `--once` — scan once and run at most one eligible job unless `--max-jobs` is higher
- `--loop` — keep polling until interrupted or `--max-jobs` is reached
- `--status` — read worker state without acquiring the lock
- `--dry-run` — list eligible jobs and write worker state/events, but run nothing
- `--fail-fast` — stop on first job failure
- `--force-lock` — replace a stale `worker.lock`

The worker reuses existing `job-run` logic and does not introduce a second execution path.

---

### daemon

Wrap the foreground worker in a background process:

```
byom-video daemon start --interval 10s
byom-video daemon status
byom-video daemon logs --lines 40
byom-video daemon stop
```

This is lifecycle management only. Job selection, approval gates, and execution still happen in `job-worker` and `job-run`.

---

### queue

Runtime control-plane summary:

```
byom-video queue
byom-video queue --json
byom-video queue health
byom-video queue health --write-report
```

This aggregates daemon state, worker state, job counts, approval-needed jobs, failed jobs, running jobs, stale runtime signals, and next suggested commands.

Prompt 060 adds `agent-plan`, which reads runtime state and proposes future actions without creating jobs yet.

Prompt 061 adds `agent-plan-to-job`, which converts approved agent plans into durable jobs without running them.

Prompt 062 adds `agent-run` and `agent-result`. `agent-run` can approve and convert plans, and can optionally run linked approved jobs only when execution flags are explicit.

Prompt 067 adds `create`, which can create and approve jobs from one scoped create session when `--yes --approval-scope ... --convert` is explicit.

---

## Policy

Each job carries a policy that governs what the run is allowed to do:

| Field | Default | Description |
|---|---|---|
| `allow_overwrite` | false | Overwrite existing artifacts |
| `allow_provider_calls` | false | Call LLM/TTS providers |
| `allow_external_network` | false | Allow external network access |
| `allow_media_writes` | true | Write media files (always true in v1) |

---

## Event Log

Every lifecycle transition emits an event to `events.jsonl`:

| Event | Description |
|---|---|
| `JOB_CREATED` | Job artifact written |
| `JOB_APPROVED` | Approval granted |
| `JOB_REJECTED` | Approval rejected |
| `JOB_CANCELLED` | Job cancelled |
| `JOB_RUN_STARTED` | Execution started |
| `JOB_ACTION_STARTED` | Action handler invoked |
| `JOB_ACTION_COMPLETED` | Action handler returned success |
| `JOB_ACTION_FAILED` | Action handler returned error |
| `JOB_RUN_COMPLETED` | Execution completed successfully |
| `JOB_RUN_FAILED` | Execution completed with failure |
| `JOB_POLICY_BLOCKED` | Execution blocked by approval gate |

---

## Workflow Example

```bash
# 1. Create
byom-video job-create --type make --goal "product demo" --preset shorts
# → Job created: 20260509T140000Z-make

# 2. Review
byom-video job-inspect 20260509T140000Z-make

# 3. Approve
byom-video job-approve 20260509T140000Z-make

# 4. Run
byom-video job-run 20260509T140000Z-make

# 5. View result
byom-video job-result 20260509T140000Z-make
```

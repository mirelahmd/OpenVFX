# Agent Plans v1

`byom-video agent-plan` creates a compact planning artifact without executing anything.

Artifacts live under:

```text
.byom-video/agent_plans/<agent_plan_id>/
  agent_plan.json
  context_snapshot.json
  policy_review.json
  creative_brief.json
  deliverables.json
  asset_requirements.json
  visual_requests.dryrun.json
  plan_review.md
  events.jsonl
```

## Commands

```sh
./byom-video agent-plan --goal "queue health"
./byom-video agent-plan --input media/Untitled.mov --goal "make a vertical short with captions and narration" --write-review
./byom-video agent-plans
./byom-video inspect-agent-plan <plan_id>
./byom-video review-agent-plan <plan_id> --write-artifact
./byom-video agent-policy <plan_id>
./byom-video approve-agent-plan <plan_id>
./byom-video reject-agent-plan <plan_id> --reason "not needed"
./byom-video agent-plan-to-job <plan_id> --dry-run
./byom-video agent-plan-to-job <plan_id> --approve-jobs
./byom-video agent-plan-jobs <plan_id>
./byom-video agent-run <plan_id> --dry-run
./byom-video agent-run <plan_id> --yes --convert --approve-jobs
./byom-video agent-result <plan_id>
./byom-video agent-result <plan_id> --write-artifact
./byom-video agent-orchestrate --input media/Untitled.mov --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions"
./byom-video visual-requests <plan_id> --overwrite
```

## Visual Request Dry-Runs

Agent plans now include `visual_requests.dryrun.json` when visual asset requirements are present. The dry-run artifact is provider-agnostic and resolves configured creative tool routes such as:

- `creative.broll_generate`
- `creative.video_generate`
- `creative.image_generate`
- `creative.visual_transform`
- `creative.style_transfer`
- `creative.object_remove`
- `creative.background_replace`

The artifact contains prompts, output contracts, backend metadata, and auth env var names only. It does not call providers and never prints raw secret values.

Execute a configured custom HTTP visual backend explicitly:

```sh
./byom-video execute-visual-requests <plan_id> --yes --allow-provider-calls --allow-external-network
./byom-video review-visual-generation <plan_id> --write-artifact
```

Only `provider: custom-http-visual` is executable in v1. Generated files are written under the plan directory and indexed by `generated_assets.json`.

## Planner Adapter Interface v1

Prompt 063 adds a pluggable planner backend. The plan contract (`openvfx_agent_plan.v1`) is fixed; the planner brain can be swapped without changing downstream code.

### Selecting a planner

```sh
# Default: deterministic rule-based planner
./byom-video agent-plan --goal "make a video" --input media/clip.mov

# Ollama LLM planner
./byom-video agent-plan --goal "make a vertical short" \
  --planner ollama \
  --planner-model llama3 \
  --planner-backend http://localhost:11434

# Ollama with automatic deterministic fallback on failure
./byom-video agent-plan --goal "make a vertical short" \
  --planner ollama \
  --planner-model llama3 \
  --planner-fallback-deterministic
```

### Planner flags

| Flag | Default | Description |
|---|---|---|
| `--planner <name>` | `deterministic` | Planner to use: `deterministic` or `ollama` |
| `--planner-model <model>` | _(from config)_ | Model name for LLM planners |
| `--planner-backend <url>` | _(from config)_ | Base URL for LLM backend |
| `--planner-route <key>` | `agent.planning` | Config route key for model lookup |
| `--planner-fallback-deterministic` | false | Fall back to deterministic if LLM planner fails |
| `--planner-timeout-seconds <n>` | 120 | HTTP timeout for LLM planner calls |
| `--planner-temperature <f>` | _(model default)_ | LLM sampling temperature |
| `--planner-max-output-chars <n>` | 0 (no limit) | Truncate raw LLM response before parsing |

### Config-based model routing

Instead of passing `--planner-model` every time, configure a route in `byom-video.yaml`:

```yaml
models:
  enabled: true
  routes:
    agent.planning: my-local-planner
  entries:
    my-local-planner:
      model: llama3
      base_url: http://localhost:11434
```

### Planner block in agent_plan.json

Every plan artifact records which planner was used:

```json
{
  "planner": {
    "mode": "ollama",
    "model": "llama3",
    "version": "v1"
  }
}
```

When `--planner-fallback-deterministic` triggers, `mode` reflects the *requested* planner and `warnings` explains the fallback.

### Schema validation

All planners — deterministic and LLM — must produce actions that pass the same schema contract before the plan is written:

- Action IDs must match `action_NNNN` format and be unique.
- Types must be one of: `make`, `revise_make`, `validate_creative_assemble`, `queue_health`.
- Each action must have a non-empty description.
- Maximum 10 actions per plan.

Policy review runs after the planner output regardless of which planner was used.

## Planner Reliability (Prompt 064)

### Extended planner metadata

The `planner` block in `agent_plan.json` now carries full observability fields:

```json
{
  "planner": {
    "mode": "ollama",
    "requested_mode": "ollama",
    "effective_mode": "deterministic",
    "backend": "http://localhost:11434",
    "route": "agent.planning",
    "provider": "ollama",
    "model": "llama3",
    "version": "v1",
    "fallback_used": true,
    "fallback_reason": "ollama planner failed; using deterministic fallback: ..."
  }
}
```

- `requested_mode`: the `--planner` flag value (what was asked for).
- `effective_mode`: the planner that actually executed (differs from `requested_mode` when fallback triggers).
- `fallback_used` / `fallback_reason`: set when `--planner-fallback-deterministic` was used and the LLM planner failed.

### planner_request.json artifact

Every plan writes a `planner_request.json` alongside `agent_plan.json`:

```text
.byom-video/agent_plans/<plan_id>/
  agent_plan.json
  context_snapshot.json
  policy_review.json
  planner_request.json   ← new
  events.jsonl
```

For the Ollama planner, this file contains the exact system and user prompts that were sent, the resolved model and backend, and (if fallback triggered) the fallback annotations. For the deterministic planner it contains the parsed goal hints.

### Diagnose command

Check planner configuration without creating a plan:

```sh
# Show what would be used
./byom-video agent-planner-diagnose
./byom-video agent-planner-diagnose --planner ollama --planner-model llama3

# Test Ollama connectivity
./byom-video agent-planner-diagnose --planner ollama --planner-model llama3 --check

# JSON output
./byom-video agent-planner-diagnose --planner ollama --json
```

The diagnose command resolves the model from `--planner-model` or from config routes (same logic as `agent-plan`), then optionally pings the Ollama `/api/tags` endpoint with `--check`. It never writes artifacts.

## Notes

- deterministic planner: no provider calls, no network
- ollama planner: calls a local Ollama server; no external network by default
- no job creation during planning
- no execution during planning

## Approval And Conversion

Prompt 061 adds the bridge from plan artifacts to durable jobs:

1. Create a plan.
2. Review policy and markdown.
3. Approve or reject the plan.
4. Preview conversion with `agent-plan-to-job --dry-run`.
5. Convert approved actions to jobs.

Supported conversions:

- `make` -> `make` job
- `revise_make` -> `revise_make` job
- `validate_creative_assemble` -> `validate_creative_assemble` job
- `queue_health` is informational and skipped with a warning

Conversion does not execute anything.

## Run Bridge And Results

Prompt 062 adds a safer one-command bridge for the common approval and conversion flow:

```sh
./byom-video agent-run <plan_id> --dry-run
./byom-video agent-run <plan_id> --yes --convert --approve-jobs
./byom-video agent-run <plan_id> --yes --convert --approve-jobs --run-jobs
./byom-video agent-run <plan_id> --yes --convert --approve-jobs --start-daemon
./byom-video agent-result <plan_id> --write-artifact
```

Default `agent-run <plan_id>` is non-mutating. It prints the staged bridge actions and next commands. It only mutates when explicit flags such as `--yes`, `--convert`, `--run-jobs`, `--worker-once`, or `--start-daemon` are present.

`agent-result` reads the agent plan, policy review, linked jobs, and current job artifacts to summarize what exists now and what to do next.

Safety boundaries remain unchanged:

- no new planner logic
- no LLM or LangGraph execution
- no provider calls unless explicitly allowed by job policy and runtime flags
- no job execution unless `--run-jobs`, `--worker-once`, or `--start-daemon` is passed

## Creative Brief Intelligence

Prompt 066 adds structured planning artifacts for richer creator prompts:

```text
.byom-video/agent_plans/<agent_plan_id>/
  creative_brief.json
  deliverables.json
  asset_requirements.json
```

Example prompt:

```text
Make a 35-second luxury fitness Instagram Reel. Use my talking clip as narration
and gym clips as b-roll. Make it cinematic, darker, premium, fast cuts in the
first 5 seconds. Generate futuristic gym/city b-roll if the backend exists.
Use bold lower-third captions. Give me Instagram caption options.
```

The planner extracts:

- target duration and platform
- style, mood, and pacing
- caption style/position
- source media roles
- deliverables such as edited video and Instagram caption options
- asset requirements such as generated b-roll or voiceover needs

`agent-orchestrate` creates the brief, plan, policy review, and then runs the LangGraph sidecar unless `--skip-graph` is passed. It does not approve, convert, or execute jobs.

## Agentic Create

Prompt 067 adds `create` as the creator-facing orchestration command:

```sh
./byom-video create media/Untitled.mov --goal "<rich brief>" --write-review
./byom-video create media/Untitled.mov --goal "<rich brief>" --yes --approval-scope local --convert --approve-jobs
```

`create` wraps the same agent plan contract and writes a separate scoped `create_session.json`. Its approvals are session-scoped, not global.

Prompt 068 upgrades the create review/result surface so `create_review.md` reads as a single creator-facing status page with brief, deliverables, asset requirements, capability gaps, jobs, outputs, and next commands.

## LangGraph Agent Sidecar (Prompt 065)

`agent-graph-run` invokes a Python LangGraph sidecar that reads the existing plan artifacts and produces a structured decision: **approve**, **flag**, **repair**, or **reject**.

```sh
# Dry-run: shows what would run without invoking Python.
./byom-video agent-graph-run <plan_id> --dry-run
./byom-video agent-graph-run <plan_id> --dry-run --json

# Live run: invokes the LangGraph sidecar.
./byom-video agent-graph-run <plan_id>
./byom-video agent-graph-run <plan_id> --json

# Use a specific workers directory (auto-detected otherwise).
./byom-video agent-graph-run <plan_id> --workers-dir /path/to/workers
```

### Graph flow

```
observe → plan_review → policy_check → decide → [repair] → END
```

| Node | What it does |
|---|---|
| `observe` | Loads `agent_plan.json`, `policy_review.json`, `context_snapshot.json` |
| `plan_review` | Validates action types, required fields, plan structure |
| `policy_check` | Evaluates `policy_review.json`; derives policy status and blocks |
| `decide` | Chooses: approve / flag / repair / reject |
| `repair` | Generates repair suggestions when blocked |

### Decisions

| Decision | Meaning |
|---|---|
| `approve` | Plan is valid and policy allows execution |
| `flag` | Plan is valid but policy has warnings; review recommended |
| `repair` | Policy blocked; specific repair commands generated |
| `reject` | Missing plan, invalid action types, or unresolvable block |

### Artifacts written

Both artifacts are written alongside the existing plan files in `.byom-video/agent_plans/<plan_id>/`:

**`graph_trace.json`** — schema `openvfx_graph_trace.v1`
```json
{
  "schema_version": "openvfx_graph_trace.v1",
  "run_id": "graphrun-...",
  "plan_id": "agentplan-...",
  "started_at": "...",
  "nodes_executed": [...],
  "edges_traversed": [...],
  "total_duration_ms": 12
}
```

**`agent_decision.json`** — schema `openvfx_agent_decision.v1`
```json
{
  "schema_version": "openvfx_agent_decision.v1",
  "run_id": "graphrun-...",
  "plan_id": "agentplan-...",
  "decision": "approve",
  "decision_reason": "Plan is valid and policy allows execution.",
  "policy_status": "allowed",
  "plan_issues": [],
  "repair_suggestions": [],
  "warnings": [],
  "next_commands": ["byom-video approve-agent-plan agentplan-..."]
}
```

### Python setup

The sidecar lives in `workers/openvfx_agent_graph/`. Install the graph extras:

```sh
pip install -e 'workers[graph]'
# or for both transcription and graph:
pip install -e 'workers[transcribe,graph]'
```

`workers-dir` auto-detection: `BYOM_VIDEO_WORKERS_DIR` env → binary-relative `workers/` → CWD ancestor traversal.

### Safety boundaries

- No job execution, no provider calls, no direct video editing
- Reads only: `agent_plan.json`, `policy_review.json`, `context_snapshot.json`
- Writes only: `graph_trace.json`, `agent_decision.json`
- No OpenAI/Claude/cloud — local graph execution only
- LangGraph is the reasoning brain; OpenVFX remains the execution spine

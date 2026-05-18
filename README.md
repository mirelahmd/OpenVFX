# BYOM Video

Local-first video workflow CLI. Transcribe, cut, caption, and plan exports from your own machine — no cloud required.

> Alpha. Schemas and commands are still evolving.

## Requirements

- Go 1.22+
- `ffmpeg` and `ffprobe` on `PATH`
- Python 3.10+ with `faster-whisper` for real transcription (optional for metadata-only runs)
- Ollama for local model expansion (optional)

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/mirelahmd/byom-video/main/install.sh | sh
source ~/.zshrc
byom-video version
byom-video doctor
```

The install script handles the Go binary, Python environment, and worker package automatically.

**Or build from source:**

```sh
git clone https://github.com/mirelahmd/byom-video.git
cd byom-video
go build -o byom-video ./cmd/byom-video
./byom-video version
```

**Or via `go install`** (requires GitHub repo named `byom-video`):

```sh
go install github.com/mirelahmd/byom-video/cmd/byom-video@latest
```

## Quickstart

```sh
# Check dependencies
byom-video doctor
byom-video doctor --media   # also checks ffmpeg filter availability (subtitles, amix)

# Initialize workspace
byom-video init

# Metadata-only run (no Python needed)
byom-video pipeline media/clip.mov --preset metadata

# Full shorts pipeline
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video pipeline media/clip.mov --preset shorts

# Inspect and export
byom-video runs
byom-video inspect <run_id>
byom-video export <run_id>

# High-level agentic create flow
byom-video create media/clip.mov --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions" --write-review
byom-video create-result <create_session_id>
byom-video create-sessions
byom-video inspect-create-session <create_session_id>
byom-video visual-requests <agent_plan_id> --overwrite
byom-video execute-visual-requests <agent_plan_id> --yes --allow-provider-calls --allow-external-network

# One-command creator flow (plan only, no --yes):
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mov --goal "make a short cinematic clip with captions"

# One-command creator flow (end-to-end, --yes):
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mov \
  --goal "make a short cinematic clip with captions" \
  --yes --burn-captions --allow-missing-captions

# Reuse existing pipeline run (skip re-transcribing):
byom-video make --goal "make a cinematic short" \
  --skip-pipeline <run_id> --yes --burn-captions --allow-missing-captions

# Review result:
byom-video makes
byom-video make-result <make_id>
byom-video inspect-make <make_id>
```

## What It Does

| Area | Commands |
|---|---|
| Creator flow | `make`, `makes`, `inspect-make`, `make-result` |
| Agentic create | `create`, `create-result`, `create-sessions`, `inspect-create-session` |
| Visual dry-runs | `visual-requests` |
| Visual execution | `execute-visual-requests`, `review-visual-generation` |
| Pipeline | `pipeline`, `run`, `batch`, `watch` |
| Inspection | `inspect`, `artifacts`, `validate`, `open-report` |
| Export | `export`, `ffmpeg-script`, `export-manifest`, `concat-plan` |
| Clips | `clip-cards`, `review-clips`, `selected-clips`, `enhance-roughcut` |
| Agent planning | `plan`, `review-plan`, `approve-plan`, `execute-plan`, `revise-plan` |
| Inference mask | `mask-plan`, `review-mask`, `revise-mask`, `mask-decisions`, `mask-decision` |
| Goal-aware cut selection | `goal-rerank`, `goal-roughcut` |
| Expansion | `expansion-plan`, `expand-local-stub`, `expand`, `verify-expansions` |
| Maintenance | `cleanup`, `retry-batch`, `rerun`, `doctor` |
| Creative registry | `tools`, `tools validate`, `tools requirements`, `creative-plan`, `creative-plans`, `inspect-creative-plan`, `review-creative-plan` |
| Creative plan approval | `approve-creative-plan`, `creative-plan-events`, `creative-preview`, `execute-creative-plan`, `creative-result`, `validate-creative-plan` |
| Creative stub execution | `creative-execute-stub`, `review-creative-outputs` |
| Creative timeline | `creative-timeline`, `creative-render-plan`, `review-creative-timeline` |
| Creative assemble | `creative-assemble`, `validate-creative-assemble`, `review-creative-assemble` |
| Script generation | `creative-generate-script`, `review-script` |
| Style pack | `style init`, `style inspect`, `style validate` |
| Job worker | `job-worker` |
| Daemon | `daemon start`, `daemon stop`, `daemon status`, `daemon logs` |
| Queue health | `queue`, `queue health` |
| Agent plan v1 | `agent-plan`, `agent-plans`, `inspect-agent-plan`, `review-agent-plan`, `agent-policy` |

## Local Model Setup (Optional)

```sh
# Python transcription
python3 -m venv ~/.byom-venv
~/.byom-venv/bin/pip install -e "workers[transcribe]"
export BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python

# Ollama for local model expansion
ollama pull qwen2.5:7b
byom-video models doctor
byom-video expand <run_id> --dry-run
```

## Goal-Aware Reranking (Optional)

Deterministic goal-aware reranking:

```sh
./byom-video goal-rerank <run_id> --goal "make a short clip under 60 seconds"
./byom-video goal-roughcut <run_id>
```

Optional local Ollama reranking:

```sh
./byom-video goal-rerank <run_id> --goal "make a cinematic short" --use-ollama --fallback-deterministic
```

This produces additive artifacts and leaves the original `highlights.json` and `roughcut.json` unchanged.

Goal-aware planning is now available from the agent layer as well:

```sh
./byom-video plan media/clip.mov --goal "make a short clip under 60 seconds" --goal-aware --dry-run
./byom-video plan media/clip.mov --goal "make a cinematic short" --goal-aware --goal-use-ollama --goal-fallback-deterministic --execute
```

After a goal-aware run completes, export-facing handoff can explicitly prefer the goal-aware cut path:

```sh
./byom-video clip-cards <run_id> --prefer-goal-roughcut
./byom-video selected-clips <run_id> --prefer-goal-roughcut
./byom-video goal-handoff <run_id> --overwrite
```

`--goal-use-ollama` is explicit. BYOM Video does not call Ollama from normal pipeline or plan execution unless the plan or command requests it.

## Style Pack (Ollama Script Generation)

BYOM Video supports local Ollama-powered script generation using a **Style Pack** — a directory of Markdown files that describe your creator voice, visual preferences, and brand rules.

```sh
# Set up your style pack
byom-video style init
# Edit .openvfx/style/{profile,script_style,captions,visual_style,do_not_do,examples}.md
byom-video style validate

# Generate a script from a creative plan using your local Ollama
byom-video creative-generate-script <creative_plan_id> [--style-dir <path>] [--no-style]
byom-video review-script <creative_plan_id> --write-artifact

# Or combine with make:
byom-video make input.mov --goal "make a cinematic short" --yes --generate-script
```

Requires an Ollama backend configured in `byom-video.yaml`:

```yaml
tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.script: local_writer
```

See [docs/style-pack.md](docs/style-pack.md) for full documentation.

## Creative Capability Registry

BYOM Video includes a provider-agnostic `tools` registry for creative-agent workflows such as:

- script generation (via Ollama — live)
- voice generation
- image or video generation
- caption generation
- music or sound generation
- render composition

This layer is config, validation, and planning. Only Ollama text generation is currently implemented as a live provider call.

```sh
./byom-video tools
./byom-video tools validate
./byom-video tools requirements --goal "make a cinematic short with narration and AI b-roll"
./byom-video creative-plan media/clip.mov --goal "make a cinematic short with narration and AI b-roll"
./byom-video creative-plans
./byom-video review-creative-plan <creative_plan_id> --write-artifact
./byom-video approve-creative-plan <creative_plan_id>
./byom-video creative-preview <creative_plan_id>
./byom-video execute-creative-plan <creative_plan_id>
./byom-video creative-result <creative_plan_id> --write-artifact
./byom-video validate-creative-plan <creative_plan_id>
./byom-video creative-plan-events <creative_plan_id>

# Stub execution (no providers, no shell commands)
./byom-video creative-execute-stub <creative_plan_id>
./byom-video creative-execute-stub <creative_plan_id> --overwrite
./byom-video review-creative-outputs <creative_plan_id> --write-artifact

# Timeline assembly (optional run clips via --run-id)
./byom-video creative-timeline <creative_plan_id>
./byom-video creative-timeline <creative_plan_id> --run-id <run_id> --prefer-goal
./byom-video creative-render-plan <creative_plan_id>
./byom-video review-creative-timeline <creative_plan_id> --write-artifact

# Draft render (requires ffmpeg; source clips must come from a run via --run-id)
./byom-video creative-assemble <creative_plan_id> --dry-run
./byom-video creative-assemble <creative_plan_id> --mode reencode
# With caption burn-in and voiceover mixing
./byom-video creative-assemble <creative_plan_id> --burn-captions --captions captions.srt --mix-voiceover --voiceover vo.wav
./byom-video validate-creative-assemble <creative_plan_id>
./byom-video review-creative-assemble <creative_plan_id> --write-artifact
```

## Job Worker

Prompt 057 adds the first foreground worker for the local job queue:

```sh
./byom-video job-worker --status
./byom-video job-worker --once
./byom-video job-worker --once --dry-run
./byom-video job-worker --loop --interval 10s --max-jobs 5
./byom-video job-worker --once --force-lock
```

It scans approved pending jobs, selects the oldest eligible job first, and runs jobs sequentially through the existing `job-run` path.

## Daemon Lifecycle

Prompt 058 adds an optional local background wrapper around `job-worker --loop`:

```sh
./byom-video daemon start --interval 10s --reset-log
./byom-video daemon status
./byom-video daemon logs --lines 40
./byom-video daemon stop
```

This daemon:

- only manages the worker process lifecycle
- writes PID, state, log, and daemon event artifacts
- does not add planner logic
- still respects the existing job approval gate

## Queue Runtime Health

Prompt 059 adds a single runtime summary for daemon, worker, and jobs:

```sh
./byom-video queue
./byom-video queue --json
./byom-video queue health
./byom-video queue health --write-report
```

This view highlights:

- jobs needing approval
- failed jobs
- stale daemon PID state
- stale worker lock state
- stale running jobs
- next commands such as `daemon start`, `job-approve`, and `job-result`

## Agent Plan Contract v1

Prompt 060 adds a deterministic planner that observes local runtime and proposes typed OpenVFX actions without executing anything:

```sh
./byom-video agent-plan --goal "queue health"
./byom-video agent-plan --input media/Untitled.mov --goal "make a vertical short with captions and narration" --write-review
./byom-video agent-plans
./byom-video inspect-agent-plan <plan_id>
./byom-video review-agent-plan <plan_id> --write-artifact
./byom-video agent-policy <plan_id>
./byom-video approve-agent-plan <plan_id>
./byom-video agent-plan-to-job <plan_id> --dry-run
./byom-video agent-plan-to-job <plan_id> --approve-jobs
./byom-video agent-plan-jobs <plan_id>
./byom-video agent-run <plan_id> --dry-run
./byom-video agent-run <plan_id> --yes --convert --approve-jobs
./byom-video agent-result <plan_id> --write-artifact
./byom-video agent-orchestrate --input media/Untitled.mov --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions"
```

This writes compact plan artifacts under `.byom-video/agent_plans/`:

- `agent_plan.json`
- `context_snapshot.json`
- `policy_review.json`
- `plan_review.md`

It is planning only:

- no provider calls
- no job creation
- no execution
- no LangGraph or LLM planner yet

Prompt 061 adds approval and conversion into durable jobs. Prompt 062 adds `agent-run` and `agent-result` as a safer bridge/result layer. Jobs are not run unless you explicitly use `job-run`, `job-worker`, `daemon`, or `agent-run --run-jobs` / `--worker-once` / `--start-daemon`.

Prompt 066 adds Creative Brief Intelligence. Rich creator prompts now produce `creative_brief.json`, `deliverables.json`, and `asset_requirements.json` alongside the agent plan. `agent-orchestrate` creates those artifacts and runs the LangGraph review sidecar without executing jobs.

Prompt 067 adds `create`, a scoped high-level creator flow. By default it is preview-only. With `--yes --approval-scope local --convert --approve-jobs`, it can approve and convert the scoped plan into local jobs. Provider calls still require `--approval-scope provider --allow-provider-calls --allow-external-network`.

Prompt 068 upgrades the create review/result surface. `create_review.md` now summarizes the creative brief, deliverables, asset requirements, capability gaps, graph/plan state, linked jobs, outputs, and next commands.

Prompt 069 adds provider-agnostic visual generation dry-runs. Visual requirements such as generated b-roll, generated images, style transfer, object removal, background replacement, and visual transforms are resolved through `tools.routes` keys like `creative.broll_generate`, `creative.image_generate`, and `creative.visual_transform`. `visual_requests.dryrun.json` shows the future request preview without calling providers or printing secret values.

Prompt 070 adds `custom-http-visual` execution for user-configured visual backends. It requires explicit `--yes --allow-provider-calls --allow-external-network`, writes `generated_assets.json`, saves generated outputs under the agent plan directory, and writes scrubbed request/response audit artifacts. No official provider SDKs or provider-specific adapters are hardcoded.

Backend names, provider strings, route keys, endpoints, and options are all user-defined. Secrets should stay in env vars. Commands only print env var names, never values.

Only currently implemented execution providers should be treated as executable. Cloud-oriented creative tool examples are illustrative placeholders.

After approved plan execution, use:

```sh
./byom-video agent-result <plan_id>
./byom-video agent-result <plan_id> --write-artifact
```

For goal-aware runs, generate a single review bundle:

```sh
./byom-video goal-review-bundle <run_id> --overwrite
```

## What's Not Here Yet

- No web UI
- No cloud provider execution (OpenAI, Anthropic, etc.)
- No DaVinci / Premiere integration
- No Docker workflow
- Goal-aware reranking is explicit; it is not the default plan or pipeline behavior

## License

[MIT](LICENSE)

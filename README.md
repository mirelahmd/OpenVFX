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
```

## What It Does

| Area | Commands |
|---|---|
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

## Creative Capability Registry

BYOM Video now includes a provider-agnostic `tools` registry for future creative-agent workflows such as:

- script generation
- voice generation
- image or video generation
- caption generation
- music or sound generation
- render composition

This layer is config, validation, and planning only. It does not call providers.

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

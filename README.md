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
git clone https://github.com/your-org/byom-video
cd byom-video
go build -o byom-video ./cmd/byom-video
./byom-video version
```

## Quickstart

```sh
# Check dependencies
./byom-video doctor

# Initialize workspace
./byom-video init

# Metadata-only run (no Python needed)
./byom-video pipeline media/clip.mov --preset metadata

# Full shorts pipeline
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/clip.mov --preset shorts

# Inspect and export
./byom-video runs
./byom-video inspect <run_id>
./byom-video export <run_id>
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
| Expansion | `expansion-plan`, `expand-local-stub`, `expand`, `verify-expansions` |
| Maintenance | `cleanup`, `retry-batch`, `rerun`, `doctor` |

## Local Model Setup (Optional)

```sh
# Install faster-whisper for transcription
python3 -m venv .venv && .venv/bin/pip install faster-whisper

# Install Ollama for local model expansion
ollama pull qwen2.5:7b
./byom-video models doctor
./byom-video expand <run_id> --dry-run
```

## What's Not Here Yet

- No web UI
- No cloud provider execution (OpenAI, Anthropic, etc.)
- No DaVinci / Premiere integration
- No Docker workflow

## License

[MIT](LICENSE)

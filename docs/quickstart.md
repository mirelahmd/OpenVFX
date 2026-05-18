# Quickstart

## Install (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/mirelahmd/byom-video/main/install.sh | sh
source ~/.zshrc   # or restart terminal
byom-video version
byom-video doctor
```

The install script sets up the Go binary and Python environment automatically.

## Install via go install

Requires Go 1.22+. The GitHub repo must be named `byom-video` for this to work without proxy flags.

```sh
go install github.com/mirelahmd/byom-video/cmd/byom-video@latest
```

## Build from source

```sh
git clone https://github.com/mirelahmd/byom-video.git
cd byom-video
go build -o byom-video ./cmd/byom-video
./byom-video version
```

## Python worker setup

Required for real transcription (not needed for `--preset metadata`):

```sh
python3 -m venv ~/.byom-venv
~/.byom-venv/bin/pip install -e "workers[transcribe]"
export BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python
```

Or set `BYOM_VIDEO_PYTHON` in your shell config permanently.

## Initialize

```sh
byom-video init
byom-video doctor
```

## First run

Metadata-only (no Python needed):

```sh
byom-video pipeline media/clip.mp4 --preset metadata
```

Full shorts pipeline:

```sh
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video pipeline media/clip.mp4 --preset shorts
```

## Inspect and export

```sh
byom-video runs
byom-video inspect <run_id>
byom-video open-report <run_id>
byom-video export <run_id>
```

## One-command creator flow

The `make` command runs pipeline → plan → assemble in one step.

**Planning mode** (no `--yes`) — pipeline + plan only, stops for review:

```sh
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mp4 \
  --goal "make a short cinematic clip with captions"
```

**Execution mode** (`--yes`) — end-to-end to `draft.mp4`:

```sh
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mp4 \
  --goal "make a short cinematic clip with captions" \
  --yes --burn-captions --allow-missing-captions
```

**Reuse an existing pipeline run** (skip re-transcribing):

```sh
byom-video make --goal "make a cinematic short" \
  --skip-pipeline <run_id> --yes --burn-captions --allow-missing-captions
```

Use `--strict-input` to fail if the input file differs from the original run manifest.

**Also export clips** after pipeline:

```sh
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mp4 \
  --goal "make a short clip" --yes --export
```

Use `--require-export` to fail if the export script is missing.

**Preset** (default: `shorts`):

```sh
# metadata preset — pipeline only, no clip assembly; planning mode or --skip-pipeline required for --yes
byom-video make media/clip.mp4 --goal "inspect clip" --preset metadata
```

**Goal-aware cut selection:**

```sh
BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python byom-video make media/clip.mp4 \
  --goal "make a short cinematic clip" --yes --goal-aware
```

Review the result:

```sh
byom-video makes
byom-video make-result <make_id>
byom-video make-result <make_id> --write-artifact
byom-video inspect-make <make_id>
byom-video inspect-creative-plan <creative_plan_id>
byom-video validate-creative-assemble <creative_plan_id>
```

### When to use planning mode vs execution mode

| Situation | Recommendation |
|---|---|
| First time with new video | Planning mode (no `--yes`) to review plan before assembly |
| Re-editing from existing run | `--skip-pipeline <run_id> --yes` |
| Fully automated pipeline | `--yes --allow-missing-captions` |
| Want to see what would run | `--dry-run` |

### Current limitations

- `make` always runs `pipeline --preset shorts` unless `--skip-pipeline` is set; `--preset` currently controls only validation.
- Goal-aware Ollama reranking requires `--goal-aware --use-ollama-goal` and a running Ollama server.
- No auto-export of pipeline clips by default; use `--export` or `byom-video export <run_id>` separately.

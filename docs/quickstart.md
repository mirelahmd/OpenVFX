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

#!/usr/bin/env sh
set -eu

python_bin="${BYOM_VIDEO_PYTHON:-python3}"
input="${1:-media/Untitled.mov}"

if [ ! -x "./byom-video" ]; then
  echo "missing ./byom-video binary"
  echo "build it with: go build ./cmd/byom-video"
  exit 1
fi

if [ ! -f "$input" ]; then
  echo "no input media found: $input"
  echo "usage: scripts/smoke-chunks.sh [input-file]"
  echo "example: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-chunks.sh media/Untitled.mov"
  exit 0
fi

if ! "$python_bin" -c "import faster_whisper" >/dev/null 2>&1; then
  echo "faster-whisper is not importable with: $python_bin"
  echo "install with: python3 -m pip install -e \"workers[transcribe]\""
  echo "or run with: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-chunks.sh $input"
  exit 1
fi

output="$(BYOM_VIDEO_PYTHON="$python_bin" ./byom-video run "$input" --with-transcript --with-chunks --transcript-model-size tiny)"
echo "$output"
run_dir="$(echo "$output" | awk -F': ' '/run directory:/ {print $2}')"
if [ -n "$run_dir" ] && [ -d "$run_dir" ]; then
  echo "artifact list:"
  find "$run_dir" -maxdepth 1 -type f | sort
fi

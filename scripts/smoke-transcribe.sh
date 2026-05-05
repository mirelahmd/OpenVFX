#!/usr/bin/env sh
set -eu

python_bin="${BYOM_VIDEO_PYTHON:-python3}"
input="${1:-media/Untitled.mov}"

if [ ! -f "$input" ]; then
  echo "no input media found: $input"
  echo "usage: scripts/smoke-transcribe.sh [input-file]"
  echo "example: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-transcribe.sh media/Untitled.mov"
  exit 0
fi

if ! "$python_bin" -c "import faster_whisper" >/dev/null 2>&1; then
  echo "faster-whisper is not importable with: $python_bin"
  echo "install with: python3 -m pip install -e \"workers[transcribe]\""
  echo "or run with: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-transcribe.sh $input"
  exit 1
fi

output="$(BYOM_VIDEO_PYTHON="$python_bin" ./byom-video run "$input" --with-transcript --transcript-model-size tiny)"
echo "$output"
echo "$output" | awk -F': ' '/run directory:/ {print "run directory: " $2}'

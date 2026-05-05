#!/usr/bin/env bash
set -euo pipefail

input="${1:-}"
if [[ -z "$input" && -f "media/Untitled.mov" ]]; then
  input="media/Untitled.mov"
fi

if [[ -z "$input" ]]; then
  echo "missing input file"
  echo "usage: scripts/smoke-pipeline.sh <input-file>"
  echo "or place a local test file at media/Untitled.mov"
  exit 1
fi

if [[ ! -f "$input" ]]; then
  echo "input file does not exist: $input"
  exit 1
fi

if [[ ! -x "./byom-video" ]]; then
  echo "missing ./byom-video binary"
  echo "run: go build ./cmd/byom-video"
  exit 1
fi

python_bin="${BYOM_VIDEO_PYTHON:-python3}"
if ! "$python_bin" -c "import faster_whisper" >/dev/null 2>&1; then
  echo "faster-whisper is not importable with: $python_bin"
  echo "try: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-pipeline.sh $input"
  exit 1
fi

./byom-video init

output="$(
  BYOM_VIDEO_PYTHON="$python_bin" ./byom-video pipeline "$input" --preset shorts
)"

echo "$output"

run_id="$(printf '%s\n' "$output" | awk -F': *' '/run id:/ {print $2; exit}')"
run_dir="$(printf '%s\n' "$output" | awk -F': *' '/run directory:/ {print $2; exit}')"

if [[ -n "$run_id" ]]; then
  echo
  echo "Run id: $run_id"
fi
if [[ -n "$run_dir" ]]; then
  echo "Run directory: $run_dir"
  echo "Report path: $run_dir/report.html"
fi

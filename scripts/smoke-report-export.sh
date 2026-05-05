#!/usr/bin/env bash
set -euo pipefail

execute_export=0
input=""

for arg in "$@"; do
  case "$arg" in
    --execute-export)
      execute_export=1
      ;;
    *)
      input="$arg"
      ;;
  esac
done

if [[ -z "$input" && -f "media/Untitled.mov" ]]; then
  input="media/Untitled.mov"
fi

if [[ -z "$input" ]]; then
  echo "missing input file"
  echo "usage: scripts/smoke-report-export.sh [--execute-export] <input-file>"
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
  echo "try: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-report-export.sh $input"
  exit 1
fi

output="$(
  BYOM_VIDEO_PYTHON="$python_bin" ./byom-video run "$input" \
    --with-transcript \
    --with-captions \
    --with-chunks \
    --with-highlights \
    --with-roughcut \
    --with-ffmpeg-script \
    --with-report \
    --transcript-model-size tiny
)"

echo "$output"

run_id="$(printf '%s\n' "$output" | awk -F': *' '/run id:/ {print $2; exit}')"
run_dir="$(printf '%s\n' "$output" | awk -F': *' '/run directory:/ {print $2; exit}')"

if [[ -z "$run_id" || -z "$run_dir" ]]; then
  echo "could not parse run id or run directory from output"
  exit 1
fi

echo
echo "Run directory: $run_dir"
echo "Report path: $run_dir/report.html"
echo "Export command: ./byom-video export $run_id"

if [[ -d "$run_dir" ]]; then
  echo
  echo "Artifacts:"
  find "$run_dir" -maxdepth 2 -type f | sort
fi

if [[ "$execute_export" == "1" ]]; then
  echo
  echo "Executing export:"
  ./byom-video export "$run_id"
  echo
  echo "Exports:"
  find "$run_dir/exports" -maxdepth 1 -type f | sort
fi

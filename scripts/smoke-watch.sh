#!/usr/bin/env bash
set -euo pipefail

stamp="$(date +%Y%m%d%H%M%S)"
watch_dir="media/watch-smoke/$stamp"
mkdir -p "$watch_dir"

if [[ -f media/Untitled.mov ]]; then
  sample="$watch_dir/sample.mov"
  cp media/Untitled.mov "$sample"
elif [[ -f examples/fixtures/tiny.mp4 ]]; then
  sample="$watch_dir/sample.mp4"
  cp examples/fixtures/tiny.mp4 "$sample"
else
  echo "No fixture media found. Add media/Untitled.mov or run scripts/make-fixture.sh first." >&2
  exit 1
fi

touch -t 202001010000 "$sample"

before_count="$(./byom-video watch-status --json | awk '/"input_path"/ {count++} END {print count+0}')"
./byom-video watch "$watch_dir" --preset metadata --once
./byom-video watch-status
after_first_count="$(./byom-video watch-status --json | awk '/"input_path"/ {count++} END {print count+0}')"

./byom-video watch "$watch_dir" --preset metadata --once
after_count="$(./byom-video watch-status --json | awk '/"input_path"/ {count++} END {print count+0}')"

if [[ "$after_first_count" -le "$before_count" ]]; then
  echo "watch did not process new fixture: before=$before_count after_first=$after_first_count" >&2
  exit 1
fi

if [[ "$before_count" != "$after_count" ]]; then
  if [[ "$after_first_count" != "$after_count" ]]; then
    echo "watch registry count changed on second pass: after_first=$after_first_count after=$after_count" >&2
    exit 1
  fi
else
  echo "watch registry count did not change after first pass" >&2
  exit 1
fi

echo "Second watch pass skipped already processed file."

# Manual full shorts watch:
# BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video watch "$watch_dir" --preset shorts

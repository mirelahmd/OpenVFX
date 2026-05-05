#!/usr/bin/env bash
set -euo pipefail

fixture_dir="media/batch-smoke"
mkdir -p "$fixture_dir"

if [[ -f media/Untitled.mov ]]; then
  cp media/Untitled.mov "$fixture_dir/sample1.mov"
  cp media/Untitled.mov "$fixture_dir/sample2.mov"
elif [[ -f examples/fixtures/tiny.mp4 ]]; then
  cp examples/fixtures/tiny.mp4 "$fixture_dir/sample1.mp4"
  cp examples/fixtures/tiny.mp4 "$fixture_dir/sample2.mp4"
else
  echo "No fixture media found. Add media/Untitled.mov or run scripts/make-fixture.sh first." >&2
  exit 1
fi

./byom-video batch "$fixture_dir" --preset metadata
./byom-video batches

latest_batch="$(./byom-video batches | awk 'NR==2 {print $1}')"
if [[ -n "$latest_batch" ]]; then
  ./byom-video inspect-batch "$latest_batch"
fi

# Manual full shorts batch:
# BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video batch "$fixture_dir" --preset shorts

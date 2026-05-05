#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f media/Untitled.mov ]]; then
  echo "media/Untitled.mov not found; add a local media file before running this smoke test." >&2
  exit 1
fi

./byom-video plan media/Untitled.mov --goal "make 5 shorts" --dry-run
latest_plan="$(./byom-video plans | awk 'NR==2 {print $1}')"

./byom-video revise-plan "$latest_plan" --request "make 3 shorts" --show-diff
./byom-video snapshots "$latest_plan"
./byom-video review-plan "$latest_plan"

echo "Plan revision smoke completed without executing the media pipeline."

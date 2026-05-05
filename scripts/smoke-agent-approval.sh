#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f media/Untitled.mov ]]; then
  echo "media/Untitled.mov not found; add a local media file before running this smoke test." >&2
  exit 1
fi

./byom-video plan media/Untitled.mov --goal "make 3 shorts" --dry-run
latest_plan="$(./byom-video plans | awk 'NR==2 {print $1}')"

./byom-video review-plan "$latest_plan"
./byom-video approve-plan "$latest_plan"
./byom-video execute-plan "$latest_plan" --dry-run
./byom-video diff-plan "$latest_plan" "$latest_plan"

echo "Approval smoke completed without executing the media pipeline."

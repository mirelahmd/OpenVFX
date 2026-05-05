#!/usr/bin/env bash
set -euo pipefail

execute=0
if [[ "${1:-}" == "--execute" ]]; then
  execute=1
fi

if [[ ! -f media/Untitled.mov ]]; then
  echo "media/Untitled.mov not found; add a local media file before running this smoke test." >&2
  exit 1
fi

if [[ "$execute" -eq 1 ]]; then
  BYOM_VIDEO_PYTHON="${BYOM_VIDEO_PYTHON:-.venv/bin/python}" ./byom-video plan media/Untitled.mov --goal "make 3 shorts" --execute
else
  ./byom-video plan media/Untitled.mov --goal "make 3 shorts" --dry-run
fi

./byom-video plans
latest_plan="$(./byom-video plans | awk 'NR==2 {print $1}')"
if [[ -n "$latest_plan" ]]; then
  ./byom-video inspect-plan "$latest_plan"
fi

echo "Optional execution command:"
echo "  BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video plan media/Untitled.mov --goal \"make 3 shorts\" --execute"

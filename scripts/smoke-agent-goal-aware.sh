#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-.}"
cd "$ROOT"

if [[ ! -f media/Untitled.mov ]]; then
  echo "media/Untitled.mov is missing."
  echo "Create or add it, then run:"
  echo "  ./byom-video plan media/Untitled.mov --goal \"make a short clip under 60 seconds\" --goal-aware --dry-run"
  exit 0
fi

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

EXECUTE=0
if [[ "${1:-}" == "--execute" ]]; then
  EXECUTE=1
fi

./byom-video plan media/Untitled.mov --goal "make a short clip under 60 seconds" --goal-aware --dry-run

if [[ "$EXECUTE" -eq 0 ]]; then
  echo
  echo "Optional execution path:"
  echo "  ./byom-video plan media/Untitled.mov --goal \"make a short clip under 60 seconds\" --goal-aware --execute"
  echo
  echo "After execution, use either:"
  echo "  ./byom-video goal-handoff <run_id> --overwrite"
  echo "or:"
  echo "  ./byom-video clip-cards <run_id> --prefer-goal-roughcut --overwrite"
  echo "  ./byom-video selected-clips <run_id> --prefer-goal-roughcut --overwrite"
  exit 0
fi

OUTPUT="$(./byom-video plan media/Untitled.mov --goal "make a short clip under 60 seconds" --goal-aware --execute)"
echo "$OUTPUT"
RUN_ID="$(printf '%s\n' "$OUTPUT" | awk '/run id:/ {print $3}' | tail -n 1)"
if [[ -z "${RUN_ID:-}" ]]; then
  echo "Could not determine run id from execution output."
  exit 1
fi

./byom-video goal-handoff "$RUN_ID" --overwrite
./byom-video inspect "$RUN_ID"

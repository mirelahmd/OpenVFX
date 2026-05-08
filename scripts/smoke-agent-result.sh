#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-.}"
cd "$ROOT"

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

latest_plan_id() {
  ./byom-video plans 2>/dev/null | awk 'NR>1 {print $1}' | head -n 1
}

PLAN_ID="$(latest_plan_id || true)"
if [[ -z "${PLAN_ID:-}" ]]; then
  if [[ -f media/Untitled.mov ]]; then
    ./byom-video plan media/Untitled.mov --goal "make a short clip under 60 seconds" --goal-aware --dry-run >/dev/null
    PLAN_ID="$(latest_plan_id || true)"
  fi
fi

if [[ -z "${PLAN_ID:-}" ]]; then
  echo "No plan found."
  echo "Create one with:"
  echo "  ./byom-video plan media/Untitled.mov --goal \"make a short clip under 60 seconds\" --goal-aware --dry-run"
  exit 0
fi

./byom-video agent-result "$PLAN_ID" --write-artifact

RUN_ID="$(./byom-video plans | awk -v plan="$PLAN_ID" '$1==plan {print $5}')"
if [[ -n "${RUN_ID:-}" && -f ".byom-video/runs/$RUN_ID/goal_rerank.json" && -f ".byom-video/runs/$RUN_ID/goal_roughcut.json" ]]; then
  ./byom-video goal-review-bundle "$RUN_ID" --overwrite
  ./byom-video inspect "$RUN_ID"
else
  echo
  echo "Goal-aware run artifacts not found for the latest executed plan."
  echo "Run scripts/smoke-agent-goal-aware.sh --execute or generate goal-rerank/goal-roughcut manually."
fi

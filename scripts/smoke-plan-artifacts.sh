#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

input="media/Untitled.mov"
if [[ ! -f "$input" ]]; then
  echo "No $input found; skipping plan artifacts smoke test."
  echo "Add a local media file at $input and rerun this script."
  exit 0
fi

./byom-video plan "$input" --goal "make 5 shorts" --dry-run
plan_id="$(./byom-video plans | awk 'NR==2 {print $1}')"

if [[ -z "$plan_id" || "$plan_id" == "PLAN" ]]; then
  echo "Could not find latest plan id."
  exit 1
fi

./byom-video revise-plan "$plan_id" --request "make captions only"
./byom-video review-plan "$plan_id" --write-artifact
./byom-video diff-snapshot "$plan_id" snapshot_0001 --write-artifact
./byom-video plan-artifacts "$plan_id"

echo "Plan artifacts smoke completed for $plan_id."

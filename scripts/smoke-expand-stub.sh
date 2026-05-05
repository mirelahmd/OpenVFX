#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

run_id=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/inference_mask.json" && -f ".byom-video/runs/$candidate/expansion_tasks.json" ]]; then
    run_id="$candidate"
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  echo "No run with inference_mask.json and expansion_tasks.json found."
  echo "Create one with:"
  echo "  scripts/smoke-mask-plan.sh"
  exit 0
fi

echo "Running expand-stub smoke for run $run_id"

echo "=== expand-stub ==="
./byom-video expand-stub "$run_id" --overwrite
echo "---"
./byom-video expand-stub "$run_id" --task-type caption_variants --overwrite
echo "---"
./byom-video expand-stub "$run_id" --overwrite --json
echo "---"

echo "=== expansion-validate ==="
./byom-video expansion-validate "$run_id"
echo "---"
./byom-video expansion-validate "$run_id" --json
echo "---"

echo "=== review-expansions ==="
./byom-video review-expansions "$run_id"
echo "---"
./byom-video review-expansions "$run_id" --write-artifact
echo "---"

echo "=== inspect-mask (should show expansion files) ==="
./byom-video inspect-mask "$run_id"

echo "Expand-stub smoke completed for $run_id."

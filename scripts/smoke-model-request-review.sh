#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

run_id=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/model_requests.dryrun.json" || -f ".byom-video/runs/$candidate/model_requests.executed.json" ]]; then
    run_id="$candidate"
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  while read -r candidate _; do
    [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
    if [[ -f ".byom-video/runs/$candidate/inference_mask.json" && -f ".byom-video/runs/$candidate/expansion_tasks.json" ]]; then
      run_id="$candidate"
      break
    fi
  done < <(./byom-video runs --all)
  if [[ -z "$run_id" ]]; then
    echo "No eligible run found."
    echo "Create one with:"
    echo "  scripts/smoke-mask-plan.sh"
    exit 0
  fi
  ./byom-video expand-dry-run "$run_id" >/dev/null
fi

echo "Running model-request review smoke for run $run_id"
./byom-video review-model-requests "$run_id"
echo "---"
./byom-video review-model-requests "$run_id" --write-artifact
echo "---"
./byom-video inspect-mask "$run_id"

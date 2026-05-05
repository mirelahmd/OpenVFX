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

echo "Running routes/mask-revision smoke for run $run_id"

./byom-video routes-plan "$run_id"
echo "---"
./byom-video routes-plan "$run_id" --write-artifact
echo "---"
./byom-video revise-mask "$run_id" --request "set captions to 12 words" --show-diff
echo "---"
./byom-video mask-snapshots "$run_id"
echo "---"
./byom-video diff-mask "$run_id" mask_snapshot_0001 --write-artifact
echo "---"
./byom-video inspect-mask "$run_id"

echo "Routes/mask-revision smoke completed for $run_id."

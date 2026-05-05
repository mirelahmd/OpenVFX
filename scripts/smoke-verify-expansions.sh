#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

run_id=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/inference_mask.json" && \
        -f ".byom-video/runs/$candidate/verification.json" && \
        -d ".byom-video/runs/$candidate/expansions" ]]; then
    run_id="$candidate"
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  echo "No run with inference_mask.json, verification.json, and expansions/ found."
  echo "Create one with:"
  echo "  scripts/smoke-mask-plan.sh"
  echo "  scripts/smoke-expand-stub.sh"
  exit 0
fi

echo "Running verify-expansions smoke for run $run_id"

echo "=== verify-expansions ==="
./byom-video verify-expansions "$run_id"
echo "---"
./byom-video verify-expansions "$run_id" --json
echo "---"
./byom-video verify-expansions "$run_id" --tolerance-seconds 0.5
echo "---"

echo "=== review-verification ==="
./byom-video review-verification "$run_id"
echo "---"
./byom-video review-verification "$run_id" --write-artifact
echo "---"
./byom-video review-verification "$run_id" --json
echo "---"

echo "=== inspect-mask (should show verification_results) ==="
./byom-video inspect-mask "$run_id"

echo "Verify-expansions smoke completed for $run_id."

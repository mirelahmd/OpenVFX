#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

./byom-video models
./byom-video models validate
./byom-video config show

echo "Example dynamic config: examples/configs/local-only.yaml"

run_id="$(./byom-video runs | awk 'NR==2 {print $1}')"
if [[ -z "$run_id" || "$run_id" == "RUN" ]]; then
  echo "No runs found; skipping mask validation smoke section."
  exit 0
fi

./byom-video mask-template "$run_id"
./byom-video mask-validate "$run_id"

echo "Dynamic model/mask smoke completed for $run_id."

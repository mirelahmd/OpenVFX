#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

./byom-video config show
./byom-video models

run_id="$(./byom-video runs | awk 'NR==2 {print $1}')"
if [[ -z "$run_id" || "$run_id" == "RUN" ]]; then
  echo "No runs found; skipping mask-template smoke section."
  echo "Create a run first, then run: ./byom-video mask-template <run_id>"
  exit 0
fi

./byom-video mask-template "$run_id"
./byom-video inspect-mask "$run_id"

echo "Model/mask smoke completed for $run_id."

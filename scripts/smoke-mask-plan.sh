#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

run_id=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/roughcut.json" || -f ".byom-video/runs/$candidate/highlights.json" ]]; then
    run_id="$candidate"
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  echo "No run with roughcut.json or highlights.json found."
  echo "Create one with:"
  echo "  BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts"
  exit 0
fi

./byom-video mask-plan "$run_id" --overwrite
./byom-video mask-validate "$run_id"
./byom-video review-mask "$run_id" --write-artifact
./byom-video expansion-plan "$run_id" --overwrite
./byom-video verification-plan "$run_id" --overwrite
./byom-video inspect-mask "$run_id"

echo "Mask plan smoke completed for $run_id."

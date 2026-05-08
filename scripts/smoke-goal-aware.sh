#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

run_id=""
roughcut_overwrite=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/highlights.json" ]]; then
    run_id="$candidate"
    if [[ -f ".byom-video/runs/$candidate/goal_roughcut.json" ]]; then
      roughcut_overwrite="--overwrite"
    fi
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  echo "No run with highlights.json found."
  echo "Create one with:"
  echo "  BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts"
  exit 0
fi

./byom-video goal-rerank "$run_id" --goal "make a short clip under 60 seconds"
if [[ -n "$roughcut_overwrite" ]]; then
  ./byom-video goal-roughcut "$run_id" --overwrite
else
  ./byom-video goal-roughcut "$run_id"
fi
./byom-video validate "$run_id"
./byom-video inspect "$run_id"

echo
echo "Optional Ollama command:"
echo "  ./byom-video goal-rerank $run_id --goal \"make a cinematic short\" --use-ollama --fallback-deterministic"

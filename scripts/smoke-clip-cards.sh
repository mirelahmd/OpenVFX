#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-.}"
cd "$ROOT"

latest_run_id() {
  find .byom-video/runs -mindepth 1 -maxdepth 1 -type d 2>/dev/null | while read -r dir; do
    if [[ -f "$dir/roughcut.json" ]]; then
      basename "$dir"
    fi
  done | sort | tail -n 1
}

RUN_ID="$(latest_run_id || true)"
if [[ -z "${RUN_ID:-}" ]]; then
  echo "No run with roughcut.json found."
  echo 'Run something like: BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts'
  exit 0
fi

RUN_DIR=".byom-video/runs/$RUN_ID"

if [[ ! -f "$RUN_DIR/expansions/caption_variants.json" ]]; then
  ./byom-video expand-local-stub "$RUN_ID" --overwrite
  ./byom-video expansion-validate "$RUN_ID"
  if [[ -f "$RUN_DIR/verification.json" ]]; then
    ./byom-video verify-expansions "$RUN_ID" || true
  else
    echo "verification.json missing; run ./byom-video verification-plan $RUN_ID first if you want verification results."
  fi
fi

./byom-video clip-cards "$RUN_ID" --overwrite
./byom-video review-clips "$RUN_ID" --write-artifact
./byom-video enhance-roughcut "$RUN_ID" --overwrite

if [[ -f "$RUN_DIR/report.html" ]]; then
  ./byom-video open-report "$RUN_ID"
else
  echo "report.html not present for $RUN_ID."
  echo 'Create one with: BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video run media/Untitled.mov --with-transcript --with-captions --with-chunks --with-highlights --with-roughcut --with-ffmpeg-script --with-report --transcript-model-size tiny'
fi

./byom-video inspect "$RUN_ID"

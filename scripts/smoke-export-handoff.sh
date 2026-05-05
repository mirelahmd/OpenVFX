#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-.}"
cd "$ROOT"

latest_run_id() {
  find .byom-video/runs -mindepth 1 -maxdepth 1 -type d 2>/dev/null | while read -r dir; do
    if [[ -f "$dir/roughcut.json" || -f "$dir/enhanced_roughcut.json" ]]; then
      basename "$dir"
    fi
  done | sort | tail -n 1
}

RUN_ID="$(latest_run_id || true)"
if [[ -z "${RUN_ID:-}" ]]; then
  echo "No run with roughcut/enhanced_roughcut found."
  echo "Run scripts/smoke-clip-cards.sh first."
  exit 0
fi

./byom-video selected-clips "$RUN_ID" --overwrite
./byom-video ffmpeg-script "$RUN_ID" --mode reencode --overwrite
./byom-video export-manifest "$RUN_ID" --overwrite
./byom-video concat-plan "$RUN_ID" --overwrite
./byom-video validate "$RUN_ID"
./byom-video inspect "$RUN_ID"

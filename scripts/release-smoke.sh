#!/usr/bin/env bash
set -euo pipefail

with_ollama=0
for arg in "$@"; do
  case "$arg" in
    --with-ollama) with_ollama=1 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

go test ./...
go build ./cmd/byom-video
python3 -m compileall -q workers/byom_video_workers

python_bin="${BYOM_VIDEO_PYTHON:-python3}"
if [[ -f media/Untitled.mov ]]; then
  if "$python_bin" -c "import faster_whisper" >/dev/null 2>&1; then
    BYOM_VIDEO_PYTHON="$python_bin" scripts/smoke-pipeline.sh media/Untitled.mov
  else
    echo "Skipping smoke-pipeline.sh: faster-whisper is not importable with $python_bin."
  fi
else
  echo "Skipping smoke-pipeline.sh: media/Untitled.mov not found."
fi

if [[ -x ./byom-video && -n "$(find .byom-video/runs -mindepth 2 -maxdepth 2 -name report.html 2>/dev/null | head -n 1)" ]]; then
  scripts/smoke-runs.sh
else
  echo "Skipping smoke-runs.sh: no run with report.html found."
fi

if [[ -d .byom-video/runs ]]; then
  scripts/smoke-mask-plan.sh
  scripts/smoke-export-handoff.sh
else
  echo "Skipping run-dependent smoke scripts: no .byom-video/runs directory."
fi

if [[ "$with_ollama" -eq 1 ]]; then
  scripts/smoke-ollama-real.sh
fi

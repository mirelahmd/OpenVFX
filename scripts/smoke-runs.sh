#!/usr/bin/env bash
set -euo pipefail

if [[ ! -x "./byom-video" ]]; then
  echo "missing ./byom-video binary"
  echo "run: go build ./cmd/byom-video"
  exit 1
fi

if [[ ! -d ".byom-video/runs" ]]; then
  echo "no runs directory found"
  echo "run: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-pipeline.sh media/Untitled.mov"
  exit 1
fi

runs_output="$(./byom-video runs --all)"
echo "$runs_output"

latest_run=""
latest_with_report=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" || "$candidate" == "No" ]] && continue
  if [[ -z "$latest_run" ]]; then
    latest_run="$candidate"
  fi
  if [[ -f ".byom-video/runs/$candidate/report.html" ]]; then
    latest_with_report="$candidate"
    break
  fi
done <<< "$runs_output"

if [[ -n "$latest_with_report" ]]; then
  latest_run="$latest_with_report"
fi
if [[ -z "$latest_run" || "$latest_run" == "No" ]]; then
  echo "no runs found"
  echo "run: BYOM_VIDEO_PYTHON=.venv/bin/python scripts/smoke-pipeline.sh media/Untitled.mov"
  exit 1
fi

echo
echo "Inspecting latest run: $latest_run"
./byom-video inspect "$latest_run"

echo
echo "Artifact paths:"
./byom-video artifacts "$latest_run"

echo
echo "Report path:"
if [[ -f ".byom-video/runs/$latest_run/report.html" ]]; then
  ./byom-video open-report "$latest_run"
else
  echo "report.html missing for $latest_run; skipping open-report"
fi

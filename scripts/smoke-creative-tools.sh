#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

./byom-video tools
./byom-video tools validate
./byom-video tools requirements --goal "make a cinematic short with narration and AI b-roll"

if [[ -f "media/Untitled.mov" ]]; then
  ./byom-video creative-plan media/Untitled.mov --goal "make a cinematic short with narration and AI b-roll"
  latest_id="$(./byom-video creative-plans | awk 'NR==2 {print $1}')"
  if [[ -n "${latest_id:-}" ]]; then
    ./byom-video inspect-creative-plan "$latest_id"
    ./byom-video review-creative-plan "$latest_id" --write-artifact
  fi
else
  echo "media/Untitled.mov not found; skipping creative-plan smoke steps."
fi

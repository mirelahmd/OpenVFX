#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f media/Untitled.mov ]]; then
  echo "media/Untitled.mov not found; add a local media file before running this smoke test." >&2
  exit 1
fi

mkdir -p media/batch-smoke
if [[ ! -f media/batch-smoke/sample1.mov ]]; then
  cp media/Untitled.mov media/batch-smoke/sample1.mov
fi

./byom-video plan media/Untitled.mov --goal "make 3 shorts" --dry-run
./byom-video plan media/batch-smoke --goal "batch process shorts" --mode batch --dry-run
./byom-video plan media/batch-smoke --goal "watch this folder for shorts" --mode watch --once --dry-run
./byom-video plans

echo "Expanded agent smoke completed without execution."

#!/usr/bin/env bash
set -euo pipefail

export_first=0
run_id=""

for arg in "$@"; do
  case "$arg" in
    --export-first)
      export_first=1
      ;;
    *)
      if [[ -n "$run_id" ]]; then
        echo "usage: scripts/smoke-validate-export.sh [--export-first] [run_id]" >&2
        exit 2
      fi
      run_id="$arg"
      ;;
  esac
done

if [[ ! -x ./byom-video ]]; then
  echo "missing ./byom-video; run: go build -o byom-video ./cmd/byom-video" >&2
  exit 1
fi

if [[ -z "$run_id" ]]; then
  if [[ ! -d .byom-video/runs ]]; then
    echo "No runs found. Create one first, for example:"
    echo "  ./byom-video pipeline media/Untitled.mov --preset shorts"
    exit 0
  fi
  run_id="$(./byom-video runs --limit 1 | awk 'NR==2 {print $1}')"
  if [[ -z "$run_id" ]]; then
    echo "No runs found. Create one first, for example:"
    echo "  ./byom-video pipeline media/Untitled.mov --preset shorts"
    exit 0
  fi
fi

if [[ "$export_first" -eq 1 ]]; then
  ./byom-video export "$run_id"
fi

./byom-video validate "$run_id"

run_dir=".byom-video/runs/$run_id"
if [[ -d "$run_dir/exports" ]]; then
  if [[ -f "$run_dir/export_validation.json" ]]; then
    echo "export validation artifact: $run_dir/export_validation.json"
  else
    echo "exports exist, but export_validation.json is not present"
    echo "Run './byom-video export $run_id' on a machine with ffprobe available to generate export validation."
  fi
fi

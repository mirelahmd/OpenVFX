#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

backup=""
if [[ -f byom-video.yaml ]]; then
  backup="$(mktemp)"
  cp byom-video.yaml "$backup"
fi
cleanup() {
  if [[ -n "$backup" && -f "$backup" ]]; then
    cp "$backup" byom-video.yaml
    rm -f "$backup"
  else
    rm -f byom-video.yaml
  fi
}
trap cleanup EXIT
cp examples/configs/local-only.yaml byom-video.yaml

run_id=""
while read -r candidate _; do
  [[ -z "${candidate:-}" || "$candidate" == "RUN" ]] && continue
  if [[ -f ".byom-video/runs/$candidate/inference_mask.json" && -f ".byom-video/runs/$candidate/expansion_tasks.json" ]]; then
    run_id="$candidate"
    break
  fi
done < <(./byom-video runs --all)

if [[ -z "$run_id" ]]; then
  echo "No run with inference_mask.json and expansion_tasks.json found."
  echo "Create one with:"
  echo "  scripts/smoke-mask-plan.sh"
  exit 0
fi

echo "Running Ollama dry-run smoke for run $run_id"
./byom-video expand-dry-run "$run_id"
echo "---"
./byom-video expand "$run_id" --dry-run
echo "---"
echo "Optional real Ollama flow:"
echo "  ollama serve"
echo "  ollama pull qwen2.5:7b"
echo "  ./byom-video expand \"$run_id\" --overwrite"
echo "  ./byom-video expansion-validate \"$run_id\""
echo "  ./byom-video verify-expansions \"$run_id\""

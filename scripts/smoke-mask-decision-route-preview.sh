#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f ./byom-video ]]; then
  go build -o byom-video ./cmd/byom-video
fi

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

echo "Running mask-decision/route-preview smoke for run $run_id"

echo "=== mask-decisions ==="
./byom-video mask-decisions "$run_id"
echo "---"
./byom-video mask-decisions "$run_id" --json
echo "---"

# Pick first decision id for targeted commands
first_decision=$(./byom-video mask-decisions "$run_id" --json | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['decisions'][0]['id'])" 2>/dev/null || true)

if [[ -n "$first_decision" ]]; then
  echo "=== mask-decision dry-run ==="
  ./byom-video mask-decision "$run_id" "$first_decision" --set candidate_keep --dry-run
  echo "---"
  ./byom-video mask-decision "$run_id" "$first_decision" --set candidate_keep --dry-run --json
  echo "---"

  echo "=== mask-decision apply ==="
  ./byom-video mask-decision "$run_id" "$first_decision" --set candidate_keep --reason "smoke test"
  echo "---"

  echo "=== mask-snapshots after decision edit ==="
  ./byom-video mask-snapshots "$run_id"
  echo "---"

  echo "=== mask-remove-decision dry-run ==="
  ./byom-video mask-remove-decision "$run_id" "$first_decision" --dry-run
  echo "---"

  echo "=== mask-decisions after edit ==="
  ./byom-video mask-decisions "$run_id"
  echo "---"

  echo "=== mask-reorder dry-run ==="
  all_ids=$(./byom-video mask-decisions "$run_id" --json | python3 -c "import sys,json; d=json.load(sys.stdin); print(','.join(x['id'] for x in reversed(d['decisions'])))" 2>/dev/null || true)
  if [[ -n "$all_ids" ]]; then
    ./byom-video mask-reorder "$run_id" --order "$all_ids" --dry-run
    echo "---"
  fi
else
  echo "No decisions found in mask; skipping decision-level commands."
fi

echo "=== route-preview ==="
./byom-video route-preview "$run_id"
echo "---"
./byom-video route-preview "$run_id" --json
echo "---"
./byom-video route-preview "$run_id" --write-artifact

echo "Mask-decision/route-preview smoke completed for $run_id."

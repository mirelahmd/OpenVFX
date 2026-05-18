#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not found"
  exit 0
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

BINARY="$WORK_DIR/byom-video"
echo "==> Building binary"
GOCACHE="${REPO_ROOT}/.cache/go-build" go build -o "$BINARY" "$REPO_ROOT/cmd/byom-video"

cd "$WORK_DIR"
"$BINARY" init >/dev/null

PASS=0
FAIL=0
pass() { echo "    PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "    FAIL: $1"; FAIL=$((FAIL+1)); }

INPUT_PATH="$REPO_ROOT/media/Untitled.mov"
if [[ ! -f "$INPUT_PATH" ]]; then
  INPUT_PATH="$WORK_DIR/placeholder.mov"
  printf 'placeholder' > "$INPUT_PATH"
fi

echo "==> Create agent plan"
"$BINARY" agent-plan --input "$INPUT_PATH" --goal "make a vertical short with captions" --write-review >/dev/null && pass "agent plan created" || fail "agent plan should create"
PLAN_ID="$("$BINARY" agent-plans --json | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["plan_id"])')"

echo "==> Approve and preview conversion"
"$BINARY" approve-agent-plan "$PLAN_ID" >/dev/null && pass "agent plan approved" || fail "agent plan should approve"
"$BINARY" agent-plan-to-job "$PLAN_ID" --dry-run >/dev/null && pass "conversion dry-run works" || fail "conversion dry-run should work"

echo "==> Convert"
"$BINARY" agent-plan-to-job "$PLAN_ID" --approve-jobs >/dev/null && pass "agent plan converted" || fail "agent plan should convert"
"$BINARY" agent-plan-jobs "$PLAN_ID" >/dev/null && pass "agent-plan-jobs works" || fail "agent-plan-jobs should work"
"$BINARY" inspect-agent-plan "$PLAN_ID" >/dev/null && pass "inspect-agent-plan works" || fail "inspect-agent-plan should work"
"$BINARY" review-agent-plan "$PLAN_ID" --write-artifact >/dev/null && pass "review-agent-plan writes artifact" || fail "review-agent-plan should write artifact"
"$BINARY" jobs >/dev/null && pass "jobs lists created job" || fail "jobs should list created job"

[[ -f ".byom-video/agent_plans/$PLAN_ID/linked_jobs.json" ]] && pass "linked_jobs.json exists" || fail "linked_jobs.json should exist"

STATUS="$(python3 - <<'PY' ".byom-video/agent_plans/$PLAN_ID/agent_plan.json"
import json, sys
with open(sys.argv[1]) as f:
    print(json.load(f)["status"])
PY
)"
[[ "$STATUS" == "converted" ]] && pass "agent plan status converted" || fail "agent plan should be converted"

JOB_COUNT="$(python3 - <<'PY' ".byom-video/agent_plans/$PLAN_ID/linked_jobs.json"
import json, sys
with open(sys.argv[1]) as f:
    print(len(json.load(f)["jobs"]))
PY
)"
[[ "$JOB_COUNT" -ge 1 ]] && pass "linked job count present" || fail "at least one linked job expected"

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

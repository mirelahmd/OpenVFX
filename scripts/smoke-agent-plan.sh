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

echo "==> Queue-health plan"
"$BINARY" agent-plan --goal "queue health" >/dev/null && pass "queue-health plan created" || fail "queue-health plan should create"

INPUT_PATH="$REPO_ROOT/media/Untitled.mov"
if [[ ! -f "$INPUT_PATH" ]]; then
  INPUT_PATH="$WORK_DIR/placeholder.mov"
  printf 'placeholder' > "$INPUT_PATH"
fi

echo "==> Make plan"
"$BINARY" agent-plan --input "$INPUT_PATH" --goal "make a vertical short with captions and narration" --write-review >/dev/null && pass "make plan created" || fail "make plan should create"

PLAN_ID="$("$BINARY" agent-plans --json | python3 -c 'import json,sys; data=json.load(sys.stdin); print(data[0]["plan_id"])')"
"$BINARY" inspect-agent-plan "$PLAN_ID" >/dev/null && pass "inspect-agent-plan works" || fail "inspect-agent-plan should work"
"$BINARY" review-agent-plan "$PLAN_ID" --write-artifact >/dev/null && pass "review-agent-plan writes artifact" || fail "review-agent-plan should write artifact"
"$BINARY" agent-policy "$PLAN_ID" >/dev/null && pass "agent-policy works" || fail "agent-policy should work"

for path in \
  ".byom-video/agent_plans/$PLAN_ID/agent_plan.json" \
  ".byom-video/agent_plans/$PLAN_ID/context_snapshot.json" \
  ".byom-video/agent_plans/$PLAN_ID/policy_review.json" \
  ".byom-video/agent_plans/$PLAN_ID/plan_review.md"
do
  [[ -f "$path" ]] && pass "$(basename "$path") exists" || fail "$(basename "$path") should exist"
done

POLICY_STATUS="$(python3 - <<'PY' ".byom-video/agent_plans/$PLAN_ID/policy_review.json"
import json, sys
with open(sys.argv[1]) as f:
    print(json.load(f)["status"])
PY
)"
[[ "$POLICY_STATUS" == "approval_required" ]] && pass "make plan policy requires approval" || fail "make plan policy should require approval"

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

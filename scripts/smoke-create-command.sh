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

GOAL="Make a 35-second luxury fitness Instagram Reel. Use my talking clip as narration and gym clips as b-roll. Make it cinematic, darker, premium, fast cuts in the first 5 seconds. Generate futuristic gym city b-roll if the backend exists. Use bold lower-third captions. Give me Instagram caption options."

echo "==> Preview create"
"$BINARY" create "$INPUT_PATH" --goal "$GOAL" --write-review --skip-graph >/dev/null && pass "create preview works" || fail "create preview should work"
SESSION_ID="$(python3 - <<'PY'
import json, pathlib
root = pathlib.Path(".byom-video/create_sessions")
sessions = []
for path in root.glob("*/create_session.json"):
    sessions.append(json.loads(path.read_text()))
sessions.sort(key=lambda x: x["created_at"], reverse=True)
print(sessions[0]["create_session_id"])
PY
)"
SESSION_DIR=".byom-video/create_sessions/$SESSION_ID"
PLAN_ID="$(python3 - <<'PY' "$SESSION_DIR/create_session.json"
import json, sys
with open(sys.argv[1]) as f:
    print(json.load(f)["linked"]["agent_plan_id"])
PY
)"
PLAN_DIR=".byom-video/agent_plans/$PLAN_ID"

[[ -f "$SESSION_DIR/create_session.json" ]] && pass "create_session.json exists" || fail "create_session.json missing"
[[ -f "$SESSION_DIR/create_review.md" ]] && pass "create_review.md exists" || fail "create_review.md missing"
[[ -f "$SESSION_DIR/linked_agent_plan.json" ]] && pass "linked_agent_plan.json exists" || fail "linked_agent_plan.json missing"
[[ -f "$PLAN_DIR/creative_brief.json" ]] && pass "creative_brief.json exists" || fail "creative_brief.json missing"
[[ -f "$PLAN_DIR/deliverables.json" ]] && pass "deliverables.json exists" || fail "deliverables.json missing"
[[ -f "$PLAN_DIR/asset_requirements.json" ]] && pass "asset_requirements.json exists" || fail "asset_requirements.json missing"
[[ -f "$PLAN_DIR/visual_requests.dryrun.json" ]] && pass "visual_requests.dryrun.json exists" || fail "visual_requests.dryrun.json missing"

"$BINARY" create-result "$SESSION_ID" --write-artifact >/dev/null && pass "create-result works" || fail "create-result should work"
"$BINARY" create-sessions >/dev/null && pass "create-sessions works" || fail "create-sessions should work"
"$BINARY" inspect-create-session "$SESSION_ID" --json >/dev/null && pass "inspect-create-session json works" || fail "inspect-create-session should work"
"$BINARY" visual-requests "$PLAN_ID" --overwrite --json >/dev/null && pass "visual-requests command works" || fail "visual-requests should work"
grep -q "## Creative Brief" "$SESSION_DIR/create_review.md" && pass "review includes creative brief" || fail "review missing creative brief"
grep -q "## Planned Deliverables" "$SESSION_DIR/create_review.md" && pass "review includes deliverables" || fail "review missing deliverables"
grep -q "## Asset Requirements" "$SESSION_DIR/create_review.md" && pass "review includes asset requirements" || fail "review missing asset requirements"
grep -q "## Visual Generation Dry-Run Requests" "$SESSION_DIR/create_review.md" && pass "review includes visual dry-runs" || fail "review missing visual dry-runs"
grep -q "## Jobs" "$SESSION_DIR/create_review.md" && pass "review includes jobs" || fail "review missing jobs"

echo "==> Convert create"
"$BINARY" create "$INPUT_PATH" --goal "$GOAL" --yes --approval-scope local --convert --approve-jobs --write-review --skip-graph >/dev/null && pass "create conversion works" || fail "create conversion should work"
CONVERTED_SESSION="$(python3 - <<'PY'
import json, pathlib
root = pathlib.Path(".byom-video/create_sessions")
sessions = []
for path in root.glob("*/create_session.json"):
    sessions.append(json.loads(path.read_text()))
sessions.sort(key=lambda x: x["created_at"], reverse=True)
print(sessions[0]["create_session_id"])
PY
)"
CONVERTED_DIR=".byom-video/create_sessions/$CONVERTED_SESSION"
[[ -f "$CONVERTED_DIR/linked_jobs.json" ]] && pass "linked_jobs.json exists" || fail "linked_jobs.json missing"
STATUS="$(python3 - <<'PY' "$CONVERTED_DIR/create_session.json"
import json, sys
with open(sys.argv[1]) as f:
    print(json.load(f)["status"])
PY
)"
[[ "$STATUS" == "converted" ]] && pass "session converted" || fail "session should be converted, got $STATUS"

JOB_COUNT="$(python3 - <<'PY' "$CONVERTED_DIR/linked_jobs.json"
import json, sys
with open(sys.argv[1]) as f:
    print(len(json.load(f)["jobs"]))
PY
)"
[[ "$JOB_COUNT" -ge 1 ]] && pass "jobs created" || fail "expected at least one job"

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

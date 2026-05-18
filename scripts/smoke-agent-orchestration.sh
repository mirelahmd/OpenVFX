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

echo "==> Create rich agent plan"
"$BINARY" agent-plan --input "$INPUT_PATH" --goal "$GOAL" --write-review >/dev/null && pass "agent plan created" || fail "agent plan should create"
PLAN_ID="$("$BINARY" agent-plans --json | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["plan_id"])')"
PLAN_DIR=".byom-video/agent_plans/$PLAN_ID"

[[ -f "$PLAN_DIR/creative_brief.json" ]] && pass "creative_brief.json exists" || fail "creative_brief.json missing"
[[ -f "$PLAN_DIR/deliverables.json" ]] && pass "deliverables.json exists" || fail "deliverables.json missing"
[[ -f "$PLAN_DIR/asset_requirements.json" ]] && pass "asset_requirements.json exists" || fail "asset_requirements.json missing"

python3 - <<'PY' "$PLAN_DIR/creative_brief.json" && pass "creative brief parsed rich fields" || fail "creative brief missing expected fields"
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
assert data["duration"]["target_seconds"] == 35
assert data["platform"] == "instagram-reel"
assert data["captions"]["style"] == "bold"
assert data["captions"]["position"] == "lower-third"
assert data["requests"]["generated_broll"] is True
assert data["requests"]["instagram_captions"] is True
PY

python3 - <<'PY' "$PLAN_DIR/deliverables.json" && pass "deliverables include caption options" || fail "caption options deliverable missing"
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
assert any(d["type"] == "social_caption_options" for d in data["deliverables"])
PY

python3 - <<'PY' "$PLAN_DIR/asset_requirements.json" && pass "asset requirements include b-roll" || fail "b-roll requirement missing"
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
assert any(r["kind"] == "generated_broll" for r in data["requirements"])
PY

"$BINARY" inspect-agent-plan "$PLAN_ID" >/dev/null && pass "inspect-agent-plan works" || fail "inspect-agent-plan should work"
"$BINARY" review-agent-plan "$PLAN_ID" --write-artifact >/dev/null && pass "review-agent-plan works" || fail "review-agent-plan should work"

echo "==> Orchestrate skip-graph path"
"$BINARY" agent-orchestrate --input "$INPUT_PATH" --goal "$GOAL" --skip-graph >/dev/null && pass "agent-orchestrate skip graph works" || fail "agent-orchestrate skip graph should work"

echo "==> Optional LangGraph live path"
if BYOM_VIDEO_PYTHON=python3 "$BINARY" agent-orchestrate --input "$INPUT_PATH" --goal "$GOAL" --workers-dir "$REPO_ROOT/workers" >/dev/null 2>&1; then
  pass "agent-orchestrate live graph works"
else
  echo "    SKIP: live agent-orchestrate graph unavailable"
fi
if BYOM_VIDEO_PYTHON=python3 "$BINARY" agent-graph-run "$PLAN_ID" --workers-dir "$REPO_ROOT/workers" --json >/dev/null 2>&1; then
  pass "agent-graph-run live works"
  [[ -f "$PLAN_DIR/agent_decision.json" ]] && pass "agent_decision.json exists" || fail "agent_decision.json missing"
  [[ -f "$PLAN_DIR/graph_trace.json" ]] && pass "graph_trace.json exists" || fail "graph_trace.json missing"
else
  echo "    SKIP: live LangGraph sidecar unavailable"
fi

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

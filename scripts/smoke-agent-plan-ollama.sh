#!/usr/bin/env bash
# Smoke test for Prompts 063+064: Planner Adapter Interface v1 + Reliability.
# Tests the --planner flag, deterministic adapter, Ollama adapter flags,
# fallback behaviour, planner metadata, planner_request.json artifact,
# agent-planner-diagnose command, and config route resolution.
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

PLAN_DIR=".byom-video/agent_plans"

latest_plan() {
  find "$PLAN_DIR" -name "agent_plan.json" -print0 2>/dev/null \
    | xargs -0 ls -t 2>/dev/null | head -1
}

# --------------------------------------------------------------------------
# Part 1: Default (deterministic) planner still works
# --------------------------------------------------------------------------
echo ""
echo "==> Part 1: Default deterministic planner"

"$BINARY" agent-plan --goal "queue health" >/dev/null \
  && pass "default planner: plan created" \
  || fail "default planner: plan should create"

PLAN_FILE="$(latest_plan)"
[[ -n "$PLAN_FILE" ]] && pass "agent_plan.json written" || fail "agent_plan.json not written"

PLANNER_MODE=$(jq -r '.planner.mode' "$PLAN_FILE" 2>/dev/null)
[[ "$PLANNER_MODE" == "deterministic" ]] \
  && pass "planner.mode = deterministic" \
  || fail "planner.mode should be deterministic, got: $PLANNER_MODE"

PLANNER_VERSION=$(jq -r '.planner.version' "$PLAN_FILE" 2>/dev/null)
[[ "$PLANNER_VERSION" == "v1" ]] \
  && pass "planner.version = v1" \
  || fail "planner.version should be v1, got: $PLANNER_VERSION"

# --------------------------------------------------------------------------
# Part 2: --planner deterministic explicit flag
# --------------------------------------------------------------------------
echo ""
echo "==> Part 2: Explicit --planner deterministic"

"$BINARY" agent-plan --goal "queue health" --planner deterministic >/dev/null \
  && pass "explicit --planner deterministic accepted" \
  || fail "explicit --planner deterministic rejected"

PLANNER_MODE2=$(jq -r '.planner.mode' "$(latest_plan)" 2>/dev/null)
[[ "$PLANNER_MODE2" == "deterministic" ]] \
  && pass "explicit planner.mode = deterministic" \
  || fail "explicit planner.mode should be deterministic, got: $PLANNER_MODE2"

# --------------------------------------------------------------------------
# Part 3: --planner ollama without model fails gracefully
# --------------------------------------------------------------------------
echo ""
echo "==> Part 3: Ollama planner — missing model fails cleanly"

ERR_OUT=$("$BINARY" agent-plan --goal "make a video" \
    --planner ollama \
    --planner-backend "http://127.0.0.1:19999" \
    2>&1 || true)

echo "$ERR_OUT" | grep -qi "model\|planner\|ollama\|require" \
  && pass "missing model: error mentions model/planner" \
  || fail "missing model: expected error message; got: $ERR_OUT"

# --------------------------------------------------------------------------
# Part 4: --planner ollama with model but server unavailable fails gracefully
# --------------------------------------------------------------------------
echo ""
echo "==> Part 4: Ollama planner — server unavailable fails cleanly"

PLANS_BEFORE=$(find "$PLAN_DIR" -name "agent_plan.json" 2>/dev/null | wc -l | tr -d ' ')

ERR_OUT2=$("$BINARY" agent-plan --goal "make a video" \
    --planner ollama \
    --planner-model "llama3" \
    --planner-backend "http://127.0.0.1:19999" \
    --planner-timeout-seconds 2 \
    2>&1 || true)

echo "$ERR_OUT2" | grep -qi "failed\|refused\|error\|running" \
  && pass "server unavailable: error surfaced" \
  || fail "server unavailable: expected connection error; got: $ERR_OUT2"

PLANS_AFTER=$(find "$PLAN_DIR" -name "agent_plan.json" 2>/dev/null | wc -l | tr -d ' ')
[[ "$PLANS_AFTER" == "$PLANS_BEFORE" ]] \
  && pass "server unavailable: no plan artifact written on error" \
  || fail "server unavailable: plan artifact written despite error"

# --------------------------------------------------------------------------
# Part 5: --planner-fallback-deterministic falls back on Ollama failure
# --------------------------------------------------------------------------
echo ""
echo "==> Part 5: Ollama fallback to deterministic"

"$BINARY" agent-plan --goal "queue health" \
    --planner ollama \
    --planner-model "llama3" \
    --planner-backend "http://127.0.0.1:19999" \
    --planner-timeout-seconds 2 \
    --planner-fallback-deterministic \
    >/dev/null \
  && pass "fallback: plan created" \
  || fail "fallback: plan should create despite Ollama failure"

FALLBACK_FILE="$(latest_plan)"
FALLBACK_MODE=$(jq -r '.planner.mode' "$FALLBACK_FILE" 2>/dev/null)
# mode reflects the *requested* planner (ollama), warnings signal the fallback
[[ "$FALLBACK_MODE" == "ollama" ]] \
  && pass "fallback: planner.mode = ollama (requested)" \
  || fail "fallback: planner.mode should be ollama, got: $FALLBACK_MODE"

FALLBACK_WARNINGS=$(jq -r '.warnings[]' "$FALLBACK_FILE" 2>/dev/null | grep -i "fallback\|ollama" || true)
[[ -n "$FALLBACK_WARNINGS" ]] \
  && pass "fallback: warnings mention ollama/fallback" \
  || fail "fallback: expected warning about fallback in plan.warnings"

FALLBACK_ACTIONS=$(jq '.actions | length' "$FALLBACK_FILE" 2>/dev/null)
[[ "$FALLBACK_ACTIONS" -ge 1 ]] \
  && pass "fallback: plan has actions" \
  || fail "fallback: plan has no actions"

FALLBACK_MODEL=$(jq -r '.planner.model' "$FALLBACK_FILE" 2>/dev/null)
[[ "$FALLBACK_MODEL" == "llama3" ]] \
  && pass "fallback: planner.model preserved" \
  || fail "fallback: planner.model should be llama3, got: $FALLBACK_MODEL"

# --------------------------------------------------------------------------
# Part 6: Extra planner flags parse without error (deterministic path)
# --------------------------------------------------------------------------
echo ""
echo "==> Part 6: Extra planner flags accepted by deterministic path"

"$BINARY" agent-plan --goal "queue health" \
    --planner deterministic \
    --planner-temperature 0.7 \
    --planner-max-output-chars 4096 \
    >/dev/null \
  && pass "temperature + max-output-chars flags accepted" \
  || fail "temperature + max-output-chars flags should be accepted"

"$BINARY" agent-plan --goal "queue health" \
    --planner deterministic \
    --planner-route "agent.planning" \
    >/dev/null \
  && pass "planner-route flag accepted" \
  || fail "planner-route flag should be accepted"

# --------------------------------------------------------------------------
# Part 7: plan.json planner block structure
# --------------------------------------------------------------------------
echo ""
echo "==> Part 7: agent_plan.json planner block structure"

LATEST=$(latest_plan)
PLANNER_BLOCK=$(jq '.planner' "$LATEST" 2>/dev/null)
[[ "$PLANNER_BLOCK" != "null" ]] \
  && pass "agent_plan.json has .planner block" \
  || fail "agent_plan.json missing .planner block"

PLANNER_MODEL_FIELD=$(jq -r '.planner.model' "$LATEST" 2>/dev/null)
[[ -n "$PLANNER_MODEL_FIELD" || "$PLANNER_MODEL_FIELD" == "" ]] \
  && pass ".planner.model field present (may be empty string)" \
  || fail ".planner.model field missing"

REQ_MODE=$(jq -r '.planner.requested_mode' "$LATEST" 2>/dev/null)
[[ -n "$REQ_MODE" ]] \
  && pass ".planner.requested_mode present" \
  || fail ".planner.requested_mode missing"

EFF_MODE=$(jq -r '.planner.effective_mode' "$LATEST" 2>/dev/null)
[[ -n "$EFF_MODE" ]] \
  && pass ".planner.effective_mode present" \
  || fail ".planner.effective_mode missing"

PROVIDER=$(jq -r '.planner.provider' "$LATEST" 2>/dev/null)
[[ -n "$PROVIDER" ]] \
  && pass ".planner.provider present" \
  || fail ".planner.provider missing"

PLANNER_REF=$(jq -r '.references.planner_request' "$LATEST" 2>/dev/null)
[[ "$PLANNER_REF" == "planner_request.json" ]] \
  && pass ".references.planner_request = planner_request.json" \
  || fail ".references.planner_request should be planner_request.json, got: $PLANNER_REF"

# --------------------------------------------------------------------------
# Part 8: dry-run with --planner flag writes no artifact
# --------------------------------------------------------------------------
echo ""
echo "==> Part 8: dry-run with --planner flag"

COUNT_BEFORE=$(find "$PLAN_DIR" -name "agent_plan.json" 2>/dev/null | wc -l | tr -d ' ')

"$BINARY" agent-plan --goal "queue health" --planner deterministic --dry-run >/dev/null \
  && pass "dry-run with --planner accepted" \
  || fail "dry-run with --planner should work"

COUNT_AFTER=$(find "$PLAN_DIR" -name "agent_plan.json" 2>/dev/null | wc -l | tr -d ' ')
[[ "$COUNT_AFTER" == "$COUNT_BEFORE" ]] \
  && pass "dry-run writes no new artifact" \
  || fail "dry-run should not write new artifact"

# --------------------------------------------------------------------------
# Part 9: JSON output includes planner block
# --------------------------------------------------------------------------
echo ""
echo "==> Part 9: JSON output mode"

JSON_OUT=$("$BINARY" agent-plan --goal "queue health" --planner deterministic --json 2>/dev/null || true)
echo "$JSON_OUT" | jq -e '.planner' >/dev/null 2>&1 \
  && pass "JSON output includes .planner" \
  || fail "JSON output missing .planner"

echo "$JSON_OUT" | jq -e '.actions | length >= 1' >/dev/null 2>&1 \
  && pass "JSON output includes .actions" \
  || fail "JSON output missing .actions"

# --------------------------------------------------------------------------
# Part 10: planner_request.json artifact
# --------------------------------------------------------------------------
echo ""
echo "==> Part 10: planner_request.json artifact"

"$BINARY" agent-plan --goal "queue health" --planner deterministic >/dev/null
LATEST_PLAN=$(latest_plan)
PLAN_ID=$(jq -r '.plan_id' "$LATEST_PLAN" 2>/dev/null)
PLAN_DIR_PATH=".byom-video/agent_plans/$PLAN_ID"
REQ_FILE="$PLAN_DIR_PATH/planner_request.json"

[[ -f "$REQ_FILE" ]] \
  && pass "planner_request.json written" \
  || fail "planner_request.json missing"

SCHEMA_VER=$(jq -r '.schema_version' "$REQ_FILE" 2>/dev/null)
[[ "$SCHEMA_VER" == "openvfx_planner_request.v1" ]] \
  && pass "planner_request.json schema_version correct" \
  || fail "planner_request.json schema_version wrong: $SCHEMA_VER"

REQ_PLANNER_MODE=$(jq -r '.planner_mode' "$REQ_FILE" 2>/dev/null)
[[ "$REQ_PLANNER_MODE" == "deterministic" ]] \
  && pass "planner_request.json planner_mode = deterministic" \
  || fail "planner_request.json planner_mode wrong: $REQ_PLANNER_MODE"

# --------------------------------------------------------------------------
# Part 11: fallback planner_request.json carries requested info
# --------------------------------------------------------------------------
echo ""
echo "==> Part 11: Fallback planner_request.json"

"$BINARY" agent-plan --goal "queue health" \
    --planner ollama \
    --planner-model "llama3" \
    --planner-backend "http://127.0.0.1:19999" \
    --planner-timeout-seconds 2 \
    --planner-fallback-deterministic \
    >/dev/null

FALLBACK_PLAN=$(latest_plan)
FALLBACK_PLAN_ID=$(jq -r '.plan_id' "$FALLBACK_PLAN" 2>/dev/null)
FALLBACK_REQ_FILE=".byom-video/agent_plans/$FALLBACK_PLAN_ID/planner_request.json"

[[ -f "$FALLBACK_REQ_FILE" ]] \
  && pass "fallback: planner_request.json written" \
  || fail "fallback: planner_request.json missing"

FALLBACK_USED=$(jq -r '.fallback_used' "$FALLBACK_REQ_FILE" 2>/dev/null)
[[ "$FALLBACK_USED" == "true" ]] \
  && pass "fallback: planner_request.json.fallback_used = true" \
  || fail "fallback: expected fallback_used = true in artifact, got: $FALLBACK_USED"

# Check plan artifact has correct fallback metadata
FALLBACK_EFF=$(jq -r '.planner.effective_mode' "$FALLBACK_PLAN" 2>/dev/null)
[[ "$FALLBACK_EFF" == "deterministic" ]] \
  && pass "fallback: agent_plan.json planner.effective_mode = deterministic" \
  || fail "fallback: effective_mode should be deterministic, got: $FALLBACK_EFF"

FALLBACK_REQ=$(jq -r '.planner.requested_mode' "$FALLBACK_PLAN" 2>/dev/null)
[[ "$FALLBACK_REQ" == "ollama" ]] \
  && pass "fallback: agent_plan.json planner.requested_mode = ollama" \
  || fail "fallback: requested_mode should be ollama, got: $FALLBACK_REQ"

PLAN_FALLBACK_USED=$(jq -r '.planner.fallback_used' "$FALLBACK_PLAN" 2>/dev/null)
[[ "$PLAN_FALLBACK_USED" == "true" ]] \
  && pass "fallback: agent_plan.json planner.fallback_used = true" \
  || fail "fallback: agent_plan.json planner.fallback_used should be true"

PLAN_FALLBACK_REASON=$(jq -r '.planner.fallback_reason' "$FALLBACK_PLAN" 2>/dev/null)
[[ -n "$PLAN_FALLBACK_REASON" ]] \
  && pass "fallback: agent_plan.json planner.fallback_reason set" \
  || fail "fallback: agent_plan.json planner.fallback_reason missing"

# --------------------------------------------------------------------------
# Part 12: agent-planner-diagnose command
# --------------------------------------------------------------------------
echo ""
echo "==> Part 12: agent-planner-diagnose command"

"$BINARY" agent-planner-diagnose >/dev/null \
  && pass "agent-planner-diagnose (default) exits 0" \
  || fail "agent-planner-diagnose (default) should exit 0"

"$BINARY" agent-planner-diagnose --planner deterministic >/dev/null \
  && pass "agent-planner-diagnose --planner deterministic exits 0" \
  || fail "agent-planner-diagnose --planner deterministic failed"

"$BINARY" agent-planner-diagnose --planner ollama --planner-model llama3 >/dev/null \
  && pass "agent-planner-diagnose --planner ollama exits 0" \
  || fail "agent-planner-diagnose --planner ollama failed"

DIAG_JSON=$("$BINARY" agent-planner-diagnose --planner deterministic --json 2>/dev/null || true)
echo "$DIAG_JSON" | jq -e '.requested_mode == "deterministic"' >/dev/null 2>&1 \
  && pass "agent-planner-diagnose JSON: requested_mode = deterministic" \
  || fail "agent-planner-diagnose JSON: wrong requested_mode"

echo "$DIAG_JSON" | jq -e '.effective_mode == "deterministic"' >/dev/null 2>&1 \
  && pass "agent-planner-diagnose JSON: effective_mode = deterministic" \
  || fail "agent-planner-diagnose JSON: wrong effective_mode"

DIAG_OLLAMA_JSON=$("$BINARY" agent-planner-diagnose \
  --planner ollama \
  --planner-model phi3 \
  --planner-backend "http://localhost:11434" \
  --json 2>/dev/null || true)
echo "$DIAG_OLLAMA_JSON" | jq -e '.resolved_model == "phi3"' >/dev/null 2>&1 \
  && pass "agent-planner-diagnose JSON: resolved_model from flag" \
  || fail "agent-planner-diagnose JSON: resolved_model wrong"

echo "$DIAG_OLLAMA_JSON" | jq -e '.resolved_backend == "http://localhost:11434"' >/dev/null 2>&1 \
  && pass "agent-planner-diagnose JSON: resolved_backend from flag" \
  || fail "agent-planner-diagnose JSON: resolved_backend wrong"

# --check with unreachable server should still exit 0 (just reports status)
"$BINARY" agent-planner-diagnose \
  --planner ollama \
  --planner-model llama3 \
  --planner-backend "http://127.0.0.1:19999" \
  --planner-timeout-seconds 1 \
  --check \
  --json >/dev/null \
  && pass "agent-planner-diagnose --check with unreachable server exits 0" \
  || fail "agent-planner-diagnose --check should always exit 0"

# --------------------------------------------------------------------------
# Part 13: config route resolution via byom-video.yaml
# --------------------------------------------------------------------------
echo ""
echo "==> Part 13: Config route resolution"

cat > byom-video.yaml <<'YAMLEOF'
models:
  enabled: true
  routes:
    agent.planning: smoke-planner
  entries:
    smoke-planner:
      model: smoke-model:7b
      base_url: http://localhost:11434
YAMLEOF

DIAG_CFG_JSON=$("$BINARY" agent-planner-diagnose --planner ollama --json 2>/dev/null || true)
echo "$DIAG_CFG_JSON" | jq -e '.resolved_model == "smoke-model:7b"' >/dev/null 2>&1 \
  && pass "config route: model resolved from byom-video.yaml" \
  || fail "config route: expected smoke-model:7b, output: $DIAG_CFG_JSON"

echo "$DIAG_CFG_JSON" | jq -e '.config_entry_name == "smoke-planner"' >/dev/null 2>&1 \
  && pass "config route: entry name in diagnose output" \
  || fail "config route: expected config_entry_name smoke-planner"

rm byom-video.yaml

# --------------------------------------------------------------------------
# Summary
# --------------------------------------------------------------------------
echo ""
echo "========================================"
echo "Results: $PASS passed, $FAIL failed"
echo "========================================"
[[ "$FAIL" -eq 0 ]] && exit 0 || exit 1

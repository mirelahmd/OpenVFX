#!/usr/bin/env bash
# Smoke test for Prompt 065: LangGraph Agent Sidecar v1.
# Tests dry-run CLI, Python sidecar integration, artifact output,
# error handling (missing plan, missing Python), and workers-dir flag.
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

# Create a plan we can run the graph against.
"$BINARY" agent-plan --goal "queue health" >/dev/null
PLAN_FILE="$(find "$PLAN_DIR" -name "agent_plan.json" -print0 2>/dev/null | xargs -0 ls -t 2>/dev/null | head -1)"
PLAN_ID="$(jq -r '.plan_id' "$PLAN_FILE" 2>/dev/null)"
PLAN_SUBDIR="$(dirname "$PLAN_FILE")"

echo ""
echo "==> Part 1: agent-graph-run dry-run (human)"

DRYRUN_OUT="$("$BINARY" agent-graph-run "$PLAN_ID" --dry-run 2>&1)"
echo "$DRYRUN_OUT" | grep -q "dry-run" \
  && pass "dry-run: header contains dry-run" \
  || fail "dry-run: missing dry-run indicator"

echo "$DRYRUN_OUT" | grep -q "$PLAN_ID" \
  && pass "dry-run: plan id shown" \
  || fail "dry-run: plan id not shown"

echo "$DRYRUN_OUT" | grep -q "openvfx_agent_graph" \
  && pass "dry-run: sidecar module shown" \
  || fail "dry-run: sidecar module not shown"

echo ""
echo "==> Part 2: agent-graph-run dry-run (JSON)"

DRY_JSON="$("$BINARY" agent-graph-run "$PLAN_ID" --dry-run --json 2>&1)"
echo "$DRY_JSON" | python3 -c "import sys,json; json.load(sys.stdin)" \
  && pass "dry-run json: valid JSON" \
  || fail "dry-run json: not valid JSON"

echo "$DRY_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['plan_id'] == '${PLAN_ID}'" \
  && pass "dry-run json: plan_id correct" \
  || fail "dry-run json: plan_id wrong"

echo "$DRY_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['dry_run'] == True" \
  && pass "dry-run json: dry_run=true" \
  || fail "dry-run json: dry_run not true"

echo "$DRY_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'run_id' in d" \
  && pass "dry-run json: run_id present" \
  || fail "dry-run json: run_id missing"

echo ""
echo "==> Part 3: agent-graph-run error handling"

# Capture output separately to avoid pipefail swallowing grep's exit status.
MISSING_OUT="$("$BINARY" agent-graph-run 2>&1 || true)"
echo "$MISSING_OUT" | grep -qi "requires a <plan_id>" \
  && pass "missing plan_id: helpful error" \
  || fail "missing plan_id: no error"

NOTFOUND_OUT="$("$BINARY" agent-graph-run "plan-does-not-exist-xyz" 2>&1 || true)"
echo "$NOTFOUND_OUT" | grep -qi "not found" \
  && pass "nonexistent plan: not found error" \
  || fail "nonexistent plan: no error"

echo ""
echo "==> Part 4: agent-graph-run --workers-dir passthrough (dry-run)"

WDIR_OUT="$("$BINARY" agent-graph-run "$PLAN_ID" --dry-run --workers-dir "/tmp/fake-workers" 2>&1)"
echo "$WDIR_OUT" | grep -q "dry-run" \
  && pass "--workers-dir: dry-run still works" \
  || fail "--workers-dir: dry-run failed"

UNKNOWN_OUT="$("$BINARY" agent-graph-run "$PLAN_ID" --unknown-flag 2>&1 || true)"
echo "$UNKNOWN_OUT" | grep -qi "unknown" \
  && pass "unknown flag: error reported" \
  || fail "unknown flag: no error"

echo ""
echo "==> Part 5: Python sidecar (if python3 and langgraph available)"

if ! command -v python3 >/dev/null 2>&1; then
  echo "    SKIP: python3 not found — skipping live sidecar tests"
else
  # Check if langgraph is available.
  if python3 -c "import langgraph" 2>/dev/null; then
    HAS_LANGGRAPH=1
  else
    HAS_LANGGRAPH=0
  fi

  if [[ "$HAS_LANGGRAPH" == "1" ]]; then
    # Write policy_review.json so the graph can run.
    echo '{"status":"allowed","blocks":[]}' > "$PLAN_SUBDIR/policy_review.json"

    WORKERS_DIR="${REPO_ROOT}/workers"
    # Force system python3 — smoke runs in a temp dir without a local .venv.
    LIVE_OUT="$(BYOM_VIDEO_PYTHON=python3 "$BINARY" agent-graph-run "$PLAN_ID" \
      --workers-dir "$WORKERS_DIR" --json 2>&1 || true)"

    echo "$LIVE_OUT" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null \
      && pass "live sidecar: JSON output" \
      || fail "live sidecar: not valid JSON (output: $LIVE_OUT)"

    echo "$LIVE_OUT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['decision'] in ('approve','flag','repair','reject')" 2>/dev/null \
      && pass "live sidecar: decision field present" \
      || fail "live sidecar: decision field missing or invalid"

    echo "$LIVE_OUT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['plan_id'] == '${PLAN_ID}'" 2>/dev/null \
      && pass "live sidecar: plan_id matches" \
      || fail "live sidecar: plan_id mismatch"

    echo "$LIVE_OUT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['run_id'].startswith('graphrun-')" 2>/dev/null \
      && pass "live sidecar: run_id starts with graphrun-" \
      || fail "live sidecar: run_id wrong prefix"

    echo "$LIVE_OUT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'policy_status' in d" 2>/dev/null \
      && pass "live sidecar: policy_status field present" \
      || fail "live sidecar: policy_status missing"

    [[ -f "$PLAN_SUBDIR/graph_trace.json" ]] \
      && pass "live sidecar: graph_trace.json written" \
      || fail "live sidecar: graph_trace.json not written"

    [[ -f "$PLAN_SUBDIR/agent_decision.json" ]] \
      && pass "live sidecar: agent_decision.json written" \
      || fail "live sidecar: agent_decision.json not written"

    TRACE_SCHEMA="$(jq -r '.schema_version' "$PLAN_SUBDIR/graph_trace.json" 2>/dev/null || true)"
    [[ "$TRACE_SCHEMA" == "openvfx_graph_trace.v1" ]] \
      && pass "graph_trace.json: schema_version correct" \
      || fail "graph_trace.json: schema_version wrong: $TRACE_SCHEMA"

    DECISION_SCHEMA="$(jq -r '.schema_version' "$PLAN_SUBDIR/agent_decision.json" 2>/dev/null || true)"
    [[ "$DECISION_SCHEMA" == "openvfx_agent_decision.v1" ]] \
      && pass "agent_decision.json: schema_version correct" \
      || fail "agent_decision.json: schema_version wrong: $DECISION_SCHEMA"

    # Human output mode.
    LIVE_HUMAN="$(BYOM_VIDEO_PYTHON=python3 "$BINARY" agent-graph-run "$PLAN_ID" \
      --workers-dir "$WORKERS_DIR" 2>&1 || true)"
    echo "$LIVE_HUMAN" | grep -q "Agent graph run" \
      && pass "live sidecar human: header present" \
      || fail "live sidecar human: header missing"
  else
    echo "    SKIP: langgraph not installed — skipping live sidecar tests"
    echo "    Install with: pip install -e '${REPO_ROOT}/workers[graph]'"
  fi
fi

echo ""
echo "==> Part 6: Python package unit tests"

if ! command -v python3 >/dev/null 2>&1; then
  echo "    SKIP: python3 not found"
else
  if python3 -m pytest --version >/dev/null 2>&1; then
    WORKERS_DIR="${REPO_ROOT}/workers"
    PYTEST_OUT="$(python3 -m pytest "$WORKERS_DIR/openvfx_agent_graph/tests/" -q \
        --tb=short 2>&1 || true)"
    echo "$PYTEST_OUT" | tail -5 | grep -q "passed" \
      && pass "Python unit tests: all pass" \
      || fail "Python unit tests: failures detected ($(echo "$PYTEST_OUT" | tail -3))"
  else
    echo "    SKIP: pytest not available"
  fi
fi

echo ""
echo "============================================================"
echo "  PASS: $PASS    FAIL: $FAIL"
echo "============================================================"

[[ "$FAIL" -eq 0 ]]

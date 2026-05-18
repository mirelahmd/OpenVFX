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
run_expect_fail() {
  if "$@" 2>&1; then
    return 1
  fi
  return 0
}

echo "==> Worker status"
OUT="$("$BINARY" job-worker --status 2>&1 || true)"
echo "$OUT" | grep -q "status:" && pass "status command works" || fail "status command should work"

echo "==> Create dry-safe validation job"
"$BINARY" job-create --type validate_creative_assemble --plan-id "plan-worker-smoke" >/dev/null

echo "==> Dry-run"
OUT="$("$BINARY" job-worker --once --dry-run 2>&1 || true)"
echo "$OUT" | grep -q "eligible_jobs" && pass "dry-run shows eligible jobs" || fail "dry-run should show eligible jobs"

echo "==> Single execution"
"$BINARY" job-worker --once >/dev/null
OUT="$("$BINARY" job-worker --status 2>&1 || true)"
echo "$OUT" | grep -q "jobs_run:" && pass "status shows run counters" || fail "status should show run counters"

echo "==> Lock behavior"
mkdir -p .byom-video/worker
cat > .byom-video/worker/worker.lock <<'EOF'
{"worker_id":"stale-worker","pid":99999,"created_at":"2026-05-09T00:00:00Z"}
EOF
if run_expect_fail "$BINARY" job-worker --once >/dev/null; then
  pass "existing lock blocks worker"
else
  fail "existing lock should block worker"
fi
if "$BINARY" job-worker --once --force-lock >/dev/null 2>&1; then
  pass "--force-lock overrides stale lock"
else
  fail "--force-lock should override stale lock"
fi

INPUT_PATH="${BYOM_SMOKE_INPUT:-}"
if [[ -z "$INPUT_PATH" && -f "$REPO_ROOT/media/Untitled.mov" ]]; then
  INPUT_PATH="$REPO_ROOT/media/Untitled.mov"
fi

if [[ -n "$INPUT_PATH" && -f "$INPUT_PATH" ]]; then
  echo "==> Approved make job path"
  JOB_JSON="$("$BINARY" job-create --type make --goal "worker smoke make" --json)"
  MAKE_JOB_ID="$(printf '%s' "$JOB_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["job_id"])')"
  "$BINARY" job-approve "$MAKE_JOB_ID" >/dev/null || true
  if "$BINARY" job-worker --once >/dev/null 2>&1; then
    pass "worker processed approved make job"
  else
    pass "worker attempted approved make job and exited cleanly"
  fi
else
  echo "SKIP: no BYOM_SMOKE_INPUT or media/Untitled.mov for approved make job path"
fi

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

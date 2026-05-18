#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not found"
  exit 0
fi

WORK_DIR="$(mktemp -d)"
trap 'cd / && if [[ -x "${BINARY:-}" ]]; then "$BINARY" daemon stop --force >/dev/null 2>&1 || true; fi; rm -rf "$WORK_DIR"' EXIT

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

echo "==> Initial status and logs"
"$BINARY" daemon status >/dev/null && pass "daemon status works before start" || fail "daemon status should work before start"
"$BINARY" daemon logs >/dev/null && pass "daemon logs works before log file exists" || fail "daemon logs should work before log file exists"

echo "==> Start daemon"
"$BINARY" daemon start --interval 2s --reset-log >/dev/null
"$BINARY" daemon status >/dev/null && pass "daemon status works after start" || fail "daemon status should work after start"
"$BINARY" daemon logs --lines 20 >/dev/null && pass "daemon logs reads log after start" || fail "daemon logs should read log after start"

echo "==> Create safe validation job"
"$BINARY" job-create --type validate_creative_assemble --plan-id "plan-daemon-smoke" >/dev/null
sleep 3
if "$BINARY" job-worker --status >/dev/null 2>&1; then
  pass "worker status accessible while daemon running"
else
  fail "worker status should be accessible while daemon running"
fi

echo "==> Stop daemon"
"$BINARY" daemon stop >/dev/null && pass "daemon stop succeeds" || fail "daemon stop should succeed"
"$BINARY" daemon status >/dev/null && pass "daemon status works after stop" || fail "daemon status should work after stop"

echo "==> Stale pid handling"
mkdir -p .byom-video/daemon
echo "999999" > .byom-video/daemon/daemon.pid
if run_expect_fail "$BINARY" daemon start --interval 2s >/dev/null; then
  pass "stale pid blocks start without force"
else
  fail "stale pid should block start without force"
fi
"$BINARY" daemon start --interval 2s --force >/dev/null && pass "daemon start --force clears stale pid" || fail "daemon start --force should clear stale pid"
"$BINARY" daemon stop --force >/dev/null 2>&1 || true

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

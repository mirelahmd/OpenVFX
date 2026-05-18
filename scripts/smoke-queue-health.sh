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

echo "==> Baseline queue commands"
"$BINARY" queue >/dev/null && pass "queue works" || fail "queue should work"
"$BINARY" queue --json >/dev/null && pass "queue --json works" || fail "queue --json should work"
"$BINARY" queue health >/dev/null && pass "queue health works" || fail "queue health should work"
"$BINARY" queue health --json >/dev/null && pass "queue health --json works" || fail "queue health --json should work"
"$BINARY" queue health --write-report >/dev/null && pass "queue health writes report" || fail "queue health --write-report should work"

echo "==> Create jobs"
"$BINARY" job-create --type validate_creative_assemble --plan-id "plan-queue-approved" >/dev/null
MAKE_INPUT="${BYOM_SMOKE_INPUT:-}"
if [[ -z "$MAKE_INPUT" && -f "$REPO_ROOT/media/Untitled.mov" ]]; then
  MAKE_INPUT="$REPO_ROOT/media/Untitled.mov"
fi
if [[ -n "$MAKE_INPUT" && -f "$MAKE_INPUT" ]]; then
  "$BINARY" job-create --type make --goal "queue smoke make" >/dev/null
else
  "$BINARY" job-create --type revise_make --make-id "mk_queue_smoke" --request "make it shorter" >/dev/null
fi

echo "==> Queue attention view"
OUT="$("$BINARY" queue 2>&1 || true)"
echo "$OUT" | grep -q "approval-needed" && pass "queue shows approval-needed section" || fail "queue should show approval-needed section"
echo "$OUT" | grep -q "pending:" && pass "queue shows counts" || fail "queue should show counts"

OUT_JSON="$("$BINARY" queue --json)"
python3 - <<'PY' "$OUT_JSON"
import json, sys
payload = json.loads(sys.argv[1])
assert payload["jobs"]["total"] >= 2
assert len(payload["jobs"]["approval_needed"]) >= 1
PY
pass "queue json summary includes jobs and approval-needed"

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

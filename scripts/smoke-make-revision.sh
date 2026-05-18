#!/usr/bin/env bash
# Smoke test for Creative Revision Loop v1 (Prompt 055).
# Tests: revise-make dry-run, planned mode, execution with fake deps (via --fallback-stub
#        or pure local actions), make-revisions, inspect-make-revision, review-make-revision,
#        make-result revision fields, provider-call guardrails.
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
go build -o "$BINARY" "$REPO_ROOT/cmd/byom-video"

echo "==> Setting up workspace"
cd "$WORK_DIR"
"$BINARY" init

PASS=0
FAIL=0

pass() { echo "    PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "    FAIL: $1"; FAIL=$((FAIL+1)); }

check_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then
    pass "$label"
  else
    fail "$label (expected '$needle' in output)"
    echo "      output: $(echo "$haystack" | head -5)"
  fi
}

check_not_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then
    fail "$label (unexpected '$needle' in output)"
    echo "      output: $(echo "$haystack" | head -5)"
  else
    pass "$label"
  fi
}

run_expect_fail() {
  if "$@" 2>&1; then
    return 1
  fi
  return 0
}

# ---- Seed a minimal make summary ----
make_fake_make() {
  local make_id="$1"
  local plan_id="$2"
  mkdir -p ".byom-video/makes/$make_id"
  cat > ".byom-video/makes/$make_id/make_summary.json" <<JSON
{
  "schema_version": "make_summary.v1",
  "make_id": "$make_id",
  "created_at": "2026-05-09T14:00:00Z",
  "input_path": "/fake/video.mov",
  "goal": "product launch short",
  "preset": "shorts",
  "status": "completed",
  "run_id": "fake-run-001",
  "creative_plan_id": "$plan_id",
  "platform_preset": "instagram-reel",
  "caption_position": "bottom",
  "caption_style": "default",
  "caption_status": "applied",
  "voiceover_status": "applied",
  "draft_path": ".byom-video/creative_plans/$plan_id/outputs/draft.mp4",
  "next_commands": []
}
JSON
}

MAKE_ID="smoke-make-rev-001"
PLAN_ID="smoke-plan-rev-001"

make_fake_make "$MAKE_ID" "$PLAN_ID"

# ============================================================
# Part 1: --request required
# ============================================================
echo ""
echo "==> Part 1: --request required"

if run_expect_fail "$BINARY" revise-make "$MAKE_ID" 2>/dev/null; then
  pass "--request missing fails"
else
  fail "--request missing should fail"
fi

# ============================================================
# Part 2: unknown request
# ============================================================
echo ""
echo "==> Part 2: unknown request"

OUT="$("$BINARY" revise-make "$MAKE_ID" --request "do some purple fire magic" 2>&1 || true)"
check_contains "unknown request fails with mapping message" "could not map revision request" "$OUT"
check_contains "unknown request shows examples" "Platform:" "$OUT"

# ============================================================
# Part 3: dry-run writes nothing
# ============================================================
echo ""
echo "==> Part 3: dry-run writes nothing"

OUT="$("$BINARY" revise-make "$MAKE_ID" --request "switch to square" --dry-run 2>&1 || true)"
check_contains "dry-run shows set_platform" "set_platform" "$OUT"
check_contains "dry-run shows dry-run message" "no files written" "$OUT"

# revisions dir must not exist after dry-run
if [ -d ".byom-video/makes/$MAKE_ID/revisions" ]; then
  fail "revisions dir should not exist after dry-run"
else
  pass "revisions dir not created in dry-run"
fi

# ============================================================
# Part 4: planned mode (no --yes) writes revision_summary.json
# ============================================================
echo ""
echo "==> Part 4: planned mode"

OUT="$("$BINARY" revise-make "$MAKE_ID" --request "move captions to center" 2>&1 || true)"
check_contains "planned mode shows action" "set_caption_position" "$OUT"
check_contains "planned mode suggests --yes" "--yes" "$OUT"

# revision_summary.json with status=planned
REV_DIR=".byom-video/makes/$MAKE_ID/revisions/revision_0001"
if [ -f "$REV_DIR/revision_summary.json" ]; then
  pass "revision_summary.json created"
  STATUS="$(python3 -c "import json,sys; d=json.load(open('$REV_DIR/revision_summary.json')); print(d.get('status',''))" 2>/dev/null || true)"
  if [ "$STATUS" = "planned" ]; then
    pass "revision_summary status=planned"
  else
    fail "revision_summary status should be planned, got: $STATUS"
  fi
else
  fail "revision_summary.json not found"
fi

# before_make_summary.json should exist (snapshot)
if [ -f "$REV_DIR/before_make_summary.json" ]; then
  pass "snapshot: before_make_summary.json exists"
else
  fail "snapshot: before_make_summary.json missing"
fi

# request.txt should exist
if [ -f "$REV_DIR/request.txt" ]; then
  pass "request.txt exists"
else
  fail "request.txt missing"
fi

# ============================================================
# Part 5: second revision increments ID
# ============================================================
echo ""
echo "==> Part 5: revision ID increments"

"$BINARY" revise-make "$MAKE_ID" --request "captions top" > /dev/null 2>&1 || true

if [ -d ".byom-video/makes/$MAKE_ID/revisions/revision_0002" ]; then
  pass "revision_0002 created"
else
  fail "revision_0002 not created"
fi

# ============================================================
# Part 6: make-revisions lists them
# ============================================================
echo ""
echo "==> Part 6: make-revisions"

OUT="$("$BINARY" make-revisions "$MAKE_ID" 2>&1 || true)"
check_contains "make-revisions shows revision_0001" "revision_0001" "$OUT"
check_contains "make-revisions shows revision_0002" "revision_0002" "$OUT"

# ============================================================
# Part 7: inspect-make-revision
# ============================================================
echo ""
echo "==> Part 7: inspect-make-revision"

OUT="$("$BINARY" inspect-make-revision "$MAKE_ID" revision_0001 2>&1 || true)"
check_contains "inspect shows revision_id" "revision_0001" "$OUT"
check_contains "inspect shows request" "move captions to center" "$OUT"
check_contains "inspect shows planned_actions" "set_caption_position" "$OUT"

# JSON mode
OUT_JSON="$("$BINARY" inspect-make-revision "$MAKE_ID" revision_0001 --json 2>&1 || true)"
SCHEMA="$(echo "$OUT_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('schema_version',''))" 2>/dev/null || true)"
if [ "$SCHEMA" = "make_revision.v1" ]; then
  pass "inspect --json has correct schema_version"
else
  fail "inspect --json: expected schema_version make_revision.v1, got: $SCHEMA"
fi

# ============================================================
# Part 8: review-make-revision
# ============================================================
echo ""
echo "==> Part 8: review-make-revision"

OUT="$("$BINARY" review-make-revision "$MAKE_ID" revision_0001 --write-artifact 2>&1 || true)"
check_contains "review writes artifact message" "Written:" "$OUT"

REV_MD="$REV_DIR/revision_review.md"
if [ -f "$REV_MD" ]; then
  pass "revision_review.md written"
  if grep -q "# Make Revision Review" "$REV_MD"; then
    pass "revision_review.md has markdown header"
  else
    fail "revision_review.md missing header"
  fi
else
  fail "revision_review.md not found"
fi

# ============================================================
# Part 9: provider guardrails
# ============================================================
echo ""
echo "==> Part 9: provider guardrails"

# generate_script without allow-provider-calls or fallback-stub should fail
OUT="$("$BINARY" revise-make "$MAKE_ID" --request "regenerate script" 2>&1 || true)"
check_contains "script blocked without --allow-provider-calls" "requires a provider call" "$OUT"
check_contains "script error mentions --allow-provider-calls" "--allow-provider-calls" "$OUT"

# generate_voiceover without allow-provider-calls should fail even with fallback-stub
OUT="$("$BINARY" revise-make "$MAKE_ID" --request "generate voiceover" --fallback-stub 2>&1 || true)"
check_contains "voiceover blocked without --allow-provider-calls even with --fallback-stub" "requires --allow-provider-calls" "$OUT"

# ============================================================
# Part 10: supported caption position/style requests
# ============================================================
echo ""
echo "==> Part 10: supported request patterns"

# These are dry-runs so they show the plan without writing files
for request in "switch to tiktok" "instagram reel" "square" "youtube" "vertical"; do
  OUT="$("$BINARY" revise-make "$MAKE_ID" --request "$request" --dry-run 2>&1 || true)"
  if echo "$OUT" | grep -q "set_platform"; then
    pass "platform request: '$request' → set_platform"
  else
    fail "platform request: '$request' should map to set_platform"
  fi
done

for request in "move captions to center" "captions top" "move captions to bottom"; do
  OUT="$("$BINARY" revise-make "$MAKE_ID" --request "$request" --dry-run 2>&1 || true)"
  if echo "$OUT" | grep -q "set_caption_position"; then
    pass "caption position request: '$request' → set_caption_position"
  else
    fail "caption position request: '$request' should map to set_caption_position"
  fi
done

for request in "boxed captions" "make captions bold" "default captions"; do
  OUT="$("$BINARY" revise-make "$MAKE_ID" --request "$request" --dry-run 2>&1 || true)"
  if echo "$OUT" | grep -q "set_caption_style"; then
    pass "caption style request: '$request' → set_caption_style"
  else
    fail "caption style request: '$request' should map to set_caption_style"
  fi
done

for request in "reassemble" "render again" "make new draft" "make it shorter" "make it longer"; do
  OUT="$("$BINARY" revise-make "$MAKE_ID" --request "$request" --dry-run 2>&1 || true)"
  if echo "$OUT" | grep -q "reassemble"; then
    pass "reassemble request: '$request' → reassemble"
  else
    fail "reassemble request: '$request' should map to reassemble"
  fi
done

# ============================================================
# Part 11: --reassemble flag appends reassemble action
# ============================================================
echo ""
echo "==> Part 11: --reassemble flag"

OUT="$("$BINARY" revise-make "$MAKE_ID" --request "move captions to center" --dry-run --reassemble 2>&1 || true)"
REASSEMBLE_COUNT="$(echo "$OUT" | grep -c "\[reassemble\]" || true)"
if [ "$REASSEMBLE_COUNT" -ge 1 ]; then
  pass "--reassemble appends reassemble action"
else
  fail "--reassemble should append reassemble action"
fi

# ============================================================
# Part 12: make-result shows revision info after --yes execution
# ============================================================
echo ""
echo "==> Part 12: make-result shows revision info"

# We'll use a local-only action (caption position) with --yes
# Execution will try to call CreativeAssemble for the reassemble step,
# which will fail (no ffmpeg / no real plan). Use set_caption_position
# without --reassemble so no external tool call happens.
# But we need --yes to write the completed revision.

# Create a fresh make with a simpler summary (no assemble needed)
MAKE2_ID="smoke-make-rev-002"
mkdir -p ".byom-video/makes/$MAKE2_ID"
cat > ".byom-video/makes/$MAKE2_ID/make_summary.json" <<JSON
{
  "schema_version": "make_summary.v1",
  "make_id": "$MAKE2_ID",
  "created_at": "2026-05-09T14:00:00Z",
  "goal": "test",
  "status": "completed",
  "next_commands": []
}
JSON

# set_caption_position with --yes should complete without any external tool call
"$BINARY" revise-make "$MAKE2_ID" --request "captions center" --yes > /dev/null 2>&1 || true

# make-result should show revision info
OUT="$("$BINARY" make-result "$MAKE2_ID" 2>&1 || true)"
check_contains "make-result shows revision_0001" "revision_0001" "$OUT"

# inspect-make should show revision count
OUT2="$("$BINARY" inspect-make "$MAKE2_ID" 2>&1 || true)"
check_contains "inspect-make shows revision count" "revision_count" "$OUT2"

# ============================================================
# Part 13: inspect-make-revision for non-existent revision fails cleanly
# ============================================================
echo ""
echo "==> Part 13: error handling"

if run_expect_fail "$BINARY" inspect-make-revision "$MAKE_ID" revision_9999 2>/dev/null; then
  pass "inspect non-existent revision fails cleanly"
else
  fail "inspect non-existent revision should fail"
fi

if run_expect_fail "$BINARY" revise-make nonexistent-make --request "reassemble" 2>/dev/null; then
  pass "revise non-existent make fails cleanly"
else
  fail "revise non-existent make should fail"
fi

# ============================================================
# Summary
# ============================================================
echo ""
echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"

if [ "$FAIL" -gt 0 ]; then
  echo "SMOKE FAILED"
  exit 1
fi
echo "SMOKE PASSED"

#!/usr/bin/env bash
# Smoke test for Job Queue Foundation v1 (Prompt 056).
# Tests: job-create (all 3 types), jobs, job-inspect, job-events,
#        job-approve, job-reject, job-cancel, job-run (approval gate + --yes bypass),
#        job-result, job-validate, policy flags, error handling.
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

# ============================================================
# Part 1: job-create required flags
# ============================================================
echo ""
echo "==> Part 1: job-create validation"

if run_expect_fail "$BINARY" job-create 2>/dev/null; then
  pass "--type missing fails"
else
  fail "--type missing should fail"
fi

if run_expect_fail "$BINARY" job-create --type make 2>/dev/null; then
  pass "--goal missing for make fails"
else
  fail "--goal missing for make should fail"
fi

if run_expect_fail "$BINARY" job-create --type revise_make --request "switch to square" 2>/dev/null; then
  pass "--make-id missing for revise_make fails"
else
  fail "--make-id missing for revise_make should fail"
fi

if run_expect_fail "$BINARY" job-create --type revise_make --make-id "make-abc" 2>/dev/null; then
  pass "--request missing for revise_make fails"
else
  fail "--request missing for revise_make should fail"
fi

if run_expect_fail "$BINARY" job-create --type validate_creative_assemble 2>/dev/null; then
  pass "--plan-id missing for validate_creative_assemble fails"
else
  fail "--plan-id missing for validate_creative_assemble should fail"
fi

if run_expect_fail "$BINARY" job-create --type unknown_type 2>/dev/null; then
  pass "unknown type fails"
else
  fail "unknown type should fail"
fi

# ============================================================
# Part 2: job-create for each action type
# ============================================================
echo ""
echo "==> Part 2: job-create for each action type"

# make type
OUT="$("$BINARY" job-create --type make --goal "product launch demo" 2>&1 || true)"
check_contains "job-create make shows Job created" "Job created:" "$OUT"
check_contains "job-create make shows pending" "pending" "$OUT"

# Get job ID
MAKE_JOB_ID="$(ls .byom-video/jobs/ | head -1)"
if [ -n "$MAKE_JOB_ID" ]; then
  pass "make job directory created"
else
  fail "make job directory not created"
fi

# Check job.json
JOB_JSON=".byom-video/jobs/$MAKE_JOB_ID/job.json"
if [ -f "$JOB_JSON" ]; then
  pass "job.json exists"
  SCHEMA="$(python3 -c "import json,sys; d=json.load(open('$JOB_JSON')); print(d.get('schema_version',''))" 2>/dev/null || true)"
  if [ "$SCHEMA" = "openvfx_job.v1" ]; then
    pass "job.json has correct schema_version"
  else
    fail "job.json schema_version should be openvfx_job.v1, got: $SCHEMA"
  fi
  APPROVAL="$(python3 -c "import json,sys; d=json.load(open('$JOB_JSON')); print(d.get('approval_status',''))" 2>/dev/null || true)"
  if [ "$APPROVAL" = "pending" ]; then
    pass "make job approval_status=pending"
  else
    fail "make job approval_status should be pending, got: $APPROVAL"
  fi
  MEDIA_WRITES="$(python3 -c "import json,sys; d=json.load(open('$JOB_JSON')); print(d.get('policy',{}).get('allow_media_writes',''))" 2>/dev/null || true)"
  if [ "$MEDIA_WRITES" = "True" ]; then
    pass "make job policy.allow_media_writes=true"
  else
    fail "make job policy.allow_media_writes should be True, got: $MEDIA_WRITES"
  fi
else
  fail "job.json not found"
fi

# events.jsonl written
EVENTS_FILE=".byom-video/jobs/$MAKE_JOB_ID/events.jsonl"
if [ -f "$EVENTS_FILE" ]; then
  pass "events.jsonl created for make job"
  if grep -q "JOB_CREATED" "$EVENTS_FILE"; then
    pass "JOB_CREATED event written"
  else
    fail "JOB_CREATED event not found in events.jsonl"
  fi
else
  fail "events.jsonl not created"
fi

# revise_make type
OUT="$("$BINARY" job-create --type revise_make --make-id "make-test-001" --request "switch to square" 2>&1 || true)"
check_contains "job-create revise_make shows Job created" "Job created:" "$OUT"

REVISE_JOB_ID="$(ls .byom-video/jobs/ | sort | tail -1)"
REVISE_JSON=".byom-video/jobs/$REVISE_JOB_ID/job.json"
REVISE_INPUT="$(python3 -c "import json,sys; d=json.load(open('$REVISE_JSON')); print(d.get('input',{}).get('make_id',''))" 2>/dev/null || true)"
if [ "$REVISE_INPUT" = "make-test-001" ]; then
  pass "revise_make job input.make_id correct"
else
  fail "revise_make job input.make_id should be make-test-001, got: $REVISE_INPUT"
fi

# validate_creative_assemble type — approval not_required
OUT="$("$BINARY" job-create --type validate_creative_assemble --plan-id "plan-smoke-001" 2>&1 || true)"
check_contains "job-create validate shows Job created" "Job created:" "$OUT"
check_contains "validate job approval shows not_required" "not_required" "$OUT"

VALIDATE_JOB_ID="$(ls .byom-video/jobs/ | sort | tail -1)"
VALIDATE_JSON=".byom-video/jobs/$VALIDATE_JOB_ID/job.json"
V_APPROVAL="$(python3 -c "import json,sys; d=json.load(open('$VALIDATE_JSON')); print(d.get('approval_status',''))" 2>/dev/null || true)"
if [ "$V_APPROVAL" = "not_required" ]; then
  pass "validate job approval_status=not_required"
else
  fail "validate job approval_status should be not_required, got: $V_APPROVAL"
fi

# ============================================================
# Part 3: job-create --json
# ============================================================
echo ""
echo "==> Part 3: job-create --json"

OUT="$("$BINARY" job-create --type make --goal "json test" --json 2>&1 || true)"
SCHEMA_JSON="$(echo "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('schema_version',''))" 2>/dev/null || true)"
if [ "$SCHEMA_JSON" = "openvfx_job.v1" ]; then
  pass "job-create --json returns valid JSON with schema_version"
else
  fail "job-create --json should return JSON with schema_version openvfx_job.v1, got: $SCHEMA_JSON"
fi

# ============================================================
# Part 4: policy flags
# ============================================================
echo ""
echo "==> Part 4: policy flags"

OUT="$("$BINARY" job-create --type make --goal "policy test" --allow-provider-calls --allow-overwrite --allow-external-network --json 2>&1 || true)"
PROV="$(echo "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('policy',{}).get('allow_provider_calls',''))" 2>/dev/null || true)"
OVER="$(echo "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('policy',{}).get('allow_overwrite',''))" 2>/dev/null || true)"
EXT="$(echo "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('policy',{}).get('allow_external_network',''))" 2>/dev/null || true)"
[ "$PROV" = "True" ] && pass "allow_provider_calls=True" || fail "allow_provider_calls should be True, got: $PROV"
[ "$OVER" = "True" ] && pass "allow_overwrite=True" || fail "allow_overwrite should be True, got: $OVER"
[ "$EXT" = "True" ] && pass "allow_external_network=True" || fail "allow_external_network should be True, got: $EXT"

# ============================================================
# Part 5: jobs (list)
# ============================================================
echo ""
echo "==> Part 5: jobs"

OUT="$("$BINARY" jobs 2>&1 || true)"
check_contains "jobs lists make action type" "make" "$OUT"
check_contains "jobs lists validate_creative_assemble" "validate_creative_assemble" "$OUT"

# --filter
OUT_FILTERED="$("$BINARY" jobs --filter completed 2>&1 || true)"
if echo "$OUT_FILTERED" | grep -qF "No jobs found"; then
  pass "jobs --filter completed returns empty (no completed jobs)"
else
  check_not_contains "jobs --filter completed shows no pending jobs" "pending" "$OUT_FILTERED"
fi

# ============================================================
# Part 6: job-inspect
# ============================================================
echo ""
echo "==> Part 6: job-inspect"

OUT="$("$BINARY" job-inspect "$MAKE_JOB_ID" 2>&1 || true)"
check_contains "job-inspect shows job_id" "$MAKE_JOB_ID" "$OUT"
check_contains "job-inspect shows action_type" "make" "$OUT"
check_contains "job-inspect shows status" "pending" "$OUT"
check_contains "job-inspect shows policy" "policy" "$OUT"
check_contains "job-inspect shows input" "input" "$OUT"

# --json
OUT_JSON="$("$BINARY" job-inspect "$MAKE_JOB_ID" --json 2>&1 || true)"
JOB_ID_FROM_JSON="$(echo "$OUT_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"
if [ "$JOB_ID_FROM_JSON" = "$MAKE_JOB_ID" ]; then
  pass "job-inspect --json returns correct job_id"
else
  fail "job-inspect --json: expected job_id $MAKE_JOB_ID, got: $JOB_ID_FROM_JSON"
fi

# non-existent job
if run_expect_fail "$BINARY" job-inspect "nonexistent-job-id" 2>/dev/null; then
  pass "job-inspect nonexistent job fails cleanly"
else
  fail "job-inspect nonexistent job should fail"
fi

# ============================================================
# Part 7: job-events
# ============================================================
echo ""
echo "==> Part 7: job-events"

OUT="$("$BINARY" job-events "$MAKE_JOB_ID" 2>&1 || true)"
check_contains "job-events shows JOB_CREATED" "JOB_CREATED" "$OUT"

# non-existent job
if run_expect_fail "$BINARY" job-events "nonexistent-job-id" 2>/dev/null; then
  pass "job-events nonexistent job fails cleanly"
else
  fail "job-events nonexistent job should fail"
fi

# ============================================================
# Part 8: job-approve
# ============================================================
echo ""
echo "==> Part 8: job-approve"

OUT="$("$BINARY" job-approve "$MAKE_JOB_ID" 2>&1 || true)"
check_contains "job-approve shows approved" "approved" "$OUT"

APPROVAL_AFTER="$(python3 -c "import json,sys; d=json.load(open('.byom-video/jobs/$MAKE_JOB_ID/job.json')); print(d.get('approval_status',''))" 2>/dev/null || true)"
if [ "$APPROVAL_AFTER" = "approved" ]; then
  pass "job.json approval_status updated to approved"
else
  fail "job.json approval_status should be approved, got: $APPROVAL_AFTER"
fi

if grep -q "JOB_APPROVED" ".byom-video/jobs/$MAKE_JOB_ID/events.jsonl" 2>/dev/null; then
  pass "JOB_APPROVED event written"
else
  fail "JOB_APPROVED event not found"
fi

# double-approve should fail
if run_expect_fail "$BINARY" job-approve "$MAKE_JOB_ID" 2>/dev/null; then
  pass "double-approve fails"
else
  fail "double-approve should fail"
fi

# approve a not_required job should fail
if run_expect_fail "$BINARY" job-approve "$VALIDATE_JOB_ID" 2>/dev/null; then
  pass "approve not_required job fails"
else
  fail "approve not_required job should fail"
fi

# ============================================================
# Part 9: job-reject
# ============================================================
echo ""
echo "==> Part 9: job-reject"

# Create a fresh job to reject
REJECT_JOB_OUT="$("$BINARY" job-create --type make --goal "reject me" --json 2>&1 || true)"
REJECT_JOB_ID="$(echo "$REJECT_JOB_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"

OUT="$("$BINARY" job-reject "$REJECT_JOB_ID" --reason "not needed" 2>&1 || true)"
check_contains "job-reject shows rejected" "rejected" "$OUT"

REJECT_STATUS="$(python3 -c "import json,sys; d=json.load(open('.byom-video/jobs/$REJECT_JOB_ID/job.json')); print(d.get('approval_status',''))" 2>/dev/null || true)"
if [ "$REJECT_STATUS" = "rejected" ]; then
  pass "job.json approval_status updated to rejected"
else
  fail "job.json approval_status should be rejected, got: $REJECT_STATUS"
fi

# double-reject should fail
if run_expect_fail "$BINARY" job-reject "$REJECT_JOB_ID" 2>/dev/null; then
  pass "double-reject fails"
else
  fail "double-reject should fail"
fi

# ============================================================
# Part 10: job-cancel
# ============================================================
echo ""
echo "==> Part 10: job-cancel"

CANCEL_JOB_OUT="$("$BINARY" job-create --type make --goal "cancel me" --json 2>&1 || true)"
CANCEL_JOB_ID="$(echo "$CANCEL_JOB_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"

OUT="$("$BINARY" job-cancel "$CANCEL_JOB_ID" --reason "changed my mind" 2>&1 || true)"
check_contains "job-cancel shows cancelled" "cancelled" "$OUT"

CANCEL_STATUS="$(python3 -c "import json,sys; d=json.load(open('.byom-video/jobs/$CANCEL_JOB_ID/job.json')); print(d.get('status',''))" 2>/dev/null || true)"
if [ "$CANCEL_STATUS" = "cancelled" ]; then
  pass "job.json status updated to cancelled"
else
  fail "job.json status should be cancelled, got: $CANCEL_STATUS"
fi

# double-cancel should fail
if run_expect_fail "$BINARY" job-cancel "$CANCEL_JOB_ID" 2>/dev/null; then
  pass "double-cancel fails"
else
  fail "double-cancel should fail"
fi

# ============================================================
# Part 11: job-run — approval gate
# ============================================================
echo ""
echo "==> Part 11: job-run approval gate"

GATE_JOB_OUT="$("$BINARY" job-create --type make --goal "gate test" --json 2>&1 || true)"
GATE_JOB_ID="$(echo "$GATE_JOB_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"

# Without approval or --yes should fail
OUT="$("$BINARY" job-run "$GATE_JOB_ID" 2>&1 || true)"
check_contains "job-run pending without --yes blocked" "requires approval" "$OUT"

# With --yes should succeed (make handler fails due to empty input_path but job infra works)
OUT="$("$BINARY" job-run "$GATE_JOB_ID" --yes 2>&1 || true)"
# The make handler will fail (no input file in tests), but check the approval gate was bypassed
GATE_STATUS="$(python3 -c "import json,sys; d=json.load(open('.byom-video/jobs/$GATE_JOB_ID/job.json')); print(d.get('status',''))" 2>/dev/null || true)"
if [ "$GATE_STATUS" = "failed" ] || [ "$GATE_STATUS" = "completed" ]; then
  pass "job-run with --yes bypasses approval gate (reached handler)"
else
  fail "job-run with --yes: expected failed or completed status, got: $GATE_STATUS"
fi

# ============================================================
# Part 12: job-run — approved path
# ============================================================
echo ""
echo "==> Part 12: job-run approved"

# validate_creative_assemble can run without approval
VALIDATE2_OUT="$("$BINARY" job-create --type validate_creative_assemble --plan-id "plan-smoke-run-001" --json 2>&1 || true)"
VALIDATE2_JOB_ID="$(echo "$VALIDATE2_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"

# job-run without --yes should work (not_required)
"$BINARY" job-run "$VALIDATE2_JOB_ID" > /dev/null 2>&1 || true

V2_STATUS="$(python3 -c "import json,sys; d=json.load(open('.byom-video/jobs/$VALIDATE2_JOB_ID/job.json')); print(d.get('status',''))" 2>/dev/null || true)"
if [ "$V2_STATUS" = "failed" ] || [ "$V2_STATUS" = "completed" ]; then
  pass "validate job ran without approval gate (reached handler)"
else
  fail "validate job should have run: expected failed or completed, got: $V2_STATUS"
fi

# job-run again (terminal state) should fail
OUT="$("$BINARY" job-run "$VALIDATE2_JOB_ID" 2>&1 || true)"
if echo "$OUT" | grep -qE "already (completed|failed|cancelled)"; then
  pass "job-run on terminal-state job fails cleanly"
else
  fail "job-run on terminal-state job should fail: got $OUT"
fi

# ============================================================
# Part 13: job-run events
# ============================================================
echo ""
echo "==> Part 13: job-run events"

VALIDATE3_OUT="$("$BINARY" job-create --type validate_creative_assemble --plan-id "plan-events-test" --json 2>&1 || true)"
VALIDATE3_JOB_ID="$(echo "$VALIDATE3_OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || true)"
"$BINARY" job-run "$VALIDATE3_JOB_ID" > /dev/null 2>&1 || true

OUT="$("$BINARY" job-events "$VALIDATE3_JOB_ID" 2>&1 || true)"
check_contains "job-events shows JOB_RUN_STARTED" "JOB_RUN_STARTED" "$OUT"
check_contains "job-events shows JOB_ACTION_STARTED" "JOB_ACTION_STARTED" "$OUT"

# ============================================================
# Part 14: job-result
# ============================================================
echo ""
echo "==> Part 14: job-result"

OUT="$("$BINARY" job-result "$VALIDATE2_JOB_ID" 2>&1 || true)"
check_contains "job-result shows action type" "validate_creative_assemble" "$OUT"

# non-existent
if run_expect_fail "$BINARY" job-result "nonexistent-job" 2>/dev/null; then
  pass "job-result nonexistent fails cleanly"
else
  fail "job-result nonexistent should fail"
fi

# ============================================================
# Part 15: job-validate
# ============================================================
echo ""
echo "==> Part 15: job-validate"

OUT="$("$BINARY" job-validate "$MAKE_JOB_ID" 2>&1 || true)"
check_contains "job-validate reports valid" "valid" "$OUT"

# non-existent
if run_expect_fail "$BINARY" job-validate "nonexistent-job" 2>/dev/null; then
  pass "job-validate nonexistent fails cleanly"
else
  fail "job-validate nonexistent should fail"
fi

# job-validate --json
OUT_JSON="$("$BINARY" job-validate "$MAKE_JOB_ID" --json 2>&1 || true)"
IS_VALID="$(echo "$OUT_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('valid',''))" 2>/dev/null || true)"
if [ "$IS_VALID" = "True" ]; then
  pass "job-validate --json returns valid=True"
else
  fail "job-validate --json: expected valid=True, got: $IS_VALID"
fi

# ============================================================
# Part 16: reject/cancel blocks job-run
# ============================================================
echo ""
echo "==> Part 16: reject/cancel blocks job-run"

# rejected job cannot run
if run_expect_fail "$BINARY" job-run "$REJECT_JOB_ID" --yes 2>/dev/null; then
  pass "job-run rejected job fails"
else
  fail "job-run rejected job should fail"
fi

# cancelled job cannot run
if run_expect_fail "$BINARY" job-run "$CANCEL_JOB_ID" --yes 2>/dev/null; then
  pass "job-run cancelled job fails"
else
  fail "job-run cancelled job should fail"
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

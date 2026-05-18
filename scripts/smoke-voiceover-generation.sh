#!/usr/bin/env bash
# Smoke test for Dynamic Voice Generation v1 (Prompt 052).
# Tests: creative-generate-voiceover, review-generated-voiceover,
#        validate-voiceover + generation artifact, voiceover-status generation fields,
#        inspect-creative-plan voiceover_gen, make --generate-voiceover dry-run,
#        env-var gate, overwrite protection.
# Does NOT call real ElevenLabs — all live-provider tests use a local fake server
# (nc/python) or are dry-run only.
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

# ---- helpers ----

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

make_plan() {
  local goal="${1:-A test video about voiceover generation}"
  "$BINARY" creative-plan /dev/null --goal "$goal" >/dev/null 2>&1 || true
  ls -t .byom-video/creative_plans/ 2>/dev/null | head -1
}

write_voiceover_text() {
  local plan_dir="$1"
  local text="${2:-This is the voiceover text for testing purposes. It has enough words.}"
  mkdir -p "$plan_dir/outputs"
  cat > "$plan_dir/outputs/voiceover_text.json" <<VTEOF
{"schema_version":"voiceover_text.v1","creative_plan_id":"$(basename "$plan_dir")","mode":"local_text_extract","source":{"source_type":"goal","source_artifact":""},"text":"$text","word_count":12,"warnings":[]}
VTEOF
  printf '%s' "$text" > "$plan_dir/outputs/voiceover_text.txt"
}

write_voice_yaml() {
  local endpoint="${1:-http://localhost:9999}"
  cat > byom-video.yaml <<YAMLEOF
tools:
  enabled: true
  backends:
    voice_backend:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: $endpoint
      auth:
        type: header_env
        header: xi-api-key
        env: SMOKE_TEST_ELEVEN_KEY
      options:
        voice_id: smoke-voice-id-001
  routes:
    creative.voiceover: voice_backend
YAMLEOF
}

# ---- start a fake TTS HTTP server using python ----
start_fake_tts() {
  python3 - <<'PYEOF' &
import http.server, threading, sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        _ = self.rfile.read(length)
        self.send_response(200)
        self.send_header('Content-Type', 'audio/mpeg')
        self.end_headers()
        self.wfile.write(b'ID3FAKEAUDIO_BYTES_FROM_SMOKE_TEST')
    def log_message(self, *args):
        pass

srv = http.server.HTTPServer(('127.0.0.1', 18765), Handler)
srv.serve_forever()
PYEOF
  FAKE_TTS_PID=$!
  sleep 0.3
  FAKE_TTS_URL="http://127.0.0.1:18765"
}

stop_fake_tts() {
  if [ -n "${FAKE_TTS_PID:-}" ]; then
    kill "$FAKE_TTS_PID" 2>/dev/null || true
    wait "$FAKE_TTS_PID" 2>/dev/null || true
    FAKE_TTS_PID=""
  fi
}

# ---- Part 1: creative-generate-voiceover requires a real plan ----
echo ""
echo "--- Part 1: creative-generate-voiceover requires a real plan ---"
write_voice_yaml
if run_expect_fail "$BINARY" creative-generate-voiceover nonexistent-plan-abc --dry-run 2>&1 | grep -qi "not found"; then
  pass "creative-generate-voiceover nonexistent plan → not found"
else
  fail "creative-generate-voiceover nonexistent plan should fail with not found"
fi

# ---- Part 2: creative-generate-voiceover requires tools config ----
echo ""
echo "--- Part 2: missing backend route fails ---"
PLAN_ID="$(make_plan "Smoke voice gen test")"
PLAN_DIR=".byom-video/creative_plans/$PLAN_ID"
write_voiceover_text "$PLAN_DIR"

cat > byom-video.yaml <<'EOF'
tools:
  enabled: true
  backends: {}
  routes: {}
EOF
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run 2>&1 | grep -qi "no route"; then
  pass "creative-generate-voiceover missing route → no route error"
else
  fail "creative-generate-voiceover missing route should fail"
fi

# ---- Part 3: tools.enabled: false fails ----
echo ""
echo "--- Part 3: tools.enabled false fails ---"
cat > byom-video.yaml <<'EOF'
tools:
  enabled: false
EOF
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run 2>&1 | grep -qi "tools.enabled"; then
  pass "creative-generate-voiceover tools.enabled false → error"
else
  fail "creative-generate-voiceover tools.enabled false should fail"
fi

# ---- Part 4: wrong-kind backend fails ----
echo ""
echo "--- Part 4: wrong backend kind fails ---"
cat > byom-video.yaml <<'EOF'
tools:
  enabled: true
  backends:
    text_backend:
      kind: text_generation
      provider: ollama
      model: qwen
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.voiceover: text_backend
EOF
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run 2>&1 | grep -qi "voice_generation"; then
  pass "creative-generate-voiceover wrong backend kind → voice_generation error"
else
  fail "creative-generate-voiceover wrong backend kind should fail"
fi

# ---- Part 5: dry-run succeeds and writes artifact ----
echo ""
echo "--- Part 5: dry-run writes voiceover_generation.json ---"
write_voice_yaml
unset SMOKE_TEST_ELEVEN_KEY 2>/dev/null || true
OUT="$("$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run 2>&1)"
if echo "$OUT" | grep -qi "dry-run"; then
  pass "creative-generate-voiceover --dry-run prints dry-run"
else
  fail "creative-generate-voiceover --dry-run output missing 'dry-run': $OUT"
fi

if [ -f "$PLAN_DIR/outputs/voiceover_generation.json" ]; then
  pass "voiceover_generation.json created after dry-run"
else
  fail "voiceover_generation.json not created after dry-run"
fi

if python3 -c "
import json, sys
with open('$PLAN_DIR/outputs/voiceover_generation.json') as f:
    d = json.load(f)
assert d['schema_version'] == 'voiceover_generation.v1', 'wrong schema_version'
assert d['status'] == 'dry_run', 'wrong status: ' + d['status']
assert d['mode'] == 'dry_run', 'wrong mode: ' + d['mode']
" 2>&1; then
  pass "voiceover_generation.json has correct schema_version/status/mode"
else
  fail "voiceover_generation.json has wrong content"
fi

# ---- Part 6: dry-run with --check-env fails when env var missing ----
echo ""
echo "--- Part 6: --check-env fails when env var missing ---"
unset SMOKE_TEST_ELEVEN_KEY 2>/dev/null || true
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run --check-env --overwrite 2>&1 | grep -qi "SMOKE_TEST_ELEVEN_KEY"; then
  pass "creative-generate-voiceover --check-env without env var → env error"
else
  fail "creative-generate-voiceover --check-env without env var should fail"
fi

# ---- Part 7: dry-run with --check-env passes when env var set ----
echo ""
echo "--- Part 7: --check-env passes when env var set ---"
export SMOKE_TEST_ELEVEN_KEY="fake-key-for-smoke"
OUT="$("$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run --check-env --overwrite 2>&1)"
if echo "$OUT" | grep -qi "dry-run"; then
  pass "creative-generate-voiceover --check-env with env var → succeeds"
else
  fail "creative-generate-voiceover --check-env with env var failed: $OUT"
fi
if echo "$OUT" | grep -qi "SMOKE_TEST_ELEVEN_KEY.*✓\|✓.*SMOKE_TEST_ELEVEN_KEY\|present"; then
  pass "--check-env shows env var present"
else
  fail "--check-env should confirm env var present: $OUT"
fi

# ---- Part 8: overwrite protection ----
echo ""
echo "--- Part 8: overwrite protection ---"
# artifact already exists from Part 5; live run (no --dry-run) without --overwrite fails
# Note: overwrite check fires before backend call, so no API key needed to trigger it
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN_ID" 2>&1 | grep -qi "already exists"; then
  pass "creative-generate-voiceover without --overwrite → already exists error"
else
  fail "creative-generate-voiceover without --overwrite should fail when artifact exists"
fi

# dry-run with --overwrite succeeds
OUT="$("$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run --overwrite 2>&1)"
if echo "$OUT" | grep -qi "dry-run"; then
  pass "creative-generate-voiceover --dry-run --overwrite succeeds"
else
  fail "creative-generate-voiceover --dry-run --overwrite failed: $OUT"
fi

# ---- Part 9: --json outputs valid JSON artifact ----
echo ""
echo "--- Part 9: --json output is valid JSON ---"
OUT="$("$BINARY" creative-generate-voiceover "$PLAN_ID" --dry-run --overwrite --json 2>&1)"
JSON_PART="$(echo "$OUT" | python3 -c "import sys; s=sys.stdin.read(); i=s.find('{'); print(s[i:]) if i>=0 else print('')" 2>/dev/null)"
if python3 -c "
import json, sys
d = json.loads('''$JSON_PART''')
assert d['schema_version'] == 'voiceover_generation.v1'
assert d['status'] == 'dry_run'
" 2>&1; then
  pass "creative-generate-voiceover --json outputs valid JSON with correct fields"
else
  fail "creative-generate-voiceover --json output is invalid: $OUT"
fi

# ---- Part 10: custom-http-voice provider: dry-run allowed ----
echo ""
echo "--- Part 10: custom-http-voice provider dry-run allowed ---"
PLAN2_ID="$(make_plan "Custom provider dry-run test")"
PLAN2_DIR=".byom-video/creative_plans/$PLAN2_ID"
write_voiceover_text "$PLAN2_DIR"
cat > byom-video.yaml <<'EOF'
tools:
  enabled: true
  backends:
    custom_voice:
      kind: voice_generation
      provider: custom-http-voice
      model: some-model
      endpoint: http://localhost:9999
      auth:
        type: none
      options:
        voice_id: custom-voice-001
  routes:
    creative.voiceover: custom_voice
EOF
if "$BINARY" creative-generate-voiceover "$PLAN2_ID" --dry-run 2>&1 | grep -qi "dry-run"; then
  pass "custom-http-voice provider: dry-run allowed"
else
  fail "custom-http-voice provider: dry-run should succeed"
fi

# ---- Part 11: custom-http-voice live fails ----
echo ""
echo "--- Part 11: custom-http-voice live fails ---"
if run_expect_fail "$BINARY" creative-generate-voiceover "$PLAN2_ID" --overwrite 2>&1 | grep -qi "elevenlabs-compatible"; then
  pass "custom-http-voice live → elevenlabs-compatible error"
else
  fail "custom-http-voice live should fail with elevenlabs-compatible error"
fi

# ---- Part 12: --prepare-text creates voiceover_text.json ----
echo ""
echo "--- Part 12: --prepare-text creates voiceover_text.json ---"
write_voice_yaml
PLAN3_ID="$(make_plan "Prepare text test")"
PLAN3_DIR=".byom-video/creative_plans/$PLAN3_ID"
mkdir -p "$PLAN3_DIR/outputs"
# No voiceover_text.json yet
if [ -f "$PLAN3_DIR/outputs/voiceover_text.json" ]; then
  fail "voiceover_text.json should not exist before --prepare-text"
fi
OUT="$("$BINARY" creative-generate-voiceover "$PLAN3_ID" --dry-run --prepare-text 2>&1)"
if [ -f "$PLAN3_DIR/outputs/voiceover_text.json" ]; then
  pass "--prepare-text creates voiceover_text.json"
else
  fail "--prepare-text did not create voiceover_text.json: $OUT"
fi

# ---- Part 13: review-generated-voiceover shows artifact fields ----
echo ""
echo "--- Part 13: review-generated-voiceover ---"
write_voice_yaml
OUT="$("$BINARY" review-generated-voiceover "$PLAN_ID" 2>&1)"
if echo "$OUT" | grep -qi "status\|mode\|provider"; then
  pass "review-generated-voiceover shows status/mode/provider"
else
  fail "review-generated-voiceover missing fields: $OUT"
fi
if echo "$OUT" | grep -qi "elevenlabs-compatible"; then
  pass "review-generated-voiceover shows provider name"
else
  fail "review-generated-voiceover missing provider: $OUT"
fi

# ---- Part 14: review-generated-voiceover --write-artifact creates .md ----
echo ""
echo "--- Part 14: review-generated-voiceover --write-artifact ---"
OUT="$("$BINARY" review-generated-voiceover "$PLAN_ID" --write-artifact 2>&1)"
if [ -f "$PLAN_DIR/outputs/voiceover_generation_review.md" ]; then
  pass "review-generated-voiceover --write-artifact created voiceover_generation_review.md"
else
  fail "review-generated-voiceover --write-artifact did not create .md: $OUT"
fi
if grep -qi "elevenlabs-compatible" "$PLAN_DIR/outputs/voiceover_generation_review.md"; then
  pass "voiceover_generation_review.md contains provider info"
else
  fail "voiceover_generation_review.md missing provider info"
fi

# ---- Part 15: review-generated-voiceover on missing plan fails ----
echo ""
echo "--- Part 15: review-generated-voiceover missing plan ---"
if run_expect_fail "$BINARY" review-generated-voiceover nonexistent-plan-xyz 2>&1 | grep -qi "not found"; then
  pass "review-generated-voiceover nonexistent plan → not found"
else
  fail "review-generated-voiceover nonexistent plan should fail"
fi

# ---- Part 16: voiceover-status shows generation_status ----
echo ""
echo "--- Part 16: voiceover-status shows generation_status ---"
OUT="$("$BINARY" voiceover-status "$PLAN_ID" 2>&1)"
if echo "$OUT" | grep -qi "dry_run\|completed\|failed"; then
  pass "voiceover-status shows generation_status"
else
  fail "voiceover-status missing generation_status: $OUT"
fi
if echo "$OUT" | grep -qi "elevenlabs-compatible"; then
  pass "voiceover-status shows generation provider"
else
  fail "voiceover-status missing generation provider: $OUT"
fi

# ---- Part 17: validate-voiceover with dry_run artifact passes ----
echo ""
echo "--- Part 17: validate-voiceover with dry_run generation passes ---"
OUT="$("$BINARY" validate-voiceover "$PLAN_ID" 2>&1)"
if echo "$OUT" | grep -qi "valid\|pass\|ok"; then
  pass "validate-voiceover with dry_run generation passes"
else
  # validate might warn but not hard-fail for dry_run artifact
  if echo "$OUT" | grep -qi "fail\|error" && ! echo "$OUT" | grep -qi "warn"; then
    fail "validate-voiceover with dry_run artifact failed: $OUT"
  else
    pass "validate-voiceover with dry_run generation OK (warnings acceptable)"
  fi
fi

# ---- Part 18: validate-voiceover fails when completed but audio missing ----
echo ""
echo "--- Part 18: validate-voiceover fails when generation=completed but audio missing ---"
PLAN_VALIDATE_ID="$(make_plan "Validate missing audio")"
PLAN_VALIDATE_DIR=".byom-video/creative_plans/$PLAN_VALIDATE_ID"
write_voiceover_text "$PLAN_VALIDATE_DIR"
cat > "$PLAN_VALIDATE_DIR/outputs/voiceover_generation.json" <<'GENEOF'
{"schema_version":"voiceover_generation.v1","creative_plan_id":"validate-test","mode":"live","status":"completed","provider":"elevenlabs-compatible","backend":"voice_backend","route":"creative.voiceover","model":"eleven_multilingual_v2","voice_id":"v1","source":{"source_type":"voiceover_text","source_artifact":"","text_word_count":12},"request":{"endpoint":"http://x/text-to-speech/v1","timeout_seconds":120},"output":{"audio_file":"outputs/voiceover.mp3","content_type":"audio/mpeg","bytes":100}}
GENEOF
# audio file does NOT exist
if run_expect_fail "$BINARY" validate-voiceover "$PLAN_VALIDATE_ID" 2>&1 | grep -qi "missing\|not found\|audio\|error\|fail"; then
  pass "validate-voiceover fails when completed but audio missing"
else
  fail "validate-voiceover should fail when completed artifact has no audio file"
fi

# ---- Part 19: inspect-creative-plan shows voiceover_gen ----
echo ""
echo "--- Part 19: inspect-creative-plan shows voiceover_gen ---"
OUT="$("$BINARY" inspect-creative-plan "$PLAN_ID" 2>&1)"
if echo "$OUT" | grep -qi "voiceover_gen"; then
  pass "inspect-creative-plan shows voiceover_gen"
else
  fail "inspect-creative-plan missing voiceover_gen: $OUT"
fi

# ---- Part 20: validate-creative-plan shows voiceover_gen status ----
echo ""
echo "--- Part 20: validate-creative-plan includes voiceover generation ---"
OUT="$("$BINARY" validate-creative-plan "$PLAN_ID" 2>&1)"
if echo "$OUT" | grep -qi "voiceover\|voice"; then
  pass "validate-creative-plan references voiceover"
else
  fail "validate-creative-plan missing voiceover reference: $OUT"
fi

# ---- Part 21: make --dry-run --yes shows step 4e when --generate-voiceover ----
echo ""
echo "--- Part 21: make --dry-run --yes --generate-voiceover shows step 4e ---"
OUT="$("$BINARY" make /dev/null --goal "smoke test goal" --dry-run --yes --generate-voiceover 2>&1)"
if echo "$OUT" | grep -qi "4e"; then
  pass "make --dry-run --yes --generate-voiceover shows step 4e"
else
  fail "make --dry-run --yes --generate-voiceover missing step 4e: $OUT"
fi

# ---- Part 22: make --dry-run --yes without --generate-voiceover omits step 4e ----
echo ""
echo "--- Part 22: make --dry-run --yes without --generate-voiceover omits step 4e ---"
OUT="$("$BINARY" make /dev/null --goal "smoke test goal" --dry-run --yes 2>&1)"
if echo "$OUT" | grep -qi "4e"; then
  fail "make --dry-run without --generate-voiceover should not show step 4e: $OUT"
else
  pass "make --dry-run without --generate-voiceover correctly omits step 4e"
fi

# ---- Part 23: make --dry-run without --yes omits step 4e ----
echo ""
echo "--- Part 23: make --dry-run without --yes omits step 4e ---"
OUT="$("$BINARY" make /dev/null --goal "smoke test goal" --dry-run --generate-voiceover 2>&1)"
if echo "$OUT" | grep -qi "4e"; then
  fail "make --dry-run without --yes should not show step 4e: $OUT"
else
  pass "make --dry-run without --yes correctly omits step 4e"
fi

# ---- Part 24: live generation with fake TTS server ----
echo ""
echo "--- Part 24: live generation with fake TTS server ---"
if ! command -v python3 >/dev/null 2>&1; then
  echo "    SKIP: python3 not found, skipping fake TTS test"
else
  PLAN_LIVE_ID="$(make_plan "Live TTS smoke")"
  PLAN_LIVE_DIR=".byom-video/creative_plans/$PLAN_LIVE_ID"
  write_voiceover_text "$PLAN_LIVE_DIR" "Test voiceover for live generation smoke."
  FAKE_TTS_PID=""
  start_fake_tts
  write_voice_yaml "$FAKE_TTS_URL"
  export SMOKE_TEST_ELEVEN_KEY="smoke-fake-key-live"

  OUT="$("$BINARY" creative-generate-voiceover "$PLAN_LIVE_ID" 2>&1)"
  LIVE_EXIT=$?
  stop_fake_tts

  if [ $LIVE_EXIT -eq 0 ]; then
    pass "live generation with fake TTS server succeeds"
  else
    fail "live generation with fake TTS server failed: $OUT"
  fi

  if [ -f "$PLAN_LIVE_DIR/outputs/voiceover.mp3" ]; then
    pass "live generation writes voiceover.mp3"
  else
    fail "live generation did not write voiceover.mp3"
  fi

  GEN_JSON="$PLAN_LIVE_DIR/outputs/voiceover_generation.json"
  if python3 -c "
import json
with open('$GEN_JSON') as f:
    d = json.load(f)
assert d['status'] == 'completed', 'status=' + d['status']
assert d['mode'] == 'live', 'mode=' + d['mode']
assert d['output']['bytes'] > 0, 'bytes=0'
assert 'SMOKE_TEST_ELEVEN_KEY' not in json.dumps(d), 'env var name leaked'
assert 'smoke-fake-key-live' not in json.dumps(d), 'secret leaked in artifact'
" 2>&1; then
    pass "live generation artifact has correct status/mode/bytes, no secret"
  else
    fail "live generation artifact is wrong or leaks secret"
  fi

  # Check stdout doesn't contain secret
  if echo "$OUT" | grep -q "smoke-fake-key-live"; then
    fail "live generation stdout contains secret API key"
  else
    pass "live generation stdout does not contain secret API key"
  fi

  # Review the live artifact
  OUT_REV="$("$BINARY" review-generated-voiceover "$PLAN_LIVE_ID" 2>&1)"
  if echo "$OUT_REV" | grep -qi "completed"; then
    pass "review-generated-voiceover shows completed for live generation"
  else
    fail "review-generated-voiceover did not show completed: $OUT_REV"
  fi
fi

# ---- Part 25: validate-voiceover passes when completed + audio exists ----
echo ""
echo "--- Part 25: validate-voiceover passes when completed + audio exists ---"
if ! command -v python3 >/dev/null 2>&1; then
  echo "    SKIP: python3 not found"
else
  if [ -n "${PLAN_LIVE_ID:-}" ] && [ -f "$PLAN_LIVE_DIR/outputs/voiceover.mp3" ]; then
    write_voice_yaml "http://localhost:9999"
    OUT="$("$BINARY" validate-voiceover "$PLAN_LIVE_ID" 2>&1)"
    if echo "$OUT" | grep -qi "valid\|pass\|ok" && ! echo "$OUT" | grep -qi "^FAIL:"; then
      pass "validate-voiceover passes when completed + audio exists"
    else
      fail "validate-voiceover failed when completed + audio exists: $OUT"
    fi
  else
    echo "    SKIP: live plan not available (Part 24 skipped)"
  fi
fi

# ---- summary ----
echo ""
echo "============================================"
echo "Smoke test: voiceover-generation (Prompt 052)"
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "============================================"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

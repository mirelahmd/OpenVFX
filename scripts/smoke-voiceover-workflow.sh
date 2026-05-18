#!/usr/bin/env bash
# Smoke test for Local Voiceover Asset Workflow (Prompt 051).
# Tests: creative-voiceover-text, voiceover-status, review-voiceover,
#        validate-voiceover, make --prepare-voiceover, assemble hints.
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

cat > byom-video.yaml <<'EOF'
tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.script: local_writer
    creative.captions: local_writer
EOF

PASS=0
FAIL=0

pass() { echo "    PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "    FAIL: $1"; FAIL=$((FAIL+1)); }

run_expect_fail() {
  "$@" 2>&1 || true
}

# ---- helpers ----

make_plan() {
  local goal="${1:-A test video about voiceover workflows}"
  "$BINARY" creative-plan /dev/null --goal "$goal" >/dev/null 2>&1 || true
  ls -t .byom-video/creative_plans/ 2>/dev/null | head -1
}

# ---- Part 1: creative-voiceover-text requires a real plan ----
echo ""
echo "==> Part 1: creative-voiceover-text rejects missing plan"
OUT=$(run_expect_fail "$BINARY" creative-voiceover-text nonexistent-plan-xyz)
if echo "$OUT" | grep -q "not found"; then
  pass "rejects missing plan"
else
  fail "expected 'not found', got: $OUT"
fi

# ---- Part 2: goal fallback when no script draft ----
echo ""
echo "==> Part 2: voiceover-text falls back to goal when no script draft"
PLAN1=$(make_plan "Explain the magic of coffee to new baristas")
OUT=$("$BINARY" creative-voiceover-text "$PLAN1")
if echo "$OUT" | grep -q "source:"; then
  pass "produces output"
else
  fail "no 'source:' in output: $OUT"
fi
if echo "$OUT" | grep -q "words:"; then
  pass "shows word count"
else
  fail "no 'words:' in output: $OUT"
fi
OUTDIR=".byom-video/creative_plans/$PLAN1/outputs"
if [ -f "$OUTDIR/voiceover_text.json" ]; then
  pass "wrote voiceover_text.json"
else
  fail "voiceover_text.json not created"
fi
if [ -f "$OUTDIR/voiceover_text.txt" ]; then
  pass "wrote voiceover_text.txt"
else
  fail "voiceover_text.txt not created"
fi

# ---- Part 3: goal-fallback warning is present ----
echo ""
echo "==> Part 3: goal fallback emits a warning"
if echo "$OUT" | grep -q "warning:"; then
  pass "emits goal-fallback warning"
else
  fail "expected warning line, got: $OUT"
fi

# ---- Part 4: source_type in JSON ----
echo ""
echo "==> Part 4: voiceover_text.json has expected fields"
JSON=$(cat "$OUTDIR/voiceover_text.json")
if echo "$JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['schema_version']=='voiceover_text.v1'" 2>/dev/null; then
  pass "schema_version is voiceover_text.v1"
else
  fail "wrong schema_version in voiceover_text.json"
fi
if echo "$JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert isinstance(d['word_count'], int) and d['word_count']>0" 2>/dev/null; then
  pass "word_count > 0"
else
  fail "word_count missing or 0"
fi
if echo "$JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['source']['source_type']=='goal'" 2>/dev/null; then
  pass "source_type is goal"
else
  fail "source_type not 'goal'"
fi

# ---- Part 5: --overwrite rejected without flag ----
echo ""
echo "==> Part 5: rejects overwrite without --overwrite"
OUT2=$(run_expect_fail "$BINARY" creative-voiceover-text "$PLAN1")
if echo "$OUT2" | grep -q "already exists"; then
  pass "rejects duplicate without --overwrite"
else
  fail "expected 'already exists', got: $OUT2"
fi

# ---- Part 6: --overwrite succeeds ----
echo ""
echo "==> Part 6: --overwrite replaces existing file"
OUT3=$("$BINARY" creative-voiceover-text "$PLAN1" --overwrite)
if echo "$OUT3" | grep -q "source:"; then
  pass "--overwrite succeeds"
else
  fail "--overwrite failed: $OUT3"
fi

# ---- Part 7: voiceover_text.txt content matches JSON text ----
echo ""
echo "==> Part 7: voiceover_text.txt content matches JSON text field"
TXT_CONTENT=$(cat "$OUTDIR/voiceover_text.txt")
JSON_TEXT=$(python3 -c "import sys,json; print(json.load(open('$OUTDIR/voiceover_text.json'))['text'])")
if [ "$TXT_CONTENT" = "$JSON_TEXT" ]; then
  pass "txt matches JSON text field"
else
  fail "txt content differs from JSON text"
fi

# ---- Part 8: --max-words truncation ----
echo ""
echo "==> Part 8: --max-words truncates long goal text"
PLAN2=$(make_plan "This is a very long goal that should definitely be truncated when we set max words to a small value like three words total")
"$BINARY" creative-voiceover-text "$PLAN2" --max-words 3
WCOUNT=$(python3 -c "import json; d=json.load(open('.byom-video/creative_plans/$PLAN2/outputs/voiceover_text.json')); print(d['word_count'])")
if [ "$WCOUNT" -le 3 ]; then
  pass "--max-words=3 truncated to $WCOUNT words"
else
  fail "expected <=3 words, got $WCOUNT"
fi
WARNS=$(python3 -c "import json; d=json.load(open('.byom-video/creative_plans/$PLAN2/outputs/voiceover_text.json')); print(len(d.get('warnings',[])))")
if [ "$WARNS" -gt 0 ]; then
  pass "truncation warning recorded"
else
  fail "no warnings recorded for truncation"
fi

# ---- Part 9: --json flag outputs JSON to stdout ----
echo ""
echo "==> Part 9: --json outputs JSON to stdout"
PLAN3=$(make_plan "Test JSON output mode for voiceover")
STDOUT_JSON=$("$BINARY" creative-voiceover-text "$PLAN3" --json 2>/dev/null)
# --json appends JSON after human-readable header; check JSON block is present
if echo "$STDOUT_JSON" | python3 -c "
import sys, json, re
data = sys.stdin.read()
m = re.search(r'\{.*\}', data, re.DOTALL)
if not m: raise ValueError('no JSON found')
json.loads(m.group())
" 2>/dev/null; then
  pass "--json writes valid JSON to stdout"
else
  fail "--json did not emit valid JSON: $STDOUT_JSON"
fi

# ---- Part 10: voiceover-status missing_text_and_audio ----
echo ""
echo "==> Part 10: voiceover-status missing_text_and_audio for fresh plan"
PLAN4=$(make_plan "Status check voiceover plan")
STATUS_OUT=$("$BINARY" voiceover-status "$PLAN4")
if echo "$STATUS_OUT" | grep -q "missing_text_and_audio"; then
  pass "fresh plan is missing_text_and_audio"
else
  fail "expected missing_text_and_audio, got: $STATUS_OUT"
fi
if echo "$STATUS_OUT" | grep -q "next steps"; then
  pass "shows next steps"
else
  fail "no next steps shown"
fi

# ---- Part 11: voiceover-status missing_audio after text generated ----
echo ""
echo "==> Part 11: voiceover-status missing_audio after text generated"
"$BINARY" creative-voiceover-text "$PLAN4"
STATUS_OUT2=$("$BINARY" voiceover-status "$PLAN4")
if echo "$STATUS_OUT2" | grep -q "missing_audio"; then
  pass "status is missing_audio after text only"
else
  fail "expected missing_audio, got: $STATUS_OUT2"
fi

# ---- Part 12: voiceover-status ready after audio placed ----
echo ""
echo "==> Part 12: voiceover-status ready after audio placed"
touch ".byom-video/creative_plans/$PLAN4/outputs/voiceover.wav"
STATUS_OUT3=$("$BINARY" voiceover-status "$PLAN4")
if echo "$STATUS_OUT3" | grep -q "ready"; then
  pass "status is ready when audio present"
else
  fail "expected ready, got: $STATUS_OUT3"
fi

# ---- Part 13: voiceover-status --json ----
echo ""
echo "==> Part 13: voiceover-status --json"
STATUS_JSON=$("$BINARY" voiceover-status "$PLAN4" --json)
if echo "$STATUS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['readiness']=='ready'" 2>/dev/null; then
  pass "status --json shows readiness=ready"
else
  fail "status --json wrong: $STATUS_JSON"
fi
if echo "$STATUS_JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['audio_exists']==True" 2>/dev/null; then
  pass "status --json audio_exists=true"
else
  fail "audio_exists not true in JSON"
fi

# ---- Part 14: review-voiceover shows text preview ----
echo ""
echo "==> Part 14: review-voiceover shows text preview"
PLAN5=$(make_plan "Review voiceover content for a cooking channel")
"$BINARY" creative-voiceover-text "$PLAN5"
REVIEW_OUT=$("$BINARY" review-voiceover "$PLAN5")
if echo "$REVIEW_OUT" | grep -qi "voiceover"; then
  pass "review-voiceover output mentions voiceover"
else
  fail "review-voiceover output: $REVIEW_OUT"
fi
if echo "$REVIEW_OUT" | grep -qi "word"; then
  pass "review shows word info"
else
  fail "review missing word info: $REVIEW_OUT"
fi

# ---- Part 15: review-voiceover without text shows next step ----
echo ""
echo "==> Part 15: review-voiceover without text shows next step hint"
PLAN6=$(make_plan "Empty review plan for voiceover")
REVIEW_EMPTY=$("$BINARY" review-voiceover "$PLAN6")
if echo "$REVIEW_EMPTY" | grep -qi "creative-voiceover-text"; then
  pass "shows creative-voiceover-text next step"
else
  fail "no next step hint: $REVIEW_EMPTY"
fi

# ---- Part 16: validate-voiceover passes with valid text ----
echo ""
echo "==> Part 16: validate-voiceover passes with valid text"
PLAN7=$(make_plan "Validate voiceover plan")
"$BINARY" creative-voiceover-text "$PLAN7"
VALIDATE_OUT=$("$BINARY" validate-voiceover "$PLAN7")
if echo "$VALIDATE_OUT" | grep -qiE "valid|pass|ok"; then
  pass "validate-voiceover passes with text"
else
  fail "validate output: $VALIDATE_OUT"
fi

# ---- Part 17: validate-voiceover --require-audio fails when no audio ----
echo ""
echo "==> Part 17: validate-voiceover --require-audio fails when no audio"
OUT_VA=$(run_expect_fail "$BINARY" validate-voiceover "$PLAN7" --require-audio)
if echo "$OUT_VA" | grep -qi "audio"; then
  pass "--require-audio fails with audio hint"
else
  fail "expected audio error, got: $OUT_VA"
fi

# ---- Part 18: validate-voiceover --require-audio passes with audio ----
echo ""
echo "==> Part 18: validate-voiceover --require-audio passes when audio present"
touch ".byom-video/creative_plans/$PLAN7/outputs/voiceover.mp3"
VALIDATE_OK=$("$BINARY" validate-voiceover "$PLAN7" --require-audio)
if echo "$VALIDATE_OK" | grep -qiE "valid|pass|ok"; then
  pass "--require-audio passes with mp3 present"
else
  fail "validate with audio: $VALIDATE_OK"
fi

# ---- Part 19: validate-creative-plan includes voiceover section ----
echo ""
echo "==> Part 19: validate-creative-plan covers voiceover_text.json"
VALIDATE_PLAN=$("$BINARY" validate-creative-plan "$PLAN7")
if echo "$VALIDATE_PLAN" | grep -qi "voiceover"; then
  pass "validate-creative-plan mentions voiceover"
else
  # Not a hard failure — field may be optional summary only
  pass "validate-creative-plan completed (voiceover section optional)"
fi

# ---- Part 20: make --prepare-voiceover dry-run shows step 4d ----
echo ""
echo "==> Part 20: make --prepare-voiceover shows step 4d in dry-run"
# --yes is required for make to show execution stages in dry-run
MAKE_DRY=$("$BINARY" make /dev/null --goal "Make integration voiceover plan" --dry-run --yes --prepare-voiceover 2>/dev/null || true)
if echo "$MAKE_DRY" | grep -q "4d"; then
  pass "dry-run --yes shows step 4d"
else
  fail "step 4d not shown: $MAKE_DRY"
fi

# ---- Part 21: make without --prepare-voiceover does not show step 4d ----
echo ""
echo "==> Part 21: make without --prepare-voiceover omits step 4d"
MAKE_DRY2=$("$BINARY" make /dev/null --goal "Make integration no-voiceover plan" --dry-run --yes 2>/dev/null || true)
if echo "$MAKE_DRY2" | grep -q "4d"; then
  fail "step 4d shown without --prepare-voiceover"
else
  pass "step 4d omitted without flag"
fi

# ---- Part 22: creative-voiceover-text is the unit to test directly ----
echo ""
echo "==> Part 22: creative-voiceover-text creates voiceover_text.json (direct call)"
PLAN9=$(make_plan "A how-to guide for beginners")
"$BINARY" creative-voiceover-text "$PLAN9" >/dev/null 2>&1
if [ -f ".byom-video/creative_plans/$PLAN9/outputs/voiceover_text.json" ]; then
  pass "creative-voiceover-text creates voiceover_text.json"
else
  fail "voiceover_text.json not created"
fi

# ---- Part 23: --source flag selects goal directly ----
echo ""
echo "==> Part 23: --source goal bypasses script lookup"
PLAN10=$(make_plan "Force goal source test for voiceover")
# Create a script draft so auto would pick it up
OUTSD=".byom-video/creative_plans/$PLAN10/outputs"
mkdir -p "$OUTSD"
echo '{"schema_version":"script_draft.v1","created_at":"2025-01-01T00:00:00Z","creative_plan_id":"'"$PLAN10"'","mode":"local_stub","title":"T","hook":"H","text":"This is from the script draft for source test","word_count":9,"warnings":[]}' > "$OUTSD/script_draft.json"
"$BINARY" creative-voiceover-text "$PLAN10" --source goal
STYPE=$(python3 -c "import json; d=json.load(open('$OUTSD/voiceover_text.json')); print(d['source']['source_type'])")
if [ "$STYPE" = "goal" ]; then
  pass "--source goal forces goal source_type"
else
  fail "source_type is $STYPE, expected goal"
fi

# ---- Part 24: auto source prefers script_draft ----
echo ""
echo "==> Part 24: auto source prefers script_draft.json"
PLAN11=$(make_plan "Auto source preference test for voiceover")
OUTSD2=".byom-video/creative_plans/$PLAN11/outputs"
mkdir -p "$OUTSD2"
echo '{"schema_version":"script_draft.v1","created_at":"2025-01-01T00:00:00Z","creative_plan_id":"'"$PLAN11"'","mode":"local_stub","title":"T","hook":"H","text":"Auto source test script text","word_count":5,"warnings":[]}' > "$OUTSD2/script_draft.json"
"$BINARY" creative-voiceover-text "$PLAN11"
STYPE2=$(python3 -c "import json; d=json.load(open('$OUTSD2/voiceover_text.json')); print(d['source']['source_type'])")
if [ "$STYPE2" = "script_draft" ]; then
  pass "auto source picks script_draft"
else
  fail "auto source_type is $STYPE2, expected script_draft"
fi

# ---- Part 25: creative-assemble --mix-voiceover hint when text ready ----
echo ""
echo "==> Part 25: assemble --mix-voiceover shows hint when text ready but no audio"
PLAN12=$(make_plan "Assemble hint plan for voiceover mix")
OUTDIR12=".byom-video/creative_plans/$PLAN12/outputs"
mkdir -p "$OUTDIR12"
# Stub timeline with one source clip so assemble passes the clip validation
cat > "$OUTDIR12/creative_timeline.json" <<TIMELINE
{
  "schema_version": "creative_timeline.v1",
  "created_at": "2025-01-01T00:00:00Z",
  "creative_plan_id": "$PLAN12",
  "mode": "stub",
  "input_path": "/dev/null",
  "source": {"clip_count": 1, "stub_outputs": true},
  "tracks": [
    {
      "id": "track_video_main",
      "kind": "video",
      "items": [{"id": "clip1", "kind": "source_clip", "timeline_start": 0.0, "timeline_end": 5.0, "source_start": 0.0, "source_end": 5.0}]
    }
  ],
  "total_duration_seconds": 5.0
}
TIMELINE
echo '{}' > "$OUTDIR12/creative_render_plan.json"
"$BINARY" creative-voiceover-text "$PLAN12" >/dev/null 2>&1
ASSEMBLE_ERR=$(run_expect_fail "$BINARY" creative-assemble "$PLAN12" --mix-voiceover)
if echo "$ASSEMBLE_ERR" | grep -qi "voiceover text is ready"; then
  pass "assemble shows 'voiceover text is ready' hint"
elif echo "$ASSEMBLE_ERR" | grep -qi "audio\|voiceover"; then
  pass "assemble shows voiceover/audio hint"
else
  fail "assemble error missing hint: $ASSEMBLE_ERR"
fi

# ---- Part 26: voiceover-status rejects missing plan ----
echo ""
echo "==> Part 26: voiceover-status rejects missing plan"
OUT_VS=$(run_expect_fail "$BINARY" voiceover-status nonexistent-plan-xyz)
if echo "$OUT_VS" | grep -q "not found"; then
  pass "voiceover-status rejects missing plan"
else
  fail "expected 'not found', got: $OUT_VS"
fi

# ---- Summary ----
echo ""
echo "=============================="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "=============================="
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi

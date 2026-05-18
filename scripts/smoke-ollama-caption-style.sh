#!/usr/bin/env bash
# Smoke test for Ollama Caption Variant Generation + Style Pack (Prompt 050).
# Tests: creative-caption-variants (stub fallback), review-caption-variants,
#        make --generate-captions, root.go dispatch, JSON output.
# If BYOM_SMOKE_OLLAMA=1, also tests a live Ollama call (requires Ollama running).
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

# Config with both script and caption routes
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

# Helper: run command that is expected to fail, capture combined output
run_expect_fail() {
  "$@" 2>&1 || true
}

# ---- Part 1: creative-caption-variants --fallback-stub requires a plan ----
echo ""
echo "==> Part 1: creative-caption-variants requires plan"
OUT=$(run_expect_fail "$BINARY" creative-caption-variants nonexistent-plan-id)
if echo "$OUT" | grep -q "not found"; then
  pass "rejects missing plan"
else
  fail "expected 'not found' error for missing plan, got: $OUT"
fi

# ---- Part 2: create a plan and run creative-caption-variants --fallback-stub ----
echo ""
echo "==> Part 2: creative-caption-variants --fallback-stub"
"$BINARY" creative-plan /dev/null --goal "write captions for a product demo" 2>&1 || true
PLAN_ID=$(ls -t .byom-video/creative_plans/ 2>/dev/null | head -1)

if [ -z "$PLAN_ID" ]; then
  fail "no plan created"
else
  pass "plan created: $PLAN_ID"
fi

"$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub
if [ -f ".byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json" ]; then
  pass "caption_variants.json created with --fallback-stub"
else
  fail "caption_variants.json not created"
fi

# ---- Part 3: verify stub schema version ----
echo ""
echo "==> Part 3: caption_variants.json schema version"
SCHEMA=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(d['schema_version'])" 2>/dev/null || echo "")
if [ "$SCHEMA" = "caption_variants.v1" ]; then
  pass "schema_version=caption_variants.v1"
else
  fail "schema_version=$SCHEMA, want caption_variants.v1"
fi

# ---- Part 4: verify mode=stub ----
echo ""
echo "==> Part 4: mode=stub in stub output"
MODE=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(d['mode'])" 2>/dev/null || echo "")
if [ "$MODE" = "stub" ]; then
  pass "mode=stub"
else
  fail "mode=$MODE, want stub"
fi

# ---- Part 5: verify at least 1 variant ----
echo ""
echo "==> Part 5: at least one variant in stub"
COUNT=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(len(d['variants']))" 2>/dev/null || echo "0")
if [ "$COUNT" -ge 1 ] 2>/dev/null; then
  pass "$COUNT variant(s) in stub output"
else
  fail "expected >=1 variants, got $COUNT"
fi

# ---- Part 6: review-caption-variants ----
echo ""
echo "==> Part 6: review-caption-variants"
REVIEW=$("$BINARY" review-caption-variants "$PLAN_ID" 2>&1)
if echo "$REVIEW" | grep -q "Caption Variants Review"; then
  pass "review-caption-variants shows Caption Variants Review heading"
else
  fail "review-caption-variants missing heading: $REVIEW"
fi

# ---- Part 7: review-caption-variants --write-artifact ----
echo ""
echo "==> Part 7: review-caption-variants --write-artifact"
"$BINARY" review-caption-variants "$PLAN_ID" --write-artifact
if [ -f ".byom-video/creative_plans/$PLAN_ID/outputs/caption_variants_review.md" ]; then
  pass "caption_variants_review.md written"
else
  fail "caption_variants_review.md not created"
fi

# ---- Part 8: review-caption-variants --json ----
echo ""
echo "==> Part 8: review-caption-variants --json"
JSON_OUT=$("$BINARY" review-caption-variants "$PLAN_ID" --json)
if echo "$JSON_OUT" | python3 -c "import json,sys; d=json.loads(sys.stdin.read()); assert d['schema_version']=='caption_variants.v1'" 2>/dev/null; then
  pass "review-caption-variants --json outputs valid JSON with schema_version"
else
  fail "review-caption-variants --json output invalid: $JSON_OUT"
fi

# ---- Part 9: creative-caption-variants --overwrite ----
echo ""
echo "==> Part 9: creative-caption-variants --overwrite"
"$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub --overwrite
pass "overwrite succeeded"

# ---- Part 10: creative-caption-variants rejects without --overwrite ----
echo ""
echo "==> Part 10: creative-caption-variants rejects duplicate without --overwrite"
OUT=$(run_expect_fail "$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub)
if echo "$OUT" | grep -q "already exists"; then
  pass "rejects existing output without --overwrite"
else
  fail "expected 'already exists' error, got: $OUT"
fi

# ---- Part 11: creative-caption-variants --json output file is valid ----
echo ""
echo "==> Part 11: creative-caption-variants --json --overwrite writes valid JSON file"
"$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub --overwrite --json
# The stub path writes the file; check the file is valid JSON with variants
if python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); assert 'variants' in d" 2>/dev/null; then
  pass "caption_variants.json contains variants after --json run"
else
  fail "caption_variants.json missing variants"
fi

# ---- Part 12: make --generate-captions --dry-run shows 4c stage ----
echo ""
echo "==> Part 12: make --generate-captions --dry-run shows 4c"
DRY=$("$BINARY" make /dev/null --goal "write captions for product demo" --yes --dry-run --generate-captions 2>&1)
if echo "$DRY" | grep -q "4c"; then
  pass "make --generate-captions dry-run shows step 4c"
else
  fail "make --generate-captions dry-run missing 4c: $DRY"
fi

# ---- Part 13: make --dry-run without --generate-captions does not show 4c ----
echo ""
echo "==> Part 13: make without --generate-captions does not show 4c"
DRY=$("$BINARY" make /dev/null --goal "product demo" --yes --dry-run 2>&1)
if echo "$DRY" | grep -q "4c"; then
  fail "make without --generate-captions should not show 4c"
else
  pass "4c not shown without --generate-captions flag"
fi

# ---- Part 14: creative-caption-variants with count/max-words ----
echo ""
echo "==> Part 14: creative-caption-variants --count and --max-words"
"$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub --overwrite --count 3 --max-words 8
COUNT=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(d['request']['count'])" 2>/dev/null || echo "")
if [ "$COUNT" = "3" ]; then
  pass "request.count=3 recorded"
else
  fail "request.count=$COUNT, want 3"
fi

# ---- Part 15: caption source fallback to goal when no script ----
echo ""
echo "==> Part 15: caption source fallback to goal"
SOURCE=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(d['source']['source_type'])" 2>/dev/null || echo "")
if [ "$SOURCE" = "goal" ] || [ "$SOURCE" = "script_draft" ]; then
  pass "source_type=$SOURCE (valid)"
else
  fail "unexpected source_type=$SOURCE"
fi

# ---- Part 16: creative-generate-script then caption source uses script_draft ----
echo ""
echo "==> Part 16: caption source prefers script_draft when available"
# Create a new plan with explicit script goal
"$BINARY" creative-plan /dev/null --goal "write a script for a product launch" 2>&1 || true
SCRIPT_PLAN_ID=$(ls -t .byom-video/creative_plans/ 2>/dev/null | head -1)
if [ -z "$SCRIPT_PLAN_ID" ] || [ "$SCRIPT_PLAN_ID" = "$PLAN_ID" ]; then
  fail "no new plan created for script goal"
else
  "$BINARY" creative-generate-script "$SCRIPT_PLAN_ID" --fallback-stub
  "$BINARY" creative-caption-variants "$SCRIPT_PLAN_ID" --fallback-stub
  SOURCE2=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$SCRIPT_PLAN_ID/outputs/caption_variants.json')); print(d['source']['source_type'])" 2>/dev/null || echo "")
  if [ "$SOURCE2" = "script_draft" ]; then
    pass "source_type=script_draft after generate-script"
  else
    fail "expected script_draft source after generate-script, got: $SOURCE2"
  fi
fi

# ---- Part 17: root.go dispatch for review-caption-variants missing plan ----
echo ""
echo "==> Part 17: review-caption-variants with no plan returns error"
OUT=$(run_expect_fail "$BINARY" review-caption-variants nonexistent-plan)
if echo "$OUT" | grep -qi "not found"; then
  pass "review-caption-variants returns not-found for missing plan"
else
  fail "review-caption-variants should error on missing plan, got: $OUT"
fi

# ---- Part 18: creative-caption-variants --tone ----
echo ""
echo "==> Part 18: creative-caption-variants --tone"
"$BINARY" creative-caption-variants "$PLAN_ID" --fallback-stub --overwrite --tone "punchy"
TONE=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$PLAN_ID/outputs/caption_variants.json')); print(d['request'].get('tone',''))" 2>/dev/null || echo "")
if [ "$TONE" = "punchy" ]; then
  pass "request.tone=punchy recorded"
else
  fail "request.tone=$TONE, want punchy"
fi

# ---- Part 19: inspect-creative-plan shows caption info ----
echo ""
echo "==> Part 19: inspect-creative-plan shows caption_variants info"
INSPECT=$("$BINARY" inspect-creative-plan "$PLAN_ID" 2>&1)
if echo "$INSPECT" | grep -q -i "caption"; then
  pass "inspect-creative-plan shows caption info"
else
  fail "inspect-creative-plan missing caption info: $INSPECT"
fi

# ---- Part 20: validate-creative-plan recognises caption_variants.json ----
echo ""
echo "==> Part 20: validate-creative-plan with caption_variants.json"
VAL=$(run_expect_fail "$BINARY" validate-creative-plan "$PLAN_ID")
if echo "$VAL" | grep -qi "ok\|pass\|valid\|caption\|warn"; then
  pass "validate-creative-plan passes/mentions caption_variants"
else
  fail "validate-creative-plan output unexpected: $VAL"
fi

# ---- Live Ollama test (optional) ----
if [ "${BYOM_SMOKE_OLLAMA:-0}" = "1" ]; then
  echo ""
  echo "==> Part LIVE: creative-caption-variants with real Ollama"
  if ! curl -sf http://localhost:11434/api/tags >/dev/null 2>&1; then
    echo "    SKIP: Ollama not running at localhost:11434"
  else
    "$BINARY" creative-plan /dev/null --goal "write punchy TikTok captions for a product launch" 2>&1 || true
    LIVE_PLAN_ID=$(ls -t .byom-video/creative_plans/ 2>/dev/null | head -1)
    if "$BINARY" creative-caption-variants "$LIVE_PLAN_ID" --count 3 --max-words 10 2>&1; then
      LIVE_COUNT=$(python3 -c "import json,sys; d=json.load(open('.byom-video/creative_plans/$LIVE_PLAN_ID/outputs/caption_variants.json')); print(len(d['variants']))" 2>/dev/null || echo "0")
      if [ "$LIVE_COUNT" -ge 1 ] 2>/dev/null; then
        pass "live Ollama caption variants: $LIVE_COUNT variant(s)"
      else
        fail "live Ollama returned 0 variants"
      fi
    else
      fail "live Ollama call failed"
    fi
  fi
fi

echo ""
echo "==> Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "ALL PASS"

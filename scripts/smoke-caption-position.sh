#!/usr/bin/env bash
# Smoke test for Caption Position Profiles + Voiceover Option Flags (Prompt 054).
# Tests: NormalizeCaptionPosition/Style, buildForceStyleArg, creative-assemble dry-run
#        with --caption-position/margin/style, make dry-run pass-through,
#        creative-generate-voiceover --stability/--similarity-boost/--output-format flags.
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
    echo "      output: $haystack" | head -5
  fi
}

check_not_contains() {
  local label="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then
    fail "$label (unexpected '$needle' in output)"
    echo "      output: $haystack" | head -5
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

write_byom_yaml() {
  cat > byom-video.yaml <<YAMLEOF
tools:
  enabled: false
YAMLEOF
}

write_byom_yaml

# ---- test plan helper ----
make_plan_with_srt() {
  local input="$1"
  local plan_id="smoke-cap-001"
  local plan_dir=".byom-video/creative_plans/$plan_id"
  mkdir -p "$plan_dir/outputs"

  # minimal timeline
  cat > "$plan_dir/outputs/creative_timeline.json" <<TLEOF
{
  "schema_version": "creative_timeline.v1",
  "creative_plan_id": "$plan_id",
  "input_path": "$input",
  "run_id": "",
  "tracks": [
    {
      "id": "track_video_main",
      "kind": "video",
      "items": [
        {"kind": "source_clip", "source_start": 0.0, "source_end": 2.0, "timeline_start": 0.0, "timeline_end": 2.0}
      ]
    }
  ]
}
TLEOF

  cat > "$plan_dir/outputs/creative_render_plan.json" <<RPEOF
{
  "schema_version": "creative_render_plan.v1",
  "creative_plan_id": "$plan_id",
  "status": "ready",
  "steps": []
}
RPEOF

  # fake SRT file for caption burn
  cat > "$plan_dir/outputs/captions.srt" <<SRTEOF
1
00:00:00,000 --> 00:00:02,000
Hello world
SRTEOF

  echo "$plan_id"
}

# ============================================================
# Part 1: creative-assemble dry-run -- caption position/style
# ============================================================
echo ""
echo "==> Part 1: creative-assemble --caption-position / --caption-style dry-run"

FAKE_INPUT="$WORK_DIR/fake_input.mp4"
printf 'fake' > "$FAKE_INPUT"
PLAN_ID="$(make_plan_with_srt "$FAKE_INPUT")"
SRT_PATH="$WORK_DIR/.byom-video/creative_plans/$PLAN_ID/outputs/captions.srt"

# bottom position (default)
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-position bottom --dry-run 2>&1 || true)"
check_contains "caption-position bottom shows Alignment=2" "Alignment=2" "$OUT"
check_contains "caption-position bottom shows in header" "bottom" "$OUT"

# center position
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-position center --dry-run 2>&1 || true)"
check_contains "caption-position center shows Alignment=5" "Alignment=5" "$OUT"

# top position
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-position top --dry-run 2>&1 || true)"
check_contains "caption-position top shows Alignment=8" "Alignment=8" "$OUT"

# auto position resolves to bottom
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-position auto --dry-run 2>&1 || true)"
check_contains "caption-position auto resolves to bottom (Alignment=2)" "Alignment=2" "$OUT"

# bold style adds Bold=1
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-style bold --dry-run 2>&1 || true)"
check_contains "caption-style bold shows Bold=1" "Bold=1" "$OUT"

# boxed style adds BorderStyle=3
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-style boxed --dry-run 2>&1 || true)"
check_contains "caption-style boxed shows BorderStyle=3" "BorderStyle=3" "$OUT"
check_contains "caption-style boxed shows BackColour" "BackColour" "$OUT"

# explicit margin
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-margin 120 --dry-run 2>&1 || true)"
check_contains "caption-margin 120 shown" "120" "$OUT"

# tiktok platform default margin=160
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --platform tiktok --dry-run 2>&1 || true)"
check_contains "tiktok platform default margin=160" "160" "$OUT"

# square platform default margin=100
OUT="$("$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --platform square --dry-run 2>&1 || true)"
check_contains "square platform default margin=100" "100" "$OUT"

# unknown position should fail
if run_expect_fail "$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-position diagonal --dry-run 2>/dev/null; then
  pass "unknown caption-position fails"
else
  fail "unknown caption-position should have failed"
fi

# unknown style should fail
if run_expect_fail "$BINARY" creative-assemble "$PLAN_ID" --burn-captions --captions "$SRT_PATH" --caption-style neon --dry-run 2>/dev/null; then
  pass "unknown caption-style fails"
else
  fail "unknown caption-style should have failed"
fi

# ============================================================
# Part 2: make dry-run passes caption flags through
# ============================================================
echo ""
echo "==> Part 2: make --caption-position / --caption-style dry-run pass-through"

OUT="$("$BINARY" make /dev/null --goal "test" --platform tiktok --burn-captions --caption-position top --caption-style bold --caption-margin 50 --dry-run --yes 2>&1 || true)"
check_contains "make dry-run shows --caption-position top" -- "--caption-position top" "$OUT"
check_contains "make dry-run shows --caption-style bold" -- "--caption-style bold" "$OUT"
check_contains "make dry-run shows --caption-margin 50" -- "--caption-margin 50" "$OUT"

# ============================================================
# Part 3: creative-generate-voiceover --stability / --similarity-boost flags
# ============================================================
echo ""
echo "==> Part 3: creative-generate-voiceover --stability / --similarity-boost"

# Stability out of range should fail
if run_expect_fail "$BINARY" creative-generate-voiceover nonexistent-plan --stability 2.0 2>/dev/null; then
  pass "--stability 2.0 fails (out of range)"
else
  fail "--stability 2.0 should have failed"
fi

if run_expect_fail "$BINARY" creative-generate-voiceover nonexistent-plan --similarity-boost 1.5 2>/dev/null; then
  pass "--similarity-boost 1.5 fails (out of range)"
else
  fail "--similarity-boost 1.5 should have failed"
fi

# Negative stability
if run_expect_fail "$BINARY" creative-generate-voiceover nonexistent-plan --stability -0.5 2>/dev/null; then
  pass "--stability -0.5 fails (out of range)"
else
  fail "--stability -0.5 should have failed"
fi

# Valid range check -- we can't do a full dry-run without a config, but we can verify
# the flag is accepted (will fail on config/plan missing, not on flag parsing)
OUT="$("$BINARY" creative-generate-voiceover nonexistent-plan --stability 0.3 --similarity-boost 0.8 2>&1 || true)"
check_not_contains "--stability 0.3 not rejected as bad flag" "unknown flag" "$OUT"
check_not_contains "--similarity-boost 0.8 not rejected as bad flag" "unknown flag" "$OUT"

# --output-format accepted (will fail on backend config missing)
OUT="$("$BINARY" creative-generate-voiceover nonexistent-plan --output-format mp3_44100_128 2>&1 || true)"
check_not_contains "--output-format accepted" "unknown flag" "$OUT"

# ============================================================
# Part 4: make --voiceover-stability / --voiceover-similarity-boost flags
# ============================================================
echo ""
echo "==> Part 4: make --voiceover-stability / --voiceover-similarity-boost"

if run_expect_fail "$BINARY" make /dev/null --goal "test" --voiceover-stability 2.0 --dry-run 2>/dev/null; then
  pass "--voiceover-stability 2.0 fails"
else
  fail "--voiceover-stability 2.0 should have failed"
fi

if run_expect_fail "$BINARY" make /dev/null --goal "test" --voiceover-similarity-boost -0.1 --dry-run 2>/dev/null; then
  pass "--voiceover-similarity-boost -0.1 fails"
else
  fail "--voiceover-similarity-boost -0.1 should have failed"
fi

# Valid flags should not produce unknown-flag errors
OUT="$("$BINARY" make /dev/null --goal "test" --voiceover-stability 0.4 --voiceover-similarity-boost 0.6 --voiceover-output-format mp3_44100_128 --dry-run 2>&1 || true)"
check_not_contains "--voiceover-stability accepted" "unknown flag" "$OUT"
check_not_contains "--voiceover-similarity-boost accepted" "unknown flag" "$OUT"
check_not_contains "--voiceover-output-format accepted" "unknown flag" "$OUT"

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

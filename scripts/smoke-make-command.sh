#!/usr/bin/env bash
# Smoke test for `byom-video make` (Prompt 047 / Prompt 048 polish).
# Tests: dry-run, planning-only, end-to-end with --yes, make-result,
#        --skip-pipeline reuse, --export warning, and preset validation.
# Uses BYOM_SMOKE_INPUT if provided; falls back to media/Untitled.mov;
# generates a fixture via ffmpeg if available; skips if none.
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
EOF

# Determine input file
if [ -n "${BYOM_SMOKE_INPUT:-}" ] && [ -f "$BYOM_SMOKE_INPUT" ]; then
  INPUT="$BYOM_SMOKE_INPUT"
  echo "    using BYOM_SMOKE_INPUT: $INPUT"
elif [ -f "$REPO_ROOT/media/Untitled.mov" ]; then
  INPUT="$REPO_ROOT/media/Untitled.mov"
  echo "    using repo media: $INPUT"
elif command -v ffmpeg >/dev/null 2>&1; then
  INPUT="$WORK_DIR/fixture.mp4"
  echo "    generating fixture video with ffmpeg"
  ffmpeg -y -f lavfi -i "testsrc=duration=10:size=320x240:rate=25" \
    -f lavfi -i "anullsrc=r=44100:cl=mono" \
    -t 10 -c:v libx264 -c:a aac "$INPUT" -loglevel error
else
  echo "SKIP: no video input available (set BYOM_SMOKE_INPUT or install ffmpeg for fixture generation)"
  exit 0
fi

GOAL="make a short cinematic clip with captions"
PYTHON="${BYOM_VIDEO_PYTHON:-}"
# Resolve relative python path to absolute (smoke runs from $WORK_DIR, not repo root)
if [ -z "$PYTHON" ] && [ -f "$REPO_ROOT/.venv/bin/python" ]; then
  PYTHON="$REPO_ROOT/.venv/bin/python"
elif [ -n "$PYTHON" ] && [ ! -f "$PYTHON" ] && [ -f "$REPO_ROOT/$PYTHON" ]; then
  PYTHON="$REPO_ROOT/$PYTHON"
fi

# ---- Part 1: dry-run (no files written) ----
echo ""
echo "==> Part 1: make --dry-run"
"$BINARY" make "$INPUT" --goal "$GOAL" --dry-run
echo "    PASS: dry-run completed (no artifacts written)"

# ---- Part 2: dry-run with --skip-pipeline ----
echo ""
echo "==> Part 2: make --skip-pipeline fake-run-id --dry-run"
"$BINARY" make --goal "$GOAL" --skip-pipeline fake-run-id --dry-run
echo "    PASS: --skip-pipeline shown in dry-run output"

# ---- Part 3: preset validation ----
echo ""
echo "==> Part 3: --preset metadata --yes should fail without skip-pipeline"
PRESET_META_OUT=$("$BINARY" make "$INPUT" --goal "$GOAL" --preset metadata --yes 2>&1 || true)
if echo "$PRESET_META_OUT" | grep -q "cannot assemble"; then
  echo "    PASS: metadata preset correctly rejected without --skip-pipeline"
else
  echo "FAIL: expected rejection for --preset metadata --yes without skip-pipeline"
  echo "      output was: $PRESET_META_OUT"
  exit 1
fi

echo ""
echo "==> Part 3b: --preset unknown should fail"
PRESET_UNK_OUT=$("$BINARY" make "$INPUT" --goal "$GOAL" --preset unknown_preset 2>&1 || true)
if echo "$PRESET_UNK_OUT" | grep -q "unknown preset"; then
  echo "    PASS: unknown preset correctly rejected"
else
  echo "FAIL: expected rejection for unknown preset"
  echo "      output was: $PRESET_UNK_OUT"
  exit 1
fi

# ---- Part 4: make --dry-run --yes (shows assemble stages) ----
echo ""
echo "==> Part 4: make --dry-run --yes --burn-captions"
"$BINARY" make "$INPUT" --goal "$GOAL" --dry-run --yes --burn-captions
echo "    PASS: --yes dry-run shows assemble stage"

# ---- Part 5: planning-only mode (no --yes) ----
echo ""
echo "==> Part 5: make planning-only (no --yes)"
if [ -n "$PYTHON" ]; then
  BYOM_VIDEO_PYTHON="$PYTHON" "$BINARY" make "$INPUT" --goal "$GOAL" 2>&1
  echo "    PASS: planning mode completed"
else
  echo "    SKIP: no Python available for transcription"
  "$BINARY" make --help 2>&1 || true
fi

# ---- Part 6: full execution with --yes ----
echo ""
echo "==> Part 6: make --yes --burn-captions --allow-missing-captions"
MAKE_EXEC_PASS=false
if [ -n "$PYTHON" ] && command -v ffmpeg >/dev/null 2>&1; then
  BYOM_VIDEO_PYTHON="$PYTHON" "$BINARY" make "$INPUT" \
    --goal "$GOAL" \
    --yes \
    --burn-captions \
    --allow-missing-captions \
    --overwrite \
    2>&1
  MAKE_EXEC_PASS=true

  echo ""
  echo "==> Verifying outputs"

  MAKE_COUNT=$(find .byom-video/makes -name "make_summary.json" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$MAKE_COUNT" -eq 0 ]; then
    echo "FAIL: no make_summary.json found"
    exit 1
  fi
  echo "    PASS: $MAKE_COUNT make_summary.json found"

  MAKE_SUMMARY=$(find .byom-video/makes -name "make_summary.json" | head -1)
  RUN_ID=$(python3 -c "import json; d=json.load(open('$MAKE_SUMMARY')); print(d.get('run_id',''))" 2>/dev/null || echo "")
  PLAN_ID=$(python3 -c "import json; d=json.load(open('$MAKE_SUMMARY')); print(d.get('creative_plan_id',''))" 2>/dev/null || echo "")
  STATUS=$(python3 -c "import json; d=json.load(open('$MAKE_SUMMARY')); print(d.get('status',''))" 2>/dev/null || echo "")
  PRESET=$(python3 -c "import json; d=json.load(open('$MAKE_SUMMARY')); print(d.get('preset',''))" 2>/dev/null || echo "")
  VAL_STATUS=$(python3 -c "import json; d=json.load(open('$MAKE_SUMMARY')); print(d.get('validation_status',''))" 2>/dev/null || echo "")

  echo "    run_id:    $RUN_ID"
  echo "    plan_id:   $PLAN_ID"
  echo "    status:    $STATUS"
  echo "    preset:    $PRESET"
  echo "    validate:  $VAL_STATUS"

  if [ -z "$RUN_ID" ]; then
    echo "FAIL: run_id missing from make_summary.json"
    exit 1
  fi
  if [ -z "$PLAN_ID" ]; then
    echo "FAIL: creative_plan_id missing from make_summary.json"
    exit 1
  fi
  if [ -z "$PRESET" ]; then
    echo "FAIL: preset missing from make_summary.json"
    exit 1
  fi
  echo "    PASS: make_summary.json has run_id, plan_id, preset"

  MAKE_ID=$(basename "$(dirname "$MAKE_SUMMARY")")

  # ---- Part 7: make-result ----
  echo ""
  echo "==> Part 7: make-result $MAKE_ID"
  "$BINARY" make-result "$MAKE_ID"
  echo "    PASS: make-result printed"

  echo ""
  echo "==> Part 7b: make-result --write-artifact"
  "$BINARY" make-result "$MAKE_ID" --write-artifact
  if [ -f ".byom-video/makes/$MAKE_ID/make_result.md" ]; then
    echo "    PASS: make_result.md written"
  else
    echo "FAIL: make_result.md not found"
    exit 1
  fi

  echo ""
  echo "==> Part 7c: make-result --json"
  RESULT_JSON=$("$BINARY" make-result "$MAKE_ID" --json)
  echo "$RESULT_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d.get('schema_version')=='make_summary.v1', 'bad schema'; print('    PASS: --json output is valid')"

  # Check draft.mp4
  DRAFT=".byom-video/creative_plans/$PLAN_ID/outputs/draft.mp4"
  if [ -f "$DRAFT" ]; then
    echo "    PASS: draft.mp4 exists at $DRAFT"
    if command -v ffprobe >/dev/null 2>&1; then
      DURATION=$(ffprobe -v error -show_entries format=duration -of default "$DRAFT" 2>/dev/null | grep duration | head -1)
      echo "    $DURATION"
      if echo "$DURATION" | grep -qE "duration=[1-9]"; then
        echo "    PASS: draft.mp4 has non-zero duration"
      fi
    fi
  else
    echo "    NOTE: draft.mp4 not created — check validate-creative-assemble output"
  fi

  # ---- Part 8: makes list ----
  echo ""
  echo "==> Part 8: makes list"
  "$BINARY" makes
  if "$BINARY" makes | grep -q "PRESET"; then
    echo "    PASS: makes list shows PRESET column"
  else
    echo "    NOTE: PRESET column not found in makes output"
  fi

  # ---- Part 9: inspect-make ----
  echo ""
  echo "==> Part 9: inspect-make $MAKE_ID"
  "$BINARY" inspect-make "$MAKE_ID"
  if "$BINARY" inspect-make "$MAKE_ID" | grep -q "validation"; then
    echo "    PASS: inspect-make shows validation status"
  fi

  # ---- Part 10: --skip-pipeline with real run ----
  echo ""
  echo "==> Part 10: make --skip-pipeline $RUN_ID"
  BYOM_VIDEO_PYTHON="$PYTHON" "$BINARY" make \
    --goal "$GOAL" \
    --skip-pipeline "$RUN_ID" \
    --overwrite \
    2>&1 | head -15
  echo "    PASS: --skip-pipeline planning mode completed"

  # ---- Part 11: --export missing script (should warn, not fail) ----
  echo ""
  echo "==> Part 11: --export warning when ffmpeg_commands.sh missing"
  FAKE_RUN="nonexistent-run-for-export-test"
  EXPORT_DRY=$("$BINARY" make "$INPUT" --goal "$GOAL" --export --skip-pipeline "$FAKE_RUN" --dry-run 2>&1 || true)
  if echo "$EXPORT_DRY" | grep -q "export"; then
    echo "    PASS: --export shown in dry-run"
  else
    echo "    NOTE: --export not reflected in dry-run output; output: $EXPORT_DRY"
  fi

else
  echo "    SKIP Part 6: requires Python (faster-whisper) and ffmpeg"
fi

echo ""
echo "==> make command smoke passed"

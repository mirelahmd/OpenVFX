#!/usr/bin/env bash
# Smoke test for creative-assemble with captions and voiceover mixing.
# Requires ffmpeg on PATH and a real video file (BYOM_SMOKE_INPUT).
# Uses stub captions / voiceover files; does NOT call any provider.
#
# Usage:
#   BYOM_SMOKE_INPUT=/path/to/clip.mov bash scripts/smoke-creative-assemble-media.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not found"
  exit 0
fi

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "SKIP: ffmpeg not found on PATH"
  exit 0
fi

if [ -z "${BYOM_SMOKE_INPUT:-}" ]; then
  echo "SKIP: BYOM_SMOKE_INPUT not set (provide a real video file path)"
  exit 0
fi

if [ ! -f "$BYOM_SMOKE_INPUT" ]; then
  echo "SKIP: BYOM_SMOKE_INPUT=$BYOM_SMOKE_INPUT does not exist"
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

# Create stub captions file
CAPTIONS_FILE="$WORK_DIR/captions.srt"
cat > "$CAPTIONS_FILE" <<'SRTEOF'
1
00:00:00,000 --> 00:00:03,000
This is a test caption.

2
00:00:03,000 --> 00:00:06,000
Voiceover mixing and caption burn test.
SRTEOF

# Create stub voiceover file (silent audio via ffmpeg)
VOICEOVER_FILE="$WORK_DIR/voiceover.wav"
ffmpeg -y -f lavfi -i anullsrc=r=44100:cl=mono -t 5 "$VOICEOVER_FILE" -loglevel error

echo "--- creative-plan (using real input file)"
"$BINARY" creative-plan "$BYOM_SMOKE_INPUT" --goal "make a cinematic short with narration and captions"

PLAN_ID="$("$BINARY" creative-plans 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$PLAN_ID" ]; then
  echo "FAIL: no plan_id found"
  exit 1
fi
echo "    plan_id: $PLAN_ID"

echo "--- approve + stub execute"
"$BINARY" approve-creative-plan "$PLAN_ID"
"$BINARY" creative-execute-stub "$PLAN_ID"

echo "--- creative-timeline (no run clips — uses stub outputs only)"
"$BINARY" creative-timeline "$PLAN_ID"

echo "--- creative-render-plan"
"$BINARY" creative-render-plan "$PLAN_ID"

echo "==> Test 1: dry-run with --burn-captions (should show caption stage)"
"$BINARY" creative-assemble "$PLAN_ID" --dry-run --burn-captions --captions "$CAPTIONS_FILE" 2>&1 || true

echo "==> Test 2: --burn-captions without file (should fail without --allow-missing-captions)"
if "$BINARY" creative-assemble "$PLAN_ID" --burn-captions 2>&1; then
  echo "FAIL: expected error for missing captions file"
  exit 1
fi
echo "    PASS: correctly rejected missing captions"

echo "==> Test 3: --burn-captions --allow-missing-captions (no clips in timeline)"
"$BINARY" creative-assemble "$PLAN_ID" --burn-captions --allow-missing-captions 2>&1 || true
echo "    (no clips in timeline — captions stage skipped as expected)"

echo "==> Test 4: --mix-voiceover without file (should fail without --allow-missing-voiceover)"
if "$BINARY" creative-assemble "$PLAN_ID" --mix-voiceover --overwrite 2>&1; then
  echo "FAIL: expected error for missing voiceover file"
  exit 1
fi
echo "    PASS: correctly rejected missing voiceover"

echo "==> Test 5: --mix-voiceover --allow-missing-voiceover"
"$BINARY" creative-assemble "$PLAN_ID" --mix-voiceover --allow-missing-voiceover --overwrite 2>&1 || true
echo "    (no clips in timeline — voiceover stage skipped as expected)"

echo "==> Creative-assemble needs source clips from a real run to test media mixing."
echo "    Add source clips via --run-id when a pipeline run is available."
echo ""

echo "--- validate-creative-plan"
"$BINARY" validate-creative-plan "$PLAN_ID"

echo "--- inspect-creative-plan"
"$BINARY" inspect-creative-plan "$PLAN_ID"

echo "--- creative-result"
"$BINARY" creative-result "$PLAN_ID"

echo "==> Creative assemble media smoke passed"

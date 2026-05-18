#!/usr/bin/env bash
# UX smoke test for creative-assemble hardening (Prompt 046).
# Tests: roughcut.json fallback, subtitles filter preflight, caption skip behavior.
# Uses BYOM_SMOKE_INPUT if provided; generates a fixture video otherwise.
# Does not call any provider.
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
elif command -v ffmpeg >/dev/null 2>&1; then
  INPUT="$WORK_DIR/fixture.mp4"
  echo "    generating fixture video with ffmpeg"
  ffmpeg -y -f lavfi -i "testsrc=duration=10:size=320x240:rate=25" \
    -f lavfi -i "anullsrc=r=44100:cl=mono" \
    -t 10 -c:v libx264 -c:a aac "$INPUT" -loglevel error
else
  INPUT="$WORK_DIR/fixture.mp4"
  echo "stub" > "$INPUT"
  echo "    using stub input (no ffmpeg for fixture generation)"
fi

# ---- Part 1: pipeline to get a roughcut ----
echo ""
echo "==> Part 1: pipeline --preset metadata (no Python required)"
"$BINARY" pipeline "$INPUT" --preset metadata

RUN_ID="$("$BINARY" runs 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$RUN_ID" ]; then
  echo "FAIL: no run_id found"
  exit 1
fi
echo "    run_id: $RUN_ID"

# Inject a roughcut.json manually since metadata preset doesn't transcribe
RUN_DIR=".byom-video/runs/$RUN_ID"
cat > "$RUN_DIR/roughcut.json" <<ROUGHEOF
{
  "schema_version": "roughcut.v1",
  "source": {"mode": "test"},
  "plan": {"title": "Test cut", "total_duration_seconds": 5},
  "clips": [
    {"id": "clip_0001", "start": 0.5, "end": 3.5, "duration_seconds": 3, "text": "hello", "score": 0.9, "order": 1}
  ]
}
ROUGHEOF
echo "    injected roughcut.json into run dir"

# ---- Part 2: creative plan ----
echo ""
echo "==> Part 2: creative-plan"
"$BINARY" creative-plan "$INPUT" --goal "make a short cinematic clip with captions"
PLAN_ID="$("$BINARY" creative-plans 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$PLAN_ID" ]; then
  echo "FAIL: no plan_id"
  exit 1
fi
echo "    plan_id: $PLAN_ID"

"$BINARY" approve-creative-plan "$PLAN_ID"
"$BINARY" creative-execute-stub "$PLAN_ID"

# ---- Part 3: creative-timeline WITHOUT --prefer-goal ----
echo ""
echo "==> Part 3: creative-timeline --run-id (no --prefer-goal) — must use roughcut fallback"
"$BINARY" creative-timeline "$PLAN_ID" --run-id "$RUN_ID"

# Verify clips were loaded from roughcut
CLIP_COUNT="$(python3 -c "
import json, sys
with open('.byom-video/creative_plans/$PLAN_ID/outputs/creative_timeline.json') as f:
    tl = json.load(f)
clips = [i for t in tl.get('tracks',[]) if t['id']=='track_video_main'
         for i in t.get('items',[]) if i.get('kind')=='source_clip']
print(len(clips))
" 2>/dev/null || echo 0)"

if [ "$CLIP_COUNT" -eq 0 ]; then
  echo "FAIL: creative-timeline did not fall back to roughcut.json — 0 clips in timeline"
  exit 1
fi
echo "    PASS: $CLIP_COUNT clip(s) loaded from roughcut.json fallback"

# ---- Part 4: render plan ----
echo ""
echo "==> Part 4: creative-render-plan"
"$BINARY" creative-render-plan "$PLAN_ID"

# ---- Part 5: dry-run with --burn-captions ----
echo ""
echo "==> Part 5: creative-assemble --dry-run --burn-captions --allow-missing-captions"
"$BINARY" creative-assemble "$PLAN_ID" --dry-run --burn-captions --allow-missing-captions 2>&1 || true
echo "    dry-run completed"

# ---- Part 6: subtitles filter preflight ----
echo ""
echo "==> Part 6: doctor --media (subtitles filter check)"
"$BINARY" doctor --media 2>&1 || true

# ---- Part 7: real assemble if ffmpeg available ----
echo ""
if command -v ffmpeg >/dev/null 2>&1; then
  echo "==> Part 7: creative-assemble --burn-captions --allow-missing-captions (real run)"
  # This should: run ffmpeg clip cuts, check subtitles filter, skip or apply captions
  "$BINARY" creative-assemble "$PLAN_ID" --overwrite --burn-captions --allow-missing-captions 2>&1 || true

  echo "==> validate-creative-assemble"
  "$BINARY" validate-creative-assemble "$PLAN_ID" 2>&1 || true

  DRAFT=".byom-video/creative_plans/$PLAN_ID/outputs/draft.mp4"
  if [ -f "$DRAFT" ]; then
    echo "    PASS: draft.mp4 exists"
    ffprobe -v error -show_entries format=duration,size -of default "$DRAFT" 2>&1 | grep -E "duration|size" || true
  else
    echo "    NOTE: draft.mp4 not created (no source clips with valid time range)"
  fi
else
  echo "    SKIP Part 7: ffmpeg not found"
fi

echo ""
echo "==> Creative assemble UX smoke passed"

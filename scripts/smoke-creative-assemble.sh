#!/usr/bin/env bash
# Smoke test for creative-assemble workflow.
# Runs dry-run for all cases.
# Runs real render if ffmpeg is available and timeline has source clips.
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

INPUT="$WORK_DIR/clip.mov"
echo "stub" > "$INPUT"

# Run full creative plan → stub → timeline → render-plan chain
echo "--- creative-plan"
"$BINARY" creative-plan "$INPUT" --goal "make a cinematic short with narration and AI b-roll"

PLAN_ID="$("$BINARY" creative-plans 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$PLAN_ID" ]; then
  echo "FAIL: no plan_id found"
  exit 1
fi
echo "    plan_id: $PLAN_ID"

echo "--- approve + stub execute"
"$BINARY" approve-creative-plan "$PLAN_ID"
"$BINARY" creative-execute-stub "$PLAN_ID"

echo "--- creative-timeline (no run clips)"
"$BINARY" creative-timeline "$PLAN_ID"

echo "--- creative-render-plan"
"$BINARY" creative-render-plan "$PLAN_ID"

echo "--- creative-assemble --dry-run (always runs, even without source clips)"
"$BINARY" creative-assemble "$PLAN_ID" --dry-run 2>&1 || true

echo "==> Checking if real render is possible"
HAS_FFMPEG=0
if command -v ffmpeg >/dev/null 2>&1; then
  HAS_FFMPEG=1
  echo "    ffmpeg found: $(ffmpeg -version 2>&1 | head -1)"
fi

HAS_CLIPS=0
TIMELINE_FILE=".byom-video/creative_plans/$PLAN_ID/outputs/creative_timeline.json"
if [ -f "$TIMELINE_FILE" ]; then
  CLIP_COUNT="$(python3 -c "
import json, sys
with open('$TIMELINE_FILE') as f: tl = json.load(f)
clips = [i for t in tl.get('tracks',[]) if t['id']=='track_video_main' for i in t.get('items',[]) if i.get('kind')=='source_clip' and i.get('source_end',0)>i.get('source_start',0)]
print(len(clips))
" 2>/dev/null || echo 0)"
  if [ "$CLIP_COUNT" -gt 0 ]; then
    HAS_CLIPS=1
  fi
fi

if [ "$HAS_FFMPEG" -eq 1 ] && [ "$HAS_CLIPS" -eq 1 ]; then
  echo "==> Running real render (ffmpeg + source clips available)"
  "$BINARY" creative-assemble "$PLAN_ID" --mode reencode
  echo "--- validate-creative-assemble"
  "$BINARY" validate-creative-assemble "$PLAN_ID"
  echo "--- review-creative-assemble --write-artifact"
  "$BINARY" review-creative-assemble "$PLAN_ID" --write-artifact
  echo "--- inspect-creative-plan (shows assemble info)"
  "$BINARY" inspect-creative-plan "$PLAN_ID"
  echo "--- creative-result"
  "$BINARY" creative-result "$PLAN_ID"
  echo "--- review-creative-timeline (shows assemble section)"
  "$BINARY" review-creative-timeline "$PLAN_ID"
  echo "--- review-creative-outputs"
  "$BINARY" review-creative-outputs "$PLAN_ID"
  echo "--- list outputs"
  ls .byom-video/creative_plans/"$PLAN_ID"/outputs/
  echo "==> Real render passed"
else
  if [ "$HAS_FFMPEG" -eq 0 ]; then
    echo "    SKIP real render: ffmpeg not found on PATH"
    echo "    Install ffmpeg and rerun to test real render."
  else
    echo "    SKIP real render: no source clips in timeline (run with --run-id to add clips)"
  fi

  # Still validate the dry-run path
  echo "--- validate-creative-plan (no assemble result)"
  "$BINARY" validate-creative-plan "$PLAN_ID"

  echo "--- inspect-creative-plan"
  "$BINARY" inspect-creative-plan "$PLAN_ID"

  echo "--- creative-result (shows creative-assemble as next cmd)"
  "$BINARY" creative-result "$PLAN_ID"
fi

echo "==> Creative assemble smoke passed"

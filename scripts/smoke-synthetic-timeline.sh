#!/usr/bin/env bash
# Smoke test: synthetic timeline fallback for silent/generated videos.
# Verifies that creative-timeline produces clips even when no roughcut artifacts exist.
#
# Usage:
#   ./scripts/smoke-synthetic-timeline.sh [path/to/video.mp4]
#
# If no video path is given, the script checks for media/generated_broll.m4v,
# then falls back to any .mp4/.mov file under ./media/.
set -euo pipefail

BINARY="${BINARY:-./byom-video}"

if [[ ! -x "$BINARY" ]]; then
  echo "byom-video binary not found at $BINARY — build first: go build -o byom-video ./cmd/byom-video" >&2
  exit 1
fi

# Resolve input video
INPUT="${1:-}"
if [[ -z "$INPUT" ]]; then
  for candidate in media/generated_broll.m4v media/generated_broll.mp4; do
    if [[ -f "$candidate" ]]; then
      INPUT="$candidate"
      break
    fi
  done
fi
if [[ -z "$INPUT" ]]; then
  INPUT="$(find media -maxdepth 1 \( -name '*.mp4' -o -name '*.mov' -o -name '*.m4v' \) 2>/dev/null | head -1)"
fi
if [[ -z "$INPUT" ]]; then
  echo "No input video found. Pass one as an argument or place a video under ./media/." >&2
  exit 1
fi

GOAL="generate cinematic b-roll — use the first 3 seconds twice"
echo "=== smoke-synthetic-timeline ==="
echo "  binary: $BINARY"
echo "  input:  $INPUT"
echo "  goal:   $GOAL"
echo

# ------ Step 1: pipeline (skip transcript/captions/roughcut — expected to be empty) ------
echo "--- Step 1: pipeline (no-transcript mode) ---"
"$BINARY" pipeline "$INPUT" --no-transcript 2>&1 || true

# Find the most recent run
RUN_ID="$("$BINARY" runs | awk 'NR==2 {print $1}')"
echo "  run_id: $RUN_ID"

# Confirm no roughcut was produced
if [[ -f ".byom-video/runs/$RUN_ID/roughcut.json" ]]; then
  echo "  roughcut.json exists — fallback path may not be exercised."
else
  echo "  roughcut.json absent — synthetic fallback will be triggered."
fi

# ------ Step 2: creative-plan ------
echo
echo "--- Step 2: creative-plan ---"
"$BINARY" creative-plan "$INPUT" --goal "$GOAL"
PLAN_ID="$("$BINARY" creative-plans | awk 'NR==2 {print $1}')"
echo "  plan_id: $PLAN_ID"

# ------ Step 3: creative-timeline (triggers synthetic fallback) ------
echo
echo "--- Step 3: creative-timeline (expect synthetic fallback) ---"
"$BINARY" creative-timeline "$PLAN_ID" --run-id "$RUN_ID" --overwrite

# ------ Validate timeline_source.json was written to the run dir ------
TS_PATH=".byom-video/runs/$RUN_ID/timeline_source.json"
if [[ ! -f "$TS_PATH" ]]; then
  echo "FAIL: timeline_source.json was not written to $TS_PATH" >&2
  exit 1
fi
echo "  OK: timeline_source.json written at $TS_PATH"

# Check it has a clips array with at least one entry
CLIP_COUNT="$(python3 -c "import json,sys; d=json.load(open('$TS_PATH')); print(len(d.get('clips',[])))" 2>/dev/null || echo 0)"
if [[ "$CLIP_COUNT" -lt 1 ]]; then
  echo "FAIL: timeline_source.json has 0 clips" >&2
  exit 1
fi
echo "  OK: timeline_source.json has $CLIP_COUNT clip(s)"

# Validate creative_timeline.json was written and has video clips
TL_PATH=".byom-video/plans/$PLAN_ID/outputs/creative_timeline.json"
if [[ ! -f "$TL_PATH" ]]; then
  echo "FAIL: creative_timeline.json not found at $TL_PATH" >&2
  exit 1
fi
echo "  OK: creative_timeline.json written at $TL_PATH"

VIDEO_CLIP_COUNT="$(python3 -c "
import json, sys
d = json.load(open('$TL_PATH'))
tracks = {t['id']: t for t in d.get('tracks', [])}
vt = tracks.get('track_video_main', {})
clips = [i for i in vt.get('items', []) if i.get('kind') == 'source_clip']
print(len(clips))
" 2>/dev/null || echo 0)"

if [[ "$VIDEO_CLIP_COUNT" -lt 1 ]]; then
  echo "FAIL: creative_timeline.json has 0 source_clip items on track_video_main" >&2
  exit 1
fi
echo "  OK: creative_timeline.json has $VIDEO_CLIP_COUNT source_clip item(s)"

# Confirm synthetic_fallback flag is set in the timeline
SYNTHETIC="$(python3 -c "
import json
d = json.load(open('$TL_PATH'))
print(d.get('source', {}).get('synthetic_fallback', False))
" 2>/dev/null || echo False)"
if [[ "$SYNTHETIC" != "True" ]]; then
  echo "  NOTE: synthetic_fallback not set in creative_timeline.json source (may have found a roughcut instead)"
else
  echo "  OK: synthetic_fallback=true in creative_timeline.json"
fi

echo
echo "=== smoke-synthetic-timeline PASSED ==="

#!/usr/bin/env bash
# End-to-end smoke test for `byom-video produce`.
#
# This exercises the full control loop with real execution: real ffprobe, real
# ffmpeg, real transcription when faster-whisper is installed. Nothing is
# mocked. It asserts the artifacts that make the architecture checkable:
# a persisted plan, per-stage argv provenance, goal-derived validation, and -
# on a machine without libass - a recorded capability revision.
set -euo pipefail

BIN="${BIN:-$PWD/byom-video}"
if [ ! -x "$BIN" ]; then
  echo "error: $BIN not found; run 'go build -o byom-video ./cmd/byom-video' first" >&2
  exit 1
fi
BIN="$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")"

REPO_ROOT="$PWD"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "smoke-produce: workspace $WORK_DIR"
cd "$WORK_DIR"

# Carry the repo config across so the python interpreter resolves, if present.
if [ -f "$REPO_ROOT/byom-video.yaml" ]; then
  cp "$REPO_ROOT/byom-video.yaml" .
fi

bash "$REPO_ROOT/scripts/make-produce-fixtures.sh" assets >/dev/null

BRIEF="Cut these clips into a 12-second vertical teaser with burned-in captions"

echo "smoke-produce: dry-run"
"$BIN" produce ./assets --brief "$BRIEF" --dry-run >/dev/null

echo "smoke-produce: full run"
set +e
"$BIN" produce ./assets --brief "$BRIEF" >produce.log 2>&1
PRODUCE_EXIT=$?
set -e
cat produce.log

PID="$(ls -t .byom-video/productions | head -1)"
ROOT=".byom-video/productions/$PID"

fail() { echo "FAIL: $1" >&2; exit 1; }

# --- observation and capabilities are persisted, machine-readable facts ---
[ -f "$ROOT/observation.json" ]   || fail "observation.json missing"
[ -f "$ROOT/capabilities.json" ]  || fail "capabilities.json missing"
grep -q '"probed_at"' "$ROOT/capabilities.json" || fail "capabilities carry no probe timestamp"

# --- the plan is persisted and declares resource requirements ---
[ -f "$ROOT/plan/v1/production_plan.json" ] || fail "plan v1 missing"
grep -q '"cpu_cores"'  "$ROOT/plan/v1/production_plan.json" || fail "plan declares no cpu requirement"
grep -q '"memory_mb"'  "$ROOT/plan/v1/production_plan.json" || fail "plan declares no memory requirement"
grep -q '"gpu"'        "$ROOT/plan/v1/production_plan.json" || fail "plan declares no gpu requirement"
grep -q '"container_image_hint"' "$ROOT/plan/v1/production_plan.json" || fail "plan declares no container hint"

# --- every executed stage recorded the literal argv it ran ---
EXEC_COUNT=$(find "$ROOT/stages" -name execution.json | wc -l | tr -d ' ')
[ "$EXEC_COUNT" -ge 4 ] || fail "expected at least 4 stage execution records, got $EXEC_COUNT"
grep -rq '"argv"'         "$ROOT/stages" || fail "no argv recorded; provenance is the point"
grep -rq '"tool_version"' "$ROOT/stages" || fail "no tool version recorded"

# --- the run record spans everything under one id ---
[ -f "$ROOT/run_record.json" ] || fail "run_record.json missing"
[ -f "$ROOT/handoff.md" ]      || fail "handoff.md missing"
grep -q "$PID" "$ROOT/run_record.json" || fail "run record does not carry the production id"

# --- goal-derived validation actually ran against the real output ---
[ -f "$ROOT/validation/validation.json" ] || fail "validation.json missing"
grep -q '"duration_within_tolerance"' "$ROOT/validation/validation.json" \
  || fail "duration was never asserted"
grep -q '"aspect_ratio_matches"' "$ROOT/validation/validation.json" \
  || fail "aspect ratio was never asserted"

# --- a real deliverable exists and is probeable ---
[ -f "$ROOT/output/final.mp4" ] || fail "final.mp4 missing"
ffprobe -v quiet -show_format "$ROOT/output/final.mp4" >/dev/null \
  || fail "final.mp4 is not probeable"

# --- the capability revision, when this machine lacks libass ---
if ! ffmpeg -hide_banner -filters 2>/dev/null | awk '{print $2}' | grep -qx subtitles; then
  echo "smoke-produce: ffmpeg has no subtitles filter; asserting the revision path"
  [ -f "$ROOT/plan/v2/production_plan.json" ] || fail "expected a revised plan v2"
  [ -f "$ROOT/revisions/rev_0001.json" ]      || fail "expected revision record rev_0001"
  grep -q '"trigger": "capability_unavailable"' "$ROOT/revisions/rev_0001.json" \
    || fail "revision does not record the capability trigger"
  grep -q 'libass' "$ROOT/revisions/rev_0001.json" \
    || fail "revision does not record the probed cause"
  # v1 must survive unchanged so the substitution is diffable.
  grep -q 'ffmpeg.filter.subtitles' "$ROOT/plan/v1/production_plan.json" \
    || fail "plan v1 was mutated; both plans must survive"
  [ -f "$ROOT/output/final.srt" ] || fail "expected a sidecar SRT from the fallback"
  echo "smoke-produce: revision path verified"
else
  echo "smoke-produce: ffmpeg has libass; burn-in path taken, revision not expected"
fi

# --- replay reproduces the work from persisted argv alone ---
echo "smoke-produce: replay"
"$BIN" replay "$PID" >replay.log 2>&1 || fail "replay failed: $(tail -5 replay.log)"
grep -q "0 failed" replay.log || fail "replay reported failures"

# --- listing works ---
"$BIN" productions --json | grep -q "$PID" || fail "productions listing omits $PID"

echo
echo "smoke-produce: PASS (production $PID, produce exit $PRODUCE_EXIT)"

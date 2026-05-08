#!/usr/bin/env bash
# Smoke test for creative-timeline and creative-render-plan workflow.
# Builds on the stub execution flow: plan → approve → stub → timeline → render-plan → review → validate.
# Does not call any provider. Does not execute shell commands.
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

echo "--- creative-plan"
"$BINARY" creative-plan "$INPUT" --goal "make a cinematic short with narration and AI b-roll captions"

PLAN_ID="$("$BINARY" creative-plans 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$PLAN_ID" ]; then
  echo "FAIL: no plan_id found"
  exit 1
fi
echo "    plan_id: $PLAN_ID"

echo "--- approve-creative-plan"
"$BINARY" approve-creative-plan "$PLAN_ID"

echo "--- creative-execute-stub"
"$BINARY" creative-execute-stub "$PLAN_ID"

echo "--- creative-timeline"
"$BINARY" creative-timeline "$PLAN_ID"

echo "--- creative-timeline --overwrite (idempotent)"
"$BINARY" creative-timeline "$PLAN_ID" --overwrite

echo "--- creative-timeline --json"
"$BINARY" creative-timeline "$PLAN_ID" --overwrite --json | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['schema_version']=='creative_timeline.v1', d['schema_version']; print('    json ok, tracks:', len(d['tracks']))"

echo "--- creative-render-plan"
"$BINARY" creative-render-plan "$PLAN_ID"

echo "--- creative-render-plan --overwrite (idempotent)"
"$BINARY" creative-render-plan "$PLAN_ID" --overwrite

echo "--- creative-render-plan --json"
"$BINARY" creative-render-plan "$PLAN_ID" --overwrite --json | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['schema_version']=='creative_render_plan.v1', d['schema_version']; assert d['planned_output']['planned_file']=='outputs/draft.mp4'; print('    json ok, steps:', len(d['steps']))"

echo "--- review-creative-timeline --write-artifact"
"$BINARY" review-creative-timeline "$PLAN_ID" --write-artifact

echo "--- validate-creative-plan (with timeline and render plan)"
"$BINARY" validate-creative-plan "$PLAN_ID"

echo "--- inspect-creative-plan (shows timeline info)"
"$BINARY" inspect-creative-plan "$PLAN_ID"

echo "--- creative-result"
"$BINARY" creative-result "$PLAN_ID"

echo "--- list outputs dir"
ls .byom-video/creative_plans/"$PLAN_ID"/outputs/

echo "==> Creative timeline smoke passed"

#!/usr/bin/env bash
# Smoke test for creative stub execution workflow.
# Creates a plan, approves it, runs stub execution, reviews outputs, validates.
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

# Reuse latest plan if present, otherwise create one
PLAN_ID=""

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

echo "--- creative-execute-stub --dry-run"
"$BINARY" creative-execute-stub "$PLAN_ID" --dry-run

echo "--- creative-execute-stub"
"$BINARY" creative-execute-stub "$PLAN_ID"

echo "--- creative-execute-stub --overwrite (idempotent)"
"$BINARY" creative-execute-stub "$PLAN_ID" --overwrite

echo "--- review-creative-outputs --write-artifact"
"$BINARY" review-creative-outputs "$PLAN_ID" --write-artifact

echo "--- creative-result --write-artifact"
"$BINARY" creative-result "$PLAN_ID" --write-artifact

echo "--- validate-creative-plan"
"$BINARY" validate-creative-plan "$PLAN_ID"

echo "--- inspect-creative-plan"
"$BINARY" inspect-creative-plan "$PLAN_ID"

echo "--- review-creative-plan --write-artifact"
"$BINARY" review-creative-plan "$PLAN_ID" --write-artifact

echo "--- creative-plan-events"
"$BINARY" creative-plan-events "$PLAN_ID"

echo "--- list outputs dir"
ls .byom-video/creative_plans/"$PLAN_ID"/outputs/

echo "==> Creative stub execution smoke passed"

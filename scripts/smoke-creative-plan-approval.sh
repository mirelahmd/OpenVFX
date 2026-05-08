#!/usr/bin/env bash
# Smoke test for creative plan approval + dry-run execution workflow.
# Runs entirely locally against a temp workspace; does not call any provider.
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

# Write a minimal tools config with one local backend
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

# Stub input file
INPUT="$WORK_DIR/clip.mov"
echo "stub" > "$INPUT"

echo "--- creative-plan"
"$BINARY" creative-plan "$INPUT" --goal "make a short with captions"

PLAN_ID="$("$BINARY" creative-plans 2>&1 | awk 'NR>1 && NF>0 {print $1; exit}')"
if [ -z "$PLAN_ID" ]; then
  echo "FAIL: no plan_id found in creative-plans output"
  exit 1
fi
echo "    plan_id: $PLAN_ID"

echo "--- inspect-creative-plan"
"$BINARY" inspect-creative-plan "$PLAN_ID"

echo "--- review-creative-plan --write-artifact"
"$BINARY" review-creative-plan "$PLAN_ID" --write-artifact

echo "--- approve-creative-plan"
"$BINARY" approve-creative-plan "$PLAN_ID"

echo "--- creative-plan-events"
"$BINARY" creative-plan-events "$PLAN_ID"

echo "--- creative-preview"
"$BINARY" creative-preview "$PLAN_ID"

echo "--- creative-preview --overwrite"
"$BINARY" creative-preview "$PLAN_ID" --overwrite

echo "--- execute-creative-plan"
"$BINARY" execute-creative-plan "$PLAN_ID"

echo "--- creative-result --write-artifact"
"$BINARY" creative-result "$PLAN_ID" --write-artifact

echo "--- validate-creative-plan"
"$BINARY" validate-creative-plan "$PLAN_ID"

echo "==> Creative plan approval smoke passed"

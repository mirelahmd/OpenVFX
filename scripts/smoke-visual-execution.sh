#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not found"
  exit 0
fi

WORK_DIR="$(mktemp -d)"
SERVER_PID=""
cleanup() {
  if [[ -n "${SERVER_PID}" ]]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

BINARY="$WORK_DIR/byom-video"
echo "==> Building binary"
GOCACHE="${REPO_ROOT}/.cache/go-build" go build -o "$BINARY" "$REPO_ROOT/cmd/byom-video"

cd "$WORK_DIR"
"$BINARY" init >/dev/null

PASS=0
FAIL=0
pass() { echo "    PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "    FAIL: $1"; FAIL=$((FAIL+1)); }

cat > visual_server.py <<'PY'
import base64
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        _ = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        body = {
            "status": "completed",
            "image_b64": base64.b64encode(b"fake-visual-asset").decode("ascii"),
        }
        data = json.dumps(body).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)
    def log_message(self, *args):
        pass

HTTPServer(("127.0.0.1", 8765), Handler).serve_forever()
PY

python3 visual_server.py &
SERVER_PID="$!"
sleep 1

export VISUAL_API_KEY="smoke-secret-token"
cat > byom-video.yaml <<'YAML'
tools:
  enabled: true
  routes:
    creative.broll_generate: smoke_visual
  backends:
    smoke_visual:
      kind: video_generation
      provider: custom-http-visual
      model: smoke-model
      endpoint: http://127.0.0.1:8765/generate
      auth:
        type: bearer_env
        env: VISUAL_API_KEY
      request:
        method: POST
        headers:
          Content-Type: application/json
        body_template:
          prompt: "{{prompt}}"
          aspect_ratio: "{{aspect_ratio}}"
          duration_seconds: "{{duration_seconds}}"
          model: "{{model}}"
      response:
        mode: sync
        output_base64_json_path: "$.image_b64"
        status_json_path: "$.status"
YAML

printf 'placeholder' > clip.mov
GOAL="make a cinematic reel and generate AI b-roll"

echo "==> Create agent plan"
"$BINARY" agent-plan --input clip.mov --goal "$GOAL" --write-review >/dev/null && pass "agent-plan works" || fail "agent-plan should work"
PLAN_ID="$(python3 - <<'PY'
import json, pathlib
plans = []
for path in pathlib.Path(".byom-video/agent_plans").glob("*/agent_plan.json"):
    plans.append(json.loads(path.read_text()))
plans.sort(key=lambda x: x["created_at"], reverse=True)
print(plans[0]["plan_id"])
PY
)"
PLAN_DIR=".byom-video/agent_plans/$PLAN_ID"

[[ -f "$PLAN_DIR/visual_requests.dryrun.json" ]] && pass "visual dry-run exists" || fail "visual dry-run missing"
"$BINARY" visual-requests "$PLAN_ID" --overwrite --json >/dev/null && pass "visual-requests refresh works" || fail "visual-requests refresh should work"

echo "==> Execute visual requests"
"$BINARY" execute-visual-requests "$PLAN_ID" --yes --allow-provider-calls --allow-external-network --json >/dev/null && pass "execute-visual-requests works" || fail "execute-visual-requests should work"
[[ -f "$PLAN_DIR/generated_assets.json" ]] && pass "generated_assets.json exists" || fail "generated_assets.json missing"
python3 - <<'PY' "$PLAN_DIR/generated_assets.json"
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert data["status"] == "completed", data
assert data["assets"], data
assert data["assets"][0]["output_file"], data
PY
pass "generated asset artifact validates"

if grep -R "smoke-secret-token" "$PLAN_DIR/outputs/visual_audits" >/dev/null 2>&1; then
  fail "audit leaked secret"
else
  pass "audit redacts secret"
fi

"$BINARY" review-visual-generation "$PLAN_ID" --write-artifact >/dev/null && pass "review-visual-generation works" || fail "review-visual-generation should work"
[[ -f "$PLAN_DIR/visual_generation_review.md" ]] && pass "visual_generation_review.md exists" || fail "visual_generation_review.md missing"

echo "==> Results"
echo "    PASS: $PASS"
echo "    FAIL: $FAIL"
if [[ "$FAIL" -ne 0 ]]; then
  exit 1
fi
echo "SMOKE PASSED"

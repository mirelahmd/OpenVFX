#!/usr/bin/env bash
# Focused smoke test for the Creative Director (Prompt 073).
#
# Runs one production from several tiny local assets and a rich creative
# request, then asserts the creative reasoning artifacts exist and actually
# reached the production plan.
#
# It runs in the default deterministic mode, so it makes NO provider call and NO
# network call. That is itself one of the assertions.
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

echo "smoke-creative-director: workspace $WORK_DIR"
cd "$WORK_DIR"

if [ -f "$REPO_ROOT/byom-video.yaml" ]; then
  cp "$REPO_ROOT/byom-video.yaml" .
fi
# The sidecar lives in the repo; point at it explicitly since we run elsewhere.
export BYOM_VIDEO_WORKERS_DIR="$REPO_ROOT/workers"

bash "$REPO_ROOT/scripts/make-produce-fixtures.sh" assets >/dev/null

BRIEF="Make a 12-second cinematic Instagram reel from these clips. Open aggressively. \
Use the last clip only where the delivery is strongest. Build tension early and then slow \
down for the final line. Keep captions bold and low. If there isn't enough atmospheric \
coverage, request generated inserts. I want it to feel premium and dramatic rather than \
like a generic social edit."

echo "smoke-creative-director: produce"
set +e
"$BIN" produce ./assets --goal "$BRIEF" >produce.log 2>&1
PRODUCE_EXIT=$?
set -e
cat produce.log

PID="$(ls -t .byom-video/productions | head -1)"
ROOT=".byom-video/productions/$PID"

fail() { echo "FAIL: $1" >&2; exit 1; }

# has <json-file> <python-expression-over-`doc`>
has() {
  python3 - "$1" "$2" <<'PYEOF'
import json, sys
path, expression = sys.argv[1], sys.argv[2]
try:
    doc = json.load(open(path))
except Exception as exc:
    print(f"could not read {path}: {exc}", file=sys.stderr)
    sys.exit(1)
sys.exit(0 if eval(expression) else 1)
PYEOF
}

# jsonval <json-file> <python-expression-over-`doc`>
jsonval() {
  python3 - "$1" "$2" <<'PYEOF'
import json, sys
doc = json.load(open(sys.argv[1]))
print(eval(sys.argv[2]))
PYEOF
}

# --- asset observations ---
[ -f "$ROOT/asset_observations.json" ] || fail "asset_observations.json missing"
has "$ROOT/asset_observations.json" "doc['schema_version']=='openvfx_asset_observations.v1'" \
  || fail "asset observations carry the wrong schema"
has "$ROOT/asset_observations.json" "len(doc['assets'])>=3" \
  || fail "multiple assets were not represented in the observations"
has "$ROOT/asset_observations.json" "all(a.get('media_type') for a in doc['assets'])" \
  || fail "assets are missing a media type"
# The artifact goes into a prompt; it must reference transcripts, not embed them.
has "$ROOT/asset_observations.json" "not any('segments' in a for a in doc['assets'])" \
  || fail "asset observations must not embed transcript bodies"

# --- creative treatment ---
[ -f "$ROOT/creative_treatment.json" ] || fail "creative_treatment.json missing"
T="$ROOT/creative_treatment.json"
has "$T" "doc['schema_version']=='openvfx_creative_treatment.v1'" || fail "treatment schema wrong"
has "$T" "bool(doc['objective'].strip())"                        || fail "treatment has no objective"

# --- narrative / pacing strategy ---
has "$T" "len(doc.get('narrative_structure',[]))>=1" || fail "no narrative structure in the treatment"
has "$T" "len(doc.get('pacing_strategy',[]))>=1"     || fail "no pacing strategy in the treatment"

# --- asset roles ---
has "$T" "len(doc.get('asset_roles',[]))>=3" || fail "asset roles missing"
has "$T" "all(r.get('confidence') in ('high','medium','low','uncertain') for r in doc['asset_roles'])" \
  || fail "asset roles must carry an explicit confidence"

# --- gap detection ---
has "$T" "len(doc.get('generated_asset_needs',[]))+len(doc.get('degraded_alternatives',[]))>=1" \
  || fail "no gap detection recorded"

# --- one bounded critique pass ---
has "$T" "doc.get('critique',{}).get('performed') is True" || fail "no critique pass recorded"
has "$T" "doc['critique'].get('passes')==1"               || fail "critique must run exactly one pass"

# --- reasoning honesty ---
has "$T" "doc['reasoning']['effective_mode'] in ('llm','deterministic')" || fail "unknown reasoning mode"
has "$T" "not (doc['reasoning']['semantic_reasoning'] and doc['reasoning']['effective_mode']!='llm')" \
  || fail "rules-based output claimed semantic reasoning"
MODE=$(jsonval "$T" "doc['reasoning']['effective_mode']")
echo "smoke-creative-director: reasoning mode = $MODE"
if [ "$MODE" = "deterministic" ]; then
  has "$T" "len(doc.get('uncertainties',[]))>=1" \
    || fail "the deterministic path must disclose its own limits"
fi

# --- graph trace ---
[ -f "$ROOT/director/graph_trace.json" ] || fail "director graph trace missing"
has "$ROOT/director/graph_trace.json" "len(doc['nodes'])>=8" \
  || fail "expected a trace entry for every graph node"
# Chain-of-thought must never reach an artifact.
grep -qiE '"(chain_of_thought|reasoning_trace|thoughts)"' "$T" \
  && fail "treatment contains a reasoning trace field" || true

# --- treatment reached the production plan ---
PLAN="$ROOT/plan/v1/production_plan.json"
[ -f "$PLAN" ] || fail "plan v1 missing"
has "$PLAN" "doc['planner'].startswith('creative_director')" \
  || fail "plan was not produced from the treatment (planner=$(jsonval "$PLAN" "doc['planner']"))"
has "$PLAN" "any(s.get('treatment_ref') or s.get('treatment_decision_id') for s in doc['stages'])" \
  || fail "no production stage references the treatment"
has "$PLAN" "any(s['type']=='select_clips' and s.get('params',{}).get('segments') for s in doc['stages'])" \
  || fail "the treatment's segments did not reach the select_clips stage"

# --- the treatment's cuts became real cuts ---
EDL=$(find "$ROOT/stages" -name edit_decisions.json | head -1)
[ -n "$EDL" ] || fail "no edit decision list was produced"
has "$EDL" "any('treatment' in n for n in doc.get('notes',[]))" \
  || fail "the edit decision list does not cite the treatment"

# --- run record carries creative provenance ---
[ -f "$ROOT/run_record.json" ] || fail "run_record.json missing"
has "$ROOT/run_record.json" "doc.get('treatment') is not None" \
  || fail "run record lost the treatment summary"
grep -q "Creative direction" "$ROOT/handoff.md" || fail "handoff does not report creative direction"

# --- no provider or network call without explicit configuration ---
has "$ROOT/run_record.json" "doc['network_egress_calls']==0" \
  || fail "a default run must make no external calls"
has "$ROOT/director/director_config.json" "doc['mode']=='deterministic'" \
  || fail "default run should not request a model"

# --- the diagnostic command works ---
"$BIN" creative-treatment "$PID" >/dev/null || fail "creative-treatment command failed"
"$BIN" creative-treatment "$PID" --json | python3 -c "import json,sys;json.load(sys.stdin)" \
  || fail "creative-treatment --json did not emit valid JSON"

echo
echo "smoke-creative-director: PASS (production $PID, mode $MODE, produce exit $PRODUCE_EXIT)"

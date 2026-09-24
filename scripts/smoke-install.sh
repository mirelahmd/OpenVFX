#!/usr/bin/env bash
# Clean-install smoke test.
#
# Installs OpenVFX from a built release archive into a throwaway HOME, then
# exercises the installed distribution from a directory that has no relationship
# to this repository. That isolation is the point: it is what catches an
# installed binary that only works because it happened to find the source tree.
#
# Usage:
#   scripts/build-release.sh v0.0.0-smoke   # or HOST_ONLY=1 for speed
#   scripts/smoke-install.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="${DIST:-$REPO_ROOT/dist}"

GOOS_LOCAL="$(go env GOOS 2>/dev/null || uname -s | tr '[:upper:]' '[:lower:]')"
case "$(uname -m)" in
  x86_64|amd64) GOARCH_LOCAL="amd64" ;;
  arm64|aarch64) GOARCH_LOCAL="arm64" ;;
  *) GOARCH_LOCAL="$(go env GOARCH 2>/dev/null || echo unknown)" ;;
esac

ARCHIVE="$(ls "$DIST"/openvfx_*_"${GOOS_LOCAL}"_"${GOARCH_LOCAL}".tar.gz 2>/dev/null | head -1 || true)"
if [ -z "$ARCHIVE" ]; then
  echo "error: no release archive for ${GOOS_LOCAL}/${GOARCH_LOCAL} in $DIST" >&2
  echo "       run: HOST_ONLY=1 scripts/build-release.sh v0.0.0-smoke" >&2
  exit 1
fi

FAKE_HOME="$(mktemp -d)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$FAKE_HOME" "$WORK_DIR"' EXIT

fail() { echo "FAIL: $1" >&2; exit 1; }

echo "smoke-install: archive  $ARCHIVE"
echo "smoke-install: HOME     $FAKE_HOME"
echo "smoke-install: workdir  $WORK_DIR"

# Scrub every development affordance. If the installed CLI needs any of these,
# an external user's install would be broken and this test must catch it.
export HOME="$FAKE_HOME"
unset BYOM_VIDEO_PYTHON BYOM_VIDEO_WORKERS_DIR OPENVFX_PYTHON OPENVFX_WORKERS_DIR OPENVFX_HOME XDG_DATA_HOME || true

echo
echo "smoke-install: === install (piped through sh, as a real user would) ==="
# Piping proves the installer never relies on $0 or on neighbouring repo files.
OPENVFX_ARCHIVE="$ARCHIVE" \
OPENVFX_BIN_DIR="$FAKE_HOME/.local/bin" \
  sh < "$REPO_ROOT/install.sh" 2>&1 | tee "$WORK_DIR/install.log"

BIN="$FAKE_HOME/.local/bin/openvfx"
[ -x "$BIN" ] || fail "openvfx was not installed to $BIN"

# --- everything below runs from a directory unrelated to the repo ---
cd "$WORK_DIR"

echo
echo "smoke-install: === verifying the installed distribution ==="

# --- version / help ---
"$BIN" --version >"$WORK_DIR/version.txt" 2>&1 || fail "openvfx --version failed"
grep -q "OpenVFX" "$WORK_DIR/version.txt" || fail "--version did not identify OpenVFX"
grep -qE "version: +v?[0-9]" "$WORK_DIR/version.txt" \
  || fail "--version reported no build version: $(cat "$WORK_DIR/version.txt")"
"$BIN" version >/dev/null 2>&1 || fail "openvfx version failed"
"$BIN" --help >/dev/null 2>&1  || fail "openvfx --help failed"
echo "  version/help: ok"

# --- the installed sidecar is discoverable without the repo ---
grep -q "agent sidecar:" "$WORK_DIR/version.txt" || fail "version does not report sidecar resolution"
SIDECAR="$(sed -n 's/.*agent sidecar: *//p' "$WORK_DIR/version.txt" | head -1)"
[ "$SIDECAR" != "not found" ] || fail "installed CLI could not locate the agent sidecar"
case "$SIDECAR" in
  "$REPO_ROOT"*) fail "installed CLI resolved the sidecar to the source checkout: $SIDECAR" ;;
esac
[ -d "$SIDECAR/openvfx_agent_graph/creative" ] \
  || fail "installed sidecar has no Creative Director at $SIDECAR"
echo "  sidecar: $SIDECAR"

# --- the isolated python env resolves and langgraph imports from it ---
PY="$(sed -n 's/.*python: *//p' "$WORK_DIR/version.txt" | head -1)"
[ "$PY" != "not found" ] || fail "installed CLI could not locate a python interpreter"
echo "  python: $PY"

VENV_PY="$FAKE_HOME/.local/share/openvfx/venv/bin/python"
if [ -x "$VENV_PY" ]; then
  [ "$PY" = "$VENV_PY" ] || fail "CLI resolved $PY instead of the installed venv $VENV_PY"
  "$VENV_PY" -c "import langgraph" 2>/dev/null \
    || fail "langgraph does not import from the installed environment"
  "$VENV_PY" -c "from openvfx_agent_graph.creative.graph import build_creative_graph; build_creative_graph()" 2>/dev/null \
    || fail "the Creative Director graph does not build from the installed environment"
  echo "  langgraph + creative graph: ok"
  HAVE_DIRECTOR=1
else
  echo "  warning: no venv installed (python3 missing on this machine); skipping agent assertions"
  HAVE_DIRECTOR=0
fi

# --- capability discovery works from the installed CLI ---
"$BIN" doctor >"$WORK_DIR/doctor.txt" 2>&1 || fail "openvfx doctor failed"
grep -q "ffprobe" "$WORK_DIR/doctor.txt" || fail "doctor did not report on ffprobe"
echo "  capability discovery: ok"

# --- no Go toolchain needed past this point ---
command -v go >/dev/null 2>&1 && echo "  note: go is present but was not used after the build"

# --- one tiny installed produce run, if ffmpeg is available ---
if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1; then
  echo
  echo "smoke-install: === installed produce run ==="
  mkdir -p assets
  for i in 1 2; do
    ffmpeg -hide_banner -loglevel error -y \
      -f lavfi -i "testsrc=size=320x180:rate=15:duration=3" \
      -f lavfi -i "sine=frequency=$((300 * i)):duration=3" \
      -c:v libx264 -preset ultrafast -pix_fmt yuv420p -c:a aac -shortest \
      "assets/clip_$i.mp4"
  done

  set +e
  "$BIN" produce ./assets --goal "Make a 4-second vertical teaser, premium and dramatic" \
    >"$WORK_DIR/produce.log" 2>&1
  PRODUCE_EXIT=$?
  set -e
  tail -25 "$WORK_DIR/produce.log"

  PID="$(ls -t .byom-video/productions 2>/dev/null | head -1 || true)"
  [ -n "$PID" ] || fail "produce created no production (exit $PRODUCE_EXIT)"
  ROOT=".byom-video/productions/$PID"

  [ -f "$ROOT/output/final.mp4" ] || fail "installed produce did not render an output"
  ffprobe -v quiet -show_format "$ROOT/output/final.mp4" >/dev/null \
    || fail "installed produce output is not probeable"
  echo "  rendered: $ROOT/output/final.mp4"

  if [ "$HAVE_DIRECTOR" = "1" ]; then
    [ -f "$ROOT/creative_treatment.json" ] \
      || fail "the installed Creative Director did not produce a treatment"
    MODE="$(python3 -c "import json;print(json.load(open('$ROOT/creative_treatment.json'))['reasoning']['effective_mode'])")"
    case "$MODE" in
      deterministic|llm) ;;
      *) fail "unexpected reasoning mode from the installed director: $MODE" ;;
    esac
    echo "  creative director (installed): reached '$MODE' mode"
    grep -q "director   unavailable" "$WORK_DIR/produce.log" \
      && fail "the installed director reported itself unavailable"
  fi
else
  echo "  note: ffmpeg/ffprobe not installed; skipped the produce run"
fi

echo
echo "smoke-install: PASS"

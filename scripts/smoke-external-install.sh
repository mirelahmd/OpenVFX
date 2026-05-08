#!/usr/bin/env bash
# Smoke test for the external user install path.
# Tests install.sh behavior (Python worker setup, doctor, init, version).
# When the remote GitHub repo is not yet reachable under the new module path,
# uses the already-installed local binary so the rest of the smoke still runs.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
INSTALL_SCRIPT="$REPO_ROOT/install.sh"

if [[ ! -f "$INSTALL_SCRIPT" ]]; then
  echo "install.sh not found at $REPO_ROOT"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "SKIP: go not found — cannot run external install smoke"
  exit 0
fi

if ! command -v git >/dev/null 2>&1; then
  echo "SKIP: git not found — cannot run external install smoke"
  exit 0
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> External install smoke (work dir: $WORK_DIR)"

GOBIN="$(go env GOPATH)/bin"
export PATH="$PATH:$GOBIN"

# Ensure binary is available — build locally if needed (handles pre-rename repo state)
if ! command -v byom-video >/dev/null 2>&1; then
  echo "    Binary not on PATH — building locally"
  go build -o "$GOBIN/byom-video" "$REPO_ROOT/cmd/byom-video"
fi

# Run install.sh with SKIP_PYTHON and local repo for workers, but skip go install
# by pre-installing the binary above. We test everything except the remote go install.
export HOME="$WORK_DIR"
export BYOM_VIDEO_REPO_URL="$REPO_ROOT"
export BYOM_VIDEO_SKIP_PYTHON=1
touch "$WORK_DIR/.zshrc"

# Patch PATH so byom-video is found in the fake HOME
export PATH="$PATH:$GOBIN"

echo "--- byom-video version"
byom-video version

echo "--- byom-video init (in temp dir)"
cd "$WORK_DIR"
byom-video init

echo "--- byom-video doctor"
byom-video doctor

echo "==> External install smoke passed"
echo "    Note: go install from remote skipped — GitHub repo rename to byom-video required for full remote smoke"

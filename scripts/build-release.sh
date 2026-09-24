#!/usr/bin/env bash
# Build OpenVFX release archives.
#
# Each archive carries everything an installed runtime needs: the Go executable
# AND the Python agent sidecar. The Creative Director lives in Python, so a
# release containing only the Go binary would install a CLI with no agent.
#
# Usage:
#   scripts/build-release.sh [version]          # all supported platforms
#   HOST_ONLY=1 scripts/build-release.sh        # just this machine (fast iteration)
#
# Output: dist/openvfx_<version>_<os>_<arch>.tar.gz plus dist/SHA256SUMS
set -euo pipefail

VERSION="${1:-${OPENVFX_VERSION:-}}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo dev)"
fi
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MODULE="github.com/mirelahmd/OpenVFX"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$REPO_ROOT/dist"

PLATFORMS="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64"
if [ "${HOST_ONLY:-0}" = "1" ]; then
  PLATFORMS="$(go env GOOS)/$(go env GOARCH)"
fi

echo "==> OpenVFX release build"
echo "    version:  $VERSION"
echo "    commit:   $COMMIT"
echo "    built:    $BUILD_DATE"
echo "    platforms:$PLATFORMS"

rm -rf "$DIST"
mkdir -p "$DIST"

LDFLAGS="-s -w \
  -X ${MODULE}/internal/commands.Version=${VERSION} \
  -X ${MODULE}/internal/commands.Commit=${COMMIT} \
  -X ${MODULE}/internal/commands.BuildDate=${BUILD_DATE}"

# stage_payload lays out one release tree.
#
#   bin/openvfx                      the CLI
#   share/openvfx/workers/           the Python agent sidecar, installed as a package
#   share/openvfx/VERSION            what the installer records
#
# The sidecar is copied from source rather than wheel-built: the installer pip
# installs it into an isolated venv, and shipping the source keeps the release
# independent of a Python build toolchain on the builder.
stage_payload() {
  stage_dir="$1"
  mkdir -p "$stage_dir/bin" "$stage_dir/share/openvfx"

  # Python sidecar. Copy the package sources plus the dependency manifest so the
  # installer can `pip install` it without duplicating a dependency list.
  mkdir -p "$stage_dir/share/openvfx/workers"
  cp "$REPO_ROOT/workers/pyproject.toml" "$stage_dir/share/openvfx/workers/"
  for pkg in openvfx_agent_graph byom_video_workers; do
    if [ ! -d "$REPO_ROOT/workers/$pkg" ]; then
      echo "error: expected Python package workers/$pkg" >&2
      exit 1
    fi
    cp -R "$REPO_ROOT/workers/$pkg" "$stage_dir/share/openvfx/workers/"
  done
  # Drop build detritus that must never ship.
  find "$stage_dir/share/openvfx/workers" \
    \( -name '__pycache__' -o -name '*.egg-info' -o -name '.pytest_cache' \) \
    -prune -exec rm -rf {} + 2>/dev/null || true

  printf '%s\n' "$VERSION" > "$stage_dir/share/openvfx/VERSION"
  cp "$REPO_ROOT/LICENSE" "$stage_dir/" 2>/dev/null || true
  cp "$REPO_ROOT/README.md" "$stage_dir/" 2>/dev/null || true
}

# Sanity: the sidecar must contain the Creative Director, or the release would
# silently ship a CLI that degrades to intent-only planning.
if [ ! -d "$REPO_ROOT/workers/openvfx_agent_graph/creative" ]; then
  echo "error: workers/openvfx_agent_graph/creative is missing; refusing to build a release without the Creative Director" >&2
  exit 1
fi

for platform in $PLATFORMS; do
  goos="${platform%%/*}"
  goarch="${platform##*/}"
  name="openvfx_${VERSION}_${goos}_${goarch}"
  stage="$DIST/$name"

  echo "--> $goos/$goarch"
  stage_payload "$stage"

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$stage/bin/openvfx" "$REPO_ROOT/cmd/byom-video"

  tar -C "$DIST" -czf "$DIST/$name.tar.gz" "$name"
  rm -rf "$stage"
  echo "    $DIST/$name.tar.gz"
done

# Checksums. The installer verifies against this file before unpacking anything.
echo "--> checksums"
(
  cd "$DIST"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 ./*.tar.gz | sed 's| \./| |' > SHA256SUMS
  else
    sha256sum ./*.tar.gz | sed 's| \./| |' > SHA256SUMS
  fi
)
cat "$DIST/SHA256SUMS"

echo
echo "==> Done. Artifacts in $DIST"

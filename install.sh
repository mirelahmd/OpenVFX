#!/bin/sh
# OpenVFX installer.
#
#   curl -fsSL https://raw.githubusercontent.com/mirelahmd/OpenVFX/main/install.sh | sh
#
# Pin a version (this form works reliably through a pipe):
#
#   curl -fsSL https://raw.githubusercontent.com/mirelahmd/OpenVFX/main/install.sh | OPENVFX_VERSION=v0.1.0 sh
#
# Requires neither Go nor a repository checkout. Installs the CLI plus the
# Python agent sidecar that the Creative Director runs in.
#
# Environment:
#   OPENVFX_VERSION           release tag to install (default: latest)
#   OPENVFX_BIN_DIR           override the executable destination
#   OPENVFX_HOME              override the data directory
#   OPENVFX_ARCHIVE           install from a local .tar.gz instead of downloading
#   OPENVFX_BASE_URL          override the release download base URL
#   OPENVFX_PYTHON            python3 to build the sidecar environment with
#   OPENVFX_SKIP_PYTHON=1     skip the sidecar environment entirely
#   OPENVFX_WITH_TRANSCRIBE=1 also install faster-whisper (large; enables captions)
#   OPENVFX_NO_COMPAT_LINK=1  do not create the byom-video compatibility symlink
set -eu

REPO="mirelahmd/OpenVFX"
VERSION="${OPENVFX_VERSION:-}"
ARCHIVE="${OPENVFX_ARCHIVE:-}"
BASE_URL="${OPENVFX_BASE_URL:-}"
SKIP_PYTHON="${OPENVFX_SKIP_PYTHON:-0}"
WITH_TRANSCRIBE="${OPENVFX_WITH_TRANSCRIBE:-0}"

TMP_DIR=""
cleanup() {
  [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ] && rm -rf "$TMP_DIR"
  return 0
}
trap cleanup EXIT HUP INT TERM

say()  { printf '%s\n' "$*"; }
step() { printf '    %s\n' "$*"; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required but was not found on PATH"
}

say "==> Installing OpenVFX"

# ---------------------------------------------------------------- platform ---

os_name="$(uname -s)"
case "$os_name" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *) die "unsupported operating system: $os_name (OpenVFX supports macOS and Linux)" ;;
esac

arch_name="$(uname -m)"
case "$arch_name" in
  x86_64|amd64)        ARCH="amd64" ;;
  arm64|aarch64)       ARCH="arm64" ;;
  *) die "unsupported architecture: $arch_name (OpenVFX supports amd64 and arm64)" ;;
esac

step "platform: ${OS}/${ARCH}"

# ------------------------------------------------------------------ fetch ---

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t openvfx)"
[ -d "$TMP_DIR" ] || die "could not create a temporary directory"

if [ -n "$ARCHIVE" ]; then
  # Local archive: used by scripts/smoke-install.sh and for air-gapped installs.
  [ -f "$ARCHIVE" ] || die "OPENVFX_ARCHIVE does not exist: $ARCHIVE"
  step "using local archive: $ARCHIVE"
  cp "$ARCHIVE" "$TMP_DIR/openvfx.tar.gz"

  sums_file="$(dirname "$ARCHIVE")/SHA256SUMS"
  if [ -f "$sums_file" ]; then
    cp "$sums_file" "$TMP_DIR/SHA256SUMS"
    archive_name="$(basename "$ARCHIVE")"
  else
    say "    warning: no SHA256SUMS beside the archive; skipping checksum verification"
    archive_name=""
  fi
else
  need curl
  need tar

  if [ -z "$VERSION" ]; then
    step "resolving latest release"
    # Parse the tag out of the releases API without needing jq.
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
      | head -1)"
    [ -n "$VERSION" ] || die "could not determine the latest release; set OPENVFX_VERSION explicitly"
  fi
  step "version: $VERSION"

  [ -n "$BASE_URL" ] || BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
  archive_name="openvfx_${VERSION}_${OS}_${ARCH}.tar.gz"

  step "downloading $archive_name"
  curl -fsSL --proto '=https' --tlsv1.2 -o "$TMP_DIR/openvfx.tar.gz" \
    "${BASE_URL}/${archive_name}" \
    || die "download failed: ${BASE_URL}/${archive_name}"

  step "downloading checksums"
  curl -fsSL --proto '=https' --tlsv1.2 -o "$TMP_DIR/SHA256SUMS" \
    "${BASE_URL}/SHA256SUMS" \
    || die "could not download SHA256SUMS from ${BASE_URL}"
fi

# ---------------------------------------------------------------- verify ---

if [ -f "$TMP_DIR/SHA256SUMS" ] && [ -n "$archive_name" ]; then
  expected="$(grep "  ${archive_name}\$" "$TMP_DIR/SHA256SUMS" 2>/dev/null | awk '{print $1}' | head -1)"
  [ -n "$expected" ] || die "no checksum recorded for $archive_name; refusing to install"

  if command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$TMP_DIR/openvfx.tar.gz" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMP_DIR/openvfx.tar.gz" | awk '{print $1}')"
  else
    die "neither shasum nor sha256sum is available; cannot verify the download"
  fi

  if [ "$expected" != "$actual" ]; then
    die "checksum mismatch for $archive_name
  expected: $expected
  actual:   $actual
The download may be corrupt or tampered with. Nothing was installed."
  fi
  step "checksum verified"
fi

# --------------------------------------------------------------- extract ---

mkdir -p "$TMP_DIR/unpack"
tar -xzf "$TMP_DIR/openvfx.tar.gz" -C "$TMP_DIR/unpack" || die "could not extract the archive"

# Archives contain a single top-level directory.
PAYLOAD="$(find "$TMP_DIR/unpack" -mindepth 1 -maxdepth 1 -type d | head -1)"
[ -n "$PAYLOAD" ] || die "unexpected archive layout"
[ -f "$PAYLOAD/bin/openvfx" ] || die "archive does not contain bin/openvfx"
[ -d "$PAYLOAD/share/openvfx/workers" ] || die "archive does not contain the Python agent sidecar"

if [ -z "${VERSION:-}" ] || [ -n "$ARCHIVE" ]; then
  if [ -f "$PAYLOAD/share/openvfx/VERSION" ]; then
    VERSION="$(cat "$PAYLOAD/share/openvfx/VERSION")"
  fi
fi
[ -n "${VERSION:-}" ] || VERSION="unknown"

# ------------------------------------------------------- install location ---

if [ -n "${OPENVFX_BIN_DIR:-}" ]; then
  BIN_DIR="$OPENVFX_BIN_DIR"
elif [ -w /usr/local/bin ] 2>/dev/null; then
  BIN_DIR="/usr/local/bin"
else
  # A user-local install beats asking for sudo.
  BIN_DIR="$HOME/.local/bin"
fi
mkdir -p "$BIN_DIR" || die "could not create $BIN_DIR"

if [ -n "${OPENVFX_HOME:-}" ]; then
  DATA_DIR="$OPENVFX_HOME"
elif [ -n "${XDG_DATA_HOME:-}" ]; then
  DATA_DIR="$XDG_DATA_HOME/openvfx"
else
  DATA_DIR="$HOME/.local/share/openvfx"
fi
mkdir -p "$DATA_DIR" || die "could not create $DATA_DIR"

VERSION_DIR="$DATA_DIR/$VERSION"

step "cli:  $BIN_DIR/openvfx"
step "data: $VERSION_DIR"

# --------------------------------------------------------------- install ---

install -m 0755 "$PAYLOAD/bin/openvfx" "$BIN_DIR/openvfx" \
  || die "could not install the executable to $BIN_DIR"

rm -rf "$VERSION_DIR"
mkdir -p "$VERSION_DIR"
cp -R "$PAYLOAD/share/openvfx/workers" "$VERSION_DIR/workers"
[ -f "$PAYLOAD/share/openvfx/VERSION" ] && cp "$PAYLOAD/share/openvfx/VERSION" "$VERSION_DIR/VERSION"

# `current` is the stable path the CLI resolves; swapping it is the upgrade.
rm -rf "$DATA_DIR/current"
ln -s "$VERSION_DIR" "$DATA_DIR/current" 2>/dev/null || cp -R "$VERSION_DIR" "$DATA_DIR/current"

if [ "${OPENVFX_NO_COMPAT_LINK:-0}" != "1" ]; then
  # Existing users and docs still refer to byom-video.
  rm -f "$BIN_DIR/byom-video"
  ln -s "$BIN_DIR/openvfx" "$BIN_DIR/byom-video" 2>/dev/null || true
fi

# ------------------------------------------------- python agent sidecar ---

VENV_DIR="$DATA_DIR/venv"
PYTHON_STATUS="skipped"
DIRECTOR_STATUS="unavailable"

if [ "$SKIP_PYTHON" = "1" ]; then
  step "skipping the Python sidecar environment (OPENVFX_SKIP_PYTHON=1)"
else
  PY="${OPENVFX_PYTHON:-}"
  if [ -z "$PY" ]; then
    for candidate in python3 python3.12 python3.11 python3.10; do
      if command -v "$candidate" >/dev/null 2>&1; then PY="$candidate"; break; fi
    done
  fi

  if [ -z "$PY" ]; then
    say "    warning: python3 not found — the Creative Director will be unavailable."
    say "             Install Python 3.10+ and re-run this installer."
  elif ! "$PY" -c 'import sys; sys.exit(0 if sys.version_info >= (3,10) else 1)' 2>/dev/null; then
    say "    warning: $PY is older than Python 3.10 — the Creative Director will be unavailable."
    say "             Install Python 3.10+ and re-run this installer."
  else
    step "creating the agent environment at $VENV_DIR"
    if "$PY" -m venv "$VENV_DIR" >/dev/null 2>&1; then
      "$VENV_DIR/bin/pip" install --quiet --upgrade pip >/dev/null 2>&1 || true

      # Install from the shipped package so dependencies come from its own
      # pyproject.toml rather than a duplicated list maintained here.
      extras="graph"
      if [ "$WITH_TRANSCRIBE" = "1" ]; then
        extras="graph,transcribe"
        step "installing agent dependencies (langgraph + faster-whisper; this takes a few minutes)"
      else
        step "installing agent dependencies (langgraph)"
      fi

      if "$VENV_DIR/bin/pip" install --quiet "$VERSION_DIR/workers[$extras]" >"$TMP_DIR/pip.log" 2>&1; then
        PYTHON_STATUS="available"
        if "$VENV_DIR/bin/python" -c 'import langgraph' 2>/dev/null; then
          DIRECTOR_STATUS="available"
        fi
      else
        say "    warning: agent dependency install failed; see the tail below"
        tail -5 "$TMP_DIR/pip.log" 2>/dev/null | sed 's/^/             /'
        say "             retry: $VENV_DIR/bin/pip install \"$VERSION_DIR/workers[$extras]\""
      fi
    else
      say "    warning: could not create a virtualenv with $PY"
    fi
  fi
fi

# --------------------------------------------------------------- report ---

say ""
say "OpenVFX installed successfully."
say ""
say "CLI:"
say "  openvfx: $BIN_DIR/openvfx"
if [ "${OPENVFX_NO_COMPAT_LINK:-0}" != "1" ] && [ -L "$BIN_DIR/byom-video" ]; then
  say "  byom-video -> openvfx (compatibility)"
fi
say ""
say "Runtime:"
printf '  Python: %s\n' "$PYTHON_STATUS"
printf '  Creative Director: %s\n' "$DIRECTOR_STATUS"

check_tool() {
  if command -v "$1" >/dev/null 2>&1; then printf '  %s: available\n' "$1"
  else printf '  %s: NOT FOUND\n' "$1"; fi
}
check_tool ffmpeg
check_tool ffprobe

# Optional ffmpeg filters decide whether caption burn-in can work at all.
say ""
say "Optional capabilities:"
if command -v ffmpeg >/dev/null 2>&1; then
  filters="$(ffmpeg -hide_banner -filters 2>/dev/null | awk '{print $2}')"
  for f in subtitles drawtext amix; do
    if printf '%s\n' "$filters" | grep -qx "$f"; then
      printf '  ffmpeg %s: available\n' "$f"
    else
      printf '  ffmpeg %s: unavailable\n' "$f"
    fi
  done
else
  say "  ffmpeg filters: unknown (ffmpeg not installed)"
fi
if [ "$WITH_TRANSCRIBE" != "1" ]; then
  say "  transcription (faster-whisper): not installed"
  say "    enable with: $VENV_DIR/bin/pip install \"$VERSION_DIR/workers[transcribe]\""
fi

say ""
say "Model:"
say "  no model configured — the Creative Director runs in deterministic mode."
say "  Configure a model (BYOM) under models.routes.creative_director in byom-video.yaml,"
say "  then run: openvfx produce ./assets --goal \"...\" --director llm"

# PATH advice only matters for a user-local install.
case ":${PATH}:" in
  *":${BIN_DIR}:"*) ;;
  *)
    say ""
    say "NOTE: $BIN_DIR is not on your PATH. Add it with:"
    say "  export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac

if ! command -v ffmpeg >/dev/null 2>&1; then
  say ""
  say "NOTE: ffmpeg and ffprobe are required for media execution."
  say "  macOS:  brew install ffmpeg"
  say "  Debian: sudo apt-get install ffmpeg"
fi

say ""
say "Next:"
say "  openvfx --version"
say "  openvfx --help"

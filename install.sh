#!/usr/bin/env sh
# BYOM Video install script
#
# Env vars:
#   BYOM_VIDEO_REPO_URL     Git repo to clone for Python workers
#                           (default: https://github.com/mirelahmd/byom-video.git)
#   BYOM_VIDEO_REF          Git ref/tag to checkout (default: main)
#   BYOM_VIDEO_INSTALL_DIR  Where to store worker source
#                           (default: $HOME/.byom-video/src)
#   BYOM_VIDEO_PYTHON       Python interpreter to use (overrides auto-detect)
#   BYOM_VIDEO_SKIP_PYTHON  Set to 1 to skip Python worker setup entirely
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mirelahmd/byom-video/main/install.sh | sh
#   BYOM_VIDEO_SKIP_PYTHON=1 sh install.sh
set -e

REPO_URL="${BYOM_VIDEO_REPO_URL:-https://github.com/mirelahmd/byom-video.git}"
REF="${BYOM_VIDEO_REF:-main}"
INSTALL_DIR="${BYOM_VIDEO_INSTALL_DIR:-$HOME/.byom-video/src}"
SRC_DIR="$INSTALL_DIR/byom-video"
VENV_DIR="$HOME/.byom-venv"
SKIP_PYTHON="${BYOM_VIDEO_SKIP_PYTHON:-0}"
SHELL_RC=""

echo "==> Installing BYOM Video"

# --- detect shell config ---
detect_shell_rc() {
  if [ -f "$HOME/.zshrc" ]; then
    echo "$HOME/.zshrc"
  elif [ -f "$HOME/.bashrc" ]; then
    echo "$HOME/.bashrc"
  elif [ -f "$HOME/.profile" ]; then
    echo "$HOME/.profile"
  fi
}
SHELL_RC="$(detect_shell_rc)"

append_to_shell_rc() {
  line="$1"
  if [ -n "$SHELL_RC" ]; then
    if ! grep -qF "$line" "$SHELL_RC" 2>/dev/null; then
      echo "$line" >> "$SHELL_RC"
    fi
  fi
}

# --- Go binary ---
if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: Go is not installed."
  echo "       Install Go 1.22+ from https://go.dev/dl/ then re-run this script."
  exit 1
fi

echo "    go install github.com/mirelahmd/byom-video/cmd/byom-video@latest"
GOPROXY=direct GONOSUMDB='*' go install github.com/mirelahmd/byom-video/cmd/byom-video@latest

# --- ensure ~/go/bin is on PATH ---
GOBIN="$(go env GOPATH)/bin"
case ":$PATH:" in
  *":$GOBIN:"*) ;;
  *)
    echo "    Adding $GOBIN to PATH"
    if [ -n "$SHELL_RC" ]; then
      append_to_shell_rc ""
      append_to_shell_rc "# byom-video"
      append_to_shell_rc "export PATH=\"\$PATH:$GOBIN\""
    fi
    export PATH="$PATH:$GOBIN"
    ;;
esac

echo "    byom-video binary installed"

# --- Python setup ---
if [ "$SKIP_PYTHON" = "1" ]; then
  echo "    Skipping Python setup (BYOM_VIDEO_SKIP_PYTHON=1)"
else
  if ! command -v python3 >/dev/null 2>&1; then
    echo "WARNING: python3 not found — skipping transcription setup."
    echo "         Install Python 3.10+ and re-run to enable transcription."
  else
    if ! command -v git >/dev/null 2>&1; then
      echo "WARNING: git not found — cannot clone worker package."
      echo "         Install git and re-run, or set BYOM_VIDEO_SKIP_PYTHON=1 to skip."
    else
      # --- clone or update worker source ---
      mkdir -p "$INSTALL_DIR"
      if [ -d "$SRC_DIR/.git" ]; then
        echo "    Updating worker source at $SRC_DIR"
        git -C "$SRC_DIR" fetch --quiet origin
        git -C "$SRC_DIR" checkout --quiet "$REF" 2>/dev/null || \
          git -C "$SRC_DIR" checkout --quiet "origin/$REF" 2>/dev/null || true
      else
        echo "    Cloning worker source to $SRC_DIR"
        git clone --quiet --depth 1 --branch "$REF" "$REPO_URL" "$SRC_DIR" 2>/dev/null || \
          git clone --quiet --depth 1 "$REPO_URL" "$SRC_DIR"
      fi

      # --- create venv ---
      echo "    Setting up Python environment at $VENV_DIR"
      python3 -m venv "$VENV_DIR"
      "$VENV_DIR/bin/pip" install --quiet --upgrade pip

      # --- install workers with transcription extras ---
      echo "    Installing byom_video_workers[transcribe]"
      if ! "$VENV_DIR/bin/pip" install --quiet -e "$SRC_DIR/workers[transcribe]"; then
        echo "WARNING: Worker install failed. You can retry manually:"
        echo "         $VENV_DIR/bin/pip install -e \"$SRC_DIR/workers[transcribe]\""
      else
        echo "    Python environment ready"
      fi

      # --- write BYOM_VIDEO_PYTHON ---
      PYTHON_LINE="export BYOM_VIDEO_PYTHON=\"$VENV_DIR/bin/python\""
      if [ -n "$SHELL_RC" ]; then
        append_to_shell_rc "$PYTHON_LINE"
      fi
      export BYOM_VIDEO_PYTHON="$VENV_DIR/bin/python"
    fi
  fi
fi

# --- ffmpeg hint ---
if ! command -v ffmpeg >/dev/null 2>&1; then
  echo ""
  echo "WARNING: ffmpeg not found. Install it for export workflows:"
  echo "         macOS:  brew install ffmpeg"
  echo "         Ubuntu: sudo apt-get install ffmpeg"
fi

# --- done ---
echo ""
echo "==> Done"
echo "    byom-video version: $(byom-video version 2>/dev/null | head -1 || echo 'installed')"
echo ""
echo "Next steps:"
echo "  1. Restart your terminal or run: source $SHELL_RC"
echo "  2. byom-video init"
echo "  3. byom-video doctor"

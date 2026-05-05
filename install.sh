#!/usr/bin/env sh
set -e

VENV_DIR="$HOME/.byom-venv"
SHELL_RC=""

echo "Installing BYOM Video..."

# --- Go binary ---
if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is not installed. Install it from https://go.dev/dl/ then re-run this script."
  exit 1
fi

echo "Building and installing byom-video..."
GOPROXY=direct GONOSUMDB='*' go install github.com/mirelahmd/OpenVFX/cmd/byom-video@latest

# --- PATH for ~/go/bin ---
GOBIN="$(go env GOPATH)/bin"
case ":$PATH:" in
  *":$GOBIN:"*) ;;
  *)
    if [ -f "$HOME/.zshrc" ]; then
      SHELL_RC="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
      SHELL_RC="$HOME/.bashrc"
    elif [ -f "$HOME/.profile" ]; then
      SHELL_RC="$HOME/.profile"
    fi
    if [ -n "$SHELL_RC" ]; then
      echo "" >> "$SHELL_RC"
      echo "# byom-video" >> "$SHELL_RC"
      echo "export PATH=\"\$PATH:$GOBIN\"" >> "$SHELL_RC"
      echo "Added $GOBIN to PATH in $SHELL_RC"
    fi
    export PATH="$PATH:$GOBIN"
    ;;
esac

# --- Python venv + faster-whisper ---
if ! command -v python3 >/dev/null 2>&1; then
  echo "Warning: python3 not found. Skipping transcription setup."
  echo "Install Python 3.10+ and re-run this script to enable transcription."
else
  echo "Setting up Python environment at $VENV_DIR..."
  python3 -m venv "$VENV_DIR"
  "$VENV_DIR/bin/pip" install --quiet --upgrade pip
  "$VENV_DIR/bin/pip" install --quiet faster-whisper

  # Install byom_video_workers from repo
  TMP_REPO="$(mktemp -d)"
  echo "Fetching worker package..."
  git clone --quiet --depth 1 https://github.com/mirelahmd/OpenVFX.git "$TMP_REPO"
  "$VENV_DIR/bin/pip" install --quiet "$TMP_REPO/workers"
  rm -rf "$TMP_REPO"

  # Write BYOM_VIDEO_PYTHON into shell config
  if [ -n "$SHELL_RC" ]; then
    echo "export BYOM_VIDEO_PYTHON=\"$VENV_DIR/bin/python\"" >> "$SHELL_RC"
  elif [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
    echo "" >> "$SHELL_RC"
    echo "# byom-video" >> "$SHELL_RC"
    echo "export BYOM_VIDEO_PYTHON=\"$VENV_DIR/bin/python\"" >> "$SHELL_RC"
  elif [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
    echo "" >> "$SHELL_RC"
    echo "# byom-video" >> "$SHELL_RC"
    echo "export BYOM_VIDEO_PYTHON=\"$VENV_DIR/bin/python\"" >> "$SHELL_RC"
  fi
  export BYOM_VIDEO_PYTHON="$VENV_DIR/bin/python"
  echo "Python environment ready."
fi

# --- ffmpeg check ---
if ! command -v ffmpeg >/dev/null 2>&1; then
  echo ""
  echo "Warning: ffmpeg not found. Install it to enable export workflows:"
  echo "  macOS:  brew install ffmpeg"
  echo "  Ubuntu: sudo apt-get install ffmpeg"
fi

# --- Done ---
echo ""
echo "Done. Run:"
echo "  byom-video version"
echo "  byom-video doctor"
echo ""
if [ -n "$SHELL_RC" ]; then
  echo "Restart your terminal or run: source $SHELL_RC"
fi

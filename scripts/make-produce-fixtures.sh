#!/usr/bin/env bash
# Generate the demo assets for `byom-video produce`.
#
# Everything here is synthesised locally with ffmpeg, so no binaries need to be
# committed. Each clip carries real speech where a speech synthesiser is
# available, which gives faster-whisper something genuine to transcribe.
#
# Usage: scripts/make-produce-fixtures.sh [output-dir]
set -euo pipefail

OUT_DIR="${1:-fixtures/produce-assets}"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "error: ffmpeg is required" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

LINES=(
  "The harbor lights came up as the fog rolled in."
  "She checked the rigging one last time before dawn."
  "Nobody expected the tide to turn that quickly."
)

# Pick whatever speech synthesiser this machine has. If none, we still produce
# valid assets - they just carry a tone instead of speech, and the production
# will honestly report that no speech was detected.
speak() {
  local text="$1" out="$2"
  if command -v say >/dev/null 2>&1; then
    say -o "$out" "$text" 2>/dev/null && return 0
  fi
  if command -v espeak-ng >/dev/null 2>&1; then
    espeak-ng -w "$out" "$text" 2>/dev/null && return 0
  fi
  if command -v espeak >/dev/null 2>&1; then
    espeak -w "$out" "$text" 2>/dev/null && return 0
  fi
  return 1
}

index=1
for line in "${LINES[@]}"; do
  clip="$OUT_DIR/clip_${index}.mp4"
  audio="$WORK_DIR/speech_${index}.aiff"

  if speak "$line" "$audio"; then
    ffmpeg -hide_banner -loglevel error -y \
      -f lavfi -i "testsrc=size=640x360:rate=25:duration=6" \
      -i "$audio" \
      -c:v libx264 -preset ultrafast -pix_fmt yuv420p \
      -c:a aac -t 6 \
      "$clip"
    echo "wrote $clip (speech: \"$line\")"
  else
    ffmpeg -hide_banner -loglevel error -y \
      -f lavfi -i "testsrc=size=640x360:rate=25:duration=6" \
      -f lavfi -i "sine=frequency=$((300 * index)):duration=6" \
      -c:v libx264 -preset ultrafast -pix_fmt yuv420p \
      -c:a aac -shortest \
      "$clip"
    echo "wrote $clip (no speech synthesiser found; tone only)"
  fi
  index=$((index + 1))
done

echo
echo "Assets ready in $OUT_DIR"
echo "Run: byom-video produce $OUT_DIR --brief \"Cut these clips into a 12-second vertical teaser with burned-in captions\""

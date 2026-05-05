#!/usr/bin/env sh
set -eu

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required to generate examples/fixtures/tiny.mp4"
  echo "Install hint: brew install ffmpeg"
  exit 1
fi

mkdir -p examples/fixtures

ffmpeg \
  -y \
  -f lavfi -i testsrc=size=320x180:rate=24:duration=2 \
  -f lavfi -i sine=frequency=440:duration=2 \
  -shortest \
  -pix_fmt yuv420p \
  examples/fixtures/tiny.mp4

echo "wrote examples/fixtures/tiny.mp4"

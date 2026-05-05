# `ffmpeg_commands.sh`

## Purpose

`ffmpeg_commands.sh` is an inspectable export script generated from `selected_clips.json` when available, or `roughcut.json` otherwise.

The script is generated only. BYOM Video does not execute FFmpeg cuts automatically.

## Behavior

- includes `#!/usr/bin/env bash`
- includes `set -euo pipefail`
- creates `exports/` relative to the run directory when executed
- writes one FFmpeg command per roughcut clip
- names outputs like `exports/clip_0001.mp4`
- supports two modes:

Stream copy:

```sh
ffmpeg -y -ss <start> -to <end> -i "<input>" -c copy "<output>"
```

Reencode:

```sh
ffmpeg -y -ss <start> -to <end> -i "<input>" -c:v libx264 -c:a aac "<output>"
```

The generated script stores its mode in a header comment: `# mode: stream-copy` or `# mode: reencode`.

## Modes

- `stream-copy`: fast, keyframe-dependent cuts
- `reencode`: slower, more frame-accurate cuts

## Roadmap

Future export stages may add:

- concat scripts
- timeline metadata
- DaVinci Resolve integration
- Premiere Pro integration

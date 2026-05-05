# `exports/`

## Purpose

`exports/` contains media files produced by explicit export execution.

Exports are not created during `byom-video run`. They are created only by:

```sh
./byom-video export <run_id>
```

## Safety Model

The export command:

- accepts a run id, not an arbitrary script path
- resolves the run under `.byom-video/runs/<run_id>`
- requires `ffmpeg_commands.sh` inside that run directory
- executes the script from inside the run directory
- uses `bash ffmpeg_commands.sh`
- records export events in `events.jsonl`
- records export status and exported files in `manifest.json`
- probes exported clips with `ffprobe` when available and writes `export_validation.json`
- refreshes `report.html` after successful export when the report artifact already exists

The generated FFmpeg script remains inspectable before execution.

## Current Output

The current export script writes clip files like:

```text
exports/clip_0001.mp4
exports/clip_0002.mp4
```

Only `mp4` export script output is supported in this milestone.

## Export Validation

After successful export execution, BYOM Video attempts to validate exported clips. The validation artifact records file existence, duration, video stream count, audio stream count, and per-file status.

If validation fails, exported files are left untouched. The manifest records the validation failure so the user can rerun or inspect locally.

## Limitations

Generated FFmpeg commands use stream copy by default. Stream-copy cuts are fast, but they may not be frame-perfect. The script also includes commented re-encode command templates for future frame-accurate workflows.

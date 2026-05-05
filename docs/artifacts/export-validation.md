# `export_validation.json`

`export_validation.json` records local integrity checks for media files produced by explicit export execution.

It is written under:

```text
.byom-video/runs/<run_id>/export_validation.json
```

The export command runs validation after `ffmpeg_commands.sh` succeeds. It probes exported clips with `ffprobe` when available and records duration plus stream counts.

Schema:

```json
{
  "schema_version": "export_validation.v1",
  "exports_dir": "exports",
  "checked_at": "2026-04-28T00:00:00Z",
  "files": [
    {
      "path": "exports/clip_0001.mp4",
      "exists": true,
      "duration_seconds": 4.48,
      "video_streams": 1,
      "audio_streams": 1,
      "status": "ok",
      "error": ""
    }
  ]
}
```

File paths are relative to the run directory.

If export succeeds but validation fails, exported files are left in place. The manifest records `export_validation_status: failed` and `export_validation_error`.

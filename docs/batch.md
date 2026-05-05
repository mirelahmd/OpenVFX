# Batch Processing

Batch processing runs the existing local pipeline over multiple media files in a folder.

```sh
./byom-video batch <input-dir> --preset metadata
./byom-video batch <input-dir> --preset shorts
```

Each media file becomes its own normal run under:

```text
.byom-video/runs/<run_id>/
```

Batch execution is sequential. A failed file is recorded in the batch summary and processing continues unless `--fail-fast` is passed.

## Presets

Supported presets:

- `metadata`: fast metadata-only runs.
- `shorts`: full shorts pipeline with transcript, captions, chunks, highlights, roughcut, FFmpeg script, and report.

The batch command reuses the same preset behavior as `byom-video pipeline`.

## File Discovery

By default, batch scans only the input directory itself. It skips directories and hidden files, then sorts files by name for deterministic processing.

Supported media extensions are case-insensitive:

```text
.mp4 .mov .m4v .mp3 .wav .m4a .aac .flac .webm .mkv
```

Use recursive scanning:

```sh
./byom-video batch media/inbox --preset metadata --recursive
```

Limit the number of processed files:

```sh
./byom-video batch media/inbox --preset metadata --limit 5
```

## Dry Run

Dry run prints the files that would be processed without creating runs or batch artifacts.

```sh
./byom-video batch media/inbox --preset metadata --dry-run
```

## Export And Validation

Batch does not export clips by default.

Validate each successful run:

```sh
./byom-video batch media/inbox --preset metadata --validate
```

Export each successful run:

```sh
./byom-video batch media/inbox --preset shorts --export
```

Export and validate:

```sh
./byom-video batch media/inbox --preset shorts --export-and-validate
```

`--export` and `--export-and-validate` require a preset that generates `ffmpeg_commands.sh`. The current `metadata` preset does not.

## Batch Summary

Non-dry-run batches write:

```text
.byom-video/batches/<batch_id>/batch_summary.json
```

Schema:

```json
{
  "schema_version": "batch_summary.v1",
  "batch_id": "20260429T010203Z-12345678",
  "created_at": "2026-04-29T01:02:03Z",
  "input_dir": "/path/to/media",
  "preset": "metadata",
  "recursive": false,
  "dry_run": false,
  "totals": {
    "discovered": 3,
    "attempted": 3,
    "succeeded": 2,
    "failed": 1,
    "skipped": 0
  },
  "items": [
    {
      "input_path": "/path/to/media/a.mp4",
      "status": "completed",
      "run_id": "20260429T010203Z-abcd1234",
      "run_dir": ".byom-video/runs/20260429T010203Z-abcd1234",
      "error": ""
    }
  ]
}
```

List batches:

```sh
./byom-video batches
```

Inspect a batch:

```sh
./byom-video inspect-batch <batch_id>
./byom-video inspect-batch <batch_id> --json
```

## Safety

- Batch accepts an input directory, not arbitrary scripts.
- Export remains explicit through `--export` or `--export-and-validate`.
- Dry run does not write artifacts.
- There is no database or web server; summaries are plain local files.

For continuous folder automation, use watch mode:

```sh
./byom-video watch media/inbox --preset metadata
```

Retry failed items from a batch:

```sh
./byom-video retry-batch <batch_id> --dry-run
./byom-video retry-batch <batch_id>
```

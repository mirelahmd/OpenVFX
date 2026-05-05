# `manifest.json`

## Purpose

`manifest.json` is the run index. It records the run identity, input file, lifecycle status, artifacts written, tool versions, export status, and failure details when applicable.

The manifest is rewritten as the run progresses. Readers should treat the final manifest as authoritative for the run outcome.

## Lifecycle Status

Current statuses:

- `running`: run folder has been created and workflow execution is in progress.
- `completed`: all requested workflow steps completed successfully.
- `failed`: a workflow step failed after run folder creation.

## Fields

- `run_id`: unique run identifier.
- `input_path`: absolute path to the input media file.
- `created_at`: run creation timestamp.
- `status`: current lifecycle status.
- `artifacts`: list of artifacts produced by the run.
- `tool_versions`: detected versions for tools used by the run, when available.
- `error_message`: failure message when `status` is `failed`.
- `exported_at`: timestamp for successful explicit export execution, when available.
- `export_status`: explicit export state, such as `completed` or `failed`, when export has been attempted.
- `exports_dir`: export directory path relative to the run directory, currently `exports`.
- `exported_files`: files generated under `exports/`, relative to the run directory.
- `export_error_message`: export failure message when explicit export fails.
- `export_validation_status`: export validation state, currently `completed` or `failed`, when validation has run.
- `export_validation_error`: export validation failure message when validation fails.

Future fields are allowed but not required. New readers should ignore unknown fields.

Validation requires artifact paths to be relative paths that do not contain `..` and do not escape the run directory.

## Artifact Entry

Each artifact entry currently includes:

- `name`: logical artifact name.
- `path`: path relative to the run directory.
- `created_at`: timestamp when the artifact was registered.

## Example

```json
{
  "run_id": "20260428T065531Z-d062c00f",
  "input_path": "/Users/example/BYOMVIDEO/examples/fixtures/tiny.mp4",
  "created_at": "2026-04-28T06:55:31.999294Z",
  "status": "completed",
  "artifacts": [
    {
      "name": "manifest",
      "path": "manifest.json",
      "created_at": "2026-04-28T06:55:31.999779Z"
    },
    {
      "name": "events",
      "path": "events.jsonl",
      "created_at": "2026-04-28T06:55:31.999780Z"
    },
    {
      "name": "metadata",
      "path": "metadata.json",
      "created_at": "2026-04-28T06:55:32.024165Z"
    },
    {
      "name": "transcript",
      "path": "transcript.json",
      "created_at": "2026-04-28T06:55:32.073514Z"
    }
  ],
  "tool_versions": {
    "ffprobe": "ffprobe version 8.1 Copyright (c) 2007-2026 the FFmpeg developers"
  }
}
```

Failed run example:

```json
{
  "run_id": "20260428T061702Z-cf4ae486",
  "input_path": "/Users/example/BYOMVIDEO/input.mp4",
  "created_at": "2026-04-28T06:17:02.518510Z",
  "status": "failed",
  "artifacts": [
    {
      "name": "manifest",
      "path": "manifest.json",
      "created_at": "2026-04-28T06:17:02.518909Z"
    },
    {
      "name": "events",
      "path": "events.jsonl",
      "created_at": "2026-04-28T06:17:02.518909Z"
    }
  ],
  "error_message": "ffprobe is missing"
}
```

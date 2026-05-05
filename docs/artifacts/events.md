# `events.jsonl`

## Purpose

`events.jsonl` is the append-only run timeline. It records workflow steps and artifact writes as they happen.

Each line is one JSON object. The file uses JSON Lines so readers can stream events without loading the whole file.

## Event Shape

Current event objects include:

- `time`: event timestamp.
- `type`: event name.
- `details`: optional event-specific data.

## Event Naming

Event names use uppercase words separated by underscores.

Current event examples:

- `RUN_STARTED`
- `ARTIFACT_WRITTEN`
- `FFPROBE_STARTED`
- `FFPROBE_COMPLETED`
- `TRANSCRIBE_STUB_STARTED`
- `TRANSCRIBE_STUB_COMPLETED`
- `TRANSCRIBE_STUB_FAILED`
- `TRANSCRIBE_STARTED`
- `TRANSCRIBE_COMPLETED`
- `TRANSCRIBE_FAILED`
- `TRANSCRIPT_VALIDATION_STARTED`
- `TRANSCRIPT_VALIDATION_COMPLETED`
- `TRANSCRIPT_VALIDATION_FAILED`
- `CAPTIONS_STARTED`
- `CAPTIONS_COMPLETED`
- `CAPTIONS_FAILED`
- `CHUNKING_STARTED`
- `CHUNKING_COMPLETED`
- `CHUNKING_FAILED`
- `CHUNKS_VALIDATION_STARTED`
- `CHUNKS_VALIDATION_COMPLETED`
- `CHUNKS_VALIDATION_FAILED`
- `HIGHLIGHTS_STARTED`
- `HIGHLIGHTS_COMPLETED`
- `HIGHLIGHTS_FAILED`
- `HIGHLIGHTS_VALIDATION_STARTED`
- `HIGHLIGHTS_VALIDATION_COMPLETED`
- `HIGHLIGHTS_VALIDATION_FAILED`
- `ROUGHCUT_STARTED`
- `ROUGHCUT_COMPLETED`
- `ROUGHCUT_FAILED`
- `ROUGHCUT_VALIDATION_STARTED`
- `ROUGHCUT_VALIDATION_COMPLETED`
- `ROUGHCUT_VALIDATION_FAILED`
- `FFMPEG_SCRIPT_STARTED`
- `FFMPEG_SCRIPT_COMPLETED`
- `FFMPEG_SCRIPT_FAILED`
- `REPORT_STARTED`
- `REPORT_COMPLETED`
- `REPORT_FAILED`
- `EXPORT_STARTED`
- `EXPORT_COMPLETED`
- `EXPORT_FAILED`
- `EXPORT_VALIDATION_STARTED`
- `EXPORT_VALIDATION_COMPLETED`
- `EXPORT_VALIDATION_FAILED`
- `RUN_COMPLETED`
- `RUN_FAILED`

Future events should follow the same naming style.

## Example

```jsonl
{"time":"2026-04-28T06:55:31.999511Z","type":"RUN_STARTED","details":{"input_path":"/Users/example/BYOMVIDEO/examples/fixtures/tiny.mp4","run_dir":".byom-video/runs/20260428T065531Z-d062c00f"}}
{"time":"2026-04-28T06:55:31.999930Z","type":"ARTIFACT_WRITTEN","details":{"path":"manifest.json"}}
{"time":"2026-04-28T06:55:31.999936Z","type":"ARTIFACT_WRITTEN","details":{"path":"events.jsonl"}}
{"time":"2026-04-28T06:55:31.999941Z","type":"FFPROBE_STARTED","details":{"input_path":"/Users/example/BYOMVIDEO/examples/fixtures/tiny.mp4"}}
{"time":"2026-04-28T06:55:32.024182Z","type":"FFPROBE_COMPLETED","details":{"path":"metadata.json"}}
{"time":"2026-04-28T06:55:32.024225Z","type":"ARTIFACT_WRITTEN","details":{"path":"metadata.json"}}
{"time":"2026-04-28T06:55:32.040989Z","type":"TRANSCRIBE_STUB_STARTED","details":{"input_path":"/Users/example/BYOMVIDEO/examples/fixtures/tiny.mp4"}}
{"time":"2026-04-28T06:55:32.073518Z","type":"TRANSCRIBE_STUB_COMPLETED","details":{"path":"transcript.json"}}
{"time":"2026-04-28T06:55:32.073553Z","type":"ARTIFACT_WRITTEN","details":{"path":"transcript.json"}}
{"time":"2026-04-28T06:55:32.073658Z","type":"RUN_COMPLETED","details":{"status":"completed"}}
```

Failure example:

```jsonl
{"time":"2026-04-28T06:07:51.611601Z","type":"FFPROBE_STARTED","details":{"input_path":"/Users/example/BYOMVIDEO/input.mp4"}}
{"time":"2026-04-28T06:07:51.611692Z","type":"RUN_FAILED","details":{"error":"ffprobe is missing"}}
```

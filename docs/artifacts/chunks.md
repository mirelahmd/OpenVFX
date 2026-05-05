# `chunks.json`

## Purpose

`chunks.json` groups transcript segments into deterministic time-window chunks.

Chunks are the next stable artifact after transcription. Future highlight detection, rough-cut planning, summaries, and masked inference steps should consume chunks instead of raw transcript segments when they need bounded source units.

## Schema Version

Current schema version:

```text
chunks.v1
```

## Source

The source object identifies the upstream artifact and deterministic strategy:

- `transcript_artifact`: transcript artifact path, currently `transcript.json`
- `mode`: `deterministic`
- `strategy`: `time_window_v1`

## Chunking Settings

- `target_seconds`: target maximum chunk duration before starting a new chunk
- `max_gap_seconds`: maximum allowed gap between adjacent transcript segments before starting a new chunk

Defaults:

- `target_seconds`: `30`
- `max_gap_seconds`: `2.0`

## Chunk Fields

Each chunk includes:

- `id`: stable chunk identifier, such as `chunk_0001`
- `start`: chunk start time in seconds
- `end`: chunk end time in seconds
- `duration_seconds`: `end - start`
- `text`: combined transcript text from source segments
- `segment_ids`: transcript segment IDs included in the chunk
- `word_count`: whitespace-based word count for the chunk text

## Deterministic Strategy

The current strategy reads transcript segments in order and groups them into chunks:

- keep adding segments while the chunk stays within `target_seconds`
- start a new chunk if adding a segment would exceed `target_seconds`
- start a new chunk if the gap from the previous segment is greater than `max_gap_seconds`
- preserve segment IDs
- join segment text with spaces
- compute duration and word count

If a transcript has one short segment, chunking produces one chunk.

## Validation Expectations

Validation checks:

- `schema_version` is `chunks.v1`
- `chunks` exists and is an array
- each chunk has `id`, `start`, `end`, `duration_seconds`, `text`, `segment_ids`, and `word_count`
- `end >= start`
- `duration_seconds >= 0`
- `text` is a string
- `segment_ids` is an array
- `word_count >= 0`

## Example

```json
{
  "schema_version": "chunks.v1",
  "source": {
    "transcript_artifact": "transcript.json",
    "mode": "deterministic",
    "strategy": "time_window_v1"
  },
  "chunking": {
    "target_seconds": 30,
    "max_gap_seconds": 2.0
  },
  "chunks": [
    {
      "id": "chunk_0001",
      "start": 0.0,
      "end": 4.48,
      "duration_seconds": 4.48,
      "text": "Okay, okay, okay.",
      "segment_ids": ["seg_0001"],
      "word_count": 3
    }
  ]
}
```

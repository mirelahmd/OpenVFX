# `roughcut.json`

## Purpose

`roughcut.json` stores a deterministic rough-cut plan derived from highlight candidates.

This is a planning artifact only. It does not export media, trim video, or create an NLE timeline.

## Schema Version

```text
roughcut.v1
```

## Strategy

Current strategy:

```text
top_highlights_v1
```

Behavior:

- select top highlights by score
- default max clips is `5`
- order selected clips by original timeline start time
- compute total rough-cut duration
- preserve source highlight text

## Example

```json
{
  "schema_version": "roughcut.v1",
  "source": {
    "highlights_artifact": "highlights.json",
    "mode": "deterministic",
    "strategy": "top_highlights_v1"
  },
  "plan": {
    "title": "Rough Cut Plan",
    "intent": "Select strongest highlight candidates in timeline order.",
    "total_duration_seconds": 58.2
  },
  "clips": [
    {
      "id": "clip_0001",
      "highlight_id": "hl_0001",
      "source_chunk_id": "chunk_0001",
      "start": 0.0,
      "end": 28.4,
      "duration_seconds": 28.4,
      "order": 1,
      "score": 0.72,
      "edit_intent": "Keep this segment as a candidate short clip.",
      "text": "Example transcript chunk text."
    }
  ]
}
```

## Validation Expectations

Validation checks:

- `schema_version` is `roughcut.v1`
- `clips` exists and is an array
- each clip has `id`, `highlight_id`, `source_chunk_id`, `start`, `end`, `duration_seconds`, `order`, `score`, `edit_intent`, and `text`
- `end >= start`
- `duration_seconds >= 0`
- `order >= 1`
- `score` is between `0` and `1`

## Limitations

This is not a final edit. It is a deterministic selection plan that future export, NLE integration, or planning layers can consume.

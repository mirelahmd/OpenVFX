# `highlights.json`

## Purpose

`highlights.json` stores deterministic highlight candidates derived from transcript chunks.

This is an editor-intelligence planning artifact. It does not cut video and does not use an LLM.

## Schema Version

```text
highlights.v1
```

## Strategy

Current strategy:

```text
heuristic_v1
```

The scorer uses deterministic local heuristics:

- prefer chunks with useful duration
- penalize chunks that are too short or too long
- prefer chunks with enough words
- boost hook phrases such as `the problem is`, `the key is`, `what matters`, and `let me explain`
- boost questions
- boost emphasis markers such as `really`, `actually`, `important`, `never`, `must`, and `need`
- penalize empty or near-empty text
- normalize score to `0.0` through `1.0`
- sort candidates by descending score

## Example

```json
{
  "schema_version": "highlights.v1",
  "source": {
    "chunks_artifact": "chunks.json",
    "mode": "deterministic",
    "strategy": "heuristic_v1"
  },
  "scoring": {
    "min_duration_seconds": 3,
    "max_duration_seconds": 90,
    "top_k": 10
  },
  "highlights": [
    {
      "id": "hl_0001",
      "chunk_id": "chunk_0001",
      "start": 0.0,
      "end": 28.4,
      "duration_seconds": 28.4,
      "score": 0.72,
      "label": "Candidate highlight",
      "reason": "Candidate selected because it has sufficient word count, contains emphasis markers.",
      "text": "Example transcript chunk text.",
      "signals": {
        "word_count": 42,
        "has_question": false,
        "has_hook_phrase": true,
        "has_emphasis_marker": true
      }
    }
  ]
}
```

## Validation Expectations

Validation checks:

- `schema_version` is `highlights.v1`
- `highlights` exists and is an array
- each highlight has `id`, `chunk_id`, `start`, `end`, `duration_seconds`, `score`, `label`, `reason`, `text`, and `signals`
- `end >= start`
- `duration_seconds >= 0`
- `score` is between `0` and `1`
- `text` is a string

## Limitations

This is a heuristic candidate detector. It is not semantic highlight detection. Future LLM or inference-mask stages may consume `highlights.json`, but this artifact remains deterministic and replayable.

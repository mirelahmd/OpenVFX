# `creative_treatment.json` and `asset_observations.json`

Schemas: `openvfx_creative_treatment.v1`, `openvfx_asset_observations.v1`

These are the two artifacts of the Creative Director, the reasoning layer that
decides **what should be made** before deterministic production planning begins.

Both live under the production root:

```
.byom-video/productions/<production_id>/
  asset_observations.json    what the source material is
  creative_treatment.json    what should be made with it
  director/
    director_config.json     the routing the Go runtime resolved
    brief.txt                the creator's request, verbatim
    graph_trace.json         per-node trace: which nodes reasoned, which fell back
```

## `asset_observations.json`

The compact, cheap, local description the director reasons over. Derived from
real ffprobe output, with no computer vision and no model calls.

```json
{
  "schema_version": "openvfx_asset_observations.v1",
  "assets": [
    {
      "id": "asset_0001",
      "path": "/work/assets/clip_1.mp4",
      "media_type": "audiovisual",
      "duration_seconds": 6.0,
      "width": 640, "height": 360,
      "aspect_ratio": "16:9",
      "frame_rate": "25",
      "has_video": true, "has_audio": true,
      "has_transcript": false,
      "origin": "source"
    }
  ],
  "totals": {"asset_count": 3, "duration_seconds": 18.0, "with_audio": 3},
  "warnings": ["notes.txt: not observed (unrecognised media extension .txt)"]
}
```

Two rules matter:

- **Transcripts are referenced, never embedded.** `transcript_ref` plus
  `transcript_chars` — never the transcript body. This artifact goes into a
  prompt and has to stay small.
- **A failed observation is a warning, not a crash.** Losing one asset must not
  cost the creator the whole session.

## `creative_treatment.json`

The director's structured interpretation. It holds conclusions, decisions,
evidence references and one-sentence rationale — and deliberately no
chain-of-thought.

Key sections:

| Field | Purpose |
|---|---|
| `reasoning` | provenance: requested vs effective mode, model, fallback, `semantic_reasoning` |
| `objective`, `audience`, `platform`, `tone`, `visual_language` | interpreted intent |
| `opening_strategy`, `narrative_structure`, `pacing_strategy` | structure and phasing |
| `asset_roles` | how each supplied asset should be used, with explicit confidence |
| `segments` | the intended cuts — **this is what becomes real ffmpeg operations** |
| `audio_strategy`, `caption_strategy` | treatment of sound and text |
| `generated_asset_needs`, `transformation_requests` | what the assets cannot cover |
| `degraded_alternatives` | what to do instead when a need cannot be met |
| `success_criteria` | the director's own statement of "done", checked by the validator |
| `decisions` | citable creative decisions; stages link back by `decision_id` |
| `critique` | findings and revisions from the single bounded critique pass |
| `uncertainties`, `assumptions` | what was inferred rather than understood |

### The reasoning contract

```json
"reasoning": {
  "requested_mode": "llm",
  "effective_mode": "deterministic",
  "provider": "", "model": "",
  "fallback_used": true,
  "fallback_reason": "ollama request failed: connection refused",
  "semantic_reasoning": false
}
```

`semantic_reasoning` is true **only** when a language model actually completed a
call. Rules-based output that claimed it would be rejected by the Go validator
(`CreativeTreatment.Validate`). This is the one invariant the whole layer rests
on: a reader can always tell whether the system understood the request or
pattern-matched it.

Three modes exist:

| Mode | Meaning |
|---|---|
| `llm` | a model reasoned; `semantic_reasoning` is true |
| `deterministic` | the graph ran with no model; rules produced a real but limited treatment |
| *(no treatment)* | the sidecar could not run at all; production falls back to intent-only planning and says so |

### Asset roles

Allowed: `primary_narration`, `hero_shot`, `hook`, `b_roll`,
`atmospheric_insert`, `reaction`, `transition_material`, `outro`, `audio_bed`,
`reference_asset`, `generated_asset_candidate`, `unknown`.

Every role carries `confidence` (`high`/`medium`/`low`/`uncertain`). Roles are
planning decisions only — no source file is moved or modified. `unknown` with
`uncertain` is the correct answer when the evidence does not support a role, and
the deterministic path uses it freely.

### Segments are the load-bearing bridge

```json
{"id": "seg_0001", "order": 1, "asset_id": "asset_0001",
 "source_in": 1.0, "source_out": 5.0,
 "purpose": "Aggressive cold open, starting after the first beat of motion",
 "beat_ref": "beat_0001", "decision_id": "dec_0001"}
```

This becomes an entry in `edit_decisions.json` and then a literal ffmpeg call:

```
ffmpeg -hide_banner -y -ss 1.000 -i .../clip_1.mp4 -t 4.000 ...
```

so a rendered frame traces back to `dec_0001` — "Start 1s into clip_1 rather
than at frame zero, because the creator asked to open aggressively."

The Go runtime treats every segment as untrusted input: unknown asset ids,
inverted ranges and out-points past the end of a clip are all rejected, and a
treatment that fails validation is discarded entirely rather than partly applied.

## Validation of model output

`CreativeTreatment.Validate` enforces:

- schema version and a non-empty objective
- a known reasoning mode, and no false `semantic_reasoning` claim
- every `asset_id` exists in the observation, with no duplicates
- `source_out > source_in`, `source_in >= 0`, and `source_out` within the asset
- every `decision_id` on a segment resolves to a real decision

A segment marked `needs_generation` may omit `asset_id`; it is excluded from the
edit and reported as a gap rather than silently dropped.

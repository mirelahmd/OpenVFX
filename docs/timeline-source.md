# Timeline Source — Synthetic Fallback for Silent/Generated Videos

## Problem

The standard pipeline is speech-driven:
transcript → chunks → highlights → roughcut → creative-timeline

AI-generated video (b-roll, synthetic clips) has no audio, so roughcut is never produced. Before this feature, `creative-timeline` would fail with "0 clips".

## Solution

`creative-timeline` now has a three-level fallback chain:

1. **Normal path** — reads `selected_clips.json`, `goal_roughcut.json`, `enhanced_roughcut.json`, or `roughcut.json` from the run directory.
2. **timeline_source.json (external)** — if none of the above exist, reads `timeline_source.json` from the run directory (written by a prior step or external tool).
3. **Synthetic build** — if nothing is found, probes the source video duration via ffprobe, parses the goal text for time instructions, and generates clips deterministically. Writes `timeline_source.json` to the run directory and sets `synthetic_fallback: true` in the timeline artifact.

## timeline_source.json schema

Schema version: `openvfx_timeline_source.v1`

```json
{
  "schema_version": "openvfx_timeline_source.v1",
  "created_at": "...",
  "source": "synthetic_time_based",
  "reason": "...",
  "input_path": "/abs/path/to/video.mp4",
  "duration_seconds": 30.0,
  "clips": [
    {
      "id": "fallback_clip_0001",
      "source_path": "/abs/path/to/video.mp4",
      "source_start": 0.0,
      "source_end": 10.0,
      "start": 0.0,
      "end": 10.0,
      "duration_seconds": 10.0,
      "repeat_index": 1,
      "description": "first 10.00 seconds (repeat 1/2)"
    }
  ],
  "warnings": []
}
```

`start`/`end` are aliases for `source_start`/`source_end` so the artifact is compatible with `readClipsFromArtifact`.

## Goal text time instructions

The parser (`ParseTimeInstruction`) extracts time ranges and repeat counts from the goal string:

| Pattern | Example | Behaviour |
|---|---|---|
| `whole/entire/full video` | "use the whole video" | start=0, end=duration |
| `first N seconds` | "first 10 seconds" | start=0, end=N (clamped) |
| `last N seconds` | "last 5 seconds" | start=duration-N, end=duration |
| `from X to Y seconds` | "from 10 to 30 seconds" | start=X, end=Y |
| `use X to Y seconds` | "use 5 to 20 seconds" | start=X, end=Y |
| `seconds X-Y` | "seconds 10-30" | start=X, end=Y |
| `loop last N seconds` | "loop the last 2 seconds" | last N seconds, repeat=2 |
| `loop last N seconds M times` | "loop last 2 seconds 3 times" | last N seconds, repeat=M |
| No time instruction | "make it cinematic" | whole video (default) |

Repeat count modifiers (applied on top of any range):

| Keyword | Repeat count |
|---|---|
| `twice` | 2 |
| `three times` | 3 |
| `N times` | N (max 10) |
| `loop`/`repeat` (alone) | 2 |

Repeat count is clamped to `maxTimelineRepeats = 10`.

All values are clamped to `[0, duration]`. Invalid ranges (end ≤ start) fall back to whole video.

## Warnings

Warnings are propagated through the chain:

- Clamp warnings from the parser appear in `timeline_source.json` → `warnings[]`
- The fallback notice ("No speech-driven roughcut clips found...") appears in `creative_timeline.json` → `warnings[]`
- `make --yes` surfaces these warnings to stdout and includes them in `make_summary.json` → `warnings[]`

## Requirements

- **ffprobe** must be on `PATH` for the synthetic build to work. If unavailable, `creative-timeline` emits a warning and produces 0 clips.
- The fallback only runs when `opts.RunID` is set (i.e. when `creative-timeline` is called with `--run-id`).

## Testing

```sh
# Unit tests
go test ./internal/commands/ -run TestParseTimeInstruction
go test ./internal/commands/ -run TestBuildSyntheticTimelineSource
go test ./internal/commands/ -run TestReadClipsFromArtifact_TimelineSource

# Smoke test (requires a video file under ./media/)
./scripts/smoke-synthetic-timeline.sh
# or with a specific file:
./scripts/smoke-synthetic-timeline.sh path/to/generated.mp4
```

## End-to-end (generated b-roll)

```sh
# Copy generated video into place
mkdir -p media
cp ~/Downloads/generated_broll.mp4 media/generated_broll.mp4

# Full make — no --goal-aware needed, synthetic fallback handles it
byom-video make media/generated_broll.mp4 \
  --goal "generate cinematic b-roll — loop the last 2 seconds 3 times" \
  --yes
```

The pipeline will:
1. Run the pipeline (transcript will be empty/skipped — silent video)
2. Skip roughcut stages gracefully
3. Hit the synthetic fallback in `creative-timeline`
4. Write `timeline_source.json` with the parsed instruction (loop last 2s × 3)
5. Assemble a draft with 3 clips × 2s = 6s total

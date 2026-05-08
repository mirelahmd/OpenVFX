# Creative Timeline Artifact

Written by `creative-timeline`. No providers are called. No rendering is performed.

## File: `outputs/creative_timeline.json`

Schema version: `creative_timeline.v1`

```json
{
  "schema_version": "creative_timeline.v1",
  "created_at": "...",
  "creative_plan_id": "...",
  "run_id": "...",
  "goal": "...",
  "input_path": "...",
  "mode": "stub",
  "source": {
    "clip_artifact": "...",
    "clip_count": 3,
    "stub_outputs": true
  },
  "tracks": [
    {
      "id": "track_video_main",
      "kind": "video",
      "items": [
        {
          "id": "video_clip_001",
          "kind": "source_clip",
          "timeline_start": 0.0,
          "timeline_end": 12.5,
          "source_start": 4.2,
          "source_end": 16.7,
          "text": "clip transcript text"
        }
      ]
    },
    {
      "id": "track_voiceover",
      "kind": "audio",
      "items": [...]
    },
    {
      "id": "track_captions",
      "kind": "text",
      "items": [...]
    },
    {
      "id": "track_visual_overlays",
      "kind": "visual",
      "items": [...]
    }
  ],
  "total_duration_seconds": 45.0,
  "warnings": []
}
```

## Tracks

| Track ID | Kind | Content |
|---|---|---|
| `track_video_main` | `video` | Source clip items from run artifacts or empty |
| `track_voiceover` | `audio` | Single voiceover placeholder spanning full duration |
| `track_captions` | `text` | Caption placeholders aligned to video clips with text |
| `track_visual_overlays` | `visual` | Visual overlay placeholders from visual_asset_prompts.json |

## Timeline Items

Each item has:

- `id` — unique within the timeline
- `kind` — `source_clip`, `voiceover_placeholder`, `caption`, `visual_overlay_placeholder`
- `timeline_start` / `timeline_end` — position in the assembled output (seconds)
- `source_start` / `source_end` — position in the source media (video clips only)
- `text` — transcript text or caption text (if applicable)
- `label` / `notes` — descriptive metadata

## Clip Source Priority

When `--run-id` is provided:

1. `selected_clips.json`

With `--prefer-goal`:

1. `goal_roughcut.json`
2. `enhanced_roughcut.json`
3. `roughcut.json`
4. `selected_clips.json`

If no clips are found, video and caption tracks are empty and a warning is added.

## Validation

`validate-creative-plan` checks:

- `schema_version = "creative_timeline.v1"`
- `tracks` field exists
- `total_duration_seconds` is non-negative

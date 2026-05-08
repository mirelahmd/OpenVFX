# Creative Render Plan Artifact

Written by `creative-render-plan`. Requires `creative_timeline.json`. No rendering is performed.

## File: `outputs/creative_render_plan.json`

Schema version: `creative_render_plan.v1`

```json
{
  "schema_version": "creative_render_plan.v1",
  "created_at": "...",
  "creative_plan_id": "...",
  "run_id": "...",
  "goal": "...",
  "mode": "stub",
  "source": {
    "timeline_artifact": "outputs/creative_timeline.json",
    "track_count": 4,
    "total_duration_seconds": 45.0
  },
  "planned_output": {
    "planned_file": "outputs/draft.mp4",
    "duration_seconds": 45.0,
    "format": "mp4",
    "mode": "stub"
  },
  "steps": [
    {
      "step_index": 0,
      "operation": "cut_source_clip",
      "item_id": "video_clip_001",
      "track_id": "track_video_main",
      "timeline_start": 0.0,
      "timeline_end": 12.5,
      "notes": "source 4.20–16.70s | transcript text"
    }
  ],
  "warnings": []
}
```

## Operations

| Track kind | Operation |
|---|---|
| `video` | `cut_source_clip` |
| `audio` | `attach_voiceover_placeholder` |
| `text` | `add_caption_placeholder` |
| `visual` | `add_visual_overlay_placeholder` |

## Planned Output

`planned_output.planned_file` is always `outputs/draft.mp4` in stub mode. No file is created.

## Validation

`validate-creative-plan` checks:

- `schema_version = "creative_render_plan.v1"`
- `planned_output.planned_file` is non-empty
- `steps` field exists

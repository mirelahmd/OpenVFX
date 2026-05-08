# Creative Outputs Artifacts

Artifacts written by `creative-execute-stub`. All outputs are local placeholders — no providers are called.

## Index: `outputs/creative_outputs.json`

Schema version: `creative_outputs.v1`

```json
{
  "schema_version": "creative_outputs.v1",
  "created_at": "...",
  "creative_plan_id": "...",
  "mode": "stub",
  "artifacts": [
    {
      "type": "script",
      "path": "outputs/script_draft.json",
      "step_id": "step_0001",
      "status": "created"
    }
  ],
  "warnings": []
}
```

Always written, even if some steps are skipped. Lists every artifact with its type, path relative to the plan directory, step ID, and status.

## Script: `outputs/script_draft.json` + `script_draft.txt`

Schema version: `creative_script.v1`. Also writes a plain-text version at `script_draft.txt`.

## Voiceover: `outputs/voiceover_plan.json`

Schema version: `voiceover_plan.v1`. Contains `script_source`, `voice_backend`, `expected_output`. No audio is generated.

## Visual Assets: `outputs/visual_asset_prompts.json`

Schema version: `visual_asset_prompts.v1`. Contains a list of prompts with `kind` (`video_generation`, `image_generation`, or `unknown`), `prompt`, and `intended_use`. No images or video are generated.

## Captions: `outputs/caption_plan.json`

Schema version: `caption_plan.v1`.

## Audio: `outputs/audio_asset_plan.json`

Schema version: `audio_asset_plan.v1`. No audio is generated.

## Visual Transform: `outputs/visual_transform_plan.json`

Schema version: `visual_transform_plan.v1`. No transform is performed.

## Translation: `outputs/translation_plan.json`

Schema version: `translation_plan.v1`.

## Composition / Render: `outputs/composition_plan.json`

Schema version: `composition_plan.v1`. Contains `inputs` (video path, script, voiceover) and `planned_output`. No media is rendered.

## Validation

`validate-creative-plan` validates `creative_outputs.json` if present:
- Checks `schema_version = "creative_outputs.v1"`
- Checks `artifacts` field exists
- Checks each listed artifact path exists on disk
- Checks each artifact file's `schema_version` matches the expected value for its type

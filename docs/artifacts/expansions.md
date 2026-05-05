# `expansions/`

The `expansions/` directory contains stub or future provider-generated expansion outputs for each task type defined in `expansion_tasks.json`.

## Files

| File | Task type |
|---|---|
| `expansions/caption_variants.json` | Per-clip caption variants |
| `expansions/timeline_labels.json` | Per-clip short timeline labels |
| `expansions/short_descriptions.json` | Per-clip short descriptions |

## Schema

All expansion output files share `expansion_output.v1`:

```json
{
  "schema_version": "expansion_output.v1",
  "created_at": "...",
  "mode": "stub",
  "task_type": "caption_variants",
  "source": {
    "inference_mask_artifact": "inference_mask.json",
    "expansion_tasks_artifact": "expansion_tasks.json",
    "task_ids": ["task_0001"]
  },
  "items": [
    {
      "id": "cap_decision_0001_0001",
      "task_id": "task_0001",
      "decision_id": "decision_0001",
      "text": "Stub caption 1 for decision_0001: ...",
      "start": 0.0,
      "end": 28.4,
      "metadata": {
        "model_route": "caption_expansion",
        "stub": true,
        "variant": 1
      }
    }
  ]
}
```

## Stub behavior

`mode: stub` means no model provider was called. All text is deterministically generated from decision IDs, timing, text previews, and reasons in `inference_mask.json`.

- `caption_variants`: generates up to `output_contract.max_items` variants (default 3) per non-rejected decision. Text is capped to `max_words`.
- `timeline_labels`: generates one label per non-rejected decision.
- `short_descriptions`: generates one description per non-rejected decision.

Decisions with `decision: reject` are excluded from all stub outputs. If all decisions are rejected, `items` is empty and a warning is recorded.

## Commands

```sh
./byom-video expand-stub <run_id>
./byom-video expand-stub <run_id> --overwrite
./byom-video expand-stub <run_id> --task-type caption_variants
./byom-video expand-stub <run_id> --json
./byom-video expansion-validate <run_id>
./byom-video expansion-validate <run_id> --json
./byom-video review-expansions <run_id>
./byom-video review-expansions <run_id> --write-artifact
./byom-video review-expansions <run_id> --json
```

`expand-stub` writes expansion output files and records them in the run manifest. `expansion-validate` checks each file's schema, item shape, timing, and whether any item references a rejected decision. `review-expansions --write-artifact` writes `expansions_review.md` and records it in the manifest.

## Future use

When real model providers are configured and `models.enabled: true`, a future `expand` command will call the resolved model route for each task and write the same `expansion_output.v1` schema with `mode: provider`. The stub outputs serve as a structural contract test for that pipeline.

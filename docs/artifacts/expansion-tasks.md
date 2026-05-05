# `expansion_tasks.json`

`expansion_tasks.json` is a deterministic task contract for future cheap/free/local expansion models.

Expansion tasks reference decisions from `inference_mask.json`. Expanders may rewrite style or produce variants inside the mask, but they may not add unsupported facts.

Schema sketch:

```json
{
  "schema_version": "expansion_tasks.v1",
  "source": {
    "inference_mask_artifact": "inference_mask.json"
  },
  "tasks": [
    {
      "id": "task_0001",
      "type": "caption_variants",
      "model_route": "caption_expansion",
      "input_refs": ["decision_0001"],
      "output_contract": {
        "max_items": 3,
        "max_words": 18
      }
    }
  ]
}
```

Current behavior:

- no expansion model is called
- no provider SDK is loaded
- `mask-template` can write `expansion_tasks.template.json`
- `expansion-plan` writes `expansion_tasks.json` from `inference_mask.json`
- generated tasks include `caption_variants`, `timeline_labels`, and `short_descriptions`

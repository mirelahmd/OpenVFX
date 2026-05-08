# `clip_cards.json`

Editor-facing clip summaries derived from `roughcut.json` plus any available mask, expansion, and verification artifacts.

Path:

```text
.byom-video/runs/<run_id>/clip_cards.json
```

Schema:

```json
{
  "schema_version": "clip_cards.v1",
  "created_at": "2026-05-02T00:00:00Z",
  "run_id": "20260502T000000Z-demo",
  "source": {
    "roughcut_artifact": "roughcut.json",
    "inference_mask_artifact": "inference_mask.json",
    "expansions_dir": "expansions"
  },
  "cards": [
    {
      "id": "card_0001",
      "clip_id": "clip_0001",
      "highlight_id": "hl_0001",
      "decision_id": "decision_0001",
      "start": 0.0,
      "end": 28.4,
      "duration_seconds": 28.4,
      "score": 0.72,
      "title": "Timeline label",
      "description": "Short editor-facing description.",
      "captions": ["Caption one", "Caption two"],
      "source_text": "Source preview text.",
      "edit_intent": "Keep the hook and thesis.",
      "verification_status": "passed",
      "warnings": []
    }
  ]
}
```

Produced by:

```sh
./byom-video clip-cards <run_id>
```

Optional goal-aware source selection:

```sh
./byom-video clip-cards <run_id> --prefer-goal-roughcut
```

When `--prefer-goal-roughcut` is used, `clip_cards.json` is built from `goal_roughcut.json` instead of the default roughcut path. This stays explicit and local.

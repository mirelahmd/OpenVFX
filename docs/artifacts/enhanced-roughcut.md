# `enhanced_roughcut.json`

An additive roughcut artifact that combines `roughcut.json` with clip-card titles, descriptions, captions, and verification status.

Path:

```text
.byom-video/runs/<run_id>/enhanced_roughcut.json
```

Schema:

```json
{
  "schema_version": "enhanced_roughcut.v1",
  "created_at": "2026-05-02T00:00:00Z",
  "run_id": "20260502T000000Z-demo",
  "source": {
    "roughcut_artifact": "roughcut.json",
    "clip_cards_artifact": "clip_cards.json"
  },
  "plan": {
    "title": "Enhanced Rough Cut Plan",
    "intent": "Editor-ready clip summary",
    "total_duration_seconds": 58.2
  },
  "clips": [
    {
      "id": "clip_0001",
      "start": 0.0,
      "end": 28.4,
      "order": 1,
      "title": "Timeline label",
      "description": "Short editor-facing description.",
      "caption_suggestions": ["Caption one", "Caption two"],
      "edit_intent": "Keep the hook and thesis.",
      "verification_status": "passed",
      "source_text": "Source preview text."
    }
  ]
}
```

Produced by:

```sh
./byom-video enhance-roughcut <run_id>
```

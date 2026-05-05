# `inference_mask.json`

`inference_mask.json` is a deterministic planning artifact generated from existing run artifacts.

The mask records compact decisions and constraints. In Prompt 025 it is generated locally from `roughcut.json` or `highlights.json`; no model provider is called.

Schema sketch:

```json
{
  "schema_version": "inference_mask.v1",
  "source": {
    "chunks_artifact": "chunks.json",
    "highlights_artifact": "highlights.json",
    "mode": "planned",
    "reasoner": "premium_reasoner"
  },
  "intent": "create_short_highlights",
  "constraints": {
    "must_include": [],
    "must_not_include": [],
    "tone": "technical, concise",
    "max_caption_words": 18
  },
  "decisions": [
    {
      "id": "decision_0001",
      "highlight_id": "hl_0001",
      "start": 0.0,
      "end": 28.4,
      "decision": "keep",
      "reason": "Strong hook and clear thesis."
    }
  ]
}
```

Valid `decision` values: `keep`, `reject`, `candidate_keep`.

Template and planning commands:

```sh
./byom-video mask-template <run_id>
./byom-video mask-plan <run_id>
./byom-video mask-validate <run_id>
./byom-video review-mask <run_id> --write-artifact
```

Decision-level editing commands (deterministic, no provider calls):

```sh
./byom-video mask-decisions <run_id> [--json]
./byom-video mask-decision <run_id> <decision_id> --set <keep|reject|candidate_keep> [--reason <text>] [--dry-run] [--json]
./byom-video mask-remove-decision <run_id> <decision_id> [--dry-run] [--json]
./byom-video mask-reorder <run_id> --order <decision_id,...> [--dry-run] [--json]
./byom-video route-preview <run_id> [--json] [--write-artifact]
```

`mask-decision` and `mask-remove-decision` snapshot `inference_mask.json` before every real mutation. All decision-editing commands accept `--dry-run` to preview changes without writing files.

`route-preview` builds exact logical payload previews per expansion task, resolving routes from `byom-video.yaml`. No provider is called. It writes `route_preview.json` when `--write-artifact` is given.

`mask-plan` records `inference_mask.json` in the run manifest. Templates are not added to the manifest.

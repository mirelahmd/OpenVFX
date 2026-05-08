# `selected_clips.json`

Export-facing clip handoff derived from `enhanced_roughcut.json`, `clip_cards.json`, or `roughcut.json`.

Path:

```text
.byom-video/runs/<run_id>/selected_clips.json
```

Produced by:

```sh
./byom-video selected-clips <run_id>
```

Optional goal-aware source selection:

```sh
./byom-video selected-clips <run_id> --prefer-goal-roughcut
```

When `--prefer-goal-roughcut` is used, BYOM Video treats `goal_roughcut.json` as the export-facing source of truth for clip ordering and timing.

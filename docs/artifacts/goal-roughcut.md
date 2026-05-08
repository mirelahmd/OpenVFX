# `goal_roughcut.json`

Goal-aware roughcut planning artifact produced by:

```sh
./byom-video goal-roughcut <run_id>
```

This artifact consumes `goal_rerank.json`, selects clips by rank, respects the parsed goal constraints, and writes a new additive roughcut plan.

It does not modify the original `roughcut.json`.

## Path

```text
.byom-video/runs/<run_id>/goal_roughcut.json
```

## Selection behavior

- starts from ranked highlights in `goal_rerank.json`
- respects:
  - `max_total_duration_seconds`
  - `max_clips`
- reorders selected clips into timeline order for editor readability

## Notes

- use `--overwrite` to replace an existing `goal_roughcut.json`
- `byom-video plan ... --goal-aware` can add `goal_roughcut` after `goal_rerank`
- `byom-video goal-review-bundle <run_id> --overwrite` creates a readable goal-aware review artifact from this plan.
- this is a planning artifact only

# `goal_rerank.json`

Goal-aware highlight reranking artifact produced by:

```sh
./byom-video goal-rerank <run_id> --goal "make a short clip under 60 seconds"
```

The artifact is local and inspectable. It can be generated in:

- `deterministic` mode: no provider call
- `ollama` mode: only when `--use-ollama` is passed

The original `highlights.json` remains unchanged.

## Path

```text
.byom-video/runs/<run_id>/goal_rerank.json
```

## Key fields

- `goal`
- `mode`
- `constraints.max_total_duration_seconds`
- `constraints.max_clips`
- `constraints.preferred_style`
- `ranked_highlights[]`

## Notes

- `--use-ollama` requires `models.enabled: true` and a configured `goal_reranking` route.
- `--fallback-deterministic` falls back to deterministic reranking if Ollama fails.
- `byom-video plan ... --goal-aware` can add `goal_rerank` as an explicit post-pipeline action.
- `byom-video goal-review-bundle <run_id> --overwrite` can summarize rerank results with downstream goal-aware artifacts.
- No cloud providers are used.

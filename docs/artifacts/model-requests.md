# model_requests.dryrun.json

`expand-dry-run <run_id>` writes `model_requests.dryrun.json`.
`expand <run_id>` writes `model_requests.executed.json`.

Schema version:

```json
{
  "schema_version": "model_requests.dryrun.v1",
  "created_at": "2026-05-02T00:00:00Z",
  "run_id": "20260502T000000Z-example",
  "requests": [
    {
      "task_id": "task_0001",
      "task_type": "caption_variants",
      "route_name": "caption_expansion",
      "model_entry_name": "local_qwen",
      "provider": "ollama",
      "model": "qwen2.5:7b",
      "role": "expander",
      "input": {
        "decisions": [],
        "constraints": {},
        "output_contract": {}
      },
      "request_preview": {
        "system": "Follow the inference mask exactly.",
        "user": "Task type: caption_variants.",
        "output_schema": "expansion_output.v1"
      },
      "status": "dry_run",
      "warnings": []
    }
  ],
  "warnings": []
}
```

This is a dry-run contract only. It does not imply that any provider was called.

Executed request log:

```json
{
  "schema_version": "model_requests.executed.v1",
  "created_at": "2026-05-02T00:00:00Z",
  "run_id": "20260502T000000Z-example",
  "requests": [
    {
      "task_id": "task_0001",
      "decision_id": "decision_0001",
      "task_type": "caption_variants",
      "model_route": "caption_expansion",
      "model_entry": "local_qwen",
      "provider": "ollama",
      "model": "qwen2.5:7b",
      "status": "completed",
      "request_preview": {
        "system": "...",
        "user": "...",
        "output_schema": "expansion_output.v1"
      },
      "response_mode": "json",
      "error": ""
    }
  ]
}
```

`review-model-requests --write-artifact` writes `model_requests_review.md`.

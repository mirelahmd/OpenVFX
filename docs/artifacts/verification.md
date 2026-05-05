# `verification.json`

`verification.json` is a future artifact contract for checking expansion drift.

The verifier compares expansion outputs to `inference_mask.json` and records whether constraints were preserved.

Schema sketch:

```json
{
  "schema_version": "verification.v1",
  "source": {
    "inference_mask_artifact": "inference_mask.json",
    "expansion_artifacts": []
  },
  "status": "pending",
  "checks": [
    {
      "id": "check_0001",
      "type": "must_not_include",
      "status": "pending",
      "message": ""
    }
  ]
}
```

Invariant:

> Cheap models can expand style. Cheap models cannot expand truth.

Current behavior:

- no verifier is implemented
- no model provider is called
- `mask-template` can write `verification.template.json`
- `mask-validate` checks the local template/schema shape
- `verification-plan` writes pending deterministic checks to `verification.json`

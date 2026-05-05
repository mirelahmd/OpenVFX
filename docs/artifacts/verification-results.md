# `verification_results.json`

`verification_results.json` is produced by `verify-expansions`. It records the outcome of all deterministic checks run against expansion outputs without calling any model provider.

## Schema

```json
{
  "schema_version": "verification_results.v1",
  "created_at": "...",
  "run_id": "...",
  "mode": "deterministic",
  "source": {
    "inference_mask_artifact": "inference_mask.json",
    "verification_artifact": "verification.json",
    "expansion_artifacts": ["expansions/caption_variants.json"]
  },
  "status": "passed",
  "summary": {
    "checks_total": 4,
    "checks_passed": 4,
    "checks_failed": 0,
    "warnings": 0
  },
  "checks": [
    {
      "id": "check_0001",
      "type": "must_not_include",
      "status": "passed",
      "message": "checked 3 banned phrases; none found",
      "details": {}
    }
  ]
}
```

## Check types

| Type | Description |
|---|---|
| `must_not_include` | Every expansion item text must not contain any phrase from `inference_mask.constraints.must_not_include` (case-insensitive) |
| `timestamp_drift` | Expansion item start/end must be within `--tolerance-seconds` (default 0.25s) of the referenced decision timing |
| `missing_required_decisions` | Every non-rejected decision must have at least one expansion item across all outputs |
| `output_contract_compliance` | Item word counts and per-decision item counts must not exceed `output_contract` limits from `expansion_tasks.json` |

## Status values

| Status | Meaning |
|---|---|
| `passed` | All checks passed |
| `failed` | One or more checks failed |
| `warning` | Only warnings, no failures |

Individual check status values: `passed`, `failed`, `warning`, `skipped`.

## Commands

```sh
./byom-video verify-expansions <run_id>
./byom-video verify-expansions <run_id> --json
./byom-video verify-expansions <run_id> --tolerance-seconds 0.5
./byom-video review-verification <run_id>
./byom-video review-verification <run_id> --write-artifact
./byom-video review-verification <run_id> --json
```

`verify-expansions` always writes `verification_results.json` and records it in the run manifest. No model provider is called.

`review-verification --write-artifact` writes `verification_review.md` and records it in the manifest.

## Relationship to `verification.json`

`verification.json` (from `verification-plan`) is the check template — it defines which check types to run. `verification_results.json` is the execution output — it records the actual outcome of running those checks against the current expansion outputs.

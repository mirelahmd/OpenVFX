# Inference Mask Architecture

The Inference Mask is an artifact-first design for model-assisted media workflows.

BYOM Video can now create deterministic mask plans from existing run artifacts. It still does not call OpenAI, Claude, Groq, NVIDIA, Ollama, Gemma, Kimi, Qwen, or any other provider.

## Division Of Labor

```text
chunks/highlights/roughcut
-> deterministic mask planner now, premium reasoner later
-> inference_mask.json
-> expansion_tasks.json
-> verification.json
-> future expansion/verifier execution
```

The important invariant:

> Cheap models can expand style. Cheap models cannot expand truth.

A deterministic planner currently creates compact editorial decisions and constraints from roughcut/highlight artifacts. A premium reasoner may eventually improve those decisions. Cheaper/free/local expanders may eventually create caption variants, timeline labels, descriptions, or summaries within those constraints. A verifier checks that expansions did not drift.

## Artifact Contracts

Current sketches:

- `inference_mask.json`: compact decisions, constraints, evidence references, and allowed expansion boundaries
- `expansion_tasks.json`: task list for bounded expansion work
- `verification.json`: drift and policy checks against the mask

See:

- [`../artifacts/inference-mask.md`](../artifacts/inference-mask.md)
- [`../artifacts/expansion-tasks.md`](../artifacts/expansion-tasks.md)
- [`../artifacts/verification.md`](../artifacts/verification.md)

## BYOM Model Config

Model routing is represented as disabled configuration only:

```yaml
models:
  enabled: false
  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434
    premium_reasoner:
      provider: openai
      model: gpt-4.1
      role: reasoner
      api_key_env: OPENAI_API_KEY
  routes:
    highlight_reasoning: premium_reasoner
    caption_expansion: local_qwen
```

This config is parsed and displayed, not executed.

## Commands

```sh
./byom-video mask-template <run_id>
./byom-video mask-plan <run_id>
./byom-video inspect-mask <run_id>
./byom-video mask-validate <run_id>
./byom-video review-mask <run_id> --write-artifact
./byom-video expansion-plan <run_id>
./byom-video verification-plan <run_id>
./byom-video routes-plan <run_id>
./byom-video routes-plan <run_id> --write-artifact
./byom-video routes-plan <run_id> --strict
./byom-video revise-mask <run_id> --request "set captions to 12 words"
./byom-video revise-mask <run_id> --request "make captions shorter" --show-diff
./byom-video revise-mask <run_id> --request "avoid hype" --dry-run
./byom-video mask-snapshots <run_id>
./byom-video inspect-mask-snapshot <run_id> <snapshot_id>
./byom-video diff-mask <run_id> <snapshot_id>
./byom-video diff-mask <run_id> <snapshot_id> --write-artifact
./byom-video mask-decisions <run_id>
./byom-video mask-decision <run_id> <decision_id> --set <keep|reject|candidate_keep>
./byom-video mask-decision <run_id> <decision_id> --set candidate_keep --reason "note" --dry-run
./byom-video mask-remove-decision <run_id> <decision_id> --dry-run
./byom-video mask-reorder <run_id> --order <decision_id,...>
./byom-video route-preview <run_id>
./byom-video route-preview <run_id> --write-artifact
./byom-video expand-dry-run <run_id>
./byom-video expand-local-stub <run_id> --overwrite
./byom-video expand <run_id> --dry-run
./byom-video expand <run_id> --overwrite
./byom-video review-model-requests <run_id>
```

`mask-template` writes:

```text
inference_mask.template.json
expansion_tasks.template.json
verification.template.json
```

Templates are not added to `manifest.json` because they are planning contracts, not generated inference results.

Generated `inference_mask.json`, `expansion_tasks.json`, `verification.json`, `mask_review.md`, and `routes_plan.json` are recorded in the run manifest.

## Routes Dry-Run

`routes-plan` resolves the configured model route for each expansion and verification task without calling any provider. It reads `byom-video.yaml`, `expansion_tasks.json`, and `verification.json` and prints which model entry each task would use.

- If models are disabled, status shows `models_disabled` (not an error).
- If a route is unconfigured, status shows `missing_route` with a warning.
- `--strict` fails non-zero if any route or entry is missing.
- `--write-artifact` writes `routes_plan.json` to the run directory.

## Mask Revision

`revise-mask` applies a deterministic revision to `inference_mask.json` without calling any model. Before modifying, it snapshots the current mask under `mask_snapshots/`.

Supported requests:

| Request | Effect |
|---|---|
| `make captions shorter` | Reduce `max_caption_words` by 3 (min 5) |
| `make captions longer` | Increase `max_caption_words` by 3 (max 40) |
| `set captions to N words` | Set `max_caption_words` to N |
| `make tone more technical` | Append `technical` to tone |
| `make tone more casual` | Append `casual` to tone |
| `avoid hype` | Add `hype` and `exaggerated claims` to `must_not_include` |
| `avoid unsupported claims` | Add `unsupported claims` to `must_not_include` |
| `require hook` | Add `strong hook` to `must_include` |

Snapshots are stored as `mask_snapshots/mask_snapshot_NNNN.json`. `diff-mask` compares the current mask to any snapshot.

## Decision-Level Editing

`mask-decisions` lists all decisions in `inference_mask.json`. `mask-decision` updates the value of a single decision by ID; it validates the proposed mask in-memory before writing and snapshots the original before every real mutation. `mask-remove-decision` removes a decision by ID with the same snapshot guarantee. `mask-reorder` reorders all decisions to a specified order.

All decision-editing commands accept `--dry-run` to preview changes without writing files and `--json` for machine-readable output.

Valid decision values: `keep`, `reject`, `candidate_keep`.

## Route Execution Preview

`route-preview` builds a logical payload preview per expansion task without calling any provider. It resolves model routes from `byom-video.yaml`, reads decisions from `inference_mask.json`, and produces a `PayloadPreview` for each task with an instruction template and output contract schema name. When `--write-artifact` is given it writes `route_preview.json` to the run directory and records it in the run manifest.

Route status values are the same as `routes-plan`: `configured`, `models_disabled`, `missing_route`, `missing_entry`.

## Adapter Dry Run

`expand-dry-run` is the first adapter-backed execution contract. It reads `inference_mask.json`, `expansion_tasks.json`, and model routes, resolves each task to a logical entry, and writes `model_requests.dryrun.json`.

No provider is called. The output is only a provider-ready request preview.

## Local Ollama Execution

`expand` is the first real provider-backed expansion path. It requires:

- `models.enabled: true`
- `inference_mask.json`
- `expansion_tasks.json`
- configured routes

It currently supports only local Ollama (`provider: ollama` or `provider: ollama-local`). It writes the same `expansion_output.v1` artifacts under `expansions/`.

`expand --dry-run` stays local and routes through the same command path without calling Ollama.

Real `expand` also writes `model_requests.executed.json` so every provider-backed request has a local execution record. If some requests fail and `--fail-fast` is not set, the command continues, writes partial successful expansion artifacts, writes failed request records, and exits non-zero afterward.

## Stub Expansion Execution

`expand-stub` proves the expansion pipeline end-to-end without calling any model provider. It reads `expansion_tasks.json` and `inference_mask.json`, generates deterministic stub text for each non-rejected decision, and writes output files under `expansions/`. Only decisions with `keep` or `candidate_keep` are included; `reject` decisions are skipped.

Output files follow `expansion_output.v1` with `mode: stub`. The schema is identical to what a real provider execution will produce (`mode: provider`), so `expansion-validate` and `review-expansions` work against either.

`expand-local-stub` follows the new adapter path and still writes deterministic local outputs. It exists to prove future real provider adapters can plug into the same request/execute flow without changing expansion artifact contracts.

`expansion-validate` checks each expansion output file for schema shape, item completeness, timing validity, and cross-references against rejected decisions in the mask.

`review-expansions` prints a human-readable summary of expansion outputs. `--write-artifact` writes `expansions_review.md` and records it in the run manifest.

`inspect-mask` now also reports the presence of expansion output files under `expansions/`.

## Deterministic Verification Execution

`verify-expansions` runs all checks defined in `verification.json` against expansion outputs without calling any model provider. It always writes `verification_results.json` (schema `verification_results.v1`) and records it in the run manifest.

Supported checks:

| Type | Description |
|---|---|
| `must_not_include` | Expansion item text must not contain banned phrases from `constraints.must_not_include` (case-insensitive) |
| `timestamp_drift` | Item start/end must be within `--tolerance-seconds` (default 0.25s) of the decision's timing |
| `missing_required_decisions` | Every non-rejected decision must have at least one expansion item |
| `output_contract_compliance` | Item word counts and per-decision counts must not exceed `output_contract` limits |

Overall status: `passed`, `failed`, or `warning`. Individual check status: `passed`, `failed`, `warning`, `skipped`.

`review-verification` reads `verification_results.json` and prints a human-readable summary. `--write-artifact` writes `verification_review.md`.

`inspect-mask` now also shows `verification_results.json` and `verification_review.md`.

## Editor-Facing Artifacts

Expansion outputs remain intermediate artifacts until they are shaped into editor-facing summaries.

```sh
./byom-video clip-cards <run_id>
./byom-video review-clips <run_id> --write-artifact
./byom-video enhance-roughcut <run_id>
```

- `clip-cards` prefers `roughcut.json` clips and enriches them with expansion labels, captions, descriptions, and verification status.
- `enhance-roughcut` preserves the original `roughcut.json` and writes an additive `enhanced_roughcut.json`.
- These commands do not call any model provider. They only transform artifacts already present on disk.

## Non-Goals

The current milestone does not include:

- provider SDKs or clients
- API key validation
- cloud model routing execution
- semantic highlight reranking
- model-generated inference masks
- verifier execution against model outputs
- Docker, vector DB, web server, or NLE integrations

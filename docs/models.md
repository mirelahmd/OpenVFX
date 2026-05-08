# BYOM Models

BYOM model configuration is a disabled-by-default skeleton for future model-assisted workflows.

It is provider-neutral: logical entry names are user-defined, provider strings are freeform, and routes map task names to logical entries.

It does not call providers, load SDKs, test connectivity, or validate API keys.

## Config

```yaml
models:
  enabled: false

  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434
      options:
        temperature: 0.2

    premium_reasoner:
      provider: openai
      model: gpt-4.1
      role: reasoner
      api_key_env: OPENAI_API_KEY
      options:
        temperature: 0.1
        max_tokens: 1200

  routes:
    highlight_reasoning: premium_reasoner
    goal_reranking: local_qwen
    caption_expansion: local_qwen
    timeline_labeling: local_qwen
    verification: premium_reasoner
```

`provider` may be `openai`, `anthropic`, `groq`, `nvidia`, `ollama`, `gemma-local`, `qwen-local`, `custom-http`, or any other non-empty string. These are labels for future adapters, not dependencies.

`role` is optional. If present, it should be one of:

- `reasoner`
- `expander`
- `verifier`
- `general`

`options` is a freeform map. It is parsed and displayed but not executed.

## Routes

Routes map work categories to logical model entries:

```yaml
routes:
  highlight_reasoning: premium_reasoner
  goal_reranking: local_qwen
  caption_expansion: local_qwen
```

Validation checks that every route points to an existing entry.

## Commands

```sh
./byom-video config show
./byom-video config show --json
./byom-video models
./byom-video models --json
./byom-video models validate
./byom-video models validate --json
./byom-video models doctor
./byom-video models doctor --json
./byom-video routes-plan <run_id>
./byom-video routes-plan <run_id> --json
./byom-video routes-plan <run_id> --write-artifact
./byom-video routes-plan <run_id> --strict
./byom-video route-preview <run_id>
./byom-video route-preview <run_id> --json
./byom-video route-preview <run_id> --write-artifact
./byom-video expand-dry-run <run_id>
./byom-video expand-dry-run <run_id> --json
./byom-video expand-dry-run <run_id> --strict
./byom-video expand-local-stub <run_id>
./byom-video expand-local-stub <run_id> --overwrite
./byom-video expand <run_id> --dry-run
./byom-video expand <run_id> --overwrite
./byom-video review-model-requests <run_id>
./byom-video goal-rerank <run_id> --goal "make a cinematic short" --use-ollama
```

The commands print logical entry names, provider type, model name, role, optional `base_url`, and `api_key_env` names only. They never print environment variable values.

`routes-plan` resolves which configured model route would handle each expansion and verification task for a given run. It reads `byom-video.yaml` plus `expansion_tasks.json` and `verification.json` from the run directory. No provider is called. Status values:

| Status | Meaning |
|---|---|
| `configured` | Route and entry both exist; models enabled |
| `models_disabled` | Route and entry exist but `models.enabled: false` |
| `missing_route` | Route name not found in `models.routes` |
| `missing_entry` | Route resolves to an entry name not in `models.entries` |

`--strict` returns exit code 1 if any route has status `missing_route` or `missing_entry`.

`route-preview` builds a logical payload preview per expansion task. It resolves the same route/entry chain as `routes-plan` and produces an instruction template and output contract schema name for each task. No provider is called. When `--write-artifact` is given it writes `route_preview.json` and records it in the run manifest.

`expand-dry-run` routes through the model adapter interface and writes `model_requests.dryrun.json`. It resolves routes and builds provider-ready request previews, but still does not call any provider.

`expand-local-stub` uses the same adapter interface as a future real provider path, but executes only a local deterministic stub and writes the same `expansion_output.v1` files as `expand-stub`.

`expand` is the first real provider execution path. In this milestone it only supports local Ollama routes, requires `models.enabled: true`, and only runs when the user explicitly invokes `expand`.

`goal-rerank --use-ollama` is the first goal-aware local reasoning path. It only uses a local Ollama route when the user explicitly requests it.

`models doctor` explicitly checks local Ollama connectivity for configured Ollama entries. It does not check cloud providers.

Real `expand` writes `model_requests.executed.json` so provider-side request execution can be reviewed afterward with `review-model-requests`.

## Compatibility

The older Prompt 023 shape is still accepted:

```yaml
models:
  providers: {}
  routing: {}
```

New configs should use `entries` and `routes`. If both old and new shapes exist, `entries` and `routes` win.

## Examples

Example configs are available under:

```text
examples/configs/local-only.yaml
examples/configs/openai-ollama.yaml
examples/configs/groq-local.yaml
examples/configs/nvidia-expander.yaml
examples/configs/custom-http.yaml
```

These examples are illustrative. They do not imply required providers.

## Current Behavior

- `models.enabled: false` keeps deterministic local behavior.
- Missing `models` config is treated as disabled.
- Unknown provider strings are allowed.
- API key values are not read.
- Provider connectivity is not tested.
- No cloud provider calls occur.
- Ollama is only called by `expand` and `models doctor`.

## Future Use

The intended direction is:

- deterministic planner creates initial Inference Mask artifacts
- premium reasoner may later improve compact Inference Mask decisions
- cheap/free/local expander creates bounded style variants
- verifier checks drift against the mask

Cheap models can expand style. Cheap models cannot expand truth.

## Creative Tool Registry

Model routes are not the only registry layer anymore. `tools` is a separate provider-agnostic capability registry for broader creative planning such as script, voice, image, video, caption, audio, and render tasks.

Use `models` for currently implemented model execution paths.

Use `tools` for capability planning:

```sh
./byom-video tools
./byom-video tools validate
./byom-video tools requirements --goal "make a cinematic short with narration and AI b-roll"
./byom-video creative-plan media/Untitled.mov --goal "make a cinematic short with narration and AI b-roll"
```

This planning layer remains local and artifact-only. It does not call providers.

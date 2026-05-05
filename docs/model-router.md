# Model Router

The model router is a provider-neutral adapter layer for future model execution.

Current adapters:

- `dry-run`
- `stub`
- `ollama`

Only the Ollama adapter makes a real provider call in this milestone, and only to local Ollama over HTTP when the user explicitly runs `expand`.

There are still no cloud provider adapters. No SDKs or API key reads are used.

## Commands

```sh
./byom-video expand-dry-run <run_id>
./byom-video expand-dry-run <run_id> --json
./byom-video expand-dry-run <run_id> --strict
./byom-video expand-dry-run <run_id> --task-type caption_variants

./byom-video expand-local-stub <run_id>
./byom-video expand-local-stub <run_id> --overwrite
./byom-video expand-local-stub <run_id> --task-type timeline_labels
./byom-video expand-local-stub <run_id> --json
./byom-video expand <run_id>
./byom-video expand <run_id> --dry-run
./byom-video expand <run_id> --overwrite
./byom-video expand <run_id> --task-type caption_variants
./byom-video models doctor
./byom-video review-model-requests <run_id>
```

`expand-dry-run` resolves routes, builds provider-ready request previews, and writes `model_requests.dryrun.json`.

`expand-local-stub` uses the same adapter interface, but still writes deterministic local stub outputs under `expansions/`.

`expand` is the first real provider-backed command. In this milestone it only supports local Ollama routes and only runs when `models.enabled: true`.

`review-model-requests` summarizes `model_requests.dryrun.json` and `model_requests.executed.json`, including provider/model distribution, statuses, failures, and response modes. `--write-artifact` writes `model_requests_review.md`.

## Adapter Contract

Adapters implement:

```go
type Adapter interface {
    Name() string
    Supports(provider string) bool
    BuildRequest(req Request) (Request, error)
    Execute(req Request) (Response, error)
}
```

In this milestone:

- `BuildRequest` creates deterministic request previews.
- `Execute` is used by `dry-run`, `stub`, and `ollama`.
- Only the Ollama adapter may call a provider, and only local Ollama over HTTP.

## Ollama v1

Supported provider strings:

- `ollama`
- `ollama-local`

Default base URL:

```text
http://localhost:11434
```

The adapter uses `POST /api/generate` with `stream: false`.

Expected JSON response shapes:

```json
{"items":[{"text":"caption"}]}
{"captions":["caption 1","caption 2"]}
{"labels":["label"]}
{"descriptions":["description"]}
```

If parsing fails, BYOM Video falls back to plain text safely and records the response mode.

If Ollama is not running, commands return a clean error:

```text
Ollama request failed. Is Ollama running at http://localhost:11434?
```

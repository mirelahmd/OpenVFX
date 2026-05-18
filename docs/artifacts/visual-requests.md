# Visual Requests Dry-Run Artifact

`visual_requests.dryrun.json` records provider-agnostic visual generation request previews.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/visual_requests.dryrun.json
```

Schema version:

- `openvfx_visual_requests.dryrun.v1`

The artifact is written during `agent-plan` creation and can be refreshed with:

```sh
./byom-video visual-requests <agent_plan_id> --overwrite
./byom-video visual-requests <agent_plan_id> --json --overwrite
```

No provider is called. No API key value is read or printed. Auth fields only show the configured auth type and env var name.

## Standard Visual Routes

OpenVFX resolves visual requirements through configured `tools.routes` keys:

- `creative.video_generate`
- `creative.image_generate`
- `creative.visual_transform`
- `creative.broll_generate`
- `creative.style_transfer`
- `creative.object_remove`
- `creative.background_replace`

Older route names such as `creative.video_broll` and `creative.visual_asset` are treated as compatibility fallbacks for dry-run planning.

## Shape

```json
{
  "schema_version": "openvfx_visual_requests.dryrun.v1",
  "created_at": "2026-05-17T00:00:00Z",
  "plan_id": "agentplan-...",
  "requests": [
    {
      "id": "visual_req_0001",
      "requirement_id": "asset_req_0001",
      "kind": "generated_broll",
      "capability": "creative.broll_generate",
      "route": "creative.broll_generate",
      "backend": "local_video",
      "provider": "custom-http",
      "model": "video-model",
      "endpoint": "http://localhost:9999/generate",
      "auth": {
        "type": "bearer_env",
        "env": "VIDEO_API_KEY"
      },
      "status": "previewed",
      "request_preview": {
        "prompt": "Generate futuristic gym/city b-roll...",
        "no_provider_calls": true
      },
      "output_contract": {
        "artifact": "asset_req_0001_generated_broll_placeholder.json",
        "format": "planned_asset_reference"
      }
    }
  ],
  "missing_capabilities": []
}
```

Request status values:

- `previewed`: a route/backend was resolved and the provider-agnostic payload preview was written
- `missing_backend`: the requirement was understood, but no configured route/backend can satisfy it

This artifact prepares future provider execution without committing OpenVFX to any specific provider.

## Execution

Prompt 070 adds the first execution bridge:

```sh
./byom-video execute-visual-requests <agent_plan_id> \
  --yes \
  --allow-provider-calls \
  --allow-external-network

./byom-video review-visual-generation <agent_plan_id> --write-artifact
```

Only `provider: custom-http-visual` is executable in v1. Other providers remain dry-run metadata until an explicit adapter is added.

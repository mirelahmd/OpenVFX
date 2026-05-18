# Generated Assets Artifact

`generated_assets.json` records visual assets produced by explicitly executed custom HTTP visual requests.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/generated_assets.json
```

Schema version:

- `openvfx_generated_assets.v1`

The artifact is written by:

```sh
./byom-video execute-visual-requests <agent_plan_id> \
  --yes \
  --allow-provider-calls \
  --allow-external-network
```

Only `provider: custom-http-visual` is executable in v1. OpenVFX does not hardcode Sora, Runway, Replicate, Pika, OpenAI, or any provider-specific SDK.

## Safety

- Source media is never mutated.
- Provider execution requires explicit `--yes`, `--allow-provider-calls`, and `--allow-external-network`.
- API key values are read only to send the configured request.
- API key values are never printed or written to audit artifacts.
- Request/response audit files redact sensitive headers.

## Related Files

```text
.byom-video/agent_plans/<agent_plan_id>/outputs/visual_assets/<visual_req_id>.<ext>
.byom-video/agent_plans/<agent_plan_id>/outputs/visual_audits/<visual_req_id>_request.json
.byom-video/agent_plans/<agent_plan_id>/outputs/visual_audits/<visual_req_id>_response.json
.byom-video/agent_plans/<agent_plan_id>/visual_generation_review.md
```

`review-visual-generation <agent_plan_id> --write-artifact` writes the markdown review.

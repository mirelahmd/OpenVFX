# Asset Requirements Artifact

`asset_requirements.json` records media or generation assets implied by a creative brief.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/asset_requirements.json
```

Schema version:

- `openvfx_asset_requirements.v1`

Each requirement includes:

- kind
- description
- capability
- status
- route/backend when configured
- degraded path when missing

Generated b-roll, voiceover, source narration, and visual transform requests are represented here as requirements. This milestone does not execute visual generation or transform media.

# Creative Brief Artifact

`creative_brief.json` stores structured intent extracted from a richer creator prompt.

Path:

```text
.byom-video/agent_plans/<agent_plan_id>/creative_brief.json
```

Schema version:

- `openvfx_creative_brief.v1`

It records:

- raw prompt
- summary
- platform and aspect ratio
- target duration
- style, mood, and visual tone
- pacing notes
- caption style and position
- source media roles such as talking clip narration or b-roll
- requests such as generated b-roll, voiceover, Instagram captions, or visual transform planning

This artifact is planning-only. It does not call providers, edit pixels, or mutate media.

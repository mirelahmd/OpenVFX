# Artifact Schemas

Artifacts are the stable contract between the Go control plane and execution workers.

Workers may change internally. Artifact schemas should remain stable, or they should be explicitly versioned when they need to change.

Current run artifacts live under:

```text
.byom-video/runs/<run_id>/
```

Current documented artifacts:

- [`manifest.json`](manifest.md)
- [`events.jsonl`](events.md)
- [`metadata.json`](metadata.md)
- [`transcript.json`](transcript.md)
- [`captions.srt`](captions.md)
- [`chunks.json`](chunks.md)
- [`highlights.json`](highlights.md)
- [`roughcut.json`](roughcut.md)
- [`ffmpeg_commands.sh`](ffmpeg-script.md)
- [`report.html`](report.md)
- [`exports/`](exports.md)
- [`export_validation.json`](export-validation.md)

Future model-assistance contracts:

- [`inference_mask.json`](inference-mask.md)
- [`expansion_tasks.json`](expansion-tasks.md)
- [`verification.json`](verification.md)
- [`model_requests.dryrun.json`](model-requests.md)
- `model_requests.executed.json`
- [`verification_results.json`](verification-results.md)
- [`clip_cards.json`](clip-cards.md)
- [`enhanced_roughcut.json`](enhanced-roughcut.md)
- [`selected_clips.json`](selected-clips.md)
- [`export_manifest.json`](export-manifest.md)
- [`concat planning artifacts`](concat-plan.md)

These artifacts are deterministic planning contracts. `mask-template <run_id>` can write local `.template.json` files for design and review. `mask-plan <run_id>`, `expansion-plan <run_id>`, and `verification-plan <run_id>` write real planning artifacts without calling any model provider. `mask-validate <run_id>` validates the known template/schema shape locally.

Architecture rule:

> Artifacts are contracts. Workers may change internally, but artifact schemas should remain stable or versioned.

Future fields are allowed when they do not break existing readers. Required schema changes should use explicit versioning.

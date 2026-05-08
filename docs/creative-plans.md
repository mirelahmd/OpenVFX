# Creative Plans

Creative plans are deterministic planning artifacts for broader creative workflows.

They answer:

- what capabilities a goal appears to require
- which configured routes and backends can satisfy those capabilities
- which steps are planned
- what is still missing

Planning does not call providers and does not modify the input file.

## Workflow

```sh
# 1. Plan
./byom-video creative-plan media/Untitled.mov --goal "make a cinematic short with narration and AI b-roll"

# 2. Review
./byom-video creative-plans
./byom-video inspect-creative-plan <creative_plan_id>
./byom-video review-creative-plan <creative_plan_id> --write-artifact

# 3. Approve
./byom-video approve-creative-plan <creative_plan_id>

# 4. Preview (generate provider-agnostic dry-run request stubs)
./byom-video creative-preview <creative_plan_id>

# 5. Execute (dry run — no provider calls)
./byom-video execute-creative-plan <creative_plan_id>

# 6. Stub execution
./byom-video creative-execute-stub <creative_plan_id>
./byom-video review-creative-outputs <creative_plan_id> --write-artifact

# 7. Timeline assembly (optional run clips via --run-id)
./byom-video creative-timeline <creative_plan_id> [--run-id <run_id>]
./byom-video creative-render-plan <creative_plan_id>
./byom-video review-creative-timeline <creative_plan_id> --write-artifact

# 8. Assemble draft video (requires source clips in timeline)
./byom-video creative-assemble <creative_plan_id> --dry-run
./byom-video creative-assemble <creative_plan_id>
./byom-video validate-creative-assemble <creative_plan_id>
./byom-video review-creative-assemble <creative_plan_id> --write-artifact

# 9. Result summary
./byom-video creative-result <creative_plan_id> --write-artifact

# Validate all artifacts
./byom-video validate-creative-plan <creative_plan_id>

# View event log
./byom-video creative-plan-events <creative_plan_id>
```

## Commands

```sh
./byom-video creative-plan <input-file> --goal <text> [--json] [--strict] [--write-artifact]
./byom-video creative-plans
./byom-video inspect-creative-plan <creative_plan_id> [--json]
./byom-video review-creative-plan <creative_plan_id> [--json] [--write-artifact]
./byom-video approve-creative-plan <creative_plan_id>
./byom-video creative-plan-events <creative_plan_id> [--json]
./byom-video creative-preview <creative_plan_id> [--json] [--strict] [--overwrite] [--check-env]
./byom-video execute-creative-plan <creative_plan_id> [--yes] [--dry-run] [--strict] [--check-env] [--json]
./byom-video creative-execute-stub <creative_plan_id> [--yes] [--overwrite] [--json] [--step-type <type>] [--dry-run]
./byom-video review-creative-outputs <creative_plan_id> [--json] [--write-artifact]
./byom-video creative-timeline <creative_plan_id> [--run-id <run_id>] [--overwrite] [--json] [--prefer-goal]
./byom-video creative-render-plan <creative_plan_id> [--overwrite] [--json]
./byom-video review-creative-timeline <creative_plan_id> [--json] [--write-artifact]
./byom-video creative-assemble <creative_plan_id> [--overwrite] [--json] [--mode <reencode|stream-copy>] [--keep-work] [--dry-run] [--max-clips <n>] [--burn-captions] [--captions <path>] [--allow-missing-captions] [--mix-voiceover] [--voiceover <path>] [--allow-missing-voiceover]
./byom-video validate-creative-assemble <creative_plan_id> [--json]
./byom-video review-creative-assemble <creative_plan_id> [--json] [--write-artifact]
./byom-video creative-result <creative_plan_id> [--json] [--write-artifact]
./byom-video validate-creative-plan <creative_plan_id> [--json]
```

## Artifacts

```text
.byom-video/creative_plans/<creative_plan_id>/creative_plan.json
.byom-video/creative_plans/<creative_plan_id>/creative_plan_review.md
.byom-video/creative_plans/<creative_plan_id>/creative_requests.dryrun.json
.byom-video/creative_plans/<creative_plan_id>/creative_result.md
.byom-video/creative_plans/<creative_plan_id>/creative_outputs_review.md
.byom-video/creative_plans/<creative_plan_id>/events.jsonl
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_outputs.json
.byom-video/creative_plans/<creative_plan_id>/outputs/script_draft.json
.byom-video/creative_plans/<creative_plan_id>/outputs/script_draft.txt
.byom-video/creative_plans/<creative_plan_id>/outputs/voiceover_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/visual_asset_prompts.json
.byom-video/creative_plans/<creative_plan_id>/outputs/caption_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/audio_asset_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/visual_transform_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/translation_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/composition_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_timeline.json
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_render_plan.json
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_timeline_review.md
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_assemble_result.json
.byom-video/creative_plans/<creative_plan_id>/outputs/draft.mp4
.byom-video/creative_plans/<creative_plan_id>/outputs/render_work/clip_NNNN.mp4
.byom-video/creative_plans/<creative_plan_id>/outputs/render_work/concat_list.txt
.byom-video/creative_plans/<creative_plan_id>/outputs/creative_assemble_review.md
```

Not all artifact files are written on every run — only those matching the plan's step types.

## Stub Execution

`creative-execute-stub` produces structured local placeholder artifacts per step. No providers are called. No shell commands are executed.

- Requires approval (or `--yes`)
- `--overwrite` required if outputs already exist
- `--step-type <type>` filters to one step type only
- `--dry-run` prints planned writes without creating files
- Writes `execution_status: stub_completed` and per-step `status: stub_completed` onto `creative_plan.json`
- Always writes `outputs/creative_outputs.json` (index of all artifacts)

## Output Artifact Schemas

| Step type | File | Schema version |
|---|---|---|
| `generate_script` | `script_draft.json` + `script_draft.txt` | `creative_script.v1` |
| `generate_voiceover` | `voiceover_plan.json` | `voiceover_plan.v1` |
| `generate_visual_asset` | `visual_asset_prompts.json` | `visual_asset_prompts.v1` |
| `generate_captions_or_caption_variants` | `caption_plan.json` | `caption_plan.v1` |
| `generate_audio_asset` | `audio_asset_plan.json` | `audio_asset_plan.v1` |
| `visual_transform` | `visual_transform_plan.json` | `visual_transform_plan.v1` |
| `translate_text` | `translation_plan.json` | `translation_plan.v1` |
| `render_draft` | `composition_plan.json` | `composition_plan.v1` |

## Timeline and Render Plan

`creative-timeline` assembles tracks from stub outputs and optional run clips into a `creative_timeline.v1` artifact.

- `--run-id <id>` — load clips from a pipeline run (selected_clips.json → roughcut.json fallback)
- `--prefer-goal` — prefer goal_roughcut.json → enhanced_roughcut.json → roughcut.json → selected_clips.json
- `--overwrite` — required to replace existing timeline
- Writes `outputs/creative_timeline.json` (schema: `creative_timeline.v1`)
- Updates `outputs/creative_outputs.json` index
- Tracks: `track_video_main` (video), `track_voiceover` (audio), `track_captions` (text), `track_visual_overlays` (visual)
- No rendering. No provider calls.

`creative-render-plan` converts a timeline into render steps.

- Reads `outputs/creative_timeline.json` — must run `creative-timeline` first
- Writes `outputs/creative_render_plan.json` (schema: `creative_render_plan.v1`)
- Each timeline item → one render step (`cut_source_clip`, `attach_voiceover_placeholder`, `add_caption_placeholder`, `add_visual_overlay_placeholder`)
- `planned_output.planned_file` is always `outputs/draft.mp4` in stub mode

`review-creative-timeline` prints a summary of all tracks and render steps.

- `--write-artifact` writes `outputs/creative_timeline_review.md` and adds it to the outputs index

## Creative Assemble

`creative-assemble` renders a draft video from the source clips referenced in `creative_timeline.json`.

**Requirements:**
- `outputs/creative_timeline.json` must exist (run `creative-timeline` first)
- `outputs/creative_render_plan.json` must exist (run `creative-render-plan` first)
- `track_video_main` must contain `source_clip` items with `source_start < source_end`
- `ffmpeg` must be on PATH (unless `--dry-run`)

**What it does:**
1. Reads source clip items from `track_video_main`
2. Cuts each clip from the source media using FFmpeg
3. Writes clips to `outputs/render_work/clip_NNNN.mp4`
4. Writes `outputs/render_work/concat_list.txt`
5. Assembles clips into `outputs/draft.mp4` (or an intermediate) via FFmpeg concat demuxer
6. Optionally mixes a voiceover file via FFmpeg `amix` filter (`--mix-voiceover`)
7. Optionally burns SRT captions via FFmpeg `subtitles` filter (`--burn-captions`)
8. Writes `outputs/creative_assemble_result.json` (schema: `creative_assemble_result.v1`)
9. Updates `creative_outputs.json` index
10. Patches `creative_plan.json` with `execution_status: assembled`

**Safety:**
- Only the `input_path` from the timeline is used as the source — no arbitrary paths
- Caption and voiceover paths must exist on disk before any FFmpeg work begins
- FFmpeg is called via `exec.Command` with argument slices, never shell strings
- Filter graph paths are escaped (`\` → `\\`, `:` → `\:`, `'` → `\'`) without shell involvement
- Original media is never modified
- Dry-run (`--dry-run`) prints all planned commands including post-processing stages and writes nothing

**Modes:**
- `--mode reencode` (default) — frame-accurate: `ffmpeg ... -c:v libx264 -c:a aac`
- `--mode stream-copy` — faster: `ffmpeg ... -c copy`

**Post-processing flags:**
- `--burn-captions` — burn SRT captions; pass `--captions <path>` or use `--allow-missing-captions` to skip if not found
- `--mix-voiceover` — mix audio via amix; pass `--voiceover <path>` or use `--allow-missing-voiceover` to skip if not found
- `--run-id <id>` — used for caption auto-discovery from a pipeline run

**Staged output** (when post-processing is active):
`draft_assembled.mp4` → `draft_audio.mp4` (voiceover) → `draft.mp4` (captions). The final `draft.mp4` is always the command output.

**Work files** are kept in `outputs/render_work/` by default (safer for alpha; allows re-assembly).

## Approval Gate

`approve-creative-plan` patches `approval_status=approved` and `approval_mode=manual` onto the plan.
`execute-creative-plan` requires approval unless `--yes` is passed.
`--yes` bypasses the gate and marks `approval_mode=yes_flag`.

## Dry-Run Preview

`creative-preview` generates `creative_requests.dryrun.json` — a provider-agnostic description of what each step would request. It does not call any provider.

Fields per request item: `step_id`, `step_type`, `capability`, `route`, `backend`, `provider`, `model`, `endpoint`, `auth` (env var name only, never value), `status`, `request_preview`.

## Missing Capabilities

Planning still succeeds when capabilities are missing. Missing items become warnings in the plan.

Use `--strict` if you want planning to fail when the goal cannot be fully satisfied by the current `tools` config.

`creative-preview --strict` also fails when any step has no configured backend.

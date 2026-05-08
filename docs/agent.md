# Agent Task Planner

Agent Task Planner v1 is deterministic and local. It turns a simple goal into an explicit BYOM Video plan artifact.

It does not call an LLM.

```sh
./byom-video plan media/Untitled.mov --goal "make 5 shorts"
```

The command writes:

```text
.byom-video/plans/<plan_id>/agent_plan.json
.byom-video/plans/<plan_id>/actions.jsonl
```

By default, planning does not run the media pipeline.

Each action includes a `command_preview`. Preset-style batch and watch actions show the matching local command. File pipeline actions with explicit stage options show exact `byom-video run` flags.

Examples:

```sh
./byom-video run "media/input.mov"
./byom-video run "media/input.mov" --with-transcript --transcript-model-size tiny
./byom-video run "media/input.mov" --with-transcript --with-captions --transcript-model-size tiny
./byom-video run "media/input.mov" --with-transcript --with-chunks --with-highlights --transcript-model-size tiny
./byom-video run "media/input.mov" --with-transcript --with-captions --with-chunks --with-highlights --with-roughcut --with-ffmpeg-script --with-report --transcript-model-size tiny --roughcut-max-clips 5
```

## Execute A Plan

```sh
./byom-video plan media/Untitled.mov --goal "make 5 shorts" --execute
```

Execution updates action statuses in `agent_plan.json` and appends action events to `actions.jsonl`.

Before execution, BYOM Video validates the plan. Invalid plans are marked failed and actions do not run.

Export is still explicit:

```sh
./byom-video plan media/Untitled.mov --goal "make 5 shorts" --execute --with-export
```

Validation can be included:

```sh
./byom-video plan media/Untitled.mov --goal "make 5 shorts" --execute --with-validate
```

## Goal Mapping

The parser is intentionally simple:

- `metadata only` -> metadata preset
- `transcribe this` -> transcript only
- `make captions` -> transcript plus captions
- `find highlights` -> transcript, chunks, highlights
- `roughcut`, `clips`, or `shorts` -> transcript, captions, chunks, highlights, roughcut, FFmpeg script, report

If a number appears before `shorts` or `clips`, it becomes `roughcut_max_clips`.

Examples:

```sh
./byom-video plan media/Untitled.mov --goal "make 3 shorts"
./byom-video plan media/Untitled.mov --goal "create 8 clips"
./byom-video plan media/Untitled.mov --goal "metadata only"
```

Unknown goals return a clean error with examples. No LLM fallback is used.

## Batch And Watch Plans

Directory targets can create batch or watch plans:

```sh
./byom-video plan media/batch-smoke --goal "batch process shorts" --mode batch
./byom-video plan media/batch-smoke --goal "watch this folder for shorts" --mode watch --once
```

If `--mode` is omitted, BYOM Video infers from the path and goal text:

- file input plus shorts goal -> file plan
- directory input plus batch/process goal -> batch plan
- directory input plus watch/monitor goal -> watch plan

Watch plan execution requires `--once` in this version so planner execution cannot accidentally run forever.

## Plan Inspection

```sh
./byom-video plans
./byom-video inspect-plan <plan_id>
./byom-video inspect-plan <plan_id> --json
./byom-video plan-artifacts <plan_id>
./byom-video plan-artifacts <plan_id> --json
```

`inspect-plan` shows the action log path, snapshot count, review artifact path when present, and diff artifacts when present. `plan-artifacts` prints local paths for `agent_plan.json`, `actions.jsonl`, `review.md`, snapshots, and diffs.

## Review, Approve, Execute

Saved plans can be reviewed and approved before execution:

```sh
./byom-video review-plan <plan_id>
./byom-video review-plan <plan_id> --write-artifact
./byom-video approve-plan <plan_id>
./byom-video execute-plan <plan_id>
```

`review-plan --write-artifact` writes `.byom-video/plans/<plan_id>/review.md`. The review artifact is generated and overwritten on each write.

`execute-plan` requires `approval_status: approved` unless `--yes` is passed:

```sh
./byom-video execute-plan <plan_id> --yes
```

Dry-run saved-plan execution:

```sh
./byom-video execute-plan <plan_id> --dry-run
```

Compare two plans:

```sh
./byom-video diff-plan <plan_id_a> <plan_id_b>
./byom-video diff-plan <plan_id_a> <plan_id_b> --json
./byom-video diff-plan <plan_id_a> <plan_id_b> --write-artifact
```

`diff-plan --write-artifact` writes a markdown diff under `.byom-video/plans/<plan_id_a>/diffs/`.

`execute-plan --with-export` and `execute-plan --with-validate` do not mutate saved plans. Create a new plan with those flags instead.

## Execution Results

After `execute-plan`, BYOM Video now prints a concise execution result summary with:

- resulting run id
- run directory
- report path when present
- goal-aware artifacts when created
- suggested next commands

You can also inspect the result later:

```sh
./byom-video agent-result <plan_id>
./byom-video agent-result <plan_id> --write-artifact
```

This writes:

```text
.byom-video/plans/<plan_id>/agent_result.md
```

## Revise Plans

Revise a plan deterministically:

```sh
./byom-video revise-plan <plan_id> --request "make 3 shorts" --show-diff
./byom-video snapshots <plan_id>
./byom-video inspect-snapshot <plan_id> snapshot_0001
./byom-video diff-snapshot <plan_id> snapshot_0001
./byom-video diff-snapshot <plan_id> snapshot_0001 --write-artifact
```

Revisions create snapshots before modifying the plan and reset approval when executable actions or options change.

## Safety Model

- Planning writes local artifacts only.
- Plans are validated before execution.
- Actions show command previews.
- Input files are not modified.
- Export requires `--with-export`.
- Watch execution requires `--once`.
- There are no provider clients, no model routing, no web server, and no vector database.

This is the first step toward agentic editing: goals become auditable local actions before any automatic execution.

## Goal-Aware Cut Selection

Agent plans can now include explicit goal-aware post-processing actions.

Plan-only preview:

```sh
./byom-video plan media/Untitled.mov --goal "make a short clip under 60 seconds" --goal-aware --dry-run
```

Optional local Ollama goal rerank inside a plan:

```sh
./byom-video plan media/Untitled.mov --goal "make a cinematic short" --goal-aware --goal-use-ollama --goal-fallback-deterministic --dry-run
```

When `--goal-aware` is present on a file plan, BYOM Video adds:

- `goal_rerank`
- `goal_roughcut`

Use:

```sh
./byom-video goal-rerank <run_id> --goal "make a short clip under 60 seconds"
./byom-video goal-roughcut <run_id>
```

Optional local Ollama reranking:

```sh
./byom-video goal-rerank <run_id> --goal "make a cinematic short" --use-ollama --fallback-deterministic
```

This writes additive artifacts:

- `goal_rerank.json`
- `goal_roughcut.json`

The original `highlights.json` and `roughcut.json` remain unchanged.

These goal-aware actions run only when the plan explicitly contains them. There are no hidden provider calls.

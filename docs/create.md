# Create Command

`byom-video create` is the high-level creator-facing path.

```sh
./byom-video create media/Untitled.mov --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions" --write-review
```

Default behavior is preview/planning only. It may write planning artifacts, but it does not approve plans, convert jobs, run jobs, call providers, or start the daemon.

## Scoped Approval

Approvals are scoped to one create session.

```sh
./byom-video create media/Untitled.mov --goal "<brief>" --yes --approval-scope local --convert --approve-jobs
```

Scopes:

- `preview`: planning and review artifacts only
- `local`: approve plan, convert jobs, approve local jobs, and run local jobs only when execution flags are explicit
- `provider`: local scope plus provider-backed jobs only when `--allow-provider-calls` and `--allow-external-network` are passed
- `full`: same as provider in v1; arbitrary shell and source mutation remain forbidden

Overwrite requires `--allow-overwrite` in every scope.

## Execution Flags

```sh
./byom-video create media/Untitled.mov --goal "<brief>" --yes --approval-scope local --convert --approve-jobs --run-jobs
./byom-video create media/Untitled.mov --goal "<brief>" --yes --approval-scope local --convert --approve-jobs --worker-once
./byom-video create media/Untitled.mov --goal "<brief>" --yes --approval-scope local --convert --approve-jobs --start-daemon
```

`--run-jobs` and `--worker-once` cannot be used together. `--start-daemon` starts the daemon and does not wait synchronously.

## Result

```sh
./byom-video create-result <create_session_id>
./byom-video create-result <create_session_id> --write-artifact
./byom-video create-sessions
./byom-video inspect-create-session <create_session_id>
```

`create-result` summarizes the session, approval scope, linked agent plan, linked jobs, capability gaps, and next commands.

`create-result --write-artifact` refreshes `create_review.md` as a creator-facing page with:

- creative brief summary
- planned deliverables
- asset requirements
- visual generation dry-run request previews
- capability gaps and suggested fixes
- agent plan policy state
- live linked job statuses
- discovered outputs
- next commands

`create-sessions` lists recent create sessions newest first. `inspect-create-session --json` emits the same live aggregate used by `create-result`.

## Visual Generation Dry-Runs

Rich prompts such as "generate futuristic b-roll", "create reference images", "make the lighting darker", "remove an object", or "replace the background" are recorded as visual asset requirements and dry-run request previews.

```sh
./byom-video visual-requests <agent_plan_id> --overwrite
```

The preview resolves configured `tools.routes` keys such as `creative.broll_generate`, `creative.image_generate`, and `creative.visual_transform`. It writes `visual_requests.dryrun.json` and shows exactly what OpenVFX would send later, without calling a provider or reading API key values.

To execute user-configured custom HTTP visual backends:

```sh
./byom-video execute-visual-requests <agent_plan_id> \
  --yes \
  --allow-provider-calls \
  --allow-external-network
./byom-video review-visual-generation <agent_plan_id> --write-artifact
```

Execution is limited to `provider: custom-http-visual` in v1. Generated outputs are saved under the agent plan directory and surfaced by `create-result`; source media is not mutated.

# Plan Revisions

Plan revisions preserve snapshots before modifying a saved agent plan.

```sh
./byom-video revise-plan <plan_id> --request "make 3 shorts"
```

Snapshots are written under:

```text
.byom-video/plans/<plan_id>/snapshots/
```

List snapshots:

```sh
./byom-video snapshots <plan_id>
```

Inspect a snapshot:

```sh
./byom-video inspect-snapshot <plan_id> snapshot_0001
./byom-video inspect-snapshot <plan_id> snapshot_0001 --json
```

Compare current plan to a snapshot:

```sh
./byom-video diff-snapshot <plan_id> snapshot_0001
./byom-video diff-snapshot <plan_id> snapshot_0001 --write-artifact
```

Diff artifacts are written under:

```text
.byom-video/plans/<plan_id>/diffs/
```

## Revision Requests

Supported deterministic requests:

- `make it shorter`
- `make it longer`
- `make 3 clips`
- `make 3 shorts`
- `add validation`
- `remove validation`
- `add export`
- `remove export`
- `captions only`
- `metadata only`
- `find highlights`

Unknown requests fail cleanly and do not mutate the plan.

## Approval Reset

If a revision changes executable actions or options, approval is reset to pending. Review and approve the revised plan before execution:

```sh
./byom-video review-plan <plan_id>
./byom-video approve-plan <plan_id>
./byom-video execute-plan <plan_id>
```

Adding export is allowed as an explicit revision request, but it resets approval because it changes execution behavior.

## Preview Updates

Revision updates command previews after changing executable options. For example, revising a shorts plan to `captions only` changes the first action preview to an exact command like:

```sh
./byom-video run "media/input.mov" --with-transcript --with-captions --transcript-model-size tiny
```

Review artifacts can be regenerated after revision:

```sh
./byom-video review-plan <plan_id> --write-artifact
./byom-video plan-artifacts <plan_id>
```

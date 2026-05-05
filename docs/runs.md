# Run Discovery

BYOM Video stores runs under:

```text
.byom-video/runs/<run_id>/
```

The run discovery commands make those folders navigable without manually digging through the filesystem.

## List Runs

```sh
./byom-video runs
```

Default output shows the newest 20 runs first.

```sh
./byom-video runs --limit 5
./byom-video runs --all
```

Columns include run id, status, created time, input file basename, artifact count, and export status.

If a run has no readable manifest, it is still listed with status `unknown`.

## Inspect A Run

```sh
./byom-video inspect <run_id>
```

This prints the run id, status, input path, created time, error message when present, full artifact paths, report path, export status, exported files, and artifact summary counts.

Machine-readable output:

```sh
./byom-video inspect <run_id> --json
```

## Artifact Paths

Print all artifact paths:

```sh
./byom-video artifacts <run_id>
```

Filter by type:

```sh
./byom-video artifacts <run_id> --type transcript
./byom-video artifacts <run_id> --type report
./byom-video artifacts <run_id> --type exports
```

Supported types:

- `manifest`
- `events`
- `metadata`
- `transcript`
- `captions`
- `chunks`
- `highlights`
- `roughcut`
- `ffmpeg-script`
- `report`
- `exports`
- `export_validation`
- `export-validation`

## Validate A Run

```sh
./byom-video validate <run_id>
```

Validation checks:

- `manifest.json` has required fields and accepted status values.
- manifest artifact paths are relative and do not escape the run directory.
- files listed in the manifest exist.
- `events.jsonl` exists, is valid JSON Lines, and contains event names and timestamps.
- known artifact schemas are checked when present: `transcript.json`, `chunks.json`, `highlights.json`, and `roughcut.json`.
- listed plain artifacts such as `captions.srt`, `ffmpeg_commands.sh`, and `report.html` must exist.

Machine-readable output:

```sh
./byom-video validate <run_id> --json
```

The command exits non-zero when errors are found. Warnings alone keep a zero exit status.

## Reports

Print a report path:

```sh
./byom-video open-report <run_id>
```

Attempt to open it with the OS:

```sh
./byom-video open-report <run_id> --open
```

The command only targets the known `report.html` artifact inside the resolved run directory.

## Rerun And Cleanup

Create a new run from an existing run's original input:

```sh
./byom-video rerun <run_id>
./byom-video rerun <run_id> --dry-run
```

List cleanup candidates without deleting anything:

```sh
./byom-video cleanup
```

## Scripting Examples

Use the latest run id:

```sh
latest_run="$(./byom-video runs --limit 1 | awk 'NR==2 {print $1}')"
./byom-video inspect "$latest_run"
./byom-video artifacts "$latest_run" --type report
```

Export the latest run after reviewing the script:

```sh
latest_run="$(./byom-video runs --limit 1 | awk 'NR==2 {print $1}')"
./byom-video artifacts "$latest_run" --type ffmpeg-script
./byom-video export "$latest_run"
./byom-video validate "$latest_run"
```

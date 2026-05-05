# Retry And Rerun

Recovery commands create new runs and leave original artifacts unchanged.

## Retry Failed Batch Items

```sh
./byom-video retry-batch <batch_id>
./byom-video retry-batch <batch_id> --dry-run
```

`retry-batch` loads:

```text
.byom-video/batches/<batch_id>/batch_summary.json
```

It retries only items with `status: failed` using the original batch preset. The retry writes a new batch summary under a new batch id.

Flags:

```sh
./byom-video retry-batch <batch_id> --limit 5
./byom-video retry-batch <batch_id> --fail-fast
./byom-video retry-batch <batch_id> --validate
./byom-video retry-batch <batch_id> --export
./byom-video retry-batch <batch_id> --export-and-validate
```

If an input file no longer exists, the retry summary records a failed item with a clean error.

## Retry Failed Watch Items

```sh
./byom-video retry-watch --preset metadata
./byom-video retry-watch --preset metadata --dry-run
```

`retry-watch` loads:

```text
.byom-video/watch/processed.json
```

It retries only registry items with `status: failed` and updates the watch registry afterward.

Flags:

```sh
./byom-video retry-watch --preset shorts
./byom-video retry-watch --limit 5
./byom-video retry-watch --fail-fast
./byom-video retry-watch --validate
./byom-video retry-watch --export
./byom-video retry-watch --export-and-validate
```

## Rerun A Run

```sh
./byom-video rerun <run_id>
./byom-video rerun <run_id> --dry-run
```

`rerun` reads the original run manifest, uses its `input_path`, and creates a new run. It does not modify the old run.

When no preset is provided, BYOM Video infers a broad preset:

- `shorts` if the old run has `roughcut.json`, `ffmpeg_commands.sh`, or `report.html`
- `metadata` otherwise

Override the preset:

```sh
./byom-video rerun <run_id> --preset shorts
./byom-video rerun <run_id> --preset metadata
```

Optional post-run work:

```sh
./byom-video rerun <run_id> --validate
./byom-video rerun <run_id> --export-and-validate
```

# Watch Folder Mode

Watch mode polls a folder for new media files and processes stable, unprocessed files with a preset.

```sh
./byom-video watch media/inbox --preset metadata
./byom-video watch media/inbox --preset shorts
```

It creates one normal run per processed file under:

```text
.byom-video/runs/<run_id>/
```

Watch mode runs until interrupted with Ctrl+C. It handles interruption gracefully.

## Polling

Watch mode uses portable polling instead of platform-specific filesystem events.

Default interval:

```text
5 seconds
```

Override it:

```sh
./byom-video watch media/inbox --preset metadata --interval-seconds 10
```

The interval must be positive.

## Stable Files

Watch mode avoids processing files while they are still being copied.

A file is stable when:

- its size is unchanged across checks, and
- its modified time is unchanged across checks.

If a file is first seen and is already older than the polling interval, it can be processed immediately. Otherwise watch mode prints that the file is seen but not stable yet and waits for a later poll.

## Registry

Watch mode records processed files in:

```text
.byom-video/watch/processed.json
```

The fingerprint uses absolute path, file size, and modified time. No content hashing is used yet.

By default, matching registry entries prevent reprocessing. Retry anyway:

```sh
./byom-video watch media/inbox --preset metadata --once --ignore-registry
```

The retry still updates the registry afterward.

Retry failed registry items:

```sh
./byom-video retry-watch --preset metadata --dry-run
./byom-video retry-watch --preset metadata
```

## Once Mode

Use `--once` to scan once, process stable unprocessed files, then exit.

```sh
./byom-video watch media/inbox --preset metadata --once
```

## Flags

```sh
./byom-video watch media/inbox --recursive
./byom-video watch media/inbox --limit 5
./byom-video watch media/inbox --fail-fast
./byom-video watch media/inbox --validate
./byom-video watch media/inbox --export
./byom-video watch media/inbox --export-and-validate
./byom-video watch media/inbox --ignore-registry
```

`--export` and `--export-and-validate` require a preset that generates `ffmpeg_commands.sh`. The current `metadata` preset does not.

## Status

```sh
./byom-video watch-status
./byom-video watch-status --json
```

Status prints total processed files, completed count, failed count, and latest registry items.

## Safety

- Hidden files are skipped.
- Directories are skipped.
- Input files are never deleted or moved.
- Runs are normal run folders and are never overwritten.
- Failures are recorded in the registry.
- There is no database or web server; registry state is a local JSON file.

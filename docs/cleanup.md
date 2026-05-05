# Cleanup

Cleanup finds failed or incomplete run folders. It is dry-run by default.

```sh
./byom-video cleanup
```

Candidates:

- failed runs
- running runs older than 24 hours
- run directories with missing manifests

Filter candidates:

```sh
./byom-video cleanup --failed
./byom-video cleanup --stale-running
./byom-video cleanup --missing-manifest
./byom-video cleanup --older-than-hours 48
./byom-video cleanup --limit 10
./byom-video cleanup --json
```

## Deletion

Cleanup does not delete anything unless `--delete` is passed.

```sh
./byom-video cleanup --failed --delete
```

Interactive deletion requires typing `yes`.

For non-interactive deletion:

```sh
./byom-video cleanup --failed --delete --yes
```

## Safety Model

- Cleanup only resolves paths under `.byom-video/runs`.
- It prints run directories before deletion.
- It never deletes input media.
- It removes only selected run directories.
- Original run ids are never reused.

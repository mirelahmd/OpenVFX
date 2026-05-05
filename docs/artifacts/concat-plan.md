# Concat Planning Artifacts

Concat planning produces two local files:

```text
.byom-video/runs/<run_id>/concat_list.txt
.byom-video/runs/<run_id>/ffmpeg_concat.sh
```

Produced by:

```sh
./byom-video concat-plan <run_id>
```

These are planning artifacts only. BYOM Video does not execute them automatically.

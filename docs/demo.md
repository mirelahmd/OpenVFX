# Demo

Use a local media file such as `media/Untitled.mov`. A coherent alpha demo looks like this:

```sh
./byom-video init
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts
./byom-video runs
./byom-video inspect <run_id>
./byom-video open-report <run_id>
./byom-video export <run_id>
./byom-video validate <run_id>
```

Optional follow-up flow:

```sh
./byom-video mask-plan <run_id>
./byom-video expand-local-stub <run_id> --overwrite
./byom-video verify-expansions <run_id>
./byom-video clip-cards <run_id>
./byom-video selected-clips <run_id>
./byom-video export-manifest <run_id>
./byom-video concat-plan <run_id>
```

# Shorts Local Workflow

```sh
./byom-video init
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts
./byom-video open-report <run_id>
./byom-video export <run_id>
./byom-video validate <run_id>
```

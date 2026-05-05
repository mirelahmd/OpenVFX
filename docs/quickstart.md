# Quickstart

Build:

```sh
go build -o byom-video ./cmd/byom-video
```

Initialize:

```sh
./byom-video init
```

Create a tiny local fixture when FFmpeg is available:

```sh
scripts/make-fixture.sh
```

Metadata-only sanity check:

```sh
./byom-video pipeline examples/fixtures/tiny.mp4 --preset metadata
```

Shorts preset:

```sh
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video pipeline media/Untitled.mov --preset shorts
```

Open the report:

```sh
./byom-video open-report <run_id>
```

Export and validate:

```sh
./byom-video export <run_id>
./byom-video validate <run_id>
```

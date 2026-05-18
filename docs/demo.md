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

## One-command creator flow

```sh
# Plan only (review before executing)
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video make media/Untitled.mov \
  --goal "make a short cinematic clip with captions"

# Execute end-to-end
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video make media/Untitled.mov \
  --goal "make a short cinematic clip with captions" \
  --yes --burn-captions --allow-missing-captions

# Reuse an existing pipeline run (skip re-transcribing)
./byom-video make --goal "make a cinematic short" \
  --skip-pipeline <run_id> --yes --burn-captions --allow-missing-captions

# Also export clips after pipeline
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video make media/Untitled.mov \
  --goal "make a short clip" --yes --export

# Review
./byom-video makes
./byom-video make-result <make_id>
./byom-video make-result <make_id> --write-artifact
./byom-video inspect-make <make_id>
```

## Agentic create preview

```sh
./byom-video create media/Untitled.mov \
  --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions" \
  --write-review

./byom-video create media/Untitled.mov \
  --goal "Make a 35-second luxury fitness Instagram Reel with bold lower-third captions" \
  --yes --approval-scope local --convert --approve-jobs --write-review

./byom-video create-sessions
./byom-video create-result <create_session_id> --write-artifact
```

The first command is preview/planning only. The second command applies a local, session-scoped approval and converts the approved plan into approved local jobs without provider calls.

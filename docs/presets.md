# Pipeline Presets

Pipeline presets map common workflows to existing `run` options.

## Shorts

```sh
./byom-video pipeline media/Untitled.mov --preset shorts
```

The `shorts` preset enables:

- real transcription
- captions
- chunks
- highlights
- roughcut
- FFmpeg script generation
- local HTML report

It is equivalent to:

```sh
./byom-video run media/Untitled.mov \
  --with-transcript \
  --with-captions \
  --with-chunks \
  --with-highlights \
  --with-roughcut \
  --with-ffmpeg-script \
  --with-report
```

The preset does not execute exports. Use this explicitly:

```sh
./byom-video export <run_id>
```

List recent preset runs:

```sh
./byom-video runs
./byom-video inspect <run_id>
```

## Metadata

```sh
./byom-video pipeline media/Untitled.mov --preset metadata
```

The `metadata` preset only extracts media metadata. It is useful for sanity checks.

## Batch Use

Presets can also be used with batch processing:

```sh
./byom-video batch media/inbox --preset metadata
./byom-video batch media/inbox --preset shorts
```

`metadata` is useful for fast folder indexing. `shorts` produces the full local planning set for each file but still does not export clips unless `--export` or `--export-and-validate` is passed.

## Overrides

CLI flags override config values:

```sh
./byom-video pipeline media/Untitled.mov --preset shorts --transcript-model-size base
```

Preset support is intentionally small. There is no plugin system or model routing layer.

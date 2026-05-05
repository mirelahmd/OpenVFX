# `report.html`

## Purpose

`report.html` is a local, readable summary of a completed run.

It is generated only when `byom-video run` is called with `--with-report`.

## Behavior

- The report is a single local HTML file.
- It uses plain HTML and CSS.
- It does not require a web server.
- It does not load external assets.
- User and media text is escaped before being written into HTML.

## Current Sections

The report includes available information from the run folder:

- run id
- input path
- created time and status
- media metadata summary from `metadata.json`
- transcript summary from `transcript.json`
- captions presence from `captions.srt`
- chunk count from `chunks.json`
- top highlights from `highlights.json`
- rough-cut clip table from `roughcut.json`
- FFmpeg script presence from `ffmpeg_commands.sh`
- exported files from `exports/` when present, including export validation status and clip durations when `export_validation.json` is available
- artifact list from `manifest.json`

Missing optional artifacts are skipped.

If `report.html` already exists, `byom-video export <run_id>` refreshes the report after export validation so the exports section can list rendered files and validation details.

## Lifecycle

Report generation happens at the end of a successful `run`.

If an earlier pipeline stage fails, the report is not generated. If `--with-report` is requested and report generation fails, the run is marked failed.

## Example

```sh
BYOM_VIDEO_PYTHON=.venv/bin/python ./byom-video run media/Untitled.mov \
  --with-transcript \
  --with-captions \
  --with-chunks \
  --with-highlights \
  --with-roughcut \
  --with-ffmpeg-script \
  --with-report
```

Open the generated file from the run directory:

```text
.byom-video/runs/<run_id>/report.html
```

Print the report path:

```sh
./byom-video open-report <run_id>
```

Attempt to open the report through the OS:

```sh
./byom-video open-report <run_id> --open
```

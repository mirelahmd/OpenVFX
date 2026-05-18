# Creative Assemble Artifact

Written by `creative-assemble`. Requires `creative_timeline.json` and `creative_render_plan.json`.

## File: `outputs/creative_assemble_result.json`

Schema version: `creative_assemble_result.v1`

```json
{
  "schema_version": "creative_assemble_result.v1",
  "created_at": "...",
  "creative_plan_id": "...",
  "mode": "reencode",
  "status": "completed",
  "output_file": "outputs/draft_assembled.mp4",
  "final_output_file": "outputs/draft.mp4",
  "work_dir": "outputs/render_work",
  "clips": [
    {
      "id": "clip_0001",
      "source_path": "/absolute/path/to/source.mov",
      "start": 4.2,
      "end": 16.7,
      "duration_seconds": 12.5,
      "work_file": "outputs/render_work/clip_0001.mp4",
      "status": "completed",
      "error": ""
    }
  ],
  "captions": {
    "requested": true,
    "source_path": "/path/to/captions.srt",
    "status": "applied"
  },
  "voiceover": {
    "requested": true,
    "source_path": "/path/to/voiceover.wav",
    "status": "applied"
  },
  "platform": {
    "requested": true,
    "normalized": "tiktok",
    "width": 1080,
    "height": 1920,
    "fit": "crop",
    "background": "black",
    "status": "applied"
  },
  "final_probe": {
    "duration_seconds": 42.5,
    "width": 1080,
    "height": 1920,
    "video_stream_count": 1,
    "audio_stream_count": 1
  },
  "stages": [
    {"name": "assembled_video",  "file": "outputs/draft_assembled.mp4",  "status": "completed"},
    {"name": "voiceover_mix",    "file": "outputs/draft_audio.mp4",      "status": "completed"},
    {"name": "platform_format",  "file": "outputs/draft_platform.mp4",   "status": "completed"},
    {"name": "caption_burn",     "file": "outputs/draft.mp4",            "status": "completed"}
  ],
  "warnings": []
}
```

When no post-processing flags are used, `output_file` and `final_output_file` both point to `outputs/draft.mp4` and `stages` contains only `assembled_video`.

## Output Files

| File | Description |
|---|---|
| `outputs/draft.mp4` | Final output video (always) |
| `outputs/draft_assembled.mp4` | Intermediate after clip assembly (post-processing only) |
| `outputs/draft_audio.mp4` | Intermediate after voiceover mix (when voiceover + captions or platform follow) |
| `outputs/draft_platform.mp4` | Intermediate after platform format (when captions follow) |
| `outputs/render_work/clip_NNNN.mp4` | Intermediate per-clip files |
| `outputs/render_work/concat_list.txt` | FFmpeg concat demuxer input list |
| `outputs/creative_assemble_result.json` | Result artifact |
| `outputs/creative_assemble_review.md` | Review markdown (`--write-artifact`) |

## Flags

```
--mode <reencode|stream-copy>   Clip cutting mode (default: reencode)
--dry-run                       Print planned commands, write nothing
--overwrite                     Required to replace an existing assemble result
--keep-work                     Keep render_work/ after assembly
--max-clips <n>                 Limit to first N clips
--burn-captions                 Burn SRT captions into the output via subtitles filter
--captions <path>               Path to SRT caption file (auto-discovered if omitted)
--allow-missing-captions        Skip caption burn if no SRT file is found
--mix-voiceover                 Mix a local audio file into the output via amix filter
--voiceover <path>              Path to audio file (auto-discovered if omitted)
--allow-missing-voiceover       Skip voiceover mix if no audio file is found
--run-id <id>                   Run ID used for caption auto-discovery
--platform <preset>             Platform export preset: original|tiktok|instagram-reel|youtube-short|youtube|square (default: original)
--fit <crop|pad>                Scale/crop or scale/pad to target dimensions (default: per-preset)
--background <color>            Pad color (default: black; used in pad mode only)
--caption-position <pos>        Caption vertical position: auto|bottom|center|top (default: auto → bottom)
--caption-margin <n>            Vertical margin in pixels (default: platform-dependent — 160 vertical, 100 square, 80 otherwise)
--caption-style <style>         Caption style: default|bold|boxed (default: default)
```

## Modes

### `reencode` (default)

```
ffmpeg -y -ss <start> -to <end> -i <source> -c:v libx264 -c:a aac <work_file>
```

Frame-accurate cutting. Slower but correct for all inputs.

### `stream-copy`

```
ffmpeg -y -ss <start> -to <end> -i <source> -c copy <work_file>
```

Faster. May produce slightly inaccurate cut points near keyframe boundaries.

## Assembly Step

After per-clip cutting, clips are assembled using the FFmpeg concat demuxer:

```
ffmpeg -y -f concat -safe 0 -i concat_list.txt -c copy outputs/draft.mp4
```

If only one clip, it is remuxed directly:

```
ffmpeg -y -i <work_clip> -c copy outputs/draft.mp4
```

## Staged Rendering (Post-Processing)

When any of `--burn-captions`, `--mix-voiceover`, or `--platform` (non-original) is used, rendering proceeds in stages:

1. **assembled_video** — clip assembly → `draft_assembled.mp4`
2. **voiceover_mix** (if `--mix-voiceover`) — amix filter:
   ```
   ffmpeg -y -i draft_assembled.mp4 -i voiceover.wav \
     -filter_complex [0:a][1:a]amix=inputs=2:duration=first:dropout_transition=2[outa] \
     -map 0:v -map [outa] -c:v copy -c:a aac draft_audio.mp4
   ```
3. **platform_format** (if `--platform` ≠ original) — scale/crop or scale/pad:
   - Crop mode (vertical/square defaults):
     ```
     ffmpeg -y -i <stage_input> -vf "scale=W:H:force_original_aspect_ratio=increase,crop=W:H" \
       -c:v libx264 -c:a copy draft_platform.mp4
     ```
   - Pad mode (YouTube default):
     ```
     ffmpeg -y -i <stage_input> -vf "scale=W:H:force_original_aspect_ratio=decrease,pad=W:H:(ow-iw)/2:(oh-ih)/2:color=COLOR" \
       -c:v libx264 -c:a copy draft_platform.mp4
     ```
4. **caption_burn** (if `--burn-captions`) — subtitles filter burned onto platform-formatted frame:
   ```
   ffmpeg -y -i <stage_input> -vf "subtitles=<escaped_path>:force_style='Alignment=2,MarginV=160'" \
     -c:a copy draft.mp4
   ```
   When `--caption-position`, `--caption-margin`, or `--caption-style` are used, `force_style` is populated with ASS style overrides.

**Captions are always burned after platform formatting**, so they render at the correct position and scale for the target aspect ratio.

The final `draft.mp4` is always the command output regardless of which stages ran.

## Caption Position Profiles

When `--burn-captions` is used, the `subtitles` filter embeds an ASS `force_style` override that controls position, margin, and style.

### Positions

| Flag value | ASS Alignment | Notes |
|------------|---------------|-------|
| `bottom` (default) | 2 | Lower center |
| `center` | 5 | Middle center |
| `top` | 8 | Upper center |
| `auto` | resolved → `bottom` | Platform-aware; currently always resolves to `bottom` |

### Default Margins (pixels)

| Platform | Default `MarginV` |
|---|---|
| `tiktok`, `instagram-reel`, `youtube-short` | 160 |
| `square` | 100 |
| `youtube`, `original`, (default) | 80 |

Override with `--caption-margin <n>`.

### Styles

| Flag value | Effect |
|---|---|
| `default` | No style overrides |
| `bold` | `Bold=1` |
| `boxed` | `BorderStyle=3,Outline=1,Shadow=0,BackColour=&H80000000` (semi-transparent black box) |

### Caption Fields in Result JSON

| Field | Type | Description |
|---|---|---|
| `position` | string | Resolved position (`bottom`, `center`, `top`) |
| `margin` | int | Vertical margin in pixels |
| `style` | string | Normalized style (`default`, `bold`, `boxed`) |
| `filter_style` | string | Full `force_style` string passed to FFmpeg |

These fields are present only when `captions.status = "applied"`.

## Subtitles Filter Preflight

Before running the caption burn stage, `creative-assemble` checks whether the `subtitles` filter
is available in the installed ffmpeg build (using `ffmpeg -hide_banner -filters`).

- If the filter **is available**: caption burn proceeds normally.
- If the filter **is missing** and `--allow-missing-captions` is set: caption burn is skipped,
  `captions.status = "skipped"`, and a warning is added explaining that libass is required.
- If the filter **is missing** and `--allow-missing-captions` is NOT set: command fails before
  any ffmpeg work with a clear error:
  ```
  ffmpeg does not support the subtitles filter required for caption burn.
  Install ffmpeg with libass support ... or rerun with --allow-missing-captions.
  ```
- The preflight check is skipped for `--dry-run`.

To check filter availability independently: `byom-video doctor --media`

The subtitles filter requires ffmpeg compiled with `--enable-libass`. The default Homebrew
ffmpeg formula does not include libass.

## FFmpeg Error Surfacing

When any ffmpeg stage fails (clip cut, concat, voiceover mix, caption burn), the last 5 lines
of ffmpeg's stderr output are captured and included in:
- `result.Warnings` (assembled video-level)
- `clip.Error` (per-clip)
- `captions.error` / `voiceover.error` (post-processing stages)

This replaces the previous behavior of showing only the exit code.

## Caption Auto-Discovery

When `--burn-captions` is set without `--captions <path>`:

1. Checks run's artifact directory for `captions.srt` (via `--run-id` or timeline's `run_id`)
2. If only `caption_plan.json` exists (stub output), logs a warning (captions not rendered yet)
3. Fails with a clear error unless `--allow-missing-captions` is set

## Voiceover Auto-Discovery

When `--mix-voiceover` is set without `--voiceover <path>`:

Checks `outputs/voiceover.{wav,mp3,m4a,aac}` in the plan's outputs directory.

## Status Values

### Result status

| Status | Meaning |
|---|---|
| `completed` | All clips rendered and assembled |
| `partial` | Some clips failed; draft assembled from successful clips |
| `failed` | All clips failed or assembly failed |

### Captions / voiceover status

| Status | Meaning |
|---|---|
| `applied` | Stage ran successfully |
| `skipped` | File not found and allow-missing flag was set |
| `failed` | FFmpeg returned an error for this stage |

## Safety

- Input files come only from `creative_timeline.json.input_path` (the original source media).
- Caption and voiceover paths must exist on disk before any FFmpeg work begins.
- No arbitrary shell strings are executed. FFmpeg is called via `exec.Command` with arg slices.
- Filter graph paths are escaped (`\` → `\\`, `:` → `\:`, `'` → `\'`) without shell involvement.
- Original media is never modified or deleted.
- If ffmpeg is not on PATH, fails with a clean error and doctor hint.
- `--dry-run` prints planned commands and writes nothing.

## Intermediate Files

Work clips in `outputs/render_work/` are kept by default (alpha behavior). This allows inspection and re-assembly without re-encoding if the concat step fails.

## Validation

`validate-creative-assemble` checks:

- `schema_version = "creative_assemble_result.v1"`
- `output_file` field is non-empty
- `draft.mp4` exists on disk (if status is `completed` or `partial`)
- Work clips exist for all `status=completed` entries
- `captions.source_path` exists when `captions.status = "applied"`
- `voiceover.source_path` exists when `voiceover.status = "applied"`
- **Platform dimension check**: if `platform.status = "applied"` and ffprobe is available, probes `draft.mp4` and fails if width/height do not match `platform.width`/`platform.height`
- If ffprobe is unavailable for platform check: warns instead of failing
- If ffprobe is available, probes `draft.mp4` for a readable duration

`validate-creative-plan` also checks assemble result if present.

## Events

| Event | When |
|---|---|
| `CREATIVE_ASSEMBLE_STARTED` | Command begins |
| `CREATIVE_ASSEMBLE_CLIP_RENDERED` | Per clip, with `status` and optional `error` |
| `CREATIVE_ASSEMBLE_COMPLETED` | Draft written successfully |
| `CREATIVE_ASSEMBLE_FAILED` | All clips or assembly step failed |
| `CREATIVE_ASSEMBLE_VOICEOVER_COMPLETED` | Voiceover mix completed |
| `CREATIVE_ASSEMBLE_PLATFORM_STARTED` | Platform format stage begins |
| `CREATIVE_ASSEMBLE_PLATFORM_COMPLETED` | Platform format stage completed |
| `CREATIVE_ASSEMBLE_PLATFORM_FAILED` | Platform format stage failed |
| `CREATIVE_ASSEMBLE_CAPTIONS_COMPLETED` | Caption burn completed |

## Platform Fields

The `platform` object is present only when `--platform` is set to a non-`original` preset.

| Field | Type | Description |
|-------|------|-------------|
| `requested` | bool | Always `true` when present |
| `normalized` | string | Canonical preset name (after alias resolution) |
| `width` | int | Target width in pixels |
| `height` | int | Target height in pixels |
| `fit` | string | `crop` or `pad` |
| `background` | string | Pad color (default `black`) |
| `status` | string | `applied` or `failed` |
| `error` | string | Truncated FFmpeg error output (when `status=failed`) |

The `final_probe` object captures ffprobe output from `draft.mp4` after all stages complete. It is `null` when ffprobe is not on PATH.

| Field | Type | Description |
|-------|------|-------------|
| `duration_seconds` | float | Duration from ffprobe format block |
| `width` | int | First video stream width |
| `height` | int | First video stream height |
| `video_stream_count` | int | Number of video streams |
| `audio_stream_count` | int | Number of audio streams |

See [docs/platform-presets.md](../platform-presets.md) for supported presets, aliases, and examples.

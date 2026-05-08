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
  "stages": [
    {"name": "assembled_video", "file": "outputs/render_work/draft_assembled.mp4", "status": "completed"},
    {"name": "voiceover_mix",   "file": "outputs/render_work/draft_audio.mp4",     "status": "completed"},
    {"name": "caption_burn",    "file": "outputs/draft.mp4",                        "status": "completed"}
  ],
  "warnings": []
}
```

When no post-processing flags are used, `output_file` and `final_output_file` both point to `outputs/draft.mp4` and `stages` contains only `assembled_video`.

## Output Files

| File | Description |
|---|---|
| `outputs/draft.mp4` | Final output video (always) |
| `outputs/render_work/draft_assembled.mp4` | Intermediate after clip assembly (post-processing only) |
| `outputs/render_work/draft_audio.mp4` | Intermediate after voiceover mix (when both voiceover + captions) |
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

When `--burn-captions` or `--mix-voiceover` is used, rendering proceeds in stages:

1. **assembled_video** — clip assembly → `draft_assembled.mp4`
2. **voiceover_mix** (if `--mix-voiceover`) — amix filter:
   ```
   ffmpeg -y -i draft_assembled.mp4 -i voiceover.wav \
     -filter_complex [0:a][1:a]amix=inputs=2:duration=first:dropout_transition=2[outa] \
     -map 0:v -map [outa] -c:v copy -c:a aac draft_audio.mp4
   ```
3. **caption_burn** (if `--burn-captions`) — subtitles filter:
   ```
   ffmpeg -y -i <stage_input> -vf subtitles=<escaped_path> -c:a copy draft.mp4
   ```

The final `draft.mp4` is always the command output regardless of which stages ran.

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
| `CREATIVE_ASSEMBLE_CAPTIONS_COMPLETED` | Caption burn completed |

# Platform Export Presets

`byom-video` supports platform-specific output dimensions via the `--platform` flag on `creative-assemble` and `make`.

---

## Supported Presets

| Preset | Width | Height | Aspect | Default Fit |
|--------|-------|--------|--------|-------------|
| `original` | — | — | source | none |
| `tiktok` | 1080 | 1920 | 9:16 | crop |
| `instagram-reel` | 1080 | 1920 | 9:16 | crop |
| `youtube-short` | 1080 | 1920 | 9:16 | crop |
| `youtube` | 1920 | 1080 | 16:9 | pad |
| `square` | 1080 | 1080 | 1:1 | crop |

`original` keeps the source video dimensions unchanged. No FFmpeg scale/crop/pad stage is run.

---

## Aliases

| Alias | Resolves to |
|-------|-------------|
| `reel` | `instagram-reel` |
| `reels` | `instagram-reel` |
| `ig` | `instagram-reel` |
| `shorts` | `youtube-short` |
| `yt-short` | `youtube-short` |
| `yt` | `youtube` |

---

## Fit Modes

### `crop` (default for vertical/square)

Fills the target frame. Scales up so neither dimension underflows, then center-crops to the exact target size. Edges may be trimmed.

FFmpeg filter:
```
scale=W:H:force_original_aspect_ratio=increase,crop=W:H
```

### `pad` (default for YouTube 16:9)

Fits inside the target frame. Scales down so neither dimension overflows, then adds padding (letterbox/pillarbox) to fill the remainder.

FFmpeg filter:
```
scale=W:H:force_original_aspect_ratio=decrease,pad=W:H:(ow-iw)/2:(oh-ih)/2:color=COLOR
```

Override the default fit with `--fit crop` or `--fit pad`.

---

## Background Color

Used in pad mode. Default: `black`.

Pass any FFmpeg color name: `black`, `white`, `0x1a1a1a`, etc.

```sh
byom-video creative-assemble <plan_id> --platform youtube --fit pad --background white
```

---

## Stage Ordering

When platform formatting is combined with other post-processing, the order is:

1. Assemble clips → `draft_assembled.mp4`
2. Mix voiceover → `draft_audio.mp4` (if `--mix-voiceover`)
3. Apply platform format → `draft_platform.mp4` (if platform ≠ original)
4. Burn captions → `draft.mp4` (if `--burn-captions`)

Captions are burned onto the final platform-formatted frame, so they appear at the correct position and scale for the target aspect ratio.

---

## Stage Files

| File | When written |
|------|-------------|
| `draft_assembled.mp4` | Always when post-processing follows |
| `draft_audio.mp4` | When `--mix-voiceover` and captions or platform follow |
| `draft_platform.mp4` | When platform ≠ original and captions follow |
| `draft.mp4` | Final output always |

---

## CLI Examples

### Make a vertical reel (TikTok / Reels / YouTube Short)

```sh
byom-video make input.mov --goal "make a vertical short" --platform instagram-reel --yes
```

Aliases work:

```sh
byom-video make input.mov --goal "make a short" --platform reels --yes
byom-video make input.mov --goal "make a short" --platform tiktok --yes
byom-video make input.mov --goal "make a short" --platform shorts --yes
```

### Make a square draft

```sh
byom-video make input.mov --goal "make a square clip" --platform square --yes
```

### Make a YouTube 16:9 draft

```sh
byom-video make input.mov --goal "make a youtube video" --platform youtube --yes
```

Pad with a white background instead of black:

```sh
byom-video make input.mov --goal "make a youtube video" --platform youtube --fit pad --background white --yes
```

### Force crop on YouTube (fills frame, may trim top/bottom)

```sh
byom-video make input.mov --goal "make a youtube video" --platform youtube --fit crop --yes
```

### Dry-run to preview commands

```sh
byom-video make input.mov --goal "test" --platform tiktok --dry-run
byom-video creative-assemble <plan_id> --platform instagram-reel --fit crop --dry-run
```

### Assemble only

```sh
byom-video creative-assemble <plan_id> --platform tiktok --overwrite
byom-video creative-assemble <plan_id> --platform youtube --fit pad --background black --overwrite
byom-video creative-assemble <plan_id> --platform square --burn-captions --overwrite
```

---

## Result Artifact

Platform fields in `outputs/creative_assemble_result.json`:

```json
"platform": {
  "requested": true,
  "normalized": "tiktok",
  "width": 1080,
  "height": 1920,
  "fit": "crop",
  "background": "black",
  "status": "applied"
}
```

`status` values:
- `applied` — platform format stage ran successfully
- `failed` — FFmpeg error; `error` field contains the truncated FFmpeg output
- `skipped` — not set (only `applied` or `failed` appear when platform ≠ original)

Platform fields are absent when `platform = original`.

---

## Validation

`validate-creative-assemble` checks platform dimensions when `status = applied`:

- If `ffprobe` is available: probes `draft.mp4` and fails if dimensions do not match expected width/height.
- If `ffprobe` is unavailable: warns instead of failing.

```sh
byom-video validate-creative-assemble <plan_id>
```

---

## Known Limitations

- No manual crop anchor — crop always centers.
- No safe-area guide overlay (title-safe, action-safe zones).
- Caption layout is not platform-aware — captions are burned at fixed positions; adjust SRT timing if needed.
- `--background` accepts FFmpeg color strings but is not validated by the CLI; invalid values will produce an FFmpeg error at runtime.
- Only `libx264` is used for the platform re-encode stage; hardware acceleration is not configurable in this version.

# Make Revisions

The `revise-make` command lets you revise an existing `make` output with natural, deterministic requests like "switch to TikTok", "move captions to center", or "reassemble". No LLM planner is involved — requests are parsed locally and mapped to safe, deterministic actions.

---

## Quick Start

```bash
# 1. Create a make first
byom-video make video.mov --goal "product launch short" --platform instagram-reel \
  --burn-captions --yes

# 2. Preview a revision (dry-run — writes nothing)
byom-video revise-make <make_id> --request "switch to square" --dry-run

# 3. Plan the revision (writes revision artifact but doesn't execute)
byom-video revise-make <make_id> --request "switch to square"

# 4. Execute the revision
byom-video revise-make <make_id> --request "switch to square" --yes --overwrite --reassemble

# 5. Review the result
byom-video make-revisions <make_id>
byom-video inspect-make-revision <make_id> revision_0001
byom-video review-make-revision <make_id> revision_0001 --write-artifact
byom-video make-result <make_id>
```

---

## Supported Revision Requests

### Platform

| Request | Result |
|---|---|
| `"switch to TikTok"`, `"tiktok"` | Platform → `tiktok` |
| `"instagram reel"`, `"reel"`, `"ig"` | Platform → `instagram-reel` |
| `"youtube short"`, `"yt-short"` | Platform → `youtube-short` |
| `"vertical"` | Platform → `tiktok` |
| `"square"` | Platform → `square` |
| `"youtube"`, `"yt"` | Platform → `youtube` |

### Caption Position

| Request | Result |
|---|---|
| `"move captions to center"`, `"captions center"` | Caption position → `center` |
| `"move captions to top"`, `"captions top"` | Caption position → `top` |
| `"move captions to bottom"`, `"captions bottom"` | Caption position → `bottom` |

### Caption Style

| Request | Result |
|---|---|
| `"boxed captions"`, `"make captions boxed"` | Caption style → `boxed` |
| `"bold captions"`, `"make captions bold"` | Caption style → `bold` |
| `"default captions"`, `"reset captions"` | Caption style → `default` |

### Duration (v1 — records intent, reassembles with current selection)

| Request | Result |
|---|---|
| `"make it shorter"`, `"shorter"` | Reassemble with `duration_hint=shorter` note |
| `"make it longer"`, `"longer"` | Reassemble with `duration_hint=longer` note |

> **v1 limitation:** Clip selection is not automatically adjusted. For true duration changes, re-run `byom-video roughcut` / `byom-video selected-clips` first, then reassemble.

### Script (requires provider)

| Request | Tone |
|---|---|
| `"regenerate script"`, `"rewrite script"` | (none) |
| `"more cinematic script"` | `cinematic` |
| `"make script funnier"` | `funny` |
| `"more professional script"` | `professional` |

Requires `--allow-provider-calls` (or `--fallback-stub` for offline generation).

### Caption Variants (requires provider)

| Request | Result |
|---|---|
| `"regenerate captions"`, `"new captions"` | Regenerate caption variants |
| `"more caption options"`, `"caption variants"` | Regenerate caption variants |

Requires `--allow-provider-calls` (or `--fallback-stub`).

### Voiceover

| Request | Action | Provider? |
|---|---|---|
| `"prepare voiceover"`, `"voiceover text"` | Run `creative-voiceover-text` | No |
| `"generate voiceover"`, `"regenerate voiceover"` | Run `creative-generate-voiceover` | Yes |
| `"mix voiceover"`, `"add voiceover"` | Reassemble with `--mix-voiceover` | No |

### Reassemble

| Request | Result |
|---|---|
| `"reassemble"`, `"render again"` | Run `creative-assemble` with current settings |
| `"make new draft"`, `"re-assemble"` | Same |

---

## Commands

### `revise-make`

```
byom-video revise-make <make_id> --request <text> [flags]
```

| Flag | Description |
|---|---|
| `--request <text>` | **Required.** Natural-language revision request |
| `--dry-run` | Show planned actions; write nothing |
| `--yes` | Execute actions immediately |
| `--overwrite` | Allow overwriting existing artifacts |
| `--reassemble` | Append a reassemble step after other actions |
| `--validate` | Append a validate step after reassemble |
| `--allow-provider-calls` | Allow script/caption/voiceover provider calls |
| `--fallback-stub` | Use stub generation when provider is unavailable |
| `--json` | Emit revision summary JSON to stdout |

**Execution modes:**

| Mode | Flags | What happens |
|---|---|---|
| Preview | `--dry-run` | Shows planned actions; writes nothing |
| Plan | *(no --yes)* | Writes `revision_summary.json` with `status=planned`; prints next command |
| Execute | `--yes` | Executes actions; writes `revision_summary.json` with `status=completed/failed` |

### `make-revisions`

```
byom-video make-revisions <make_id> [--json]
```

Lists all revisions for a make, sorted by creation time.

### `inspect-make-revision`

```
byom-video inspect-make-revision <make_id> <revision_id> [--json]
```

Prints the revision summary in human-readable or JSON format.

### `review-make-revision`

```
byom-video review-make-revision <make_id> <revision_id> [--json] [--write-artifact]
```

Renders a Markdown review of the revision. With `--write-artifact`, writes `revision_review.md` to the revision directory.

---

## Revision Behavior

### Snapshot

Before mutating any artifacts, the revision command creates a snapshot under:

```
.byom-video/makes/<make_id>/revisions/<revision_id>/
  request.txt
  before_make_summary.json
  before_creative_plan.json       (if present)
  before_script_draft.json        (if present)
  before_caption_variants.json    (if present)
  before_voiceover_text.json      (if present)
  before_creative_timeline.json   (if present)
  before_creative_render_plan.json (if present)
  before_creative_assemble_result.json (if present)
```

MP4, audio, and other media files are **never copied** into the snapshot (use `--snapshot-media` when that feature is added).

### Reassemble Settings Inheritance

When a reassemble action runs, it inherits settings from the original make summary, overridden by the current revision:

| Setting | Source |
|---|---|
| Platform | revision state → original `platform_preset` |
| Caption position | revision state → original `caption_position` |
| Caption style | revision state → original `caption_style` |
| Caption margin | revision state → original `caption_margin` |
| Burn captions | `caption_status == "applied"` from original |
| Mix voiceover | `voiceover_status == "applied"` or explicit `mix_voiceover` action |
| Allow missing captions | Always `true` (safe for reassemble) |
| Allow missing voiceover | Always `true` (safe for reassemble) |

### Make Summary Integration

After a successful revision, `make_summary.json` is updated with:
- `latest_revision_id` — the most recent revision ID
- `revision_count` — number of revisions performed
- `revision_status` — `planned|completed|failed`

`make-result` and `inspect-make` both show revision information when present.

---

## Provider-Call Guardrails

Actions that require an external provider (Ollama, ElevenLabs) are blocked by default:

- **Script/caption generation:** blocked unless `--allow-provider-calls` or `--fallback-stub`
- **Voiceover generation:** blocked unless `--allow-provider-calls`
- **Platform/caption changes, voiceover prep, reassemble:** always safe, no provider needed

---

## Examples

```bash
# Switch platform and reassemble
byom-video revise-make <make_id> --request "switch to square" --yes --overwrite --reassemble

# Move captions and re-render
byom-video revise-make <make_id> --request "move captions to center" --yes --overwrite --reassemble --validate

# Regenerate script (with live provider)
byom-video revise-make <make_id> --request "more cinematic script" --yes --overwrite --allow-provider-calls

# Regenerate captions offline (fallback stub)
byom-video revise-make <make_id> --request "regenerate captions" --yes --overwrite --fallback-stub

# Prepare voiceover text
byom-video revise-make <make_id> --request "prepare voiceover" --yes --overwrite

# Mix voiceover into existing draft
byom-video revise-make <make_id> --request "mix voiceover" --yes --overwrite

# Full inspection after revision
byom-video make-revisions <make_id>
byom-video inspect-make-revision <make_id> revision_0001
byom-video review-make-revision <make_id> revision_0001 --write-artifact
```

---

## Known Limitations (v1)

- Duration changes (`shorter`/`longer`) only record intent and reassemble with the existing clip selection. Clip selection must be changed manually via `byom-video roughcut` / `byom-video selected-clips`.
- `--new-make` flag (creating a new make instead of revising the existing one) is parsed but not yet implemented.
- No media files are copied into snapshots; the original draft MP4 is never moved or deleted.
- Only `elevenlabs-compatible` provider supports live voiceover generation.
- Revision history is append-only; there is no rollback mechanism yet.

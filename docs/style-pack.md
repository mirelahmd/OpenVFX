# Style Pack

A Style Pack lets you teach `byom-video` your voice, visual preferences, and brand rules so that AI-generated scripts reflect your actual creative identity rather than generic output.

Style Packs are a directory of Markdown files. The default location is `.openvfx/style/` relative to your working directory.

---

## Quick Start

```bash
# Create the style pack directory with template files
byom-video style init

# Edit the files to describe your channel
# (profile.md, script_style.md, captions.md, visual_style.md, do_not_do.md, examples.md)

# Verify your edits
byom-video style validate

# Inspect file presence and size
byom-video style inspect
```

---

## Style Pack Files

Each file is a Markdown document with a specific role. All are optional — missing files are silently skipped.

| File | Purpose |
|------|---------|
| `profile.md` | Who you are: channel name, audience, personality, target platforms |
| `script_style.md` | Tone, pacing, hook style, structure, target length |
| `captions.md` | Caption casing, emoji rules, per-line limits |
| `visual_style.md` | Visual tone, cut pacing, b-roll preference, color mood |
| `do_not_do.md` | Banned phrases, claims to avoid, styles to avoid |
| `examples.md` | Good and bad hooks, sample scripts in your voice |

---

## Commands

### `byom-video style init`

Creates `.openvfx/style/` and writes template files.

```
byom-video style init [--force] [--style-dir <path>] [--json]
```

- `--force` — overwrite existing files
- `--style-dir <path>` — use a custom directory instead of `.openvfx/style/`
- `--json` — machine-readable output

### `byom-video style inspect`

Shows file presence, file size, and a one-line preview of each file.

```
byom-video style inspect [--style-dir <path>] [--json]
```

### `byom-video style validate`

Checks that files exist and are not still full of template placeholders.

```
byom-video style validate [--style-dir <path>] [--strict] [--json]
```

- `--strict` — treat missing files as errors (fails validation)
- Without `--strict` — missing files produce warnings but validation passes

---

## Using Style Packs with Script Generation

When you run `byom-video creative-generate-script`, the style pack is automatically loaded and injected into the Ollama prompt:

```bash
byom-video creative-generate-script <plan_id> [--style-dir <path>] [--no-style]
```

- `--no-style` — skip the style pack even if one exists
- `--style-dir <path>` — override the default style pack location

Style context is recorded in `script_draft.json` under `style_context`, so you can always see which files were used.

You can also generate a script inline with `byom-video make`:

```bash
byom-video make input.mov \
  --goal "make a product reveal short" \
  --yes \
  --generate-script \
  --style-dir my-style/
```

---

## Using Style Packs with Caption Variant Generation

Caption variants inherit your style pack automatically. The `captions.md` file is especially influential here.

```bash
byom-video creative-caption-variants <plan_id> [--style-dir <path>] [--no-style] [--count <n>] [--max-words <n>] [--tone <text>]
```

- `--count <n>` — number of variants to generate (default: 5)
- `--max-words <n>` — maximum words per caption (default: 12)
- `--tone <text>` — tone hint (e.g. "punchy", "conversational", "professional")
- `--no-style` — skip the style pack
- `--fallback-stub` — write a stub if Ollama is unreachable

Review the generated variants:

```bash
byom-video review-caption-variants <plan_id> [--write-artifact]
```

Generate captions inline with `make`:

```bash
byom-video make input.mov \
  --goal "make a product reveal short" \
  --yes \
  --generate-script \
  --generate-captions \
  --caption-count 5 \
  --style-dir my-style/
```

The caption variants are written to `outputs/caption_variants.json` and reference the script draft as their source when one is available.

---

## Style Pack Format

Files are standard Markdown. Use `##` headings to organise sections. The tool reads all non-empty files and concatenates them with `---` separators.

**Example `profile.md`:**

```markdown
# Creator Profile

## Channel / Creator Name
TechWithTom

## Audience
Software developers and startup founders aged 25-40

## Target Platforms
- YouTube Shorts
- LinkedIn

## Personality
Direct, no-fluff, occasionally dry-humoured. Respects the audience's time.
```

**Example `do_not_do.md`:**

```markdown
# Do Not Do

## Banned Phrases
- "In this video..."
- "Hey guys..."
- "Don't forget to like and subscribe"

## Claims to Avoid
- Do not invent benchmark numbers
- Do not make promises about product features

## Style to Avoid
- Passive voice
- Long warm-up intros before getting to the point
```

---

## Content Limits

Style pack content is capped at **8,000 characters** total (across all files). If your pack exceeds this limit, content is truncated with a warning recorded in the script output.

Truncation note appears in `script_draft.json` under `style_context.warnings`.

---

## Template Detection

`style validate` warns when files still contain unedited template placeholders such as:

- `My Channel`
- `General audience`
- `[topic]`
- `<!-- Your name`

These warnings do not fail validation (unless `--strict` is used). They are a reminder that the file has not been customised yet.

---

## Custom Style Pack Location

You can keep style packs outside the default `.openvfx/style/` location — useful for multi-channel setups:

```bash
byom-video style init --style-dir channels/cooking/style
byom-video creative-generate-script <plan_id> --style-dir channels/cooking/style
```

---

## JSON Output

All style commands support `--json` for scripting and CI integration:

```bash
byom-video style validate --json
# {
#   "valid": true,
#   "style_dir": ".openvfx/style",
#   "warnings": ["captions.md: appears to still contain template placeholders"]
# }
```

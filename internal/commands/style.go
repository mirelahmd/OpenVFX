package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultStyleDir = ".openvfx/style"
const styleMaxChars = 8000

// styleFiles lists the required style pack markdown files in order.
var styleFiles = []string{
	"profile.md",
	"script_style.md",
	"captions.md",
	"visual_style.md",
	"do_not_do.md",
	"examples.md",
}

// ---- options ----

type StyleInitOptions struct {
	Force    bool
	JSON     bool
	StyleDir string
}

type StyleInspectOptions struct {
	JSON     bool
	StyleDir string
}

type StyleValidateOptions struct {
	JSON     bool
	Strict   bool
	StyleDir string
}

// StyleContext records which style files were used when building a prompt.
type StyleContext struct {
	Enabled   bool     `json:"enabled"`
	StyleDir  string   `json:"style_dir"`
	FilesUsed []string `json:"files_used"`
	Truncated bool     `json:"truncated,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	content   string   // not serialised
}

// ---- style init ----

func StyleInit(stdout io.Writer, opts StyleInitOptions) error {
	dir := opts.StyleDir
	if dir == "" {
		dir = defaultStyleDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create style dir: %w", err)
	}

	type result struct {
		File    string `json:"file"`
		Written bool   `json:"written"`
		Reason  string `json:"reason,omitempty"`
	}
	var results []result

	for _, name := range styleFiles {
		path := filepath.Join(dir, name)
		reason := ""
		written := false
		if _, err := os.Stat(path); err == nil {
			if !opts.Force {
				reason = "already exists (use --force to overwrite)"
			} else {
				if err := os.WriteFile(path, []byte(styleFileTemplate(name)), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", name, err)
				}
				written = true
				reason = "overwritten (--force)"
			}
		} else {
			if err := os.WriteFile(path, []byte(styleFileTemplate(name)), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
			written = true
		}
		results = append(results, result{File: filepath.Join(dir, name), Written: written, Reason: reason})
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"style_dir": dir,
			"files":     results,
		})
	}

	fmt.Fprintf(stdout, "Style pack: %s\n", dir)
	for _, r := range results {
		status := "created"
		if !r.Written {
			status = "skipped"
		}
		fmt.Fprintf(stdout, "  %-12s %s", status, r.File)
		if r.Reason != "" {
			fmt.Fprintf(stdout, " — %s", r.Reason)
		}
		fmt.Fprintln(stdout)
	}
	if !opts.Force {
		anySkipped := false
		for _, r := range results {
			if !r.Written {
				anySkipped = true
				break
			}
		}
		if anySkipped {
			fmt.Fprintf(stdout, "\nTip: use --force to overwrite existing files.\n")
		}
	}
	return nil
}

// ---- style inspect ----

type StyleInspectResult struct {
	StyleDir string            `json:"style_dir"`
	Files    []StyleFileStatus `json:"files"`
}

type StyleFileStatus struct {
	Name     string `json:"name"`
	Present  bool   `json:"present"`
	SizeBytes int64 `json:"size_bytes,omitempty"`
	Preview  string `json:"preview,omitempty"`
}

func StyleInspect(stdout io.Writer, opts StyleInspectOptions) error {
	dir := opts.StyleDir
	if dir == "" {
		dir = defaultStyleDir
	}

	result := StyleInspectResult{StyleDir: dir}
	for _, name := range styleFiles {
		path := filepath.Join(dir, name)
		stat, err := os.Stat(path)
		sf := StyleFileStatus{Name: name}
		if err == nil {
			sf.Present = true
			sf.SizeBytes = stat.Size()
			data, readErr := os.ReadFile(path)
			if readErr == nil && len(data) > 0 {
				line := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
				if len(line) > 80 {
					line = line[:77] + "..."
				}
				sf.Preview = line
			}
		}
		result.Files = append(result.Files, sf)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(stdout, "Style pack: %s\n", dir)
	fmt.Fprintf(stdout, "%-22s %-8s %-10s %s\n", "FILE", "PRESENT", "SIZE", "PREVIEW")
	for _, sf := range result.Files {
		present := "no"
		size := ""
		preview := ""
		if sf.Present {
			present = "yes"
			size = fmt.Sprintf("%dB", sf.SizeBytes)
			preview = sf.Preview
		}
		fmt.Fprintf(stdout, "%-22s %-8s %-10s %s\n", sf.Name, present, size, preview)
	}
	return nil
}

// ---- style validate ----

type StyleValidateResult struct {
	Valid    bool     `json:"valid"`
	StyleDir string   `json:"style_dir"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// templateMarkers are strings that indicate a file is still mostly template content.
var templateMarkers = []string{
	"<!-- Your name",
	"<!-- Who watches",
	"<!-- e.g.",
	"[topic]",
	"My Channel",
	"General audience",
}

func StyleValidate(stdout io.Writer, opts StyleValidateOptions) error {
	dir := opts.StyleDir
	if dir == "" {
		dir = defaultStyleDir
	}

	result := StyleValidateResult{StyleDir: dir, Valid: true}

	for _, name := range styleFiles {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			msg := fmt.Sprintf("%s: missing", name)
			if opts.Strict {
				result.Errors = append(result.Errors, msg)
				result.Valid = false
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
			continue
		}
		text := strings.TrimSpace(string(data))
		if len(text) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: empty", name))
			continue
		}
		// Check if still mostly template
		isTemplate := false
		for _, marker := range templateMarkers {
			if strings.Contains(text, marker) {
				isTemplate = true
				break
			}
		}
		if isTemplate {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: appears to still contain template placeholders — customize it for better results", name))
		}
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Valid && len(result.Errors) == 0 {
		fmt.Fprintf(stdout, "Style pack: %s — valid\n", dir)
	} else {
		fmt.Fprintf(stdout, "Style pack: %s — validation failed\n", dir)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	for _, e := range result.Errors {
		fmt.Fprintf(stdout, "  error:   %s\n", e)
	}
	if !result.Valid {
		return fmt.Errorf("style validate failed: %d error(s)", len(result.Errors))
	}
	return nil
}

// ---- style load (internal) ----

// LoadStylePack reads style pack files into a StyleContext.
// If styleDir is empty, defaults to defaultStyleDir.
// maxChars limits total content size; content is truncated with a warning if exceeded.
func LoadStylePack(styleDir string, maxChars int) *StyleContext {
	if styleDir == "" {
		styleDir = defaultStyleDir
	}
	if maxChars <= 0 {
		maxChars = styleMaxChars
	}

	ctx := &StyleContext{
		StyleDir: styleDir,
	}

	if _, err := os.Stat(styleDir); err != nil {
		// Style dir doesn't exist — silently return disabled
		ctx.Enabled = false
		return ctx
	}

	var parts []string
	for _, name := range styleFiles {
		path := filepath.Join(styleDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", name, text))
		ctx.FilesUsed = append(ctx.FilesUsed, name)
	}

	if len(parts) == 0 {
		ctx.Enabled = false
		return ctx
	}

	ctx.Enabled = true
	full := strings.Join(parts, "\n\n---\n\n")

	if len(full) > maxChars {
		ctx.content = full[:maxChars]
		ctx.Truncated = true
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("style pack content truncated to %d characters (total was %d)", maxChars, len(full)))
	} else {
		ctx.content = full
	}

	return ctx
}

// ---- style file templates ----

func styleFileTemplate(name string) string {
	switch name {
	case "profile.md":
		return `# Creator Profile

## Channel / Creator Name
<!-- Your name, channel, or brand name -->
My Channel

## Audience
<!-- Who watches your content -->
General audience interested in [topic]

## Target Platforms
<!-- YouTube, TikTok, Instagram Reels, etc. -->
- YouTube Shorts
- TikTok

## Personality
<!-- How would you describe your on-camera or scripted voice? -->
Conversational, approachable, slightly educational
`
	case "script_style.md":
		return `# Script Style

## Tone
<!-- e.g. casual, formal, energetic, calm, inspirational -->
Casual and direct

## Pacing
<!-- e.g. fast-cut, slow build, punchy, narrative -->
Fast-paced with clear beats

## Hook Style
<!-- e.g. question hook, bold statement, story open -->
Open with a strong question or bold claim

## Structure
<!-- e.g. hook → context → value → CTA -->
Hook → Problem → Solution → CTA

## Length Target
<!-- Short (30-60s), Medium (1-3min), Long (5min+) -->
Short (under 60 seconds)
`
	case "captions.md":
		return `# Caption Style

## Casing
<!-- e.g. ALL CAPS, Title Case, sentence case -->
ALL CAPS for emphasis words, sentence case otherwise

## Emoji Preference
<!-- None, occasional, frequent -->
Occasional — only for emphasis

## Style Notes
<!-- Additional caption preferences -->
Keep captions short and punchy. Max 5 words per caption line.
`
	case "visual_style.md":
		return `# Visual Style

## Visual Tone
<!-- e.g. cinematic, raw/authentic, branded, minimalist -->
Clean and cinematic

## Pacing
<!-- e.g. quick cuts every 2s, slower deliberate shots -->
Quick cuts every 2-3 seconds

## B-Roll Preference
<!-- e.g. product shots, lifestyle, close-ups -->
Lifestyle and context shots

## Color/Mood
<!-- e.g. warm, cool, high contrast, moody -->
Warm and high-contrast
`
	case "do_not_do.md":
		return `# Do Not Do

## Banned Phrases
<!-- Phrases to avoid in scripts -->
- "In this video..."
- "Don't forget to like and subscribe..."
- "Hey guys..."

## Claims to Avoid
<!-- Things you don't want the model to claim or invent -->
- Do not invent statistics without a source
- Do not make medical or legal claims
- Do not use superlatives without evidence

## Style to Avoid
- Overly formal or academic tone
- Long-winded intros
- Passive voice
`
	case "examples.md":
		return `# Examples

## Good Hooks
<!-- Examples of hooks that worked well -->
- "Most people do this wrong — and don't even know it."
- "What if you could [benefit] in under 60 seconds?"

## Bad Hooks
<!-- Examples of hooks to avoid -->
- "Hi, welcome back to my channel."
- "Today I'm going to show you..."

## Sample Scripts
<!-- Short script examples in your voice -->

### Example 1
Hook: "Most people waste their first 10 minutes of the day."
Body: "Here's what the top 1% do instead..."
CTA: "Try this tomorrow and see what changes."
`
	default:
		return fmt.Sprintf("# %s\n\n<!-- Add your style notes here -->\n", strings.TrimSuffix(name, ".md"))
	}
}

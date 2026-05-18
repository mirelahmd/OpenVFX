package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mirelahmd/OpenVFX/internal/media"
)

// ---- preset definitions ----

type PlatformPreset struct {
	Name       string
	Width      int
	Height     int
	DefaultFit string // "crop" | "pad"
}

var platformPresets = map[string]PlatformPreset{
	"original": {Name: "original", Width: 0, Height: 0, DefaultFit: ""},
	"tiktok":         {Name: "tiktok", Width: 1080, Height: 1920, DefaultFit: "crop"},
	"instagram-reel": {Name: "instagram-reel", Width: 1080, Height: 1920, DefaultFit: "crop"},
	"youtube-short":  {Name: "youtube-short", Width: 1080, Height: 1920, DefaultFit: "crop"},
	"youtube":        {Name: "youtube", Width: 1920, Height: 1080, DefaultFit: "pad"},
	"square":         {Name: "square", Width: 1080, Height: 1080, DefaultFit: "crop"},
}

var platformAliases = map[string]string{
	"reels":    "instagram-reel",
	"reel":     "instagram-reel",
	"shorts":   "youtube-short",
	"yt-short": "youtube-short",
	"yt":       "youtube",
	"ig":       "instagram-reel",
}

// supportedPlatformNames is used in error messages and docs.
var supportedPlatformNames = []string{"original", "tiktok", "instagram-reel", "youtube-short", "youtube", "square"}

// NormalizePlatform resolves aliases and validates the preset name.
func NormalizePlatform(name string) (string, error) {
	if name == "" {
		return "original", nil
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if alias, ok := platformAliases[lower]; ok {
		lower = alias
	}
	if _, ok := platformPresets[lower]; ok {
		return lower, nil
	}
	return "", fmt.Errorf("unknown platform preset %q; supported: %s", name, strings.Join(supportedPlatformNames, ", "))
}

// LookupPlatform returns the PlatformPreset for a normalized preset name.
func LookupPlatform(normalized string) PlatformPreset {
	if p, ok := platformPresets[normalized]; ok {
		return p
	}
	return platformPresets["original"]
}

// DefaultFitForPlatform returns the fit mode to use when none is specified.
func DefaultFitForPlatform(normalized string) string {
	p := LookupPlatform(normalized)
	if p.DefaultFit == "" {
		return "crop"
	}
	return p.DefaultFit
}

// ---- FFmpeg filter construction ----

// buildPlatformArgs returns the FFmpeg arg slice to scale/crop/pad inputFile into outputFile
// at the target dimensions using the given fit mode.
// Both crop and pad modes preserve all audio streams from the input.
func buildPlatformArgs(inputFile, outputFile string, preset PlatformPreset, fit, background string) []string {
	if background == "" {
		background = "black"
	}
	w := preset.Width
	h := preset.Height

	var vf string
	switch fit {
	case "pad":
		// scale to fit inside target, then pad to fill
		vf = fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=%s",
			w, h, w, h, background,
		)
	default: // crop
		// scale to fill target (may overshoot), then center-crop
		vf = fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d",
			w, h, w, h,
		)
	}

	return []string{
		"-y",
		"-i", inputFile,
		"-vf", vf,
		"-c:v", "libx264",
		"-c:a", "copy",
		outputFile,
	}
}

// ---- dimension probing ----

// probeVideoDimensions calls ffprobe on a file and returns the first video stream's width and height.
// Returns (0, 0, nil) if ffprobe is not available.
// Returns an error only when ffprobe is available but the call or parse fails.
func probeVideoDimensions(path string) (width, height int, err error) {
	if _, ferr := media.FindExecutable("ffprobe"); ferr != nil {
		return 0, 0, nil // ffprobe not installed; callers should warn, not fail
	}
	data, probeErr := media.Probe(path)
	if probeErr != nil {
		return 0, 0, fmt.Errorf("ffprobe %s: %w", path, probeErr)
	}
	var result struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if jsonErr := json.Unmarshal(data, &result); jsonErr != nil {
		return 0, 0, fmt.Errorf("parse ffprobe output: %w", jsonErr)
	}
	for _, s := range result.Streams {
		if s.CodecType == "video" && s.Width > 0 && s.Height > 0 {
			return s.Width, s.Height, nil
		}
	}
	return 0, 0, nil
}

// probeFullResult calls ffprobe and returns a fully-populated AssembleFinalProbe.
// Returns nil (no error) when ffprobe is not available.
func probeFullResult(path string) (*AssembleFinalProbe, error) {
	if _, ferr := media.FindExecutable("ffprobe"); ferr != nil {
		return nil, nil
	}
	data, probeErr := media.Probe(path)
	if probeErr != nil {
		return nil, fmt.Errorf("ffprobe: %w", probeErr)
	}
	var raw struct {
		Format  struct{ Duration string } `json:"format"`
		Streams []struct {
			CodecType string  `json:"codec_type"`
			Width     int     `json:"width"`
			Height    int     `json:"height"`
			Duration  float64 `json:"duration"`
		} `json:"streams"`
	}
	if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
		return nil, fmt.Errorf("parse ffprobe: %w", jsonErr)
	}
	probe := &AssembleFinalProbe{}
	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			probe.VideoStreamCount++
			if probe.Width == 0 {
				probe.Width = s.Width
				probe.Height = s.Height
			}
		case "audio":
			probe.AudioStreamCount++
		}
	}
	// duration from format block
	var dur float64
	fmt.Sscanf(raw.Format.Duration, "%f", &dur)
	probe.DurationSeconds = dur
	return probe, nil
}

package commands

import (
	"fmt"
	"strings"
)

// ASS subtitle alignment values (numpad layout: 1=bl, 2=bc, 3=br, 4=ml, 5=mc, 6=mr, 7=tl, 8=tc, 9=tr).
const (
	assAlignBottom = 2
	assAlignCenter = 5
	assAlignTop    = 8
)

var supportedCaptionPositions = []string{"auto", "bottom", "center", "top"}
var supportedCaptionStyles = []string{"default", "bold", "boxed"}

// NormalizeCaptionPosition lowercases and validates the position value.
func NormalizeCaptionPosition(pos string) (string, error) {
	if pos == "" {
		return "auto", nil
	}
	lower := strings.ToLower(strings.TrimSpace(pos))
	for _, s := range supportedCaptionPositions {
		if lower == s {
			return lower, nil
		}
	}
	return "", fmt.Errorf("unknown caption position %q; supported: %s", pos, strings.Join(supportedCaptionPositions, ", "))
}

// NormalizeCaptionStyle lowercases and validates the style value.
func NormalizeCaptionStyle(style string) (string, error) {
	if style == "" {
		return "default", nil
	}
	lower := strings.ToLower(strings.TrimSpace(style))
	for _, s := range supportedCaptionStyles {
		if lower == s {
			return lower, nil
		}
	}
	return "", fmt.Errorf("unknown caption style %q; supported: %s", style, strings.Join(supportedCaptionStyles, ", "))
}

// ResolveCaptionPosition maps "auto" to a concrete position.
// All platforms currently default to "bottom".
func ResolveCaptionPosition(position, _ string) string {
	if position == "auto" || position == "" {
		return "bottom"
	}
	return position
}

// DefaultCaptionMargin returns the default vertical margin (pixels) for the platform.
func DefaultCaptionMargin(normalizedPlatform string) int {
	switch normalizedPlatform {
	case "tiktok", "instagram-reel", "youtube-short":
		return 160
	case "square":
		return 100
	default: // youtube, original, ""
		return 80
	}
}

// buildForceStyleArg returns the ASS force_style string for the given position/margin/style.
// The returned string is suitable for embedding in:
//
//	subtitles=<path>:force_style='<returned value>'
func buildForceStyleArg(position string, margin int, style string) string {
	var parts []string

	align := assAlignBottom
	switch position {
	case "center":
		align = assAlignCenter
	case "top":
		align = assAlignTop
	}
	parts = append(parts, fmt.Sprintf("Alignment=%d", align))
	parts = append(parts, fmt.Sprintf("MarginV=%d", margin))

	switch style {
	case "bold":
		parts = append(parts, "Bold=1")
	case "boxed":
		parts = append(parts, "BorderStyle=3", "Outline=1", "Shadow=0", "BackColour=&H80000000")
	}

	return strings.Join(parts, ",")
}

// buildCaptionFilterString returns the complete -vf value for the subtitles filter.
// escapedSRTPath must already be escaped via escapeFilterPath.
// When forceStyle is empty the plain subtitles=<path> form is used.
func buildCaptionFilterString(escapedSRTPath, forceStyle string) string {
	if forceStyle == "" {
		return "subtitles=" + escapedSRTPath
	}
	return "subtitles=" + escapedSRTPath + ":force_style='" + forceStyle + "'"
}

package commands

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const maxTimelineRepeats = 10

// ParsedTimeInstruction describes a time-based edit extracted from a free-text goal.
type ParsedTimeInstruction struct {
	Start       float64  // source start in seconds, >= 0
	End         float64  // source end in seconds, <= duration
	RepeatCount int      // how many times to include this clip (1 = no repeat)
	Description string   // human-readable summary of what was parsed
	Warnings    []string // non-fatal issues (clamping, etc.)
}

var (
	wholeVideoRe = regexp.MustCompile(`\b(whole|entire|full)\s+video\b`)
	firstSecRe   = regexp.MustCompile(`\bfirst\s+(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)\b`)
	lastSecRe    = regexp.MustCompile(`\blast\s+(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)\b`)

	// "from X to Y" / "use X to Y" / "clip from X to Y" / "use seconds X to Y"
	fromToRe = regexp.MustCompile(
		`\b(?:from|use|clip\s+from|use\s+seconds?)\s+` +
			`(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)?\s+to\s+` +
			`(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)?\b`)

	// "seconds X-Y" or "seconds X–Y"
	secDashRe = regexp.MustCompile(`\bseconds?\s+(\d+(?:\.\d+)?)\s*[-–—]\s*(\d+(?:\.\d+)?)\b`)

	// "loop/repeat last N seconds [count]" — combined phrase
	loopLastRe = regexp.MustCompile(
		`\b(?:loop|repeat)\s+(?:the\s+)?last\s+(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)\b`)

	// repeat count patterns
	nTimesRe     = regexp.MustCompile(`\b(\d+)\s+times\b`)
	twiceRe      = regexp.MustCompile(`\btwice\b`)
	threeTimesRe = regexp.MustCompile(`\bthree\s+times\b`)

	// mm:ss or mm:ss.f
	mmssRe = regexp.MustCompile(`\b(\d{1,2}):(\d{2}(?:\.\d+)?)\b`)
)

// ParseTimeInstruction extracts a time-based edit instruction from a free-text goal.
// duration is the total source video duration in seconds.
// If no explicit instruction is found, it defaults to the whole video (no repeat).
func ParseTimeInstruction(goal string, duration float64) ParsedTimeInstruction {
	lower := strings.ToLower(goal)

	repeatCount, repeatWarn := parseRepeatCount(lower)

	// Combined "loop/repeat last N seconds [M times]" — handled as one phrase.
	if m := loopLastRe.FindStringSubmatch(lower); m != nil {
		n := parseSecondValue(m[1])
		start := clampFloat(duration-n, 0, duration)
		if n > duration {
			start = 0
		}
		count := repeatCount
		if count <= 1 {
			count = 2 // "loop" without explicit count defaults to 2
		}
		count = clampInt(count, 1, maxTimelineRepeats)
		desc := fmt.Sprintf("last %.2f seconds repeated %d time(s)", duration-start, count)
		inst := ParsedTimeInstruction{
			Start: start, End: duration, RepeatCount: count,
			Description: desc,
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// Explicit range: "from X to Y", "use X to Y", etc.
	if m := fromToRe.FindStringSubmatch(lower); m != nil {
		start := parseSecondValue(m[1])
		end := parseSecondValue(m[2])
		start, end, warns := clampRange(start, end, duration)
		inst := ParsedTimeInstruction{
			Start: start, End: end, RepeatCount: repeatCount,
			Description: fmt.Sprintf("%.2f to %.2f seconds", start, end),
			Warnings:    warns,
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// "seconds X-Y"
	if m := secDashRe.FindStringSubmatch(lower); m != nil {
		start := parseSecondValue(m[1])
		end := parseSecondValue(m[2])
		start, end, warns := clampRange(start, end, duration)
		inst := ParsedTimeInstruction{
			Start: start, End: end, RepeatCount: repeatCount,
			Description: fmt.Sprintf("%.2f to %.2f seconds", start, end),
			Warnings:    warns,
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// "whole/entire/full video"
	if wholeVideoRe.MatchString(lower) {
		inst := ParsedTimeInstruction{
			Start: 0, End: duration, RepeatCount: repeatCount,
			Description: "whole video",
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// "first N seconds"
	if m := firstSecRe.FindStringSubmatch(lower); m != nil {
		n := parseSecondValue(m[1])
		end := n
		var warns []string
		if n > duration {
			warns = append(warns, fmt.Sprintf("requested first %.2fs but video is only %.2fs; clamped", n, duration))
			end = duration
		}
		inst := ParsedTimeInstruction{
			Start: 0, End: end, RepeatCount: repeatCount,
			Description: fmt.Sprintf("first %.2f seconds", end),
			Warnings:    warns,
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// "last N seconds"
	if m := lastSecRe.FindStringSubmatch(lower); m != nil {
		n := parseSecondValue(m[1])
		start := duration - n
		var warns []string
		if n > duration {
			warns = append(warns, fmt.Sprintf("requested last %.2fs but video is only %.2fs; using whole video", n, duration))
			start = 0
		}
		start = clampFloat(start, 0, duration)
		inst := ParsedTimeInstruction{
			Start: start, End: duration, RepeatCount: repeatCount,
			Description: fmt.Sprintf("last %.2f seconds", duration-start),
			Warnings:    warns,
		}
		if repeatWarn != "" {
			inst.Warnings = append(inst.Warnings, repeatWarn)
		}
		return inst
	}

	// No time instruction found — default to whole video.
	// Still apply repeat count if "loop"/"repeat" keywords appeared.
	inst := ParsedTimeInstruction{
		Start: 0, End: duration, RepeatCount: repeatCount,
		Description: "whole video (default — no time instruction found)",
	}
	if repeatWarn != "" {
		inst.Warnings = append(inst.Warnings, repeatWarn)
	}
	return inst
}

// parseRepeatCount extracts an explicit or implied repeat count from the goal text.
// Returns (count, warningIfClamped). count >= 1.
func parseRepeatCount(lower string) (int, string) {
	// Explicit N times
	if m := nTimesRe.FindStringSubmatch(lower); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n > maxTimelineRepeats {
			return maxTimelineRepeats, fmt.Sprintf("repeat count %d clamped to %d", n, maxTimelineRepeats)
		}
		if n < 1 {
			return 1, ""
		}
		return n, ""
	}
	// "three times"
	if threeTimesRe.MatchString(lower) {
		return 3, ""
	}
	// "twice"
	if twiceRe.MatchString(lower) {
		return 2, ""
	}
	// "loop" or "repeat" keyword without explicit count — default 2, but only if
	// it doesn't refer to a time range (handled by loopLastRe above this call site).
	// We return 2 here; the caller can override if loopLastRe matched.
	if strings.Contains(lower, "loop") || strings.Contains(lower, "repeat") {
		return 2, ""
	}
	return 1, ""
}

// parseSecondValue converts a string like "1.5", "2", etc. to float64.
// It also handles mm:ss and mm:ss.f formats.
func parseSecondValue(s string) float64 {
	s = strings.TrimSpace(s)
	// Try mm:ss format
	if m := mmssRe.FindStringSubmatch(s); m != nil {
		mins, _ := strconv.ParseFloat(m[1], 64)
		secs, _ := strconv.ParseFloat(m[2], 64)
		return mins*60 + secs
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// clampRange validates and clamps start/end within [0, duration].
func clampRange(start, end, duration float64) (float64, float64, []string) {
	var warns []string
	if start < 0 {
		warns = append(warns, fmt.Sprintf("start %.2f clamped to 0", start))
		start = 0
	}
	if end > duration {
		warns = append(warns, fmt.Sprintf("end %.2f clamped to duration %.2f", end, duration))
		end = duration
	}
	if end <= start {
		warns = append(warns, fmt.Sprintf("end %.2f <= start %.2f; falling back to whole video", end, start))
		start = 0
		end = duration
	}
	return start, end, warns
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

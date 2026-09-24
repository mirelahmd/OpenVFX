package commands

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/media"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
)

const (
	timelineSourceSchema   = "openvfx_timeline_source.v1"
	timelineSourceArtifact = "timeline_source.json"
)

// TimelineSourceClip is one clip entry in timeline_source.json.
// start/end are aliases for source_start/source_end so readClipsFromArtifact can consume them.
type TimelineSourceClip struct {
	ID              string  `json:"id"`
	SourcePath      string  `json:"source_path"`
	SourceStart     float64 `json:"source_start"`
	SourceEnd       float64 `json:"source_end"`
	Start           float64 `json:"start"`            // alias — same as SourceStart
	End             float64 `json:"end"`              // alias — same as SourceEnd
	DurationSeconds float64 `json:"duration_seconds"`
	RepeatIndex     int     `json:"repeat_index"`
	Description     string  `json:"description"`
}

// TimelineSourceArtifact is the timeline_source.json artifact.
type TimelineSourceArtifact struct {
	SchemaVersion   string               `json:"schema_version"`
	CreatedAt       time.Time            `json:"created_at"`
	Source          string               `json:"source"`           // always "synthetic_time_based"
	Reason          string               `json:"reason"`
	InputPath       string               `json:"input_path"`
	DurationSeconds float64              `json:"duration_seconds"`
	Clips           []TimelineSourceClip `json:"clips"`
	Warnings        []string             `json:"warnings,omitempty"`
}

// BuildSyntheticTimelineSource builds a timeline_source.json artifact from a source video path,
// its known duration, and a free-text goal. It does not mutate source media.
func BuildSyntheticTimelineSource(inputPath string, duration float64, goal string) TimelineSourceArtifact {
	inst := ParseTimeInstruction(goal, duration)

	clips := make([]TimelineSourceClip, 0, inst.RepeatCount)
	clipDur := inst.End - inst.Start
	for i := 0; i < inst.RepeatCount; i++ {
		clips = append(clips, TimelineSourceClip{
			ID:              fmt.Sprintf("fallback_clip_%04d", i+1),
			SourcePath:      inputPath,
			SourceStart:     inst.Start,
			SourceEnd:       inst.End,
			Start:           inst.Start,
			End:             inst.End,
			DurationSeconds: clipDur,
			RepeatIndex:     i + 1,
			Description:     fmt.Sprintf("%s (repeat %d/%d)", inst.Description, i+1, inst.RepeatCount),
		})
	}

	reason := fmt.Sprintf(
		"no selected_clips/roughcut artifacts found; generated fallback from source duration (%.2fs) and prompt",
		duration)

	return TimelineSourceArtifact{
		SchemaVersion:   timelineSourceSchema,
		CreatedAt:       time.Now().UTC(),
		Source:          "synthetic_time_based",
		Reason:          reason,
		InputPath:       inputPath,
		DurationSeconds: duration,
		Clips:           clips,
		Warnings:        inst.Warnings,
	}
}

// WriteTimelineSourceToRun writes timeline_source.json into the given pipeline run directory.
func WriteTimelineSourceToRun(runID string, tls TimelineSourceArtifact) (string, error) {
	runDir, err := runstore.ResolveRunDir(runID)
	if err != nil {
		return "", fmt.Errorf("resolve run dir: %w", err)
	}
	path := filepath.Join(runDir, timelineSourceArtifact)
	if err := writeJSONFile(path, tls); err != nil {
		return "", fmt.Errorf("write timeline_source.json: %w", err)
	}
	return path, nil
}

// probeMediaDuration uses ffprobe to get the duration of a video file in seconds.
// Returns an error if ffprobe is unavailable or the file cannot be probed.
func probeMediaDuration(inputPath string) (float64, error) {
	if _, err := media.FindExecutable("ffprobe"); err != nil {
		return 0, fmt.Errorf("cannot build synthetic timeline source without ffprobe duration")
	}
	data, err := media.Probe(inputPath)
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	var raw struct {
		Format struct{ Duration string } `json:"format"`
	}
	if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
		return 0, fmt.Errorf("parse ffprobe output: %w", jsonErr)
	}
	var dur float64
	fmt.Sscanf(raw.Format.Duration, "%f", &dur)
	if dur <= 0 {
		return 0, fmt.Errorf("ffprobe returned zero or invalid duration for %q", inputPath)
	}
	return dur, nil
}

// SyntheticTimelineFallbackMessage formats the user-facing warning string shown when
// synthetic timeline fallback is triggered.
func SyntheticTimelineFallbackMessage(tls TimelineSourceArtifact) string {
	if len(tls.Clips) == 0 {
		return "No speech-driven roughcut clips found. Using synthetic time-based timeline source (whole video)."
	}
	c := tls.Clips[0]
	dur := c.DurationSeconds * float64(len(tls.Clips))
	return fmt.Sprintf(
		"No speech-driven roughcut clips found. Using synthetic time-based timeline source: %s (total %.2fs).",
		c.Description, dur)
}

package captions

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Summary struct {
	ArtifactPath string
	CueCount     int
}

type transcriptDocument struct {
	Segments []segment `json:"segments"`
}

type segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func WriteFromTranscript(transcriptPath string, outputPath string) (Summary, error) {
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		return Summary{}, fmt.Errorf("read transcript for captions: %w", err)
	}
	var transcript transcriptDocument
	if err := json.Unmarshal(data, &transcript); err != nil {
		return Summary{}, fmt.Errorf("decode transcript for captions: %w", err)
	}

	var builder strings.Builder
	cueCount := 0
	for _, segment := range transcript.Segments {
		if strings.TrimSpace(segment.Text) == "" {
			continue
		}
		cueCount++
		fmt.Fprintf(&builder, "%d\n", cueCount)
		fmt.Fprintf(&builder, "%s --> %s\n", FormatSRTTimestamp(segment.Start), FormatSRTTimestamp(segment.End))
		fmt.Fprintf(&builder, "%s\n\n", strings.TrimSpace(segment.Text))
	}
	if err := os.WriteFile(outputPath, []byte(builder.String()), 0o644); err != nil {
		return Summary{}, fmt.Errorf("write captions: %w", err)
	}
	return Summary{ArtifactPath: "captions.srt", CueCount: cueCount}, nil
}

func FormatSRTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	duration := time.Duration(seconds * float64(time.Second))
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	secs := int(duration / time.Second)
	duration -= time.Duration(secs) * time.Second
	millis := int(duration / time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

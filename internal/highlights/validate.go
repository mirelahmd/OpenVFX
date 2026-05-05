package highlights

import (
	"encoding/json"
	"fmt"
	"os"
)

type validateDocument struct {
	SchemaVersion *string              `json:"schema_version"`
	Highlights    *[]validateHighlight `json:"highlights"`
}

type validateHighlight struct {
	ID              *string         `json:"id"`
	ChunkID         *string         `json:"chunk_id"`
	Start           *float64        `json:"start"`
	End             *float64        `json:"end"`
	DurationSeconds *float64        `json:"duration_seconds"`
	Score           *float64        `json:"score"`
	Label           *string         `json:"label"`
	Reason          *string         `json:"reason"`
	Text            *string         `json:"text"`
	Signals         *map[string]any `json:"signals"`
}

func ValidateFile(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read highlights: %w", err)
	}
	var doc validateDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Summary{}, fmt.Errorf("decode highlights JSON: %w", err)
	}
	if doc.SchemaVersion == nil || *doc.SchemaVersion != "highlights.v1" {
		return Summary{}, fmt.Errorf("highlights schema_version must be highlights.v1")
	}
	if doc.Highlights == nil {
		return Summary{}, fmt.Errorf("highlights array is required")
	}
	summary := Summary{ArtifactPath: "highlights.json", Count: len(*doc.Highlights)}
	for index, highlight := range *doc.Highlights {
		if highlight.ID == nil || *highlight.ID == "" {
			return Summary{}, fmt.Errorf("highlight %d id is required", index)
		}
		if highlight.ChunkID == nil || *highlight.ChunkID == "" || highlight.Start == nil || highlight.End == nil || highlight.DurationSeconds == nil || highlight.Score == nil || highlight.Label == nil || highlight.Reason == nil || highlight.Text == nil || highlight.Signals == nil {
			return Summary{}, fmt.Errorf("highlight %s is missing required fields", *highlight.ID)
		}
		if *highlight.End < *highlight.Start {
			return Summary{}, fmt.Errorf("highlight %s end must be greater than or equal to start", *highlight.ID)
		}
		if *highlight.DurationSeconds < 0 {
			return Summary{}, fmt.Errorf("highlight %s duration_seconds must be non-negative", *highlight.ID)
		}
		if *highlight.Score < 0 || *highlight.Score > 1 {
			return Summary{}, fmt.Errorf("highlight %s score must be between 0 and 1", *highlight.ID)
		}
		if index == 0 {
			score := *highlight.Score
			start := *highlight.Start
			end := *highlight.End
			summary.TopScore = &score
			summary.TopStart = &start
			summary.TopEnd = &end
		}
	}
	return summary, nil
}

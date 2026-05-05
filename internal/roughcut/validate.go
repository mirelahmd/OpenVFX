package roughcut

import (
	"encoding/json"
	"fmt"
	"os"
)

type validateDocument struct {
	SchemaVersion *string         `json:"schema_version"`
	Plan          *Plan           `json:"plan"`
	Clips         *[]validateClip `json:"clips"`
}

type validateClip struct {
	ID              *string  `json:"id"`
	HighlightID     *string  `json:"highlight_id"`
	SourceChunkID   *string  `json:"source_chunk_id"`
	Start           *float64 `json:"start"`
	End             *float64 `json:"end"`
	DurationSeconds *float64 `json:"duration_seconds"`
	Order           *int     `json:"order"`
	Score           *float64 `json:"score"`
	EditIntent      *string  `json:"edit_intent"`
	Text            *string  `json:"text"`
}

func ValidateFile(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read roughcut: %w", err)
	}
	var doc validateDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Summary{}, fmt.Errorf("decode roughcut JSON: %w", err)
	}
	if doc.SchemaVersion == nil || *doc.SchemaVersion != "roughcut.v1" {
		return Summary{}, fmt.Errorf("roughcut schema_version must be roughcut.v1")
	}
	if doc.Clips == nil {
		return Summary{}, fmt.Errorf("roughcut clips array is required")
	}
	summary := Summary{ArtifactPath: "roughcut.json", ClipCount: len(*doc.Clips)}
	if doc.Plan != nil {
		summary.TotalDurationSeconds = doc.Plan.TotalDurationSeconds
	}
	for index, clip := range *doc.Clips {
		if clip.ID == nil || *clip.ID == "" {
			return Summary{}, fmt.Errorf("clip %d id is required", index)
		}
		if clip.HighlightID == nil || clip.SourceChunkID == nil || clip.Start == nil || clip.End == nil || clip.DurationSeconds == nil || clip.Order == nil || clip.Score == nil || clip.EditIntent == nil || clip.Text == nil {
			return Summary{}, fmt.Errorf("clip %s is missing required fields", *clip.ID)
		}
		if *clip.End < *clip.Start {
			return Summary{}, fmt.Errorf("clip %s end must be greater than or equal to start", *clip.ID)
		}
		if *clip.DurationSeconds < 0 {
			return Summary{}, fmt.Errorf("clip %s duration_seconds must be non-negative", *clip.ID)
		}
		if *clip.Order < 1 {
			return Summary{}, fmt.Errorf("clip %s order must be >= 1", *clip.ID)
		}
		if *clip.Score < 0 || *clip.Score > 1 {
			return Summary{}, fmt.Errorf("clip %s score must be between 0 and 1", *clip.ID)
		}
	}
	return summary, nil
}

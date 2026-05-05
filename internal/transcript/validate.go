package transcript

import (
	"encoding/json"
	"fmt"
	"os"
)

type Summary struct {
	ArtifactPath    string
	Language        string
	SegmentCount    int
	DurationSeconds *float64
	ModelSize       string
}

type document struct {
	SchemaVersion *string `json:"schema_version"`
	Source        *struct {
		Mode      *string `json:"mode"`
		ModelSize *string `json:"model_size"`
	} `json:"source"`
	Language        string     `json:"language"`
	DurationSeconds *float64   `json:"duration_seconds"`
	Segments        *[]segment `json:"segments"`
}

type segment struct {
	ID    *string  `json:"id"`
	Start *float64 `json:"start"`
	End   *float64 `json:"end"`
	Text  *string  `json:"text"`
}

func ValidateFile(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read transcript: %w", err)
	}

	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Summary{}, fmt.Errorf("decode transcript JSON: %w", err)
	}

	if doc.SchemaVersion == nil || *doc.SchemaVersion != "transcript.v1" {
		return Summary{}, fmt.Errorf("transcript schema_version must be transcript.v1")
	}
	if doc.Source == nil {
		return Summary{}, fmt.Errorf("transcript source is required")
	}
	if doc.Source.Mode == nil || *doc.Source.Mode == "" {
		return Summary{}, fmt.Errorf("transcript source.mode is required")
	}
	if doc.Segments == nil {
		return Summary{}, fmt.Errorf("transcript segments array is required")
	}

	for index, segment := range *doc.Segments {
		if segment.ID == nil || *segment.ID == "" {
			return Summary{}, fmt.Errorf("transcript segment %d id is required", index)
		}
		if segment.Start == nil {
			return Summary{}, fmt.Errorf("transcript segment %s start is required", *segment.ID)
		}
		if segment.End == nil {
			return Summary{}, fmt.Errorf("transcript segment %s end is required", *segment.ID)
		}
		if segment.Text == nil {
			return Summary{}, fmt.Errorf("transcript segment %s text is required", *segment.ID)
		}
		if *segment.End < *segment.Start {
			return Summary{}, fmt.Errorf("transcript segment %s end must be greater than or equal to start", *segment.ID)
		}
	}

	summary := Summary{
		ArtifactPath:    "transcript.json",
		Language:        doc.Language,
		SegmentCount:    len(*doc.Segments),
		DurationSeconds: doc.DurationSeconds,
	}
	if doc.Source.ModelSize != nil {
		summary.ModelSize = *doc.Source.ModelSize
	}
	if summary.Language == "" {
		summary.Language = "unknown"
	}
	return summary, nil
}

package chunks

import (
	"encoding/json"
	"fmt"
	"os"
)

type validateDocument struct {
	SchemaVersion *string          `json:"schema_version"`
	Chunks        *[]validateChunk `json:"chunks"`
}

type validateChunk struct {
	ID              *string   `json:"id"`
	Start           *float64  `json:"start"`
	End             *float64  `json:"end"`
	DurationSeconds *float64  `json:"duration_seconds"`
	Text            *string   `json:"text"`
	SegmentIDs      *[]string `json:"segment_ids"`
	WordCount       *int      `json:"word_count"`
}

func ValidateFile(path string) (Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read chunks: %w", err)
	}

	var doc validateDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Summary{}, fmt.Errorf("decode chunks JSON: %w", err)
	}
	if doc.SchemaVersion == nil || *doc.SchemaVersion != "chunks.v1" {
		return Summary{}, fmt.Errorf("chunks schema_version must be chunks.v1")
	}
	if doc.Chunks == nil {
		return Summary{}, fmt.Errorf("chunks array is required")
	}
	for index, chunk := range *doc.Chunks {
		if chunk.ID == nil || *chunk.ID == "" {
			return Summary{}, fmt.Errorf("chunk %d id is required", index)
		}
		if chunk.Start == nil {
			return Summary{}, fmt.Errorf("chunk %s start is required", *chunk.ID)
		}
		if chunk.End == nil {
			return Summary{}, fmt.Errorf("chunk %s end is required", *chunk.ID)
		}
		if chunk.DurationSeconds == nil {
			return Summary{}, fmt.Errorf("chunk %s duration_seconds is required", *chunk.ID)
		}
		if chunk.Text == nil {
			return Summary{}, fmt.Errorf("chunk %s text is required", *chunk.ID)
		}
		if chunk.SegmentIDs == nil {
			return Summary{}, fmt.Errorf("chunk %s segment_ids is required", *chunk.ID)
		}
		if chunk.WordCount == nil {
			return Summary{}, fmt.Errorf("chunk %s word_count is required", *chunk.ID)
		}
		if *chunk.End < *chunk.Start {
			return Summary{}, fmt.Errorf("chunk %s end must be greater than or equal to start", *chunk.ID)
		}
		if *chunk.DurationSeconds < 0 {
			return Summary{}, fmt.Errorf("chunk %s duration_seconds must be non-negative", *chunk.ID)
		}
		if *chunk.WordCount < 0 {
			return Summary{}, fmt.Errorf("chunk %s word_count must be non-negative", *chunk.ID)
		}
	}
	return Summary{ArtifactPath: "chunks.json", ChunkCount: len(*doc.Chunks)}, nil
}

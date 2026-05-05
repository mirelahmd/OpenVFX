package roughcut

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type Options struct {
	MaxClips int
}

type Summary struct {
	ArtifactPath         string
	ClipCount            int
	TotalDurationSeconds float64
}

type highlightsDocument struct {
	Highlights []highlight `json:"highlights"`
}

type highlight struct {
	ID              string  `json:"id"`
	ChunkID         string  `json:"chunk_id"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	Score           float64 `json:"score"`
	Text            string  `json:"text"`
}

type Document struct {
	SchemaVersion string `json:"schema_version"`
	Source        Source `json:"source"`
	Plan          Plan   `json:"plan"`
	Clips         []Clip `json:"clips"`
}

type Source struct {
	HighlightsArtifact string `json:"highlights_artifact"`
	Mode               string `json:"mode"`
	Strategy           string `json:"strategy"`
}

type Plan struct {
	Title                string  `json:"title"`
	Intent               string  `json:"intent"`
	TotalDurationSeconds float64 `json:"total_duration_seconds"`
}

type Clip struct {
	ID              string  `json:"id"`
	HighlightID     string  `json:"highlight_id"`
	SourceChunkID   string  `json:"source_chunk_id"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	Order           int     `json:"order"`
	Score           float64 `json:"score"`
	EditIntent      string  `json:"edit_intent"`
	Text            string  `json:"text"`
}

func WriteFromHighlights(highlightsPath string, outputPath string, opts Options) (Summary, error) {
	data, err := os.ReadFile(highlightsPath)
	if err != nil {
		return Summary{}, fmt.Errorf("read highlights for roughcut: %w", err)
	}
	var highlights highlightsDocument
	if err := json.Unmarshal(data, &highlights); err != nil {
		return Summary{}, fmt.Errorf("decode highlights for roughcut: %w", err)
	}
	selected := append([]highlight(nil), highlights.Highlights...)
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Score > selected[j].Score
	})
	if len(selected) > opts.MaxClips {
		selected = selected[:opts.MaxClips]
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Start < selected[j].Start
	})
	clips := make([]Clip, 0, len(selected))
	total := 0.0
	for index, highlight := range selected {
		total += highlight.DurationSeconds
		clips = append(clips, Clip{
			ID:              fmt.Sprintf("clip_%04d", index+1),
			HighlightID:     highlight.ID,
			SourceChunkID:   highlight.ChunkID,
			Start:           highlight.Start,
			End:             highlight.End,
			DurationSeconds: highlight.DurationSeconds,
			Order:           index + 1,
			Score:           highlight.Score,
			EditIntent:      "Keep this segment as a candidate short clip.",
			Text:            highlight.Text,
		})
	}
	doc := Document{
		SchemaVersion: "roughcut.v1",
		Source: Source{
			HighlightsArtifact: "highlights.json",
			Mode:               "deterministic",
			Strategy:           "top_highlights_v1",
		},
		Plan: Plan{
			Title:                "Rough Cut Plan",
			Intent:               "Select strongest highlight candidates in timeline order.",
			TotalDurationSeconds: total,
		},
		Clips: clips,
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Summary{}, fmt.Errorf("encode roughcut: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		return Summary{}, fmt.Errorf("write roughcut: %w", err)
	}
	return Summary{ArtifactPath: "roughcut.json", ClipCount: len(clips), TotalDurationSeconds: total}, nil
}

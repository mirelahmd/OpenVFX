package goalartifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type GoalConstraints struct {
	MaxTotalDurationSeconds float64 `json:"max_total_duration_seconds"`
	MaxClips                int     `json:"max_clips"`
	PreferredStyle          string  `json:"preferred_style"`
}

type GoalRerankSource struct {
	HighlightsArtifact string `json:"highlights_artifact"`
	ChunksArtifact     string `json:"chunks_artifact,omitempty"`
}

type RankedHighlight struct {
	HighlightID     string  `json:"highlight_id"`
	ChunkID         string  `json:"chunk_id,omitempty"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	OriginalScore   float64 `json:"original_score"`
	GoalScore       float64 `json:"goal_score"`
	Rank            int     `json:"rank"`
	Reason          string  `json:"reason"`
	Text            string  `json:"text"`
}

type GoalRerank struct {
	SchemaVersion    string            `json:"schema_version"`
	CreatedAt        time.Time         `json:"created_at"`
	RunID            string            `json:"run_id"`
	Goal             string            `json:"goal"`
	Mode             string            `json:"mode"`
	Source           GoalRerankSource  `json:"source"`
	Constraints      GoalConstraints   `json:"constraints"`
	RankedHighlights []RankedHighlight `json:"ranked_highlights"`
	Warnings         []string          `json:"warnings,omitempty"`
}

type GoalRoughcutSource struct {
	GoalRerankArtifact string `json:"goal_rerank_artifact"`
	RoughcutArtifact   string `json:"roughcut_artifact,omitempty"`
}

type GoalRoughcutPlan struct {
	Title                string  `json:"title"`
	Intent               string  `json:"intent"`
	TotalDurationSeconds float64 `json:"total_duration_seconds"`
}

type GoalRoughcutClip struct {
	ID              string  `json:"id"`
	HighlightID     string  `json:"highlight_id"`
	ChunkID         string  `json:"chunk_id,omitempty"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	Order           int     `json:"order"`
	GoalScore       float64 `json:"goal_score"`
	Reason          string  `json:"reason"`
	Text            string  `json:"text"`
}

type GoalRoughcut struct {
	SchemaVersion string             `json:"schema_version"`
	CreatedAt     time.Time          `json:"created_at"`
	RunID         string             `json:"run_id"`
	Goal          string             `json:"goal"`
	Source        GoalRoughcutSource `json:"source"`
	Plan          GoalRoughcutPlan   `json:"plan"`
	Clips         []GoalRoughcutClip `json:"clips"`
}

func ReadGoalRerank(path string) (GoalRerank, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoalRerank{}, fmt.Errorf("read goal rerank: %w", err)
	}
	var doc GoalRerank
	if err := json.Unmarshal(data, &doc); err != nil {
		return GoalRerank{}, fmt.Errorf("decode goal rerank: %w", err)
	}
	return doc, nil
}

func ReadGoalRoughcut(path string) (GoalRoughcut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoalRoughcut{}, fmt.Errorf("read goal roughcut: %w", err)
	}
	var doc GoalRoughcut
	if err := json.Unmarshal(data, &doc); err != nil {
		return GoalRoughcut{}, fmt.Errorf("decode goal roughcut: %w", err)
	}
	return doc, nil
}

func ValidateGoalRerankFile(path string) (GoalRerank, error) {
	doc, err := ReadGoalRerank(path)
	if err != nil {
		return GoalRerank{}, err
	}
	if doc.SchemaVersion != "goal_rerank.v1" {
		return GoalRerank{}, fmt.Errorf("schema_version must be goal_rerank.v1")
	}
	if doc.CreatedAt.IsZero() {
		return GoalRerank{}, fmt.Errorf("created_at is required")
	}
	if doc.Goal == "" {
		return GoalRerank{}, fmt.Errorf("goal is required")
	}
	if doc.Mode != "deterministic" && doc.Mode != "ollama" {
		return GoalRerank{}, fmt.Errorf("mode must be deterministic or ollama")
	}
	if doc.Source.HighlightsArtifact == "" {
		return GoalRerank{}, fmt.Errorf("source.highlights_artifact is required")
	}
	if doc.Constraints.MaxTotalDurationSeconds <= 0 {
		return GoalRerank{}, fmt.Errorf("constraints.max_total_duration_seconds must be positive")
	}
	if doc.Constraints.MaxClips <= 0 {
		return GoalRerank{}, fmt.Errorf("constraints.max_clips must be positive")
	}
	if doc.Constraints.PreferredStyle == "" {
		return GoalRerank{}, fmt.Errorf("constraints.preferred_style is required")
	}
	if doc.RankedHighlights == nil {
		return GoalRerank{}, fmt.Errorf("ranked_highlights array is required")
	}
	for i, item := range doc.RankedHighlights {
		if item.HighlightID == "" {
			return GoalRerank{}, fmt.Errorf("ranked_highlights[%d].highlight_id is required", i)
		}
		if item.Reason == "" {
			return GoalRerank{}, fmt.Errorf("ranked_highlights[%d].reason is required", i)
		}
		if item.Text == "" {
			return GoalRerank{}, fmt.Errorf("ranked_highlights[%d].text is required", i)
		}
		if item.End < item.Start {
			return GoalRerank{}, fmt.Errorf("ranked_highlights[%d].end must be greater than or equal to start", i)
		}
		if item.Rank <= 0 {
			return GoalRerank{}, fmt.Errorf("ranked_highlights[%d].rank must be positive", i)
		}
	}
	return doc, nil
}

func ValidateGoalRoughcutFile(path string) (GoalRoughcut, error) {
	doc, err := ReadGoalRoughcut(path)
	if err != nil {
		return GoalRoughcut{}, err
	}
	if doc.SchemaVersion != "goal_roughcut.v1" {
		return GoalRoughcut{}, fmt.Errorf("schema_version must be goal_roughcut.v1")
	}
	if doc.CreatedAt.IsZero() {
		return GoalRoughcut{}, fmt.Errorf("created_at is required")
	}
	if doc.Goal == "" {
		return GoalRoughcut{}, fmt.Errorf("goal is required")
	}
	if doc.Source.GoalRerankArtifact == "" {
		return GoalRoughcut{}, fmt.Errorf("source.goal_rerank_artifact is required")
	}
	if doc.Plan.Title == "" {
		return GoalRoughcut{}, fmt.Errorf("plan.title is required")
	}
	if doc.Clips == nil {
		return GoalRoughcut{}, fmt.Errorf("clips array is required")
	}
	for i, clip := range doc.Clips {
		if clip.ID == "" {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].id is required", i)
		}
		if clip.HighlightID == "" {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].highlight_id is required", i)
		}
		if clip.Reason == "" {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].reason is required", i)
		}
		if clip.Text == "" {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].text is required", i)
		}
		if clip.End < clip.Start {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].end must be greater than or equal to start", i)
		}
		if clip.Order <= 0 {
			return GoalRoughcut{}, fmt.Errorf("clips[%d].order must be positive", i)
		}
	}
	return doc, nil
}

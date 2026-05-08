package editorartifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ClipCards struct {
	SchemaVersion string          `json:"schema_version"`
	CreatedAt     time.Time       `json:"created_at"`
	RunID         string          `json:"run_id"`
	Source        ClipCardsSource `json:"source"`
	Cards         []ClipCard      `json:"cards"`
}

type ClipCardsSource struct {
	RoughcutArtifact      string `json:"roughcut_artifact"`
	GoalRoughcutArtifact  string `json:"goal_roughcut_artifact,omitempty"`
	InferenceMaskArtifact string `json:"inference_mask_artifact,omitempty"`
	ExpansionsDir         string `json:"expansions_dir,omitempty"`
}

type ClipCard struct {
	ID                 string   `json:"id"`
	ClipID             string   `json:"clip_id"`
	HighlightID        string   `json:"highlight_id,omitempty"`
	DecisionID         string   `json:"decision_id,omitempty"`
	Start              float64  `json:"start"`
	End                float64  `json:"end"`
	DurationSeconds    float64  `json:"duration_seconds"`
	Score              float64  `json:"score,omitempty"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Captions           []string `json:"captions,omitempty"`
	SourceText         string   `json:"source_text,omitempty"`
	EditIntent         string   `json:"edit_intent,omitempty"`
	VerificationStatus string   `json:"verification_status"`
	Warnings           []string `json:"warnings,omitempty"`
}

type EnhancedRoughcut struct {
	SchemaVersion string                 `json:"schema_version"`
	CreatedAt     time.Time              `json:"created_at"`
	RunID         string                 `json:"run_id"`
	Source        EnhancedRoughcutSource `json:"source"`
	Plan          EnhancedRoughcutPlan   `json:"plan"`
	Clips         []EnhancedRoughcutClip `json:"clips"`
}

type EnhancedRoughcutSource struct {
	RoughcutArtifact  string `json:"roughcut_artifact"`
	ClipCardsArtifact string `json:"clip_cards_artifact,omitempty"`
}

type EnhancedRoughcutPlan struct {
	Title                string  `json:"title"`
	Intent               string  `json:"intent,omitempty"`
	TotalDurationSeconds float64 `json:"total_duration_seconds"`
}

type EnhancedRoughcutClip struct {
	ID                 string   `json:"id"`
	Start              float64  `json:"start"`
	End                float64  `json:"end"`
	Order              int      `json:"order"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	CaptionSuggestions []string `json:"caption_suggestions,omitempty"`
	EditIntent         string   `json:"edit_intent,omitempty"`
	VerificationStatus string   `json:"verification_status"`
	SourceText         string   `json:"source_text,omitempty"`
}

func ReadClipCards(path string) (ClipCards, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ClipCards{}, fmt.Errorf("read clip cards: %w", err)
	}
	var doc ClipCards
	if err := json.Unmarshal(data, &doc); err != nil {
		return ClipCards{}, fmt.Errorf("decode clip cards: %w", err)
	}
	return doc, nil
}

func ReadEnhancedRoughcut(path string) (EnhancedRoughcut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EnhancedRoughcut{}, fmt.Errorf("read enhanced roughcut: %w", err)
	}
	var doc EnhancedRoughcut
	if err := json.Unmarshal(data, &doc); err != nil {
		return EnhancedRoughcut{}, fmt.Errorf("decode enhanced roughcut: %w", err)
	}
	return doc, nil
}

func ValidateClipCardsFile(path string) (ClipCards, error) {
	doc, err := ReadClipCards(path)
	if err != nil {
		return ClipCards{}, err
	}
	if doc.SchemaVersion != "clip_cards.v1" {
		return ClipCards{}, fmt.Errorf("schema_version must be clip_cards.v1")
	}
	if doc.CreatedAt.IsZero() {
		return ClipCards{}, fmt.Errorf("created_at is required")
	}
	if doc.Cards == nil {
		return ClipCards{}, fmt.Errorf("cards array is required")
	}
	for i, card := range doc.Cards {
		if card.ID == "" {
			return ClipCards{}, fmt.Errorf("cards[%d].id is required", i)
		}
		if card.ClipID == "" {
			return ClipCards{}, fmt.Errorf("cards[%d].clip_id is required", i)
		}
		if card.Title == "" {
			return ClipCards{}, fmt.Errorf("cards[%d].title is required", i)
		}
		if card.Description == "" {
			return ClipCards{}, fmt.Errorf("cards[%d].description is required", i)
		}
		if card.VerificationStatus == "" {
			return ClipCards{}, fmt.Errorf("cards[%d].verification_status is required", i)
		}
		if card.End < card.Start {
			return ClipCards{}, fmt.Errorf("cards[%d].end must be greater than or equal to start", i)
		}
		if card.Captions == nil {
			continue
		}
		for j, caption := range card.Captions {
			if caption == "" {
				return ClipCards{}, fmt.Errorf("cards[%d].captions[%d] is required", i, j)
			}
		}
	}
	return doc, nil
}

func ValidateEnhancedRoughcutFile(path string) (EnhancedRoughcut, error) {
	doc, err := ReadEnhancedRoughcut(path)
	if err != nil {
		return EnhancedRoughcut{}, err
	}
	if doc.SchemaVersion != "enhanced_roughcut.v1" {
		return EnhancedRoughcut{}, fmt.Errorf("schema_version must be enhanced_roughcut.v1")
	}
	if doc.CreatedAt.IsZero() {
		return EnhancedRoughcut{}, fmt.Errorf("created_at is required")
	}
	if doc.Plan.Title == "" {
		return EnhancedRoughcut{}, fmt.Errorf("plan.title is required")
	}
	if doc.Clips == nil {
		return EnhancedRoughcut{}, fmt.Errorf("clips array is required")
	}
	for i, clip := range doc.Clips {
		if clip.ID == "" {
			return EnhancedRoughcut{}, fmt.Errorf("clips[%d].id is required", i)
		}
		if clip.Title == "" {
			return EnhancedRoughcut{}, fmt.Errorf("clips[%d].title is required", i)
		}
		if clip.Description == "" {
			return EnhancedRoughcut{}, fmt.Errorf("clips[%d].description is required", i)
		}
		if clip.VerificationStatus == "" {
			return EnhancedRoughcut{}, fmt.Errorf("clips[%d].verification_status is required", i)
		}
		if clip.End < clip.Start {
			return EnhancedRoughcut{}, fmt.Errorf("clips[%d].end must be greater than or equal to start", i)
		}
		for j, caption := range clip.CaptionSuggestions {
			if caption == "" {
				return EnhancedRoughcut{}, fmt.Errorf("clips[%d].caption_suggestions[%d] is required", i, j)
			}
		}
	}
	return doc, nil
}

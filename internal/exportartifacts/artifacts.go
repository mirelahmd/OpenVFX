package exportartifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type SelectedClips struct {
	SchemaVersion string              `json:"schema_version"`
	CreatedAt     time.Time           `json:"created_at"`
	RunID         string              `json:"run_id"`
	Source        SelectedClipsSource `json:"source"`
	InputPath     string              `json:"input_path"`
	Clips         []SelectedClip      `json:"clips"`
}

type SelectedClipsSource struct {
	GoalRoughcutArtifact     string `json:"goal_roughcut_artifact,omitempty"`
	EnhancedRoughcutArtifact string `json:"enhanced_roughcut_artifact,omitempty"`
	ClipCardsArtifact        string `json:"clip_cards_artifact,omitempty"`
	RoughcutArtifact         string `json:"roughcut_artifact,omitempty"`
}

type SelectedClip struct {
	ID                 string   `json:"id"`
	Order              int      `json:"order"`
	Start              float64  `json:"start"`
	End                float64  `json:"end"`
	DurationSeconds    float64  `json:"duration_seconds"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	CaptionSuggestions []string `json:"caption_suggestions,omitempty"`
	SourceText         string   `json:"source_text,omitempty"`
	OutputFilename     string   `json:"output_filename"`
}

type ExportManifest struct {
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	RunID         string                `json:"run_id"`
	InputPath     string                `json:"input_path"`
	ExportsDir    string                `json:"exports_dir"`
	Clips         []ExportManifestClip  `json:"clips"`
	Summary       ExportManifestSummary `json:"summary"`
}

type ExportManifestClip struct {
	ID              string  `json:"id"`
	Order           int     `json:"order"`
	PlannedOutput   string  `json:"planned_output"`
	Exists          bool    `json:"exists"`
	Validated       bool    `json:"validated"`
	DurationSeconds float64 `json:"duration_seconds"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
}

type ExportManifestSummary struct {
	Planned   int `json:"planned"`
	Exported  int `json:"exported"`
	Validated int `json:"validated"`
	Missing   int `json:"missing"`
}

func ReadSelectedClips(path string) (SelectedClips, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SelectedClips{}, fmt.Errorf("read selected clips: %w", err)
	}
	var doc SelectedClips
	if err := json.Unmarshal(data, &doc); err != nil {
		return SelectedClips{}, fmt.Errorf("decode selected clips: %w", err)
	}
	return doc, nil
}

func ReadExportManifest(path string) (ExportManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExportManifest{}, fmt.Errorf("read export manifest: %w", err)
	}
	var doc ExportManifest
	if err := json.Unmarshal(data, &doc); err != nil {
		return ExportManifest{}, fmt.Errorf("decode export manifest: %w", err)
	}
	return doc, nil
}

func ValidateSelectedClipsFile(path string) (SelectedClips, error) {
	doc, err := ReadSelectedClips(path)
	if err != nil {
		return SelectedClips{}, err
	}
	if doc.SchemaVersion != "selected_clips.v1" {
		return SelectedClips{}, fmt.Errorf("schema_version must be selected_clips.v1")
	}
	if doc.CreatedAt.IsZero() {
		return SelectedClips{}, fmt.Errorf("created_at is required")
	}
	if doc.InputPath == "" {
		return SelectedClips{}, fmt.Errorf("input_path is required")
	}
	if doc.Clips == nil {
		return SelectedClips{}, fmt.Errorf("clips array is required")
	}
	for i, clip := range doc.Clips {
		if clip.ID == "" {
			return SelectedClips{}, fmt.Errorf("clips[%d].id is required", i)
		}
		if clip.Order <= 0 {
			return SelectedClips{}, fmt.Errorf("clips[%d].order must be positive", i)
		}
		if clip.End < clip.Start {
			return SelectedClips{}, fmt.Errorf("clips[%d].end must be greater than or equal to start", i)
		}
		if clip.DurationSeconds < 0 {
			return SelectedClips{}, fmt.Errorf("clips[%d].duration_seconds must be non-negative", i)
		}
		if clip.OutputFilename == "" {
			return SelectedClips{}, fmt.Errorf("clips[%d].output_filename is required", i)
		}
		if clip.Title == "" {
			return SelectedClips{}, fmt.Errorf("clips[%d].title is required", i)
		}
		if clip.Description == "" {
			return SelectedClips{}, fmt.Errorf("clips[%d].description is required", i)
		}
	}
	return doc, nil
}

func ValidateExportManifestFile(path string) (ExportManifest, error) {
	doc, err := ReadExportManifest(path)
	if err != nil {
		return ExportManifest{}, err
	}
	if doc.SchemaVersion != "export_manifest.v1" {
		return ExportManifest{}, fmt.Errorf("schema_version must be export_manifest.v1")
	}
	if doc.CreatedAt.IsZero() {
		return ExportManifest{}, fmt.Errorf("created_at is required")
	}
	if doc.InputPath == "" {
		return ExportManifest{}, fmt.Errorf("input_path is required")
	}
	if doc.ExportsDir == "" {
		return ExportManifest{}, fmt.Errorf("exports_dir is required")
	}
	if doc.Clips == nil {
		return ExportManifest{}, fmt.Errorf("clips array is required")
	}
	for i, clip := range doc.Clips {
		if clip.ID == "" {
			return ExportManifest{}, fmt.Errorf("clips[%d].id is required", i)
		}
		if clip.Order <= 0 {
			return ExportManifest{}, fmt.Errorf("clips[%d].order must be positive", i)
		}
		if clip.PlannedOutput == "" {
			return ExportManifest{}, fmt.Errorf("clips[%d].planned_output is required", i)
		}
		if clip.Title == "" {
			return ExportManifest{}, fmt.Errorf("clips[%d].title is required", i)
		}
		if clip.Description == "" {
			return ExportManifest{}, fmt.Errorf("clips[%d].description is required", i)
		}
	}
	return doc, nil
}

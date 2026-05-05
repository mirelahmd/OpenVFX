package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/editorartifacts"
	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/exportartifacts"
	"github.com/mirelahmd/OpenVFX/internal/exporter"
	"github.com/mirelahmd/OpenVFX/internal/exportscript"
	"github.com/mirelahmd/OpenVFX/internal/manifest"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
)

type SelectedClipsOptions struct {
	Overwrite bool
	JSON      bool
}

type ExportManifestOptions struct {
	Overwrite bool
	JSON      bool
}

type FFmpegScriptCommandOptions struct {
	Mode      string
	Overwrite bool
	JSON      bool
}

type ConcatPlanOptions struct {
	Overwrite bool
	JSON      bool
}

type exportManifestSummaryView struct {
	Planned   int `json:"planned"`
	Exported  int `json:"exported"`
	Validated int `json:"validated"`
	Missing   int `json:"missing"`
}

func SelectedClipsCommand(runID string, stdout io.Writer, opts SelectedClipsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("SELECTED_CLIPS_STARTED", map[string]any{"run_id": runID})
	}
	outPath := filepath.Join(runDir, "selected_clips.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "SELECTED_CLIPS_FAILED", "selected_clips.json already exists; pass --overwrite")
			return fmt.Errorf("selected_clips.json already exists; pass --overwrite")
		}
	}
	doc, err := buildSelectedClips(runDir, runID)
	if err != nil {
		writeMaskFailure(log, "SELECTED_CLIPS_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "SELECTED_CLIPS_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "selected_clips", "selected_clips.json"); err != nil {
		writeMaskFailure(log, "SELECTED_CLIPS_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "SELECTED_CLIPS_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("SELECTED_CLIPS_COMPLETED", map[string]any{"path": "selected_clips.json", "clips": len(doc.Clips)})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Selected clips created")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  path:   %s\n", outPath)
	fmt.Fprintf(stdout, "  clips:  %d\n", len(doc.Clips))
	return nil
}

func ExportManifestCommand(runID string, stdout io.Writer, opts ExportManifestOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPORT_MANIFEST_STARTED", map[string]any{"run_id": runID})
	}
	outPath := filepath.Join(runDir, "export_manifest.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "EXPORT_MANIFEST_FAILED", "export_manifest.json already exists; pass --overwrite")
			return fmt.Errorf("export_manifest.json already exists; pass --overwrite")
		}
	}
	doc, err := buildExportManifest(runDir, runID)
	if err != nil {
		writeMaskFailure(log, "EXPORT_MANIFEST_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "EXPORT_MANIFEST_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "export_manifest", "export_manifest.json"); err != nil {
		writeMaskFailure(log, "EXPORT_MANIFEST_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "EXPORT_MANIFEST_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("EXPORT_MANIFEST_COMPLETED", map[string]any{"path": "export_manifest.json", "planned": doc.Summary.Planned})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Export manifest created")
	fmt.Fprintf(stdout, "  run id:    %s\n", runID)
	fmt.Fprintf(stdout, "  path:      %s\n", outPath)
	fmt.Fprintf(stdout, "  planned:   %d\n", doc.Summary.Planned)
	fmt.Fprintf(stdout, "  exported:  %d\n", doc.Summary.Exported)
	fmt.Fprintf(stdout, "  validated: %d\n", doc.Summary.Validated)
	return nil
}

func FFmpegScriptCommand(runID string, stdout io.Writer, opts FFmpegScriptCommandOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	if opts.Mode == "" {
		opts.Mode = "stream-copy"
	}
	if opts.Mode != "stream-copy" && opts.Mode != "reencode" {
		return fmt.Errorf("--mode must be stream-copy or reencode")
	}
	outPath := filepath.Join(runDir, "ffmpeg_commands.sh")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("ffmpeg_commands.sh already exists; pass --overwrite")
		}
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	sourcePath := filepath.Join(runDir, "selected_clips.json")
	if _, err := os.Stat(sourcePath); err != nil {
		sourcePath = filepath.Join(runDir, "roughcut.json")
	}
	summary, err := exportscript.WriteFFmpegScript(sourcePath, outPath, m.InputPath, "mp4", opts.Mode)
	if err != nil {
		return err
	}
	if err := addManifestArtifact(runDir, "ffmpeg_script", "ffmpeg_commands.sh"); err != nil {
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		return err
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "FFmpeg script created")
	fmt.Fprintf(stdout, "  run id:   %s\n", runID)
	fmt.Fprintf(stdout, "  path:     %s\n", outPath)
	fmt.Fprintf(stdout, "  mode:     %s\n", summary.Mode)
	fmt.Fprintf(stdout, "  commands: %d\n", summary.CommandCount)
	return nil
}

func ConcatPlanCommand(runID string, stdout io.Writer, opts ConcatPlanOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("CONCAT_PLAN_STARTED", map[string]any{"run_id": runID})
	}
	selectedPath := filepath.Join(runDir, "selected_clips.json")
	doc, err := exportartifacts.ReadSelectedClips(selectedPath)
	if err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return err
	}
	listPath := filepath.Join(runDir, "concat_list.txt")
	scriptPath := filepath.Join(runDir, "ffmpeg_concat.sh")
	if !opts.Overwrite {
		if _, err := os.Stat(listPath); err == nil {
			writeMaskFailure(log, "CONCAT_PLAN_FAILED", "concat_list.txt already exists; pass --overwrite")
			return fmt.Errorf("concat_list.txt already exists; pass --overwrite")
		}
		if _, err := os.Stat(scriptPath); err == nil {
			writeMaskFailure(log, "CONCAT_PLAN_FAILED", "ffmpeg_concat.sh already exists; pass --overwrite")
			return fmt.Errorf("ffmpeg_concat.sh already exists; pass --overwrite")
		}
	}
	var listBuilder strings.Builder
	for _, clip := range doc.Clips {
		listBuilder.WriteString("file ")
		listBuilder.WriteString(exportscript.ShellQuote(filepath.ToSlash(filepath.Join("exports", clip.OutputFilename))))
		listBuilder.WriteString("\n")
	}
	if err := os.WriteFile(listPath, []byte(listBuilder.String()), 0o644); err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return fmt.Errorf("write concat_list.txt: %w", err)
	}
	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!/usr/bin/env bash\n")
	scriptBuilder.WriteString("set -euo pipefail\n\n")
	scriptBuilder.WriteString("# Generated by BYOM Video. Planning artifact only.\n")
	scriptBuilder.WriteString("ffmpeg -y -f concat -safe 0 -i concat_list.txt -c copy exports/assembly.mp4\n")
	if err := os.WriteFile(scriptPath, []byte(scriptBuilder.String()), 0o755); err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return fmt.Errorf("write ffmpeg_concat.sh: %w", err)
	}
	if err := addManifestArtifact(runDir, "concat_list", "concat_list.txt"); err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "ffmpeg_concat", "ffmpeg_concat.sh"); err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "CONCAT_PLAN_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("CONCAT_PLAN_COMPLETED", map[string]any{"list_path": "concat_list.txt", "script_path": "ffmpeg_concat.sh"})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(map[string]any{
			"run_id":      runID,
			"list_path":   listPath,
			"script_path": scriptPath,
			"clips":       len(doc.Clips),
		}, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Concat plan created")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  list:   %s\n", listPath)
	fmt.Fprintf(stdout, "  script: %s\n", scriptPath)
	fmt.Fprintf(stdout, "  clips:  %d\n", len(doc.Clips))
	return nil
}

func buildSelectedClips(runDir string, runID string) (exportartifacts.SelectedClips, error) {
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return exportartifacts.SelectedClips{}, fmt.Errorf("read manifest: %w", err)
	}
	doc := exportartifacts.SelectedClips{
		SchemaVersion: "selected_clips.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		InputPath:     m.InputPath,
	}
	if _, err := os.Stat(filepath.Join(runDir, "enhanced_roughcut.json")); err == nil {
		enhanced, err := editorartifacts.ReadEnhancedRoughcut(filepath.Join(runDir, "enhanced_roughcut.json"))
		if err != nil {
			return exportartifacts.SelectedClips{}, err
		}
		doc.Source.EnhancedRoughcutArtifact = "enhanced_roughcut.json"
		if _, err := os.Stat(filepath.Join(runDir, "clip_cards.json")); err == nil {
			doc.Source.ClipCardsArtifact = "clip_cards.json"
		}
		doc.Source.RoughcutArtifact = "roughcut.json"
		for _, clip := range enhanced.Clips {
			duration := clip.End - clip.Start
			doc.Clips = append(doc.Clips, exportartifacts.SelectedClip{
				ID:                 clip.ID,
				Order:              clip.Order,
				Start:              clip.Start,
				End:                clip.End,
				DurationSeconds:    duration,
				Title:              clip.Title,
				Description:        clip.Description,
				CaptionSuggestions: clip.CaptionSuggestions,
				SourceText:         clip.SourceText,
				OutputFilename:     clip.ID + ".mp4",
			})
		}
		return doc, nil
	}

	roughcutDoc, err := readRoughcutDocument(filepath.Join(runDir, "roughcut.json"))
	if err != nil {
		return exportartifacts.SelectedClips{}, fmt.Errorf("read roughcut: %w", err)
	}
	doc.Source.RoughcutArtifact = "roughcut.json"
	cardMap := map[string]editorartifacts.ClipCard{}
	if _, err := os.Stat(filepath.Join(runDir, "clip_cards.json")); err == nil {
		cards, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
		if err == nil {
			doc.Source.ClipCardsArtifact = "clip_cards.json"
			for _, card := range cards.Cards {
				cardMap[card.ClipID] = card
			}
		}
	}
	for _, clip := range roughcutDoc.Clips {
		title := fallbackClipTitle(clip.Text)
		description := fallbackClipDescription(clip.Text, clip.EditIntent)
		captions := []string{}
		sourceText := clip.Text
		if card, ok := cardMap[clip.ID]; ok {
			title = card.Title
			description = card.Description
			captions = card.Captions
			sourceText = nonEmptyString(card.SourceText, sourceText)
		}
		doc.Clips = append(doc.Clips, exportartifacts.SelectedClip{
			ID:                 clip.ID,
			Order:              clip.Order,
			Start:              clip.Start,
			End:                clip.End,
			DurationSeconds:    positiveDuration(clip.DurationSeconds, clip.Start, clip.End),
			Title:              title,
			Description:        description,
			CaptionSuggestions: captions,
			SourceText:         sourceText,
			OutputFilename:     clip.ID + ".mp4",
		})
	}
	return doc, nil
}

func buildExportManifest(runDir string, runID string) (exportartifacts.ExportManifest, error) {
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return exportartifacts.ExportManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	selectedPath := filepath.Join(runDir, "selected_clips.json")
	if _, err := os.Stat(selectedPath); err != nil {
		selected, buildErr := buildSelectedClips(runDir, runID)
		if buildErr != nil {
			return exportartifacts.ExportManifest{}, buildErr
		}
		if err := writeJSONFile(selectedPath, selected); err != nil {
			return exportartifacts.ExportManifest{}, err
		}
		if err := addManifestArtifact(runDir, "selected_clips", "selected_clips.json"); err != nil {
			return exportartifacts.ExportManifest{}, err
		}
	}
	selected, err := exportartifacts.ReadSelectedClips(selectedPath)
	if err != nil {
		return exportartifacts.ExportManifest{}, err
	}
	validationByPath := map[string]exporter.ExportValidationFile{}
	if validation, err := exporter.ReadExportValidation(runDir); err == nil {
		for _, file := range validation.Files {
			validationByPath[file.Path] = file
		}
	}
	doc := exportartifacts.ExportManifest{
		SchemaVersion: "export_manifest.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		InputPath:     m.InputPath,
		ExportsDir:    "exports",
	}
	for _, clip := range selected.Clips {
		planned := filepath.ToSlash(filepath.Join("exports", clip.OutputFilename))
		exists := false
		if _, err := os.Stat(filepath.Join(runDir, filepath.FromSlash(planned))); err == nil {
			exists = true
		}
		validated := false
		duration := clip.DurationSeconds
		if validation, ok := validationByPath[planned]; ok {
			validated = validation.Status == "ok"
			if validation.DurationSeconds != nil {
				duration = *validation.DurationSeconds
			}
		}
		doc.Clips = append(doc.Clips, exportartifacts.ExportManifestClip{
			ID:              clip.ID,
			Order:           clip.Order,
			PlannedOutput:   planned,
			Exists:          exists,
			Validated:       validated,
			DurationSeconds: duration,
			Title:           clip.Title,
			Description:     clip.Description,
		})
		if exists {
			doc.Summary.Exported++
		} else {
			doc.Summary.Missing++
		}
		if validated {
			doc.Summary.Validated++
		}
	}
	doc.Summary.Planned = len(doc.Clips)
	return doc, nil
}

func positiveDuration(duration float64, start float64, end float64) float64 {
	if duration > 0 {
		return duration
	}
	if end >= start {
		return end - start
	}
	return 0
}

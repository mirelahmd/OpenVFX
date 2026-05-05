package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"byom-video/internal/media"
)

const ExportValidationArtifactPath = "export_validation.json"

type ExportValidation struct {
	SchemaVersion string                 `json:"schema_version"`
	ExportsDir    string                 `json:"exports_dir"`
	CheckedAt     time.Time              `json:"checked_at"`
	Files         []ExportValidationFile `json:"files"`
}

type ExportValidationFile struct {
	Path            string   `json:"path"`
	Exists          bool     `json:"exists"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	VideoStreams    int      `json:"video_streams"`
	AudioStreams    int      `json:"audio_streams"`
	Status          string   `json:"status"`
	Error           string   `json:"error"`
}

type probeDocument struct {
	Format *struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
	} `json:"streams"`
}

func ValidateExports(runDir string, exportedFiles []string) (ExportValidation, error) {
	result := ExportValidation{
		SchemaVersion: "export_validation.v1",
		ExportsDir:    "exports",
		CheckedAt:     time.Now().UTC(),
		Files:         []ExportValidationFile{},
	}
	var failed []string
	for _, relPath := range exportedFiles {
		fileResult := ExportValidationFile{
			Path:   relPath,
			Exists: false,
			Status: "ok",
			Error:  "",
		}
		absPath := filepath.Join(runDir, filepath.FromSlash(relPath))
		if info, err := os.Stat(absPath); err != nil {
			fileResult.Status = "failed"
			fileResult.Error = err.Error()
			failed = append(failed, relPath)
			result.Files = append(result.Files, fileResult)
			continue
		} else if info.IsDir() {
			fileResult.Status = "failed"
			fileResult.Error = "export path is a directory"
			failed = append(failed, relPath)
			result.Files = append(result.Files, fileResult)
			continue
		}
		fileResult.Exists = true
		probeData, err := media.Probe(absPath)
		if err != nil {
			fileResult.Status = "failed"
			fileResult.Error = err.Error()
			failed = append(failed, relPath)
			result.Files = append(result.Files, fileResult)
			continue
		}
		if err := ApplyProbeMetadata(probeData, &fileResult); err != nil {
			fileResult.Status = "failed"
			fileResult.Error = err.Error()
			failed = append(failed, relPath)
		}
		result.Files = append(result.Files, fileResult)
	}
	if err := WriteExportValidation(runDir, result); err != nil {
		return result, err
	}
	if len(failed) > 0 {
		return result, fmt.Errorf("export validation failed for %d file(s)", len(failed))
	}
	return result, nil
}

func ApplyProbeMetadata(data []byte, file *ExportValidationFile) error {
	var doc probeDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode ffprobe JSON: %w", err)
	}
	if doc.Format != nil && doc.Format.Duration != "" {
		duration, err := strconv.ParseFloat(doc.Format.Duration, 64)
		if err != nil {
			return fmt.Errorf("parse ffprobe duration: %w", err)
		}
		file.DurationSeconds = &duration
	}
	for _, stream := range doc.Streams {
		switch stream.CodecType {
		case "video":
			file.VideoStreams++
		case "audio":
			file.AudioStreams++
		}
	}
	return nil
}

func WriteExportValidation(runDir string, result ExportValidation) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export validation: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(runDir, ExportValidationArtifactPath), data, 0o644); err != nil {
		return fmt.Errorf("write export validation: %w", err)
	}
	return nil
}

func ReadExportValidation(runDir string) (ExportValidation, error) {
	data, err := os.ReadFile(filepath.Join(runDir, ExportValidationArtifactPath))
	if err != nil {
		return ExportValidation{}, err
	}
	var result ExportValidation
	if err := json.Unmarshal(data, &result); err != nil {
		return ExportValidation{}, fmt.Errorf("decode export validation: %w", err)
	}
	return result, nil
}

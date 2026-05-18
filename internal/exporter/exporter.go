package exporter

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/manifest"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
)

type Summary struct {
	RunID                  string
	RunDir                 string
	ScriptPath             string
	ExportsDir             string
	ExportedFiles          []string
	ExportValidationStatus string
	ExportValidationError  string
}

func Run(runID string, stdout io.Writer) (Summary, error) {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return Summary{}, err
	}
	scriptPath := filepath.Join(runDir, "ffmpeg_commands.sh")
	if info, err := os.Stat(scriptPath); err != nil {
		if os.IsNotExist(err) {
			return Summary{}, fmt.Errorf("ffmpeg export script is missing: %s", scriptPath)
		}
		return Summary{}, fmt.Errorf("stat ffmpeg export script: %w", err)
	} else if info.IsDir() {
		return Summary{}, fmt.Errorf("ffmpeg export script path is a directory: %s", scriptPath)
	}

	eventLog, err := events.Open(filepath.Join(runDir, "events.jsonl"))
	if err != nil {
		return Summary{}, err
	}
	defer eventLog.Close()

	manifestPath := filepath.Join(runDir, "manifest.json")
	m, err := manifest.Read(manifestPath)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		RunID:      runID,
		RunDir:     runDir,
		ScriptPath: scriptPath,
		ExportsDir: filepath.Join(runDir, "exports"),
	}

	if err := eventLog.Write("EXPORT_STARTED", map[string]any{"script_path": "ffmpeg_commands.sh"}); err != nil {
		return Summary{}, err
	}

	cmd := exec.Command("bash", "ffmpeg_commands.sh")
	cmd.Dir = runDir
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Run(); err != nil {
		m.ExportStatus = "failed"
		m.ExportErrorMessage = err.Error()
		_ = manifest.Write(manifestPath, m)
		_ = eventLog.Write("EXPORT_FAILED", map[string]any{"error": err.Error()})
		return summary, fmt.Errorf("execute ffmpeg export script: %w", err)
	}

	exportedFiles, err := DiscoverExportedFiles(runDir)
	if err != nil {
		m.ExportStatus = "failed"
		m.ExportErrorMessage = err.Error()
		_ = manifest.Write(manifestPath, m)
		_ = eventLog.Write("EXPORT_FAILED", map[string]any{"error": err.Error()})
		return summary, err
	}
	now := time.Now().UTC()
	m.ExportedAt = &now
	m.ExportStatus = "completed"
	m.ExportErrorMessage = ""
	m.ExportsDir = "exports"
	m.ExportedFiles = exportedFiles
	if err := manifest.Write(manifestPath, m); err != nil {
		return summary, err
	}
	if err := eventLog.Write("EXPORT_COMPLETED", map[string]any{"exports_dir": "exports", "exported_files": exportedFiles}); err != nil {
		return summary, err
	}
	summary.ExportedFiles = exportedFiles
	if err := eventLog.Write("EXPORT_VALIDATION_STARTED", map[string]any{"exported_files": exportedFiles}); err != nil {
		return summary, err
	}
	validation, validationErr := ValidateExports(runDir, exportedFiles)
	m.ExportValidationStatus = "completed"
	m.ExportValidationError = ""
	if validationErr != nil {
		m.ExportValidationStatus = "failed"
		m.ExportValidationError = validationErr.Error()
		summary.ExportValidationStatus = "failed"
		summary.ExportValidationError = validationErr.Error()
		_ = manifest.Write(manifestPath, m)
		_ = eventLog.Write("EXPORT_VALIDATION_FAILED", map[string]any{"error": validationErr.Error()})
		return summary, nil
	}
	m.AddArtifact("export_validation", ExportValidationArtifactPath)
	if err := manifest.Write(manifestPath, m); err != nil {
		return summary, err
	}
	if err := eventLog.Write("EXPORT_VALIDATION_COMPLETED", map[string]any{"path": ExportValidationArtifactPath, "files": len(validation.Files)}); err != nil {
		return summary, err
	}
	summary.ExportValidationStatus = "completed"
	return summary, nil
}

func DiscoverExportedFiles(runDir string) ([]string, error) {
	exportsDir := filepath.Join(runDir, "exports")
	entries, err := os.ReadDir(exportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read exports directory: %w", err)
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.ToSlash(filepath.Join("exports", entry.Name())))
	}
	sort.Strings(files)
	return files, nil
}

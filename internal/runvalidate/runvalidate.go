package runvalidate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/chunks"
	"github.com/mirelahmd/OpenVFX/internal/editorartifacts"
	"github.com/mirelahmd/OpenVFX/internal/exportartifacts"
	"github.com/mirelahmd/OpenVFX/internal/highlights"
	"github.com/mirelahmd/OpenVFX/internal/manifest"
	"github.com/mirelahmd/OpenVFX/internal/roughcut"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
	"github.com/mirelahmd/OpenVFX/internal/transcript"
)

type Result struct {
	RunID        string   `json:"run_id"`
	RunDir       string   `json:"run_dir"`
	ManifestOK   bool     `json:"manifest_ok"`
	EventsOK     bool     `json:"events_ok"`
	ChecksPassed []string `json:"checks_passed"`
	Warnings     []string `json:"warnings"`
	Errors       []string `json:"errors"`
}

func (r Result) HasErrors() bool {
	return len(r.Errors) > 0
}

func Validate(runID string) (Result, error) {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return Result{}, err
	}
	result := Result{RunID: runID, RunDir: runDir, ChecksPassed: []string{}, Warnings: []string{}, Errors: []string{}}
	m, manifestLoaded := validateManifest(runDir, &result)
	validateEvents(runDir, manifestLoaded, m, &result)
	if manifestLoaded {
		validateArtifactFiles(runDir, m, &result)
		validateKnownSchemas(runDir, &result)
	}
	return result, nil
}

func validateManifest(runDir string, result *Result) (manifest.Manifest, bool) {
	manifestPath := filepath.Join(runDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: %v", err))
		return manifest.Manifest{}, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: decode failed: %v", err))
		return manifest.Manifest{}, false
	}
	var m manifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: decode failed: %v", err))
		return manifest.Manifest{}, false
	}
	if strings.TrimSpace(m.RunID) == "" {
		result.Errors = append(result.Errors, "manifest.json: run_id is required")
	}
	if strings.TrimSpace(m.InputPath) == "" {
		result.Errors = append(result.Errors, "manifest.json: input_path is required")
	}
	if _, ok := raw["created_at"]; !ok || m.CreatedAt.IsZero() {
		result.Errors = append(result.Errors, "manifest.json: created_at is required")
	}
	if !allowed(m.Status, manifest.StatusRunning, manifest.StatusCompleted, manifest.StatusFailed) {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: invalid status %q", m.Status))
	}
	if _, ok := raw["artifacts"]; !ok || m.Artifacts == nil {
		result.Errors = append(result.Errors, "manifest.json: artifacts list is required")
	}
	for _, artifact := range m.Artifacts {
		if !safeRelPath(artifact.Path) {
			result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: unsafe artifact path %q", artifact.Path))
		}
	}
	if m.Status == manifest.StatusFailed && strings.TrimSpace(m.ErrorMessage) == "" {
		result.Warnings = append(result.Warnings, "manifest.json: failed run has no error_message")
	}
	if m.ExportStatus != "" && !allowed(m.ExportStatus, "completed", "failed", "not_started") {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: invalid export_status %q", m.ExportStatus))
	}
	if m.ExportValidationStatus != "" && !allowed(m.ExportValidationStatus, "completed", "failed") {
		result.Errors = append(result.Errors, fmt.Sprintf("manifest.json: invalid export_validation_status %q", m.ExportValidationStatus))
	}
	if len(result.Errors) == 0 {
		result.ManifestOK = true
		result.ChecksPassed = append(result.ChecksPassed, "manifest.json structure")
	}
	return m, true
}

func validateEvents(runDir string, manifestLoaded bool, m manifest.Manifest, result *Result) {
	path := filepath.Join(runDir, "events.jsonl")
	file, err := os.Open(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("events.jsonl: %v", err))
		return
	}
	defer file.Close()
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("events.jsonl:%d: invalid JSON: %v", lineNumber, err))
			continue
		}
		eventName := rawString(event, "event")
		if eventName == "" {
			eventName = rawString(event, "type")
		}
		if eventName == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("events.jsonl:%d: event/type is required", lineNumber))
		} else {
			seen[eventName] = true
		}
		if rawString(event, "timestamp") == "" && rawString(event, "created_at") == "" && rawString(event, "time") == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("events.jsonl:%d: timestamp/created_at/time is required", lineNumber))
		}
	}
	if err := scanner.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("events.jsonl: read failed: %v", err))
	}
	if !seen["RUN_STARTED"] {
		result.Warnings = append(result.Warnings, "events.jsonl: missing RUN_STARTED")
	}
	if manifestLoaded && m.Status == manifest.StatusCompleted && !seen["RUN_COMPLETED"] {
		result.Warnings = append(result.Warnings, "events.jsonl: completed manifest has no RUN_COMPLETED")
	}
	if manifestLoaded && m.Status == manifest.StatusFailed && !seen["RUN_FAILED"] {
		result.Warnings = append(result.Warnings, "events.jsonl: failed manifest has no RUN_FAILED")
	}
	if !hasPrefix(result.Errors, "events.jsonl") {
		result.EventsOK = true
		result.ChecksPassed = append(result.ChecksPassed, "events.jsonl structure")
	}
}

func validateArtifactFiles(runDir string, m manifest.Manifest, result *Result) {
	paths := map[string]bool{}
	for _, artifact := range m.Artifacts {
		paths[artifact.Path] = true
	}
	for _, exportedFile := range m.ExportedFiles {
		paths[exportedFile] = true
	}
	for path := range paths {
		if !safeRelPath(path) {
			continue
		}
		info, err := os.Stat(filepath.Join(runDir, filepath.FromSlash(path)))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("artifact missing: %s", path))
			continue
		}
		if info.IsDir() {
			result.Errors = append(result.Errors, fmt.Sprintf("artifact path is a directory: %s", path))
		}
	}
	if len(paths) > 0 && !hasPrefix(result.Errors, "artifact ") {
		result.ChecksPassed = append(result.ChecksPassed, "manifest artifact paths exist")
	}
}

func validateKnownSchemas(runDir string, result *Result) {
	checks := []struct {
		path string
		fn   func(string) error
	}{
		{"transcript.json", func(path string) error { _, err := transcript.ValidateFile(path); return err }},
		{"chunks.json", func(path string) error { _, err := chunks.ValidateFile(path); return err }},
		{"highlights.json", func(path string) error { _, err := highlights.ValidateFile(path); return err }},
		{"roughcut.json", func(path string) error { _, err := roughcut.ValidateFile(path); return err }},
		{"clip_cards.json", func(path string) error { _, err := editorartifacts.ValidateClipCardsFile(path); return err }},
		{"enhanced_roughcut.json", func(path string) error { _, err := editorartifacts.ValidateEnhancedRoughcutFile(path); return err }},
		{"selected_clips.json", func(path string) error { _, err := exportartifacts.ValidateSelectedClipsFile(path); return err }},
		{"export_manifest.json", func(path string) error { _, err := exportartifacts.ValidateExportManifestFile(path); return err }},
	}
	for _, check := range checks {
		path := filepath.Join(runDir, check.path)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := check.fn(path); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", check.path, err))
			continue
		}
		result.ChecksPassed = append(result.ChecksPassed, check.path+" schema")
	}
	for _, plain := range []string{"captions.srt", "ffmpeg_commands.sh", "report.html", "concat_list.txt", "ffmpeg_concat.sh"} {
		path := filepath.Join(runDir, plain)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			result.ChecksPassed = append(result.ChecksPassed, plain+" exists")
		}
	}
}

func safeRelPath(path string) bool {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	return !strings.Contains(filepath.ToSlash(clean), "../")
}

func allowed(value string, allowedValues ...string) bool {
	for _, allowedValue := range allowedValues {
		if value == allowedValue {
			return true
		}
	}
	return false
}

func rawString(raw map[string]json.RawMessage, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var t time.Time
	if err := json.Unmarshal(value, &t); err == nil && !t.IsZero() {
		return t.Format(time.RFC3339)
	}
	return ""
}

func hasPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

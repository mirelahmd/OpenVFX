package runinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/exporter"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type RunListOptions struct {
	Limit int
	All   bool
}

type RunRow struct {
	RunID         string    `json:"run_id"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	InputBasename string    `json:"input_basename"`
	ArtifactCount int       `json:"artifact_count"`
	ExportStatus  string    `json:"export_status,omitempty"`
	ManifestError string    `json:"manifest_error,omitempty"`
}

type InspectSummary struct {
	RunID                  string                 `json:"run_id"`
	Status                 string                 `json:"status"`
	InputPath              string                 `json:"input_path"`
	CreatedAt              string                 `json:"created_at"`
	ErrorMessage           string                 `json:"error_message,omitempty"`
	Artifacts              []Path                 `json:"artifacts"`
	Warnings               []string               `json:"warnings,omitempty"`
	ExportStatus           string                 `json:"export_status,omitempty"`
	ExportValidationStatus string                 `json:"export_validation_status,omitempty"`
	ExportsDir             string                 `json:"exports_dir,omitempty"`
	ExportedFiles          []string               `json:"exported_files,omitempty"`
	ReportPath             string                 `json:"report_path,omitempty"`
	GoalReviewBundlePath   string                 `json:"goal_review_bundle_path,omitempty"`
	AgentResultPath        string                 `json:"agent_result_path,omitempty"`
	TranscriptSegmentCount *int                   `json:"transcript_segment_count,omitempty"`
	ChunkCount             *int                   `json:"chunk_count,omitempty"`
	HighlightCount         *int                   `json:"highlight_count,omitempty"`
	RoughcutClipCount      *int                   `json:"roughcut_clip_count,omitempty"`
	ClipCardCount          *int                   `json:"clip_card_count,omitempty"`
	EnhancedRoughcutCount  *int                   `json:"enhanced_roughcut_count,omitempty"`
	SelectedClipCount      *int                   `json:"selected_clip_count,omitempty"`
	SelectedClipSource     string                 `json:"selected_clip_source,omitempty"`
	GoalRerankCount        *int                   `json:"goal_rerank_count,omitempty"`
	GoalRerankMode         string                 `json:"goal_rerank_mode,omitempty"`
	GoalRoughcutClipCount  *int                   `json:"goal_roughcut_clip_count,omitempty"`
	ExportManifestSummary  *ExportManifestSummary `json:"export_manifest_summary,omitempty"`
	ConcatPlanPresent      bool                   `json:"concat_plan_present,omitempty"`
	ExportedFileCount      int                    `json:"exported_file_count"`
}

type ExportManifestSummary struct {
	Planned   int `json:"planned"`
	Exported  int `json:"exported"`
	Validated int `json:"validated"`
	Missing   int `json:"missing"`
}

type Path struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func ListRuns(opts RunListOptions) ([]RunRow, error) {
	entries, err := os.ReadDir(runstore.RunsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunRow{}, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}
	rows := []RunRow{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		row := RunRow{RunID: runID, Status: "unknown"}
		runDir := filepath.Join(runstore.RunsRoot, runID)
		m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
		if err != nil {
			row.ManifestError = err.Error()
			if info, statErr := entry.Info(); statErr == nil {
				row.CreatedAt = info.ModTime()
			}
			rows = append(rows, row)
			continue
		}
		row.Status = emptyDefault(m.Status, "unknown")
		row.CreatedAt = m.CreatedAt
		row.InputBasename = filepath.Base(m.InputPath)
		row.ArtifactCount = len(m.Artifacts)
		row.ExportStatus = m.ExportStatus
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if !opts.All && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}
	return rows, nil
}

func Inspect(runID string) (InspectSummary, error) {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return InspectSummary{}, err
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return InspectSummary{}, err
	}
	summary := InspectSummary{
		RunID:                  emptyDefault(m.RunID, runID),
		Status:                 m.Status,
		InputPath:              m.InputPath,
		CreatedAt:              m.CreatedAt.Format(time.RFC3339),
		ErrorMessage:           m.ErrorMessage,
		ExportStatus:           m.ExportStatus,
		ExportValidationStatus: m.ExportValidationStatus,
		ExportsDir:             m.ExportsDir,
		ExportedFiles:          m.ExportedFiles,
		ExportedFileCount:      len(m.ExportedFiles),
	}
	for _, artifact := range m.Artifacts {
		fullPath := filepath.Join(runDir, artifact.Path)
		summary.Artifacts = append(summary.Artifacts, Path{Name: artifact.Name, Path: fullPath})
		if _, err := os.Stat(fullPath); err != nil {
			summary.Warnings = append(summary.Warnings, fmt.Sprintf("missing artifact: %s", fullPath))
		}
		if artifact.Path == "report.html" {
			summary.ReportPath = fullPath
		}
		if artifact.Path == "goal_review_bundle.md" {
			summary.GoalReviewBundlePath = fullPath
		}
	}
	if agentResultPath, ok := findAgentResultForRun(runID); ok {
		summary.AgentResultPath = agentResultPath
	}
	if len(summary.ExportedFiles) == 0 {
		files, err := exporter.DiscoverExportedFiles(runDir)
		if err == nil {
			summary.ExportedFiles = files
			summary.ExportedFileCount = len(files)
		}
	}
	if count, ok := countJSONList(filepath.Join(runDir, "transcript.json"), "segments"); ok {
		summary.TranscriptSegmentCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "chunks.json"), "chunks"); ok {
		summary.ChunkCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "highlights.json"), "highlights"); ok {
		summary.HighlightCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "roughcut.json"), "clips"); ok {
		summary.RoughcutClipCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "clip_cards.json"), "cards"); ok {
		summary.ClipCardCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "enhanced_roughcut.json"), "clips"); ok {
		summary.EnhancedRoughcutCount = &count
	}
	if count, ok := countJSONList(filepath.Join(runDir, "selected_clips.json"), "clips"); ok {
		summary.SelectedClipCount = &count
	}
	if source, ok := readSelectedClipSource(filepath.Join(runDir, "selected_clips.json")); ok {
		summary.SelectedClipSource = source
	}
	if count, ok := countJSONList(filepath.Join(runDir, "goal_rerank.json"), "ranked_highlights"); ok {
		summary.GoalRerankCount = &count
	}
	if mode, ok := readStringField(filepath.Join(runDir, "goal_rerank.json"), "mode"); ok {
		summary.GoalRerankMode = mode
	}
	if count, ok := countJSONList(filepath.Join(runDir, "goal_roughcut.json"), "clips"); ok {
		summary.GoalRoughcutClipCount = &count
	}
	if manifestSummary, ok := readExportManifestSummary(filepath.Join(runDir, "export_manifest.json")); ok {
		summary.ExportManifestSummary = &manifestSummary
	}
	if _, err := os.Stat(filepath.Join(runDir, "concat_list.txt")); err == nil {
		summary.ConcatPlanPresent = true
	}
	return summary, nil
}

func ArtifactPaths(runID string, artifactType string) ([]string, error) {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return nil, err
	}
	if artifactType == "exports" {
		files, err := exporter.DiscoverExportedFiles(runDir)
		if err != nil {
			return nil, err
		}
		for i := range files {
			files[i] = filepath.Join(runDir, files[i])
		}
		return files, nil
	}
	name, err := artifactName(artifactType)
	if err != nil {
		return nil, err
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, artifact := range m.Artifacts {
		if name == "" || artifact.Name == name {
			paths = append(paths, filepath.Join(runDir, artifact.Path))
		}
	}
	return paths, nil
}

func artifactName(artifactType string) (string, error) {
	switch artifactType {
	case "":
		return "", nil
	case "manifest", "events", "metadata", "transcript", "captions", "chunks", "highlights", "roughcut", "report", "export_validation", "clip_cards", "clip_cards_review", "enhanced_roughcut", "selected_clips", "export_manifest", "concat_list", "ffmpeg_concat", "goal_rerank", "goal_roughcut", "goal_review_bundle":
		return artifactType, nil
	case "export-validation":
		return "export_validation", nil
	case "ffmpeg-script":
		return "ffmpeg_script", nil
	default:
		return "", fmt.Errorf("unknown artifact type %q", artifactType)
	}
}

func countJSONList(path string, key string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, false
	}
	raw, ok := doc[key]
	if !ok {
		return 0, false
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return 0, false
	}
	return len(values), true
}

func readExportManifestSummary(path string) (ExportManifestSummary, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExportManifestSummary{}, false
	}
	var doc struct {
		Summary ExportManifestSummary `json:"summary"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ExportManifestSummary{}, false
	}
	return doc.Summary, true
}

func readStringField(path string, key string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	raw, ok := doc[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

func readSelectedClipSource(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var doc struct {
		Source struct {
			GoalRoughcutArtifact     string `json:"goal_roughcut_artifact"`
			EnhancedRoughcutArtifact string `json:"enhanced_roughcut_artifact"`
			ClipCardsArtifact        string `json:"clip_cards_artifact"`
			RoughcutArtifact         string `json:"roughcut_artifact"`
		} `json:"source"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	switch {
	case strings.TrimSpace(doc.Source.GoalRoughcutArtifact) != "":
		return "goal_roughcut", true
	case strings.TrimSpace(doc.Source.EnhancedRoughcutArtifact) != "":
		return "enhanced_roughcut", true
	case strings.TrimSpace(doc.Source.ClipCardsArtifact) != "":
		return "clip_cards", true
	case strings.TrimSpace(doc.Source.RoughcutArtifact) != "":
		return "roughcut", true
	default:
		return "", false
	}
}

func emptyDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func findAgentResultForRun(runID string) (string, bool) {
	entries, err := os.ReadDir(".byom-video/plans")
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		planPath := filepath.Join(".byom-video/plans", entry.Name(), "agent_plan.json")
		data, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}
		var plan struct {
			Actions []struct {
				RunID string `json:"run_id"`
			} `json:"actions"`
		}
		if err := json.Unmarshal(data, &plan); err != nil {
			continue
		}
		for _, action := range plan.Actions {
			if strings.TrimSpace(action.RunID) == runID {
				path := filepath.Join(".byom-video/plans", entry.Name(), "agent_result.md")
				if _, err := os.Stat(path); err == nil {
					return path, true
				}
			}
		}
	}
	return "", false
}

package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Manifest struct {
	RunID                  string            `json:"run_id"`
	InputPath              string            `json:"input_path"`
	CreatedAt              time.Time         `json:"created_at"`
	Status                 string            `json:"status"`
	Artifacts              []Artifact        `json:"artifacts"`
	ToolVersions           map[string]string `json:"tool_versions,omitempty"`
	ErrorMessage           string            `json:"error_message,omitempty"`
	ExportedAt             *time.Time        `json:"exported_at,omitempty"`
	ExportStatus           string            `json:"export_status,omitempty"`
	ExportsDir             string            `json:"exports_dir,omitempty"`
	ExportedFiles          []string          `json:"exported_files,omitempty"`
	ExportErrorMessage     string            `json:"export_error_message,omitempty"`
	ExportValidationStatus string            `json:"export_validation_status,omitempty"`
	ExportValidationError  string            `json:"export_validation_error,omitempty"`
}

type Artifact struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

func New(runID string, inputPath string, createdAt time.Time) Manifest {
	return Manifest{
		RunID:        runID,
		InputPath:    inputPath,
		CreatedAt:    createdAt,
		Status:       StatusRunning,
		Artifacts:    []Artifact{},
		ToolVersions: map[string]string{},
	}
}

func (m *Manifest) AddArtifact(name string, path string) {
	for _, artifact := range m.Artifacts {
		if artifact.Path == path {
			return
		}
	}
	m.Artifacts = append(m.Artifacts, Artifact{
		Name:      name,
		Path:      path,
		CreatedAt: time.Now().UTC(),
	})
}

func Write(path string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func Read(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return m, nil
}

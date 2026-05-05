package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManifestDefaults(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 6, 30, 0, 0, time.UTC)

	m := New("run-1", "/tmp/input.mp4", createdAt)

	if m.RunID != "run-1" {
		t.Fatalf("RunID = %q, want run-1", m.RunID)
	}
	if m.Status != StatusRunning {
		t.Fatalf("Status = %q, want %q", m.Status, StatusRunning)
	}
	if len(m.Artifacts) != 0 {
		t.Fatalf("Artifacts length = %d, want 0", len(m.Artifacts))
	}
	if m.ToolVersions == nil {
		t.Fatal("ToolVersions is nil")
	}
}

func TestManifestWriteRewritesFileAndKeepsArtifactsUnique(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	m := New("run-1", "/tmp/input.mp4", time.Date(2026, 4, 28, 6, 30, 0, 0, time.UTC))

	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("manifest duplicate", "manifest.json")
	if err := Write(path, m); err != nil {
		t.Fatalf("first Write returned error: %v", err)
	}

	m.Status = StatusCompleted
	m.ToolVersions["ffprobe"] = "ffprobe version test"
	m.AddArtifact("metadata", "metadata.json")
	if err := Write(path, m); err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("manifest JSON did not decode: %v", err)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", got.Status, StatusCompleted)
	}
	if len(got.Artifacts) != 2 {
		t.Fatalf("Artifacts length = %d, want 2", len(got.Artifacts))
	}
	if got.ToolVersions["ffprobe"] != "ffprobe version test" {
		t.Fatalf("ffprobe version = %q", got.ToolVersions["ffprobe"])
	}
}

package runvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/manifest"
)

func TestValidateAcceptsValidManifestAndEvents(t *testing.T) {
	t.Chdir(t.TempDir())
	writeValidationRun(t, "run-1", func(m *manifest.Manifest) {})
	result, err := Validate("run-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if result.HasErrors() {
		t.Fatalf("errors = %#v", result.Errors)
	}
	if !result.ManifestOK || !result.EventsOK {
		t.Fatalf("manifest/events status = %v/%v", result.ManifestOK, result.EventsOK)
	}
}

func TestValidateRejectsUnsafeArtifactPath(t *testing.T) {
	t.Chdir(t.TempDir())
	writeValidationRun(t, "run-1", func(m *manifest.Manifest) {
		m.AddArtifact("unsafe", "../outside.json")
	})
	result, err := Validate("run-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !contains(result.Errors, "unsafe artifact path") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestValidateRejectsInvalidStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	writeValidationRun(t, "run-1", func(m *manifest.Manifest) {
		m.Status = "stuck"
	})
	result, err := Validate("run-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !contains(result.Errors, "invalid status") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestValidateRejectsInvalidJSONL(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeValidationRun(t, "run-1", func(m *manifest.Manifest) {})
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Validate("run-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !contains(result.Errors, "invalid JSON") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestValidateReportsMissingArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	writeValidationRun(t, "run-1", func(m *manifest.Manifest) {
		m.AddArtifact("metadata", "metadata.json")
	})
	result, err := Validate("run-1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !contains(result.Errors, "artifact missing: metadata.json") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func writeValidationRun(t *testing.T, runID string, mutate func(*manifest.Manifest)) string {
	t.Helper()
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "/tmp/input.mp4", time.Now().UTC())
	m.Status = manifest.StatusCompleted
	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("events", "events.jsonl")
	mutate(&m)
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	events := `{"type":"RUN_STARTED","time":"2026-04-28T00:00:00Z"}` + "\n" +
		`{"type":"RUN_COMPLETED","time":"2026-04-28T00:00:01Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return runDir
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

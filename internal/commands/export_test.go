package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExportRejectsMissingRun(t *testing.T) {
	t.Chdir(t.TempDir())
	err := Export("missing-run", &bytes.Buffer{})
	if err == nil {
		t.Fatal("Export returned nil error")
	}
}

func TestExportRejectsMissingScript(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	runDir := filepath.Join(cwd, ".byom-video", "runs", "run-1")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), []byte(`{"run_id":"run-1","status":"completed","artifacts":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Export("run-1", &bytes.Buffer{})
	if err == nil {
		t.Fatal("Export returned nil error")
	}
}

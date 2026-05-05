package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"byom-video/internal/manifest"
)

func TestFindCandidates(t *testing.T) {
	t.Chdir(t.TempDir())
	writeManifest(t, "failed-run", manifest.StatusFailed, time.Now().UTC())
	writeManifest(t, "running-old", manifest.StatusRunning, time.Now().UTC().Add(-48*time.Hour))
	if err := os.MkdirAll(filepath.Join(".byom-video", "runs", "missing"), 0o755); err != nil {
		t.Fatal(err)
	}
	candidates, err := FindCandidates(Options{Now: time.Now().UTC(), OlderThan: 24 * time.Hour})
	if err != nil {
		t.Fatalf("FindCandidates returned error: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestDeleteCandidateRejectsUnsafeMismatch(t *testing.T) {
	t.Chdir(t.TempDir())
	err := DeleteCandidate(Candidate{RunID: "run-1", RunDir: "../outside", Reason: "failed"})
	if err == nil {
		t.Fatal("DeleteCandidate returned nil error")
	}
}

func TestDeleteCandidateRemovesRunDir(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := filepath.Join(".byom-video", "runs", "failed-run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCandidate(Candidate{RunID: "failed-run", RunDir: runDir, Reason: "failed"}); err != nil {
		t.Fatalf("DeleteCandidate returned error: %v", err)
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("run dir still exists or stat failed unexpectedly: %v", err)
	}
}

func writeManifest(t *testing.T, runID string, status string, createdAt time.Time) {
	t.Helper()
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "/tmp/input.mp4", createdAt)
	m.Status = status
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
}

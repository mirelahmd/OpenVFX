package runinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/manifest"
)

func TestListRunsNewestFirst(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRunManifest(t, "old", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), "old.mp4")
	writeRunManifest(t, "new", time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), "new.mp4")

	rows, err := ListRuns(RunListOptions{All: true})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].RunID != "new" || rows[1].RunID != "old" {
		t.Fatalf("rows sorted incorrectly: %#v", rows)
	}
}

func TestListRunsHandlesMissingManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".byom-video", "runs", "missing-manifest"), 0o755); err != nil {
		t.Fatal(err)
	}
	rows, err := ListRuns(RunListOptions{All: true})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "unknown" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestInspectSummaryWithCounts(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRunManifest(t, "run-1", time.Now().UTC(), "input.mp4")
	runDir := filepath.Join(".byom-video", "runs", "run-1")
	writeFile(t, filepath.Join(runDir, "transcript.json"), `{"segments":[{},{}]}`)
	writeFile(t, filepath.Join(runDir, "chunks.json"), `{"chunks":[{}]}`)
	writeFile(t, filepath.Join(runDir, "highlights.json"), `{"highlights":[{},{}]}`)
	writeFile(t, filepath.Join(runDir, "roughcut.json"), `{"clips":[{}]}`)

	summary, err := Inspect("run-1")
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if summary.TranscriptSegmentCount == nil || *summary.TranscriptSegmentCount != 2 {
		t.Fatalf("TranscriptSegmentCount = %v", summary.TranscriptSegmentCount)
	}
	if summary.ChunkCount == nil || *summary.ChunkCount != 1 {
		t.Fatalf("ChunkCount = %v", summary.ChunkCount)
	}
	if summary.HighlightCount == nil || *summary.HighlightCount != 2 {
		t.Fatalf("HighlightCount = %v", summary.HighlightCount)
	}
	if summary.RoughcutClipCount == nil || *summary.RoughcutClipCount != 1 {
		t.Fatalf("RoughcutClipCount = %v", summary.RoughcutClipCount)
	}
}

func TestArtifactPathsFiltersByType(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRunManifest(t, "run-1", time.Now().UTC(), "input.mp4")
	paths, err := ArtifactPaths("run-1", "metadata")
	if err != nil {
		t.Fatalf("ArtifactPaths returned error: %v", err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "metadata.json" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestArtifactPathsRejectsUnknownType(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRunManifest(t, "run-1", time.Now().UTC(), "input.mp4")
	if _, err := ArtifactPaths("run-1", "unknown"); err == nil {
		t.Fatal("ArtifactPaths returned nil error")
	}
}

func TestInspectRejectsUnsafeRunID(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := Inspect("../outside"); err == nil {
		t.Fatal("Inspect returned nil error")
	}
}

func writeRunManifest(t *testing.T, runID string, createdAt time.Time, input string) {
	t.Helper()
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "/tmp/"+input, createdAt)
	m.Status = manifest.StatusCompleted
	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("metadata", "metadata.json")
	m.AddArtifact("transcript", "transcript.json")
	m.AddArtifact("chunks", "chunks.json")
	m.AddArtifact("highlights", "highlights.json")
	m.AddArtifact("roughcut", "roughcut.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runDir, "metadata.json"), `{}`)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

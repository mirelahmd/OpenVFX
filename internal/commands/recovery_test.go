package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/batch"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/watch"
)

func TestRetryBatchDryRunFindsFailedItems(t *testing.T) {
	t.Chdir(t.TempDir())
	summary := batch.Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       "batch-1",
		CreatedAt:     time.Now().UTC(),
		Preset:        "metadata",
		Items: []batch.Item{
			{InputPath: "/tmp/ok.mp4", Status: "completed"},
			{InputPath: "/tmp/fail.mp4", Status: "failed", Error: "boom"},
		},
	}
	if err := batch.WriteSummary(summary); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RetryBatch("batch-1", &out, RetryBatchOptions{DryRun: true}); err != nil {
		t.Fatalf("RetryBatch returned error: %v", err)
	}
	if strings.Contains(out.String(), "ok.mp4") || !strings.Contains(out.String(), "fail.mp4") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestRetryBatchMissingInputHandledCleanly(t *testing.T) {
	t.Chdir(t.TempDir())
	summary := batch.Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       "batch-1",
		CreatedAt:     time.Now().UTC(),
		Preset:        "metadata",
		Items:         []batch.Item{{InputPath: "/tmp/does-not-exist.mp4", Status: "failed"}},
	}
	if err := batch.WriteSummary(summary); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RetryBatch("batch-1", &out, RetryBatchOptions{}); err != nil {
		t.Fatalf("RetryBatch returned error: %v", err)
	}
	if !strings.Contains(out.String(), "input file unavailable") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestRetryWatchDryRunFindsFailedRegistryItems(t *testing.T) {
	t.Chdir(t.TempDir())
	registry := watch.Registry{SchemaVersion: "watch_processed.v1", Items: []watch.RegistryItem{
		{InputPath: "/tmp/ok.mp4", Fingerprint: "ok", Status: "completed"},
		{InputPath: "/tmp/fail.mp4", Fingerprint: "fail", Status: "failed"},
	}}
	if err := watch.SaveRegistry(&registry); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RetryWatch(&out, RetryWatchOptions{Preset: "metadata", RunOptions: RunOptions{}, DryRun: true}); err != nil {
		t.Fatalf("RetryWatch returned error: %v", err)
	}
	if strings.Contains(out.String(), "ok.mp4") || !strings.Contains(out.String(), "fail.mp4") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestInferPresetFromArtifacts(t *testing.T) {
	m := manifest.New("run-1", "/tmp/input.mp4", time.Now().UTC())
	if got := InferPreset(m); got != "metadata" {
		t.Fatalf("InferPreset = %q, want metadata", got)
	}
	m.AddArtifact("roughcut", "roughcut.json")
	if got := InferPreset(m); got != "shorts" {
		t.Fatalf("InferPreset = %q, want shorts", got)
	}
}

func TestCleanupDoesNotDeleteWithoutDelete(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeRecoveryManifest(t, "failed-run", manifest.StatusFailed)
	var out bytes.Buffer
	if err := Cleanup(&out, CleanupOptions{Failed: true}); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("run dir was deleted: %v", err)
	}
}

func TestCleanupDeleteRequiresConfirmation(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRecoveryManifest(t, "failed-run", manifest.StatusFailed)
	var out bytes.Buffer
	err := Cleanup(&out, CleanupOptions{Failed: true, Delete: true, ConfirmInput: strings.NewReader("no\n")})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err = %v", err)
	}
}

func TestCleanupDeleteWithYes(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeRecoveryManifest(t, "failed-run", manifest.StatusFailed)
	var out bytes.Buffer
	if err := Cleanup(&out, CleanupOptions{Failed: true, Delete: true, Yes: true}); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("run dir still exists or stat failed unexpectedly: %v", err)
	}
}

func writeRecoveryManifest(t *testing.T, runID string, status string) string {
	t.Helper()
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "/tmp/input.mp4", time.Now().UTC())
	m.Status = status
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	return runDir
}

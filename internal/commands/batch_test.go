package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/batch"
)

func TestBatchRejectsExportForMetadataPreset(t *testing.T) {
	var out bytes.Buffer
	err := Batch(t.TempDir(), &out, BatchOptions{Preset: "metadata", RunOptions: RunOptions{}, Export: true})
	if err == nil || !strings.Contains(err.Error(), "--export requires") {
		t.Fatalf("err = %v", err)
	}
}

func TestBatchesAndInspectBatch(t *testing.T) {
	t.Chdir(t.TempDir())
	summary := batch.Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       "batch-1",
		CreatedAt:     time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC),
		InputDir:      "/tmp/media",
		Preset:        "metadata",
		Totals:        batch.Totals{Attempted: 1, Succeeded: 1},
		Items:         []batch.Item{{InputPath: "/tmp/media/a.mp4", Status: "completed", RunID: "run-1"}},
	}
	if err := batch.WriteSummary(summary); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Batches(&out); err != nil {
		t.Fatalf("Batches returned error: %v", err)
	}
	if !strings.Contains(out.String(), "batch-1") {
		t.Fatalf("unexpected batches output: %s", out.String())
	}
	out.Reset()
	if err := InspectBatch("batch-1", &out, InspectBatchOptions{JSON: true}); err != nil {
		t.Fatalf("InspectBatch returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"batch_id": "batch-1"`) {
		t.Fatalf("unexpected inspect output: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(".byom-video", "batches", "batch-1", "batch_summary.json")); err != nil {
		t.Fatal(err)
	}
}

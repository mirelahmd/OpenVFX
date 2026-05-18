package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/watch"
)

func TestWatchRejectsExportForMetadataPreset(t *testing.T) {
	err := Watch(t.TempDir(), &bytes.Buffer{}, WatchOptions{Preset: "metadata", RunOptions: RunOptions{}, Export: true, IntervalSeconds: 5})
	if err == nil || !strings.Contains(err.Error(), "--export requires") {
		t.Fatalf("err = %v", err)
	}
}

func TestWatchStatusJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	registry := watch.Registry{
		SchemaVersion: "watch_processed.v1",
		UpdatedAt:     time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC),
		Items: []watch.RegistryItem{{
			InputPath:   "/tmp/a.mp4",
			Fingerprint: "fp",
			ProcessedAt: time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC),
			RunID:       "run-1",
			Status:      "completed",
		}},
	}
	if err := watch.SaveRegistry(&registry); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WatchStatus(&out, WatchStatusOptions{JSON: true}); err != nil {
		t.Fatalf("WatchStatus returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"schema_version": "watch_processed.v1"`) || !strings.Contains(out.String(), `"run_id": "run-1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

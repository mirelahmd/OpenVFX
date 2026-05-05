package batch

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsMediaFile(t *testing.T) {
	for _, name := range []string{"a.mp4", "a.MOV", "a.m4v", "a.mp3", "a.wav", "a.m4a", "a.aac", "a.flac", "a.webm", "a.mkv"} {
		if !IsMediaFile(name) {
			t.Fatalf("IsMediaFile(%q) = false, want true", name)
		}
	}
	if IsMediaFile("notes.txt") {
		t.Fatal("IsMediaFile(notes.txt) = true, want false")
	}
}

func TestDiscoverMediaFilesNonRecursiveSkipsHiddenAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.mov"))
	writeFile(t, filepath.Join(dir, "a.mp4"))
	writeFile(t, filepath.Join(dir, ".hidden.mp4"))
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "nested", "c.mp4"))
	files, err := DiscoverMediaFiles(dir, false)
	if err != nil {
		t.Fatalf("DiscoverMediaFiles returned error: %v", err)
	}
	if got := basenames(files); strings.Join(got, ",") != "a.mp4,b.mov" {
		t.Fatalf("files = %#v", got)
	}
}

func TestDiscoverMediaFilesRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mp4"))
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "nested", "b.mov"))
	if err := os.Mkdir(filepath.Join(dir, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".hidden", "c.mp4"))
	files, err := DiscoverMediaFiles(dir, true)
	if err != nil {
		t.Fatalf("DiscoverMediaFiles returned error: %v", err)
	}
	if got := basenames(files); strings.Join(got, ",") != "a.mp4,b.mov" {
		t.Fatalf("files = %#v", got)
	}
}

func TestRunLimitAndSummaryGeneration(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "a.mp4"))
	writeFile(t, filepath.Join(inputDir, "b.mp4"))
	var out bytes.Buffer
	summary, err := Run(Options{InputDir: inputDir, Preset: "metadata", Limit: 1}, Hooks{
		Now: fixedNow,
		Run: func(inputPath string, stdout io.Writer) error {
			fmt.Fprintln(stdout, "Run completed")
			fmt.Fprintln(stdout, "  run id:        run-a")
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if summary.Totals.Discovered != 2 || summary.Totals.Attempted != 1 || summary.Totals.Skipped != 1 || summary.Totals.Succeeded != 1 {
		t.Fatalf("totals = %#v", summary.Totals)
	}
	if _, err := os.Stat(filepath.Join(".byom-video", "batches", summary.BatchID, "batch_summary.json")); err != nil {
		t.Fatalf("summary was not written: %v", err)
	}
}

func TestDryRunDoesNotCreateSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "a.mp4"))
	var out bytes.Buffer
	summary, err := Run(Options{InputDir: inputDir, Preset: "metadata", DryRun: true}, Hooks{
		Now: fixedNow,
		Run: func(inputPath string, stdout io.Writer) error {
			t.Fatal("run hook should not be called for dry run")
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !summary.DryRun || len(summary.Items) != 1 || summary.Items[0].Status != "dry_run" {
		t.Fatalf("summary = %#v", summary)
	}
	if _, err := os.Stat(".byom-video"); !os.IsNotExist(err) {
		t.Fatalf("dry run created artifacts or unexpected stat error: %v", err)
	}
}

func TestFailFastStopsAfterFirstFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "a.mp4"))
	writeFile(t, filepath.Join(inputDir, "b.mp4"))
	attempts := 0
	var out bytes.Buffer
	summary, err := Run(Options{InputDir: inputDir, Preset: "metadata", FailFast: true}, Hooks{
		Now: fixedNow,
		Run: func(inputPath string, stdout io.Writer) error {
			attempts++
			return fmt.Errorf("boom")
		},
	}, &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if attempts != 1 || summary.Totals.Attempted != 1 || summary.Totals.Failed != 1 {
		t.Fatalf("attempts=%d totals=%#v", attempts, summary.Totals)
	}
}

func TestListAndReadSummaries(t *testing.T) {
	t.Chdir(t.TempDir())
	summary := Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       "batch-1",
		CreatedAt:     fixedNow(),
		InputDir:      "/tmp/media",
		Preset:        "metadata",
		Totals:        Totals{Attempted: 1, Succeeded: 1},
		Items:         []Item{{InputPath: "/tmp/media/a.mp4", Status: "completed", RunID: "run-1"}},
	}
	if err := WriteSummary(summary); err != nil {
		t.Fatal(err)
	}
	read, err := ReadSummary("batch-1")
	if err != nil {
		t.Fatalf("ReadSummary returned error: %v", err)
	}
	if read.BatchID != "batch-1" || read.Totals.Succeeded != 1 {
		t.Fatalf("read = %#v", read)
	}
	list, err := ListSummaries()
	if err != nil {
		t.Fatalf("ListSummaries returned error: %v", err)
	}
	if len(list) != 1 || list[0].BatchID != "batch-1" {
		t.Fatalf("list = %#v", list)
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func basenames(paths []string) []string {
	out := []string{}
	for _, path := range paths {
		out = append(out, filepath.Base(path))
	}
	return out
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC)
}

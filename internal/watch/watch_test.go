package watch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStableFileDetection(t *testing.T) {
	now := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	current := FileState{Path: "/tmp/a.mp4", Size: 10, ModTime: now.Add(-10 * time.Second)}
	if !IsStable(FileState{}, current, false, 5*time.Second, now) {
		t.Fatal("old first-seen file should be stable")
	}
	newFile := FileState{Path: "/tmp/a.mp4", Size: 10, ModTime: now.Add(-1 * time.Second)}
	if IsStable(FileState{}, newFile, false, 5*time.Second, now) {
		t.Fatal("recent first-seen file should not be stable")
	}
	if !IsStable(current, current, true, 5*time.Second, now) {
		t.Fatal("unchanged seen file should be stable")
	}
	changed := FileState{Path: "/tmp/a.mp4", Size: 11, ModTime: current.ModTime}
	if IsStable(current, changed, true, 5*time.Second, now) {
		t.Fatal("changed file should not be stable")
	}
}

func TestFingerprintGeneration(t *testing.T) {
	state := FileState{Path: "/tmp/a.mp4", Size: 12, ModTime: time.Unix(1, 2).UTC()}
	got := Fingerprint(state)
	if !strings.Contains(got, "/tmp/a.mp4") || !strings.Contains(got, ":12:") {
		t.Fatalf("fingerprint = %q", got)
	}
}

func TestRegistryLoadSaveUpdate(t *testing.T) {
	t.Chdir(t.TempDir())
	registry, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	item := RegistryItem{InputPath: "/tmp/a.mp4", Fingerprint: "fp", ProcessedAt: fixedWatchNow(), Status: "completed"}
	registry.Upsert(item, fixedWatchNow())
	if err := SaveRegistry(&registry); err != nil {
		t.Fatalf("SaveRegistry returned error: %v", err)
	}
	read, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	if !read.Has("fp") || len(read.Items) != 1 {
		t.Fatalf("registry = %#v", read)
	}
	item.Status = "failed"
	registry.Upsert(item, fixedWatchNow())
	if len(registry.Items) != 1 || registry.Items[0].Status != "failed" {
		t.Fatalf("upsert registry = %#v", registry)
	}
}

func TestWatchOnceProcessesExpectedFilesAndSkipsHidden(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeWatchFile(t, filepath.Join(inputDir, "a.mp4"), fixedWatchNow().Add(-10*time.Second))
	writeWatchFile(t, filepath.Join(inputDir, ".hidden.mp4"), fixedWatchNow().Add(-10*time.Second))
	var out bytes.Buffer
	runs := 0
	err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second}, Hooks{
		Now: fixedWatchNow,
		Run: func(inputPath string, stdout io.Writer) error {
			runs++
			fmt.Fprintln(stdout, "Run completed")
			fmt.Fprintln(stdout, "  run id:        run-1")
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
	registry, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Items) != 1 || registry.Items[0].RunID != "run-1" {
		t.Fatalf("registry = %#v", registry)
	}
}

func TestRegistryPreventsReprocessing(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	path := filepath.Join(inputDir, "a.mp4")
	writeWatchFile(t, path, fixedWatchNow().Add(-10*time.Second))
	runHook := func(inputPath string, stdout io.Writer) error {
		fmt.Fprintln(stdout, "Run completed")
		fmt.Fprintln(stdout, "  run id:        run-1")
		return nil
	}
	if err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second}, Hooks{Now: fixedWatchNow, Run: runHook}, io.Discard); err != nil {
		t.Fatal(err)
	}
	runs := 0
	if err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second}, Hooks{
		Now: fixedWatchNow,
		Run: func(inputPath string, stdout io.Writer) error {
			runs++
			return nil
		},
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("runs = %d, want 0", runs)
	}
}

func TestIgnoreRegistryAllowsReprocessing(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeWatchFile(t, filepath.Join(inputDir, "a.mp4"), fixedWatchNow().Add(-10*time.Second))
	runHook := func(inputPath string, stdout io.Writer) error {
		fmt.Fprintln(stdout, "Run completed")
		fmt.Fprintln(stdout, "  run id:        run-1")
		return nil
	}
	if err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second}, Hooks{Now: fixedWatchNow, Run: runHook}, io.Discard); err != nil {
		t.Fatal(err)
	}
	runs := 0
	if err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second, IgnoreRegistry: true}, Hooks{
		Now: fixedWatchNow,
		Run: func(inputPath string, stdout io.Writer) error {
			runs++
			fmt.Fprintln(stdout, "Run completed")
			fmt.Fprintln(stdout, "  run id:        run-2")
			return nil
		},
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

func TestWatchLimit(t *testing.T) {
	t.Chdir(t.TempDir())
	inputDir := t.TempDir()
	writeWatchFile(t, filepath.Join(inputDir, "a.mp4"), fixedWatchNow().Add(-10*time.Second))
	writeWatchFile(t, filepath.Join(inputDir, "b.mp4"), fixedWatchNow().Add(-10*time.Second))
	runs := 0
	if err := Run(context.Background(), Options{InputDir: inputDir, Preset: "metadata", Once: true, Interval: 5 * time.Second, Limit: 1}, Hooks{
		Now: fixedWatchNow,
		Run: func(inputPath string, stdout io.Writer) error {
			runs++
			fmt.Fprintln(stdout, "Run completed")
			fmt.Fprintln(stdout, "  run id:        run-1")
			return nil
		},
	}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

func writeWatchFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func fixedWatchNow() time.Time {
	return time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC)
}

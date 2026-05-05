package exporter

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverExportedFiles(t *testing.T) {
	runDir := t.TempDir()
	exportsDir := filepath.Join(runDir, "exports")
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportsDir, "clip_0002.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(exportsDir, "clip_0001.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := DiscoverExportedFiles(runDir)
	if err != nil {
		t.Fatalf("DiscoverExportedFiles returned error: %v", err)
	}
	want := []string{"exports/clip_0001.mp4", "exports/clip_0002.mp4"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
}

func TestApplyProbeMetadata(t *testing.T) {
	data := []byte(`{
		"format": {"duration": "4.480000"},
		"streams": [
			{"codec_type": "video"},
			{"codec_type": "audio"},
			{"codec_type": "audio"}
		]
	}`)
	file := ExportValidationFile{Path: "exports/clip_0001.mp4"}
	if err := ApplyProbeMetadata(data, &file); err != nil {
		t.Fatalf("ApplyProbeMetadata returned error: %v", err)
	}
	if file.DurationSeconds == nil || *file.DurationSeconds != 4.48 {
		t.Fatalf("duration = %v, want 4.48", file.DurationSeconds)
	}
	if file.VideoStreams != 1 || file.AudioStreams != 2 {
		t.Fatalf("streams = video %d audio %d, want 1/2", file.VideoStreams, file.AudioStreams)
	}
}

func TestDiscoverExportedFilesMissingDirectory(t *testing.T) {
	files, err := DiscoverExportedFiles(t.TempDir())
	if err != nil {
		t.Fatalf("DiscoverExportedFiles returned error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files = %#v, want empty", files)
	}
}

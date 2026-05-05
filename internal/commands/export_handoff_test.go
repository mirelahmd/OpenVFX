package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"byom-video/internal/exportartifacts"
	"byom-video/internal/exporter"
	"byom-video/internal/manifest"
)

func TestSelectedClipsFromEnhancedRoughcut(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ClipCardsCommand(runID, &bytes.Buffer{}, ClipCardsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := EnhanceRoughcut(runID, &bytes.Buffer{}, EnhanceRoughcutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := exportartifacts.ReadSelectedClips(filepath.Join(runDir, "selected_clips.json"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source.EnhancedRoughcutArtifact != "enhanced_roughcut.json" {
		t.Fatalf("unexpected source: %+v", doc.Source)
	}
	if len(doc.Clips) != 1 || doc.Clips[0].OutputFilename != "clip_0001.mp4" {
		t.Fatalf("unexpected clips: %+v", doc.Clips)
	}
}

func TestSelectedClipsFallbackToRoughcut(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := exportartifacts.ReadSelectedClips(filepath.Join(runDir, "selected_clips.json"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source.RoughcutArtifact != "roughcut.json" {
		t.Fatalf("unexpected source: %+v", doc.Source)
	}
}

func TestExportManifestBeforeExportsExist(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ExportManifestCommand(runID, &out, ExportManifestOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "planned:   1") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestExportManifestAfterExportsExistWithValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "exports"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "exports", "clip_0001.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	duration := 4.48
	validation := exporter.ExportValidation{
		SchemaVersion: "export_validation.v1",
		ExportsDir:    "exports",
		CheckedAt:     time.Now().UTC(),
		Files: []exporter.ExportValidationFile{
			{Path: "exports/clip_0001.mp4", Exists: true, DurationSeconds: &duration, Status: "ok"},
		},
	}
	if err := exporter.WriteExportValidation(runDir, validation); err != nil {
		t.Fatal(err)
	}
	if err := ExportManifestCommand(runID, &bytes.Buffer{}, ExportManifestOptions{}); err != nil {
		t.Fatal(err)
	}
	doc, err := exportartifacts.ReadExportManifest(filepath.Join(runDir, "export_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Summary.Exported != 1 || doc.Summary.Validated != 1 || !doc.Clips[0].Exists || !doc.Clips[0].Validated {
		t.Fatalf("unexpected manifest: %+v", doc)
	}
}

func TestFFmpegScriptCommandModes(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := FFmpegScriptCommand(runID, &bytes.Buffer{}, FFmpegScriptCommandOptions{Mode: "stream-copy", Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "ffmpeg_commands.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "-c copy") {
		t.Fatalf("expected stream-copy script: %s", string(data))
	}
	if err := FFmpegScriptCommand(runID, &bytes.Buffer{}, FFmpegScriptCommandOptions{Mode: "reencode", Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(runDir, "ffmpeg_commands.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "-c:v libx264 -c:a aac") {
		t.Fatalf("expected reencode script: %s", string(data))
	}
}

func TestConcatPlanWritesArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ConcatPlanCommand(runID, &bytes.Buffer{}, ConcatPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"concat_list.txt", "ffmpeg_concat.sh"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestValidateCatchesInvalidSelectedClipTiming(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeCommandRunManifest(t, "run-1")
	writeEventsForValidation(t, runDir)
	doc := exportartifacts.SelectedClips{
		SchemaVersion: "selected_clips.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         "run-1",
		InputPath:     "/tmp/input.mp4",
		Clips: []exportartifacts.SelectedClip{
			{ID: "clip_0001", Order: 1, Start: 9, End: 2, DurationSeconds: 0, Title: "Bad", Description: "Bad", OutputFilename: "clip_0001.mp4"},
		},
	}
	if err := writeJSONFile(filepath.Join(runDir, "selected_clips.json"), doc); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("events", "events.jsonl")
	m.AddArtifact("selected_clips", "selected_clips.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Validate("run-1", &out, ValidateOptions{})
	if err == nil || !strings.Contains(out.String(), "selected_clips.json") {
		t.Fatalf("expected selected_clips validation failure, err=%v out=%s", err, out.String())
	}
}

func TestInspectShowsSelectedAndExportSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)
	if err := SelectedClipsCommand(runID, &bytes.Buffer{}, SelectedClipsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExportManifestCommand(runID, &bytes.Buffer{}, ExportManifestOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ConcatPlanCommand(runID, &bytes.Buffer{}, ConcatPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Inspect(runID, &out, InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "selected clips:") || !strings.Contains(text, "export manifest:") || !strings.Contains(text, "concat plan:") {
		t.Fatalf("inspect output missing export handoff summary: %s", text)
	}
}

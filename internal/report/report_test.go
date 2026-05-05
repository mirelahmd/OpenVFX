package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"byom-video/internal/manifest"
)

func TestEscape(t *testing.T) {
	got := Escape(`<script>alert("x")</script>`)
	want := `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`
	if got != want {
		t.Fatalf("Escape = %q, want %q", got, want)
	}
}

func TestWriteMinimalReport(t *testing.T) {
	runDir := t.TempDir()
	m := manifest.New("run-1", "/tmp/input.mp4", time.Date(2026, 4, 28, 1, 2, 3, 0, time.UTC))
	m.Status = manifest.StatusCompleted
	m.AddArtifact("manifest", "manifest.json")

	summary, err := Write(runDir, m)
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if summary.ArtifactPath != "report.html" {
		t.Fatalf("ArtifactPath = %q", summary.ArtifactPath)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.html"))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	text := string(data)
	for _, want := range []string{"BYOM Video Run Report", "run-1", "/tmp/input.mp4", "manifest.json"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q: %s", want, text)
		}
	}
}

func TestWriteReportWithHighlightsAndRoughcut(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "highlights.json"), []byte(`{"highlights":[{"id":"hl_0001","start":1,"end":2,"score":0.9,"text":"the key is speed","reason":"hook"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "roughcut.json"), []byte(`{"plan":{"total_duration_seconds":1},"clips":[{"id":"clip_0001","highlight_id":"hl_0001","source_chunk_id":"chunk_0001","start":1,"end":2,"duration_seconds":1,"order":1,"score":0.9,"text":"the key is speed"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.New("run-1", "/tmp/input.mp4", time.Now().UTC())
	m.Status = manifest.StatusCompleted

	if _, err := Write(runDir, m); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Highlights", "Rough Cut", "the key is speed", "clip_0001"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

func TestWriteReportWithClipCardsAndEnhancedRoughcut(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runDir, "expansions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "clip_cards.json"), []byte(`{"schema_version":"clip_cards.v1","created_at":"2026-05-02T00:00:00Z","run_id":"run-1","source":{"roughcut_artifact":"roughcut.json"},"cards":[{"id":"card_0001","clip_id":"clip_0001","decision_id":"decision_0001","start":1,"end":2,"duration_seconds":1,"title":"Label: speed","description":"Short description","captions":["Caption one"],"source_text":"the key is speed","edit_intent":"Keep pace","verification_status":"passed","warnings":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "enhanced_roughcut.json"), []byte(`{"schema_version":"enhanced_roughcut.v1","created_at":"2026-05-02T00:00:00Z","run_id":"run-1","source":{"roughcut_artifact":"roughcut.json","clip_cards_artifact":"clip_cards.json"},"plan":{"title":"Enhanced Rough Cut Plan","intent":"Editor plan","total_duration_seconds":1},"clips":[{"id":"clip_0001","start":1,"end":2,"order":1,"title":"Label: speed","description":"Short description","caption_suggestions":["Caption one"],"edit_intent":"Keep pace","verification_status":"passed","source_text":"the key is speed"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "selected_clips.json"), []byte(`{"schema_version":"selected_clips.v1","created_at":"2026-05-02T00:00:00Z","run_id":"run-1","source":{"enhanced_roughcut_artifact":"enhanced_roughcut.json","clip_cards_artifact":"clip_cards.json","roughcut_artifact":"roughcut.json"},"input_path":"/tmp/input.mp4","clips":[{"id":"clip_0001","order":1,"start":1,"end":2,"duration_seconds":1,"title":"Label: speed","description":"Short description","caption_suggestions":["Caption one"],"source_text":"the key is speed","output_filename":"clip_0001.mp4"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "export_manifest.json"), []byte(`{"schema_version":"export_manifest.v1","created_at":"2026-05-02T00:00:00Z","run_id":"run-1","input_path":"/tmp/input.mp4","exports_dir":"exports","clips":[{"id":"clip_0001","order":1,"planned_output":"exports/clip_0001.mp4","exists":true,"validated":true,"duration_seconds":1,"title":"Label: speed","description":"Short description"}],"summary":{"planned":1,"exported":1,"validated":1,"missing":0}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "concat_list.txt"), []byte("file 'exports/clip_0001.mp4'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "ffmpeg_concat.sh"), []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "verification_results.json"), []byte(`{"schema_version":"verification_results.v1","created_at":"2026-05-02T00:00:00Z","run_id":"run-1","mode":"deterministic","source":{"inference_mask_artifact":"inference_mask.json","verification_artifact":"verification.json","expansion_artifacts":["expansions/caption_variants.json"]},"status":"passed","summary":{"checks_total":4,"checks_passed":4,"checks_failed":0,"warnings":0},"checks":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "ffmpeg_commands.sh"), []byte("#!/usr/bin/env bash\n# mode: reencode\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "expansions", "caption_variants.json"), []byte(`{"schema_version":"expansion_output.v1","created_at":"2026-05-02T00:00:00Z","mode":"stub","task_type":"caption_variants","source":{"inference_mask_artifact":"inference_mask.json","expansion_tasks_artifact":"expansion_tasks.json","task_ids":["task_0001"]},"items":[{"id":"cap_1","task_id":"task_0001","decision_id":"decision_0001","text":"Caption one","start":1,"end":2,"metadata":{}}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.New("run-1", "/tmp/input.mp4", time.Now().UTC())
	m.Status = manifest.StatusCompleted

	if _, err := Write(runDir, m); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Clip Cards", "Enhanced Roughcut", "Selected Clips", "Export Manifest", "Concat Plan", "Expansion Outputs", "Verification Summary", "Caption one", "Mode: <strong>reencode</strong>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

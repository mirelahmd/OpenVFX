package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSyntheticTimelineSource_WholeVideo(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/video.mp4", 30.0, "make it look cinematic")
	if tls.SchemaVersion != timelineSourceSchema {
		t.Errorf("want schema %q, got %q", timelineSourceSchema, tls.SchemaVersion)
	}
	if tls.Source != "synthetic_time_based" {
		t.Errorf("want source=synthetic_time_based, got %q", tls.Source)
	}
	if len(tls.Clips) != 1 {
		t.Fatalf("want 1 clip (whole video, no repeat), got %d", len(tls.Clips))
	}
	c := tls.Clips[0]
	if c.SourceStart != 0 || c.SourceEnd != 30 {
		t.Errorf("want source_start=0 source_end=30, got %.2f–%.2f", c.SourceStart, c.SourceEnd)
	}
	if c.Start != c.SourceStart || c.End != c.SourceEnd {
		t.Error("start/end aliases must match source_start/source_end")
	}
	if c.SourcePath != "/tmp/video.mp4" {
		t.Errorf("want source_path=/tmp/video.mp4, got %q", c.SourcePath)
	}
	if c.DurationSeconds != 30.0 {
		t.Errorf("want duration_seconds=30, got %.2f", c.DurationSeconds)
	}
	if c.RepeatIndex != 1 {
		t.Errorf("want repeat_index=1, got %d", c.RepeatIndex)
	}
}

func TestBuildSyntheticTimelineSource_FirstNSeconds(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 60.0, "use the first 10 seconds")
	if len(tls.Clips) != 1 {
		t.Fatalf("want 1 clip, got %d", len(tls.Clips))
	}
	c := tls.Clips[0]
	if c.SourceStart != 0 || c.SourceEnd != 10 {
		t.Errorf("want 0–10, got %.2f–%.2f", c.SourceStart, c.SourceEnd)
	}
}

func TestBuildSyntheticTimelineSource_LoopRepeat(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 10.0, "loop the last 2 seconds 3 times")
	if len(tls.Clips) != 3 {
		t.Fatalf("want 3 clips (repeat=3), got %d", len(tls.Clips))
	}
	for i, c := range tls.Clips {
		if c.RepeatIndex != i+1 {
			t.Errorf("clip %d: want repeat_index=%d, got %d", i, i+1, c.RepeatIndex)
		}
		if c.SourceStart != 8 || c.SourceEnd != 10 {
			t.Errorf("clip %d: want 8–10, got %.2f–%.2f", i, c.SourceStart, c.SourceEnd)
		}
	}
}

func TestBuildSyntheticTimelineSource_ClipIDs(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 5.0, "whole video twice")
	if len(tls.Clips) != 2 {
		t.Fatalf("want 2 clips, got %d", len(tls.Clips))
	}
	if tls.Clips[0].ID == tls.Clips[1].ID {
		t.Error("clip IDs must be unique")
	}
	if !strings.HasPrefix(tls.Clips[0].ID, "fallback_clip_") {
		t.Errorf("unexpected clip ID prefix: %q", tls.Clips[0].ID)
	}
}

func TestBuildSyntheticTimelineSource_Warnings(t *testing.T) {
	// First 100s of a 5s video should produce a clamping warning
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 5.0, "first 100 seconds")
	if len(tls.Warnings) == 0 {
		t.Error("expected clamp warning, got none")
	}
}

func TestWriteTimelineSourceToRun(t *testing.T) {
	// Create a temp run dir to simulate the runstore layout
	tmp := t.TempDir()
	runID := "test-run-001"
	runDir := filepath.Join(tmp, ".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Override the runstore root for this test by writing directly
	outPath := filepath.Join(runDir, timelineSourceArtifact)
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 20.0, "first 10 seconds")
	data, err := json.MarshalIndent(tls, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Round-trip: read back and validate
	readData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got TimelineSourceArtifact
	if err := json.Unmarshal(readData, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SchemaVersion != timelineSourceSchema {
		t.Errorf("want schema %q, got %q", timelineSourceSchema, got.SchemaVersion)
	}
	if len(got.Clips) == 0 {
		t.Error("expected at least one clip after round-trip")
	}
	if got.Clips[0].Start != got.Clips[0].SourceStart {
		t.Error("start alias must equal source_start after round-trip")
	}
}

func TestSyntheticTimelineFallbackMessage_WithClips(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 10.0, "loop the last 2 seconds 3 times")
	msg := SyntheticTimelineFallbackMessage(tls)
	if msg == "" {
		t.Error("expected non-empty message")
	}
	if !strings.Contains(msg, "synthetic time-based timeline source") {
		t.Errorf("message missing expected phrase, got: %q", msg)
	}
}

func TestSyntheticTimelineFallbackMessage_NoClips(t *testing.T) {
	tls := TimelineSourceArtifact{}
	msg := SyntheticTimelineFallbackMessage(tls)
	if !strings.Contains(msg, "whole video") {
		t.Errorf("no-clips message should mention 'whole video', got: %q", msg)
	}
}

func TestReadClipsFromArtifact_TimelineSource(t *testing.T) {
	tls := BuildSyntheticTimelineSource("/tmp/v.mp4", 30.0, "first 10 seconds twice")
	data, err := json.Marshal(tls)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmp := t.TempDir()
	tsPath := filepath.Join(tmp, "timeline_source.json")
	if err := os.WriteFile(tsPath, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	clips, err := readClipsFromArtifact(tsPath)
	if err != nil {
		t.Fatalf("readClipsFromArtifact: %v", err)
	}
	if len(clips) != 2 {
		t.Fatalf("want 2 clips, got %d", len(clips))
	}
	for _, c := range clips {
		if c.SourcePath != "/tmp/v.mp4" {
			t.Errorf("want source_path=/tmp/v.mp4, got %q", c.SourcePath)
		}
		if c.Start != 0 || c.End != 10 {
			t.Errorf("want 0–10, got %.2f–%.2f", c.Start, c.End)
		}
	}
}

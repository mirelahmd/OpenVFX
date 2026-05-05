package captions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSRTTimestamp(t *testing.T) {
	got := FormatSRTTimestamp(3661.234)
	want := "01:01:01,234"
	if got != want {
		t.Fatalf("FormatSRTTimestamp = %q, want %q", got, want)
	}
}

func TestWriteFromTranscriptOneSegment(t *testing.T) {
	transcript := writeTranscript(t, `[{"start":0,"end":2.5,"text":"Hello world"}]`)
	output := filepath.Join(t.TempDir(), "captions.srt")

	summary, err := WriteFromTranscript(transcript, output)
	if err != nil {
		t.Fatalf("WriteFromTranscript returned error: %v", err)
	}
	if summary.CueCount != 1 {
		t.Fatalf("CueCount = %d, want 1", summary.CueCount)
	}
	data, _ := os.ReadFile(output)
	if !strings.Contains(string(data), "00:00:00,000 --> 00:00:02,500") {
		t.Fatalf("SRT missing timestamp: %s", string(data))
	}
}

func TestWriteFromTranscriptMultipleSegments(t *testing.T) {
	transcript := writeTranscript(t, `[{"start":0,"end":1,"text":"One"},{"start":1,"end":2,"text":"Two"}]`)
	output := filepath.Join(t.TempDir(), "captions.srt")

	summary, err := WriteFromTranscript(transcript, output)
	if err != nil {
		t.Fatalf("WriteFromTranscript returned error: %v", err)
	}
	if summary.CueCount != 2 {
		t.Fatalf("CueCount = %d, want 2", summary.CueCount)
	}
	data, _ := os.ReadFile(output)
	if !strings.Contains(string(data), "2\n00:00:01,000 --> 00:00:02,000") {
		t.Fatalf("SRT missing second cue: %s", string(data))
	}
}

func writeTranscript(t *testing.T, segments string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.json")
	content := `{"schema_version":"transcript.v1","source":{"mode":"stub"},"segments":` + segments + `}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

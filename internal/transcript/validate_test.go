package transcript

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFileAcceptsValidTranscript(t *testing.T) {
	path := writeTranscript(t, `{
	  "schema_version": "transcript.v1",
	  "source": {
	    "input_path": "input.mp4",
	    "mode": "real",
	    "engine": "faster-whisper",
	    "model_size": "tiny"
	  },
	  "language": "en",
	  "duration_seconds": 5.0,
	  "segments": [
	    {"id": "seg_0001", "start": 0.0, "end": 1.0, "text": "hello"}
	  ]
	}`)

	summary, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile returned error: %v", err)
	}
	if summary.Language != "en" {
		t.Fatalf("Language = %q, want en", summary.Language)
	}
	if summary.SegmentCount != 1 {
		t.Fatalf("SegmentCount = %d, want 1", summary.SegmentCount)
	}
	if summary.DurationSeconds == nil || *summary.DurationSeconds != 5.0 {
		t.Fatalf("DurationSeconds = %v, want 5.0", summary.DurationSeconds)
	}
	if summary.ModelSize != "tiny" {
		t.Fatalf("ModelSize = %q, want tiny", summary.ModelSize)
	}
}

func TestValidateFileRejectsInvalidTranscript(t *testing.T) {
	path := writeTranscript(t, `{
	  "schema_version": "transcript.v1",
	  "source": {"mode": "real"},
	  "segments": [
	    {"id": "seg_0001", "start": 2.0, "end": 1.0, "text": "bad"}
	  ]
	}`)

	_, err := ValidateFile(path)
	if err == nil {
		t.Fatal("ValidateFile returned nil error")
	}
}

func writeTranscript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

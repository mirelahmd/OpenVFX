package roughcut

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFromHighlightsSelectsMaxClips(t *testing.T) {
	path := writeHighlights(t, []highlight{
		{ID: "hl_0001", ChunkID: "chunk_0001", Start: 0, End: 5, DurationSeconds: 5, Score: 0.9, Text: "one"},
		{ID: "hl_0002", ChunkID: "chunk_0002", Start: 5, End: 10, DurationSeconds: 5, Score: 0.8, Text: "two"},
	})
	output := filepath.Join(t.TempDir(), "roughcut.json")

	summary, err := WriteFromHighlights(path, output, Options{MaxClips: 1})
	if err != nil {
		t.Fatalf("WriteFromHighlights returned error: %v", err)
	}
	if summary.ClipCount != 1 {
		t.Fatalf("ClipCount = %d, want 1", summary.ClipCount)
	}
}

func TestWriteFromHighlightsOrdersClipsByTimeline(t *testing.T) {
	path := writeHighlights(t, []highlight{
		{ID: "hl_0001", ChunkID: "chunk_0001", Start: 10, End: 15, DurationSeconds: 5, Score: 0.9, Text: "late"},
		{ID: "hl_0002", ChunkID: "chunk_0002", Start: 0, End: 5, DurationSeconds: 5, Score: 0.8, Text: "early"},
	})
	output := filepath.Join(t.TempDir(), "roughcut.json")

	_, err := WriteFromHighlights(path, output, Options{MaxClips: 2})
	if err != nil {
		t.Fatalf("WriteFromHighlights returned error: %v", err)
	}

	doc := readRoughcut(t, output)
	if doc.Clips[0].HighlightID != "hl_0002" {
		t.Fatalf("first clip highlight = %q, want hl_0002", doc.Clips[0].HighlightID)
	}
}

func TestValidateFileAcceptsValidRoughcut(t *testing.T) {
	path := writeRoughcut(t, `{"schema_version":"roughcut.v1","plan":{"total_duration_seconds":2},"clips":[{"id":"clip_0001","highlight_id":"hl_0001","source_chunk_id":"chunk_0001","start":0,"end":2,"duration_seconds":2,"order":1,"score":0.5,"edit_intent":"Keep","text":"hello"}]}`)

	summary, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile returned error: %v", err)
	}
	if summary.ClipCount != 1 {
		t.Fatalf("ClipCount = %d, want 1", summary.ClipCount)
	}
}

func TestValidateFileRejectsInvalidClipTiming(t *testing.T) {
	path := writeRoughcut(t, `{"schema_version":"roughcut.v1","clips":[{"id":"clip_0001","highlight_id":"hl_0001","source_chunk_id":"chunk_0001","start":2,"end":1,"duration_seconds":-1,"order":1,"score":0.5,"edit_intent":"Keep","text":"hello"}]}`)

	_, err := ValidateFile(path)
	if err == nil {
		t.Fatal("ValidateFile returned nil error")
	}
}

func writeHighlights(t *testing.T, highlights []highlight) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "highlights.json")
	doc := struct {
		SchemaVersion string      `json:"schema_version"`
		Highlights    []highlight `json:"highlights"`
	}{SchemaVersion: "highlights.v1", Highlights: highlights}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func writeRoughcut(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "roughcut.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func readRoughcut(t *testing.T, path string) Document {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	return doc
}

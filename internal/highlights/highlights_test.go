package highlights

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHighlightScoringWithHookPhrase(t *testing.T) {
	path := writeChunks(t, []chunk{{ID: "chunk_0001", Start: 0, End: 10, DurationSeconds: 10, Text: "Here is why the problem is important", WordCount: 8}})
	output := filepath.Join(t.TempDir(), "highlights.json")

	_, err := WriteFromChunks(path, output, Options{MinDurationSeconds: 3, MaxDurationSeconds: 90, TopK: 10})
	if err != nil {
		t.Fatalf("WriteFromChunks returned error: %v", err)
	}

	doc := readHighlights(t, output)
	if len(doc.Highlights) != 1 {
		t.Fatalf("highlight count = %d, want 1", len(doc.Highlights))
	}
	if !doc.Highlights[0].Signals.HasHookPhrase {
		t.Fatal("HasHookPhrase = false, want true")
	}
}

func TestHighlightScoringWithQuestion(t *testing.T) {
	path := writeChunks(t, []chunk{{ID: "chunk_0001", Start: 0, End: 10, DurationSeconds: 10, Text: "What matters when this happens?", WordCount: 5}})
	output := filepath.Join(t.TempDir(), "highlights.json")

	_, err := WriteFromChunks(path, output, Options{MinDurationSeconds: 3, MaxDurationSeconds: 90, TopK: 10})
	if err != nil {
		t.Fatalf("WriteFromChunks returned error: %v", err)
	}

	doc := readHighlights(t, output)
	if !doc.Highlights[0].Signals.HasQuestion {
		t.Fatal("HasQuestion = false, want true")
	}
}

func TestHighlightSortingAndTopK(t *testing.T) {
	path := writeChunks(t, []chunk{
		{ID: "chunk_0001", Start: 0, End: 10, DurationSeconds: 10, Text: "ok", WordCount: 1},
		{ID: "chunk_0002", Start: 10, End: 20, DurationSeconds: 10, Text: "the key is this is important and really useful", WordCount: 9},
	})
	output := filepath.Join(t.TempDir(), "highlights.json")

	_, err := WriteFromChunks(path, output, Options{MinDurationSeconds: 3, MaxDurationSeconds: 90, TopK: 1})
	if err != nil {
		t.Fatalf("WriteFromChunks returned error: %v", err)
	}

	doc := readHighlights(t, output)
	if len(doc.Highlights) != 1 {
		t.Fatalf("highlight count = %d, want 1", len(doc.Highlights))
	}
	if doc.Highlights[0].ChunkID != "chunk_0002" {
		t.Fatalf("top chunk = %q, want chunk_0002", doc.Highlights[0].ChunkID)
	}
}

func TestValidateFileAcceptsValidHighlights(t *testing.T) {
	path := writeHighlights(t, `{"schema_version":"highlights.v1","highlights":[{"id":"hl_0001","chunk_id":"chunk_0001","start":0,"end":2,"duration_seconds":2,"score":0.5,"label":"Candidate highlight","reason":"ok","text":"hello","signals":{}}]}`)

	summary, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile returned error: %v", err)
	}
	if summary.Count != 1 {
		t.Fatalf("Count = %d, want 1", summary.Count)
	}
}

func TestValidateFileRejectsInvalidScore(t *testing.T) {
	path := writeHighlights(t, `{"schema_version":"highlights.v1","highlights":[{"id":"hl_0001","chunk_id":"chunk_0001","start":0,"end":2,"duration_seconds":2,"score":2,"label":"Candidate highlight","reason":"ok","text":"hello","signals":{}}]}`)

	_, err := ValidateFile(path)
	if err == nil {
		t.Fatal("ValidateFile returned nil error")
	}
}

func writeChunks(t *testing.T, chunks []chunk) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chunks.json")
	doc := struct {
		SchemaVersion string  `json:"schema_version"`
		Chunks        []chunk `json:"chunks"`
	}{SchemaVersion: "chunks.v1", Chunks: chunks}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func writeHighlights(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "highlights.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func readHighlights(t *testing.T, path string) Document {
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

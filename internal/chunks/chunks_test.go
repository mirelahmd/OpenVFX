package chunks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFromTranscriptOneShortSegment(t *testing.T) {
	transcript := writeTranscript(t, `[{"id":"seg_0001","start":0,"end":2,"text":"hello world"}]`)
	output := filepath.Join(t.TempDir(), "chunks.json")

	summary, err := WriteFromTranscript(transcript, output, Options{TargetSeconds: 30, MaxGapSeconds: 2})
	if err != nil {
		t.Fatalf("WriteFromTranscript returned error: %v", err)
	}
	if summary.ChunkCount != 1 {
		t.Fatalf("ChunkCount = %d, want 1", summary.ChunkCount)
	}

	doc := readChunks(t, output)
	if len(doc.Chunks) != 1 {
		t.Fatalf("chunks length = %d, want 1", len(doc.Chunks))
	}
	if doc.Chunks[0].Text != "hello world" {
		t.Fatalf("chunk text = %q", doc.Chunks[0].Text)
	}
	if doc.Chunks[0].WordCount != 2 {
		t.Fatalf("word count = %d, want 2", doc.Chunks[0].WordCount)
	}
}

func TestWriteFromTranscriptSplitsByTargetDuration(t *testing.T) {
	transcript := writeTranscript(t, `[
	  {"id":"seg_0001","start":0,"end":10,"text":"one"},
	  {"id":"seg_0002","start":10,"end":20,"text":"two"},
	  {"id":"seg_0003","start":20,"end":35,"text":"three"}
	]`)
	output := filepath.Join(t.TempDir(), "chunks.json")

	_, err := WriteFromTranscript(transcript, output, Options{TargetSeconds: 30, MaxGapSeconds: 2})
	if err != nil {
		t.Fatalf("WriteFromTranscript returned error: %v", err)
	}

	doc := readChunks(t, output)
	if len(doc.Chunks) != 2 {
		t.Fatalf("chunks length = %d, want 2", len(doc.Chunks))
	}
	if got := doc.Chunks[0].SegmentIDs; len(got) != 2 {
		t.Fatalf("first chunk segment count = %d, want 2", len(got))
	}
}

func TestWriteFromTranscriptSplitsByGap(t *testing.T) {
	transcript := writeTranscript(t, `[
	  {"id":"seg_0001","start":0,"end":2,"text":"one"},
	  {"id":"seg_0002","start":5.5,"end":7,"text":"two"}
	]`)
	output := filepath.Join(t.TempDir(), "chunks.json")

	_, err := WriteFromTranscript(transcript, output, Options{TargetSeconds: 30, MaxGapSeconds: 2})
	if err != nil {
		t.Fatalf("WriteFromTranscript returned error: %v", err)
	}

	doc := readChunks(t, output)
	if len(doc.Chunks) != 2 {
		t.Fatalf("chunks length = %d, want 2", len(doc.Chunks))
	}
}

func TestValidateFileAcceptsValidChunks(t *testing.T) {
	path := writeChunks(t, `{
	  "schema_version":"chunks.v1",
	  "chunks":[{"id":"chunk_0001","start":0,"end":2,"duration_seconds":2,"text":"hello","segment_ids":["seg_0001"],"word_count":1}]
	}`)

	summary, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile returned error: %v", err)
	}
	if summary.ChunkCount != 1 {
		t.Fatalf("ChunkCount = %d, want 1", summary.ChunkCount)
	}
}

func TestValidateFileRejectsInvalidChunks(t *testing.T) {
	path := writeChunks(t, `{
	  "schema_version":"chunks.v1",
	  "chunks":[{"id":"chunk_0001","start":2,"end":1,"duration_seconds":-1,"text":"bad","segment_ids":[],"word_count":0}]
	}`)

	_, err := ValidateFile(path)
	if err == nil {
		t.Fatal("ValidateFile returned nil error")
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

func writeChunks(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chunks.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func readChunks(t *testing.T, path string) Document {
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

package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLogWritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")

	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := log.Write("RUN_STARTED", map[string]any{"input_path": "input.mp4"}); err != nil {
		t.Fatalf("Write RUN_STARTED returned error: %v", err)
	}
	if err := log.Write("RUN_COMPLETED", map[string]any{"status": "completed"}); err != nil {
		t.Fatalf("Write RUN_COMPLETED returned error: %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open event file returned error: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var eventTypes []string
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("event line did not decode: %v", err)
		}
		if event.Time.IsZero() {
			t.Fatal("event time was zero")
		}
		eventTypes = append(eventTypes, event.Type)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner returned error: %v", err)
	}
	if len(eventTypes) != 2 {
		t.Fatalf("event count = %d, want 2", len(eventTypes))
	}
	if eventTypes[0] != "RUN_STARTED" || eventTypes[1] != "RUN_COMPLETED" {
		t.Fatalf("event types = %#v", eventTypes)
	}
}

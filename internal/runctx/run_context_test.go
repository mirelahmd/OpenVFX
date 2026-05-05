package runctx

import (
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestNewRunIDUsesTimestampAndRandomSuffix(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 6, 30, 0, 0, time.UTC)

	runID, err := NewRunID(createdAt)
	if err != nil {
		t.Fatalf("NewRunID returned error: %v", err)
	}

	matched := regexp.MustCompile(`^20260428T063000Z-[0-9a-f]{8}$`).MatchString(runID)
	if !matched {
		t.Fatalf("run id %q did not match expected format", runID)
	}
}

func TestNewCreatesRunContext(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 6, 30, 0, 0, time.UTC)
	inputPath := "/tmp/input.mp4"

	ctx, err := New(inputPath, createdAt)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if ctx.InputPath != inputPath {
		t.Fatalf("InputPath = %q, want %q", ctx.InputPath, inputPath)
	}
	if !ctx.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %s, want %s", ctx.CreatedAt, createdAt)
	}
	wantDir := filepath.Join(".byom-video", "runs", ctx.RunID)
	if ctx.Dir != wantDir {
		t.Fatalf("Dir = %q, want %q", ctx.Dir, wantDir)
	}
}

package fsx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireFileAcceptsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := RequireFile(path); err != nil {
		t.Fatalf("RequireFile returned error: %v", err)
	}
}

func TestRequireFileRejectsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.mp4")

	err := RequireFile(path)
	if err == nil {
		t.Fatal("RequireFile returned nil error for missing file")
	}
	if !strings.Contains(err.Error(), "input file does not exist") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRequireFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()

	err := RequireFile(dir)
	if err == nil {
		t.Fatal("RequireFile returned nil error for directory")
	}
	if !strings.Contains(err.Error(), "input path is a directory") {
		t.Fatalf("error = %q", err.Error())
	}
}

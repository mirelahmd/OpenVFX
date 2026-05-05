package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

func TestInitCreatesConfigAndFolders(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := Init(&bytes.Buffer{}, false); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	for _, path := range []string{config.DefaultPath, "media", "exports", ".byom-video"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestInitDoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, config.DefaultPath)
	if err := os.WriteFile(path, []byte("project:\n  name: keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(&bytes.Buffer{}, false); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: keep") {
		t.Fatalf("config was overwritten: %s", string(data))
	}
}

func TestInitOverwritesWithForce(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, config.DefaultPath)
	if err := os.WriteFile(path, []byte("project:\n  name: keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(&bytes.Buffer{}, true); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: byom-video-project") {
		t.Fatalf("config was not overwritten: %s", string(data))
	}
}

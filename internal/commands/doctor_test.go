package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestResolvePythonWithSource_EnvVar(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BYOM_VIDEO_PYTHON", "/usr/bin/fakepython")
	got, src := resolvePythonWithSource()
	if got != "/usr/bin/fakepython" {
		t.Errorf("expected env var value, got %q", got)
	}
	if src != "BYOM_VIDEO_PYTHON" {
		t.Errorf("expected source BYOM_VIDEO_PYTHON, got %q", src)
	}
}

func TestResolvePythonWithSource_EnvVarTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	// Write a config with a different interpreter
	if err := os.WriteFile("byom-video.yaml", []byte("python:\n  interpreter: /from/config/python\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BYOM_VIDEO_PYTHON", "/from/env/python")
	got, src := resolvePythonWithSource()
	if got != "/from/env/python" {
		t.Errorf("expected env var to win, got %q", got)
	}
	if src != "BYOM_VIDEO_PYTHON" {
		t.Errorf("expected source BYOM_VIDEO_PYTHON, got %q", src)
	}
}

func TestResolvePythonWithSource_Config(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("BYOM_VIDEO_PYTHON", "")
	if err := os.WriteFile("byom-video.yaml", []byte("python:\n  interpreter: /from/config/python\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, src := resolvePythonWithSource()
	if got != "/from/config/python" {
		t.Errorf("expected config value, got %q", got)
	}
	if src != "config" {
		t.Errorf("expected source config, got %q", src)
	}
}

func TestResolvePythonWithSource_FallsBackToPath(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BYOM_VIDEO_PYTHON", "")
	// No config file in temp dir; should fall back to python3 on PATH if available
	got, src := resolvePythonWithSource()
	if got == "" {
		t.Skip("python3 not on PATH in this environment")
	}
	if src != "PATH" {
		t.Errorf("expected source PATH, got %q", src)
	}
}

func TestCheckPythonImport_KnownModule(t *testing.T) {
	py, err := findPython3()
	if err != nil {
		t.Skip("python3 not available")
	}
	if !checkPythonImport(py, "os") {
		t.Error("expected 'os' module to be importable")
	}
}

func TestCheckPythonImport_FakeModule(t *testing.T) {
	py, err := findPython3()
	if err != nil {
		t.Skip("python3 not available")
	}
	if checkPythonImport(py, "totally_nonexistent_module_xyz") {
		t.Error("expected fake module to not be importable")
	}
}

func TestDoctorRuns(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BYOM_VIDEO_PYTHON", "")
	var buf bytes.Buffer
	if err := Doctor(&buf, DoctorOptions{}); err != nil {
		t.Fatalf("Doctor returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "byom-video doctor") {
		t.Error("expected header in doctor output")
	}
	if !strings.Contains(out, "go runtime") {
		t.Error("expected go runtime in doctor output")
	}
}

func TestDoctorTranscriptionFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("BYOM_VIDEO_PYTHON", "")
	var buf bytes.Buffer
	if err := Doctor(&buf, DoctorOptions{Transcription: true}); err != nil {
		t.Fatalf("Doctor returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Transcription check active") {
		t.Error("expected transcription check note in output")
	}
}

func TestVersionDefaults(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Commit == "" {
		t.Error("Commit should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := VersionCommand(&buf); err != nil {
		t.Fatalf("VersionCommand returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "BYOM Video") {
		t.Error("expected BYOM Video in version output")
	}
	if !strings.Contains(out, Version) {
		t.Error("expected version string in version output")
	}
}

// findPython3 returns the path to python3 if available.
func findPython3() (string, error) {
	p, _ := resolvePythonWithSource()
	if p == "" {
		return "", os.ErrNotExist
	}
	return p, nil
}

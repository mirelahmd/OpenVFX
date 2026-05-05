package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"byom-video/internal/manifest"
)

func TestRunsPrintsRows(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCommandRunManifest(t, "run-1")
	var out bytes.Buffer
	if err := Runs(&out, RunsOptions{All: true}); err != nil {
		t.Fatalf("Runs returned error: %v", err)
	}
	if !strings.Contains(out.String(), "run-1") {
		t.Fatalf("output missing run id: %s", out.String())
	}
}

func TestInspectPrintsMinimalManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCommandRunManifest(t, "run-1")
	var out bytes.Buffer
	if err := Inspect("run-1", &out, InspectOptions{}); err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Run inspection") || !strings.Contains(out.String(), "metadata.json") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestInspectJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCommandRunManifest(t, "run-1")
	var out bytes.Buffer
	if err := Inspect("run-1", &out, InspectOptions{JSON: true}); err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"run_id": "run-1"`) {
		t.Fatalf("unexpected JSON output: %s", out.String())
	}
}

func TestArtifactsPrintsFilteredPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCommandRunManifest(t, "run-1")
	var out bytes.Buffer
	if err := Artifacts("run-1", &out, ArtifactsOptions{Type: "metadata"}); err != nil {
		t.Fatalf("Artifacts returned error: %v", err)
	}
	if !strings.Contains(out.String(), "metadata.json") || strings.Contains(out.String(), "manifest.json") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestValidateJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeCommandRunManifest(t, "run-1")
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(`{"type":"RUN_STARTED","time":"2026-04-28T00:00:00Z"}`+"\n"+`{"type":"RUN_COMPLETED","time":"2026-04-28T00:00:01Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("events", "events.jsonl")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Validate("run-1", &out, ValidateOptions{JSON: true}); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !strings.Contains(out.String(), `"run_id": "run-1"`) || !strings.Contains(out.String(), `"manifest_ok": true`) {
		t.Fatalf("unexpected JSON output: %s", out.String())
	}
}

func TestValidateCommandReportsMissingArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	runDir := writeCommandRunManifest(t, "run-1")
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(`{"type":"RUN_STARTED","time":"2026-04-28T00:00:00Z"}`+"\n"+`{"type":"RUN_COMPLETED","time":"2026-04-28T00:00:01Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("events", "events.jsonl")
	m.AddArtifact("missing", "missing.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Validate("run-1", &out, ValidateOptions{})
	if err == nil {
		t.Fatal("Validate returned nil error, want validation failure")
	}
	if !strings.Contains(out.String(), "artifact missing: missing.json") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestOpenReportCommandSelection(t *testing.T) {
	command, args, ok := OpenReportCommand("/tmp/report.html", "darwin")
	if !ok || command != "open" || args[0] != "/tmp/report.html" {
		t.Fatalf("darwin command = %q %#v %v", command, args, ok)
	}
	command, args, ok = OpenReportCommand("/tmp/report.html", "linux")
	if !ok || command != "xdg-open" || args[0] != "/tmp/report.html" {
		t.Fatalf("linux command = %q %#v %v", command, args, ok)
	}
}

func TestOpenReportPrintsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCommandRunManifest(t, "run-1")
	runDir := filepath.Join(".byom-video", "runs", "run-1")
	if err := os.WriteFile(filepath.Join(runDir, "report.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("report", "report.html")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := OpenReport("run-1", &out, false); err != nil {
		t.Fatalf("OpenReport returned error: %v", err)
	}
	if !strings.Contains(out.String(), "report.html") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func writeCommandRunManifest(t *testing.T, runID string) string {
	t.Helper()
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "/tmp/input.mp4", time.Now().UTC())
	m.Status = manifest.StatusCompleted
	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("metadata", "metadata.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "metadata.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return runDir
}

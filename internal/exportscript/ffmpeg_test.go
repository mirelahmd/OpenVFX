package exportscript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFFmpegScriptOneClip(t *testing.T) {
	roughcut := writeRoughcut(t, `[{"id":"clip_0001","start":0,"end":2}]`)
	output := filepath.Join(t.TempDir(), "ffmpeg_commands.sh")

	summary, err := WriteFFmpegScript(roughcut, output, "/tmp/input.mov", "mp4", "stream-copy")
	if err != nil {
		t.Fatalf("WriteFFmpegScript returned error: %v", err)
	}
	if summary.CommandCount != 1 {
		t.Fatalf("CommandCount = %d, want 1", summary.CommandCount)
	}
	data, _ := os.ReadFile(output)
	if !strings.Contains(string(data), "exports/clip_0001.mp4") {
		t.Fatalf("script missing output: %s", string(data))
	}
	if !strings.Contains(string(data), "-c copy") {
		t.Fatalf("expected stream-copy command: %s", string(data))
	}
}

func TestWriteFFmpegScriptMultipleClips(t *testing.T) {
	roughcut := writeRoughcut(t, `[{"id":"clip_0001","start":0,"end":2},{"id":"clip_0002","start":3,"end":5}]`)
	output := filepath.Join(t.TempDir(), "ffmpeg_commands.sh")

	summary, err := WriteFFmpegScript(roughcut, output, "/tmp/input.mov", "mp4", "stream-copy")
	if err != nil {
		t.Fatalf("WriteFFmpegScript returned error: %v", err)
	}
	if summary.CommandCount != 2 {
		t.Fatalf("CommandCount = %d, want 2", summary.CommandCount)
	}
}

func TestShellQuoteHandlesSpaces(t *testing.T) {
	got := ShellQuote("/tmp/input file.mov")
	want := "'/tmp/input file.mov'"
	if got != want {
		t.Fatalf("ShellQuote = %q, want %q", got, want)
	}
}

func TestWriteFFmpegScriptReencodeMode(t *testing.T) {
	roughcut := writeRoughcut(t, `[{"id":"clip_0001","start":0,"end":2}]`)
	output := filepath.Join(t.TempDir(), "ffmpeg_commands.sh")

	summary, err := WriteFFmpegScript(roughcut, output, "/tmp/input.mov", "mp4", "reencode")
	if err != nil {
		t.Fatalf("WriteFFmpegScript returned error: %v", err)
	}
	if summary.Mode != "reencode" {
		t.Fatalf("Mode = %q", summary.Mode)
	}
	data, _ := os.ReadFile(output)
	text := string(data)
	if !strings.Contains(text, "-c:v libx264 -c:a aac") || strings.Contains(text, "-c copy") {
		t.Fatalf("expected reencode command only: %s", text)
	}
}

func writeRoughcut(t *testing.T, clips string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "roughcut.json")
	content := `{"schema_version":"roughcut.v1","clips":` + clips + `}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

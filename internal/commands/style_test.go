package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- StyleInit ----

func TestStyleInit_CreatesAllFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	styleDir := "mystyle"
	err := StyleInit(&out, StyleInitOptions{StyleDir: styleDir})
	if err != nil {
		t.Fatalf("StyleInit failed: %v", err)
	}
	for _, name := range styleFiles {
		path := filepath.Join(styleDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist", path)
		}
	}
	text := out.String()
	if !strings.Contains(text, styleDir) {
		t.Errorf("output should mention style dir; got: %s", text)
	}
}

func TestStyleInit_DefaultDir(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := StyleInit(&out, StyleInitOptions{}); err != nil {
		t.Fatalf("StyleInit failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(defaultStyleDir, "profile.md")); err != nil {
		t.Errorf("profile.md should exist in default style dir")
	}
}

func TestStyleInit_SkipsExistingWithoutForce(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testskip"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	// Write custom content to one file
	customContent := "# My Custom Profile"
	profilePath := filepath.Join(styleDir, "profile.md")
	if err := os.WriteFile(profilePath, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-run without force
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(profilePath)
	if string(data) != customContent {
		t.Errorf("existing file should not be overwritten without --force")
	}
}

func TestStyleInit_ForceOverwrites(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testforce"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(styleDir, "profile.md")
	if err := os.WriteFile(profilePath, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir, Force: true}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(profilePath)
	if string(data) == "custom" {
		t.Errorf("file should be overwritten with --force")
	}
}

func TestStyleInit_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := StyleInit(&out, StyleInitOptions{StyleDir: "jsontest", JSON: true}); err != nil {
		t.Fatalf("StyleInit --json failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if _, ok := result["style_dir"]; !ok {
		t.Errorf("JSON should contain style_dir key; got: %s", out.String())
	}
	if _, ok := result["files"]; !ok {
		t.Errorf("JSON should contain files key; got: %s", out.String())
	}
}

// ---- StyleInspect ----

func TestStyleInspect_ShowsMissingFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := StyleInspect(&out, StyleInspectOptions{StyleDir: "nonexistent"}); err != nil {
		t.Fatalf("StyleInspect failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "no") {
		t.Errorf("output should show files as missing; got: %s", text)
	}
}

func TestStyleInspect_ShowsPresentFile(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testinspect"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := StyleInspect(&out, StyleInspectOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "yes") {
		t.Errorf("output should show files as present; got: %s", text)
	}
	if !strings.Contains(text, "profile.md") {
		t.Errorf("output should list profile.md; got: %s", text)
	}
}

func TestStyleInspect_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "inspjson"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := StyleInspect(&out, StyleInspectOptions{StyleDir: styleDir, JSON: true}); err != nil {
		t.Fatal(err)
	}
	var result StyleInspectResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(result.Files) != len(styleFiles) {
		t.Errorf("expected %d files, got %d", len(styleFiles), len(result.Files))
	}
}

// ---- StyleValidate ----

func TestStyleValidate_PassesOnCustomFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testvalidate"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range styleFiles {
		content := "# " + name + "\n\nThis is customized content for the " + name + " file."
		if err := os.WriteFile(filepath.Join(styleDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := StyleValidate(&out, StyleValidateOptions{StyleDir: styleDir}); err != nil {
		t.Fatalf("StyleValidate should pass for customized files: %v", err)
	}
	if !strings.Contains(out.String(), "valid") {
		t.Errorf("output should say valid; got: %s", out.String())
	}
}

func TestStyleValidate_WarnsOnTemplateContent(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testtemplate"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	// Should succeed (not error) but warn about template content
	err := StyleValidate(&out, StyleValidateOptions{StyleDir: styleDir})
	if err != nil {
		t.Fatalf("StyleValidate should not fail on template content (non-strict): %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "warning") && !strings.Contains(text, "template") {
		t.Errorf("output should warn about template content; got: %s", text)
	}
}

func TestStyleValidate_StrictFailsOnMissingFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	err := StyleValidate(ioDiscard{}, StyleValidateOptions{StyleDir: "nonexistent", Strict: true})
	if err == nil {
		t.Fatal("expected error for missing files in strict mode")
	}
	if !strings.Contains(err.Error(), "validate failed") {
		t.Errorf("error should mention validate failed; got: %v", err)
	}
}

func TestStyleValidate_NonStrictPassesOnMissingFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	// In non-strict mode, missing files produce warnings but don't fail
	err := StyleValidate(&out, StyleValidateOptions{StyleDir: "nonexistent", Strict: false})
	if err != nil {
		t.Fatalf("non-strict mode should not fail on missing files: %v", err)
	}
	if !strings.Contains(out.String(), "warning") {
		t.Errorf("output should warn about missing files; got: %s", out.String())
	}
}

func TestStyleValidate_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "valjson"
	if err := StyleInit(ioDiscard{}, StyleInitOptions{StyleDir: styleDir}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_ = StyleValidate(&out, StyleValidateOptions{StyleDir: styleDir, JSON: true})
	var result StyleValidateResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if result.StyleDir != styleDir {
		t.Errorf("expected style_dir=%s; got %s", styleDir, result.StyleDir)
	}
}

// ---- LoadStylePack ----

func TestLoadStylePack_DisabledWhenDirMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	ctx := LoadStylePack("nonexistent", 0)
	if ctx.Enabled {
		t.Error("style pack should be disabled when dir is missing")
	}
}

func TestLoadStylePack_LoadsFilesCorrectly(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testload"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(styleDir, "profile.md"), []byte("# Profile\n\nCustom profile."), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := LoadStylePack(styleDir, 0)
	if !ctx.Enabled {
		t.Error("style pack should be enabled when files present")
	}
	if len(ctx.FilesUsed) != 1 {
		t.Errorf("expected 1 file used; got %d", len(ctx.FilesUsed))
	}
	if ctx.FilesUsed[0] != "profile.md" {
		t.Errorf("expected profile.md; got %s", ctx.FilesUsed[0])
	}
	if !strings.Contains(ctx.content, "Custom profile") {
		t.Errorf("content should contain file content; got: %s", ctx.content)
	}
}

func TestLoadStylePack_TruncatesAtMaxChars(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testtrunc"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a large file
	large := strings.Repeat("x", 500)
	if err := os.WriteFile(filepath.Join(styleDir, "profile.md"), []byte("# Profile\n\n"+large), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := LoadStylePack(styleDir, 100) // low maxChars
	if !ctx.Truncated {
		t.Error("expected Truncated=true")
	}
	if len(ctx.Warnings) == 0 {
		t.Error("expected truncation warning")
	}
	if len(ctx.content) > 100 {
		t.Errorf("content should be truncated to 100 chars; got %d", len(ctx.content))
	}
}

func TestLoadStylePack_SkipsEmptyFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	styleDir := "testempty"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write one real file and one empty file
	if err := os.WriteFile(filepath.Join(styleDir, "profile.md"), []byte("# Profile\n\nReal content here."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(styleDir, "script_style.md"), []byte("   "), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := LoadStylePack(styleDir, 0)
	if !ctx.Enabled {
		t.Error("should be enabled when at least one non-empty file exists")
	}
	for _, f := range ctx.FilesUsed {
		if f == "script_style.md" {
			t.Error("empty file should not be included in FilesUsed")
		}
	}
}

func TestLoadStylePack_DefaultDir(t *testing.T) {
	t.Chdir(t.TempDir())
	ctx := LoadStylePack("", 0)
	// Should be disabled (no style dir in temp), not error
	if ctx == nil {
		t.Error("LoadStylePack should return a non-nil context")
	}
}

package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/config"
)

// ---- helpers ----

const voiceoverTestConfig = `tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.script: local_writer
`

func makePlanForVoiceover(t *testing.T) string {
	t.Helper()
	if err := os.WriteFile(config.DefaultPath, []byte(voiceoverTestConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	planID := "20260509T000000Z-test-voiceover-plan"
	planDir := filepath.Join(creativePlansRoot, planID)
	if err := os.MkdirAll(filepath.Join(planDir, "outputs"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := CreativePlan{
		SchemaVersion: "creative_plan.v1",
		PlanID:        planID,
		Goal:          "write a product demo narration",
		Steps:         []CreativeStep{{ID: "step_0001", Type: "generate_script", Capability: "text_generation", Description: "Draft script."}},
	}
	if err := writeJSONFile(filepath.Join(planDir, "creative_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	return planID
}

func writeScriptDraftForVoiceover(t *testing.T, planID, text string) {
	t.Helper()
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)
	sd := CreativeScriptOutput{
		SchemaVersion:  "creative_script.v1",
		CreativePlanID: planID,
		Text:           text,
	}
	if err := writeJSONFile(filepath.Join(outputsDir, "script_draft.json"), sd); err != nil {
		t.Fatal(err)
	}
}

// ---- Part A: VoiceoverTextCommand tests ----

func TestVoiceoverText_UsesScriptDraft(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	writeScriptDraftForVoiceover(t, planID, "Welcome to the product demo. Today we show you something amazing.")

	err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.json"))
	if err != nil {
		t.Fatal("voiceover_text.json not created")
	}
	var vt VoiceoverTextOutput
	if err := json.Unmarshal(data, &vt); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if vt.SchemaVersion != "voiceover_text.v1" {
		t.Fatalf("schema_version=%q", vt.SchemaVersion)
	}
	if vt.Source.SourceType != "script_draft" {
		t.Fatalf("source_type=%q, want script_draft", vt.Source.SourceType)
	}
	if !strings.Contains(vt.Text, "product demo") {
		t.Fatalf("text missing script content: %q", vt.Text)
	}
	if vt.WordCount <= 0 {
		t.Fatalf("word_count=%d, want >0", vt.WordCount)
	}
}

func TestVoiceoverText_FallsBackToScriptDraftTxt(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	// Write only script_draft.txt, not .json
	if err := os.WriteFile(filepath.Join(outputsDir, "script_draft.txt"), []byte("Text from plain file."), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := VoiceoverTextCommand(planID, &out, VoiceoverTextOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(outputsDir, "voiceover_text.json"))
	var vt VoiceoverTextOutput
	_ = json.Unmarshal(data, &vt)
	if vt.Source.SourceType != "script_text" {
		t.Fatalf("source_type=%q, want script_text", vt.Source.SourceType)
	}
	if !strings.Contains(vt.Text, "Text from plain file") {
		t.Fatalf("text missing content from txt file: %q", vt.Text)
	}
}

func TestVoiceoverText_FallsBackToGoalWithWarning(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)

	var out bytes.Buffer
	if err := VoiceoverTextCommand(planID, &out, VoiceoverTextOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.json"))
	var vt VoiceoverTextOutput
	_ = json.Unmarshal(data, &vt)

	if vt.Source.SourceType != "goal" {
		t.Fatalf("source_type=%q, want goal", vt.Source.SourceType)
	}
	if len(vt.Warnings) == 0 {
		t.Fatal("expected warning when falling back to goal")
	}
	found := false
	for _, w := range vt.Warnings {
		if strings.Contains(w, "No script draft") || strings.Contains(w, "goal") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected goal-fallback warning, got: %v", vt.Warnings)
	}
}

func TestVoiceoverText_MaxWordsTruncation(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	longScript := strings.Repeat("word ", 200) // 200 words
	writeScriptDraftForVoiceover(t, planID, longScript)

	if err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{MaxWords: 50}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.json"))
	var vt VoiceoverTextOutput
	_ = json.Unmarshal(data, &vt)

	if vt.WordCount > 50 {
		t.Fatalf("word_count=%d, want <=50", vt.WordCount)
	}
	truncWarning := false
	for _, w := range vt.Warnings {
		if strings.Contains(w, "truncated") {
			truncWarning = true
		}
	}
	if !truncWarning {
		t.Fatalf("expected truncation warning, got: %v", vt.Warnings)
	}
}

func TestVoiceoverText_RejectsIfAlreadyExists(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), []byte(`{}`), 0o644)

	err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{})
	if err == nil {
		t.Fatal("expected error when file exists without --overwrite")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error should mention already exists, got: %v", err)
	}
}

func TestVoiceoverText_OverwriteSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), []byte(`{"old": true}`), 0o644)

	if err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{Overwrite: true}); err != nil {
		t.Fatalf("unexpected error with --overwrite: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(outputsDir, "voiceover_text.json"))
	var vt VoiceoverTextOutput
	if err := json.Unmarshal(data, &vt); err != nil {
		t.Fatalf("invalid JSON after overwrite: %v", err)
	}
	if vt.SchemaVersion != "voiceover_text.v1" {
		t.Fatalf("schema_version=%q after overwrite", vt.SchemaVersion)
	}
}

func TestVoiceoverText_WritesTextFile(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	writeScriptDraftForVoiceover(t, planID, "Hello world from script.")

	if err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	txt, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.txt"))
	if err != nil {
		t.Fatal("voiceover_text.txt not created")
	}
	if !strings.Contains(string(txt), "Hello world") {
		t.Fatalf("voiceover_text.txt missing script content: %q", string(txt))
	}
}

func TestVoiceoverText_SourceGoalFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	// Write a script draft, but force source=goal
	writeScriptDraftForVoiceover(t, planID, "Script content that should be ignored.")

	if err := VoiceoverTextCommand(planID, ioDiscard{}, VoiceoverTextOptions{Source: "goal"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.json"))
	var vt VoiceoverTextOutput
	_ = json.Unmarshal(data, &vt)
	if vt.Source.SourceType != "goal" {
		t.Fatalf("source_type=%q, want goal when --source=goal", vt.Source.SourceType)
	}
}

func TestVoiceoverText_MissingPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	err := VoiceoverTextCommand("nonexistent-plan", ioDiscard{}, VoiceoverTextOptions{})
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention not found, got: %v", err)
	}
}

// ---- Part B: VoiceoverStatus tests ----

func TestVoiceoverStatus_MissingTextAndAudio(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)

	var out bytes.Buffer
	if err := VoiceoverStatus(planID, &out, VoiceoverStatusOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "missing_text_and_audio") {
		t.Fatalf("expected missing_text_and_audio readiness: %s", text)
	}
}

func TestVoiceoverStatus_TextOnlyMissingAudio(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	// Write voiceover_text.json
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 42,
		Text: "hello world",
		Source: VoiceoverTextSource{SourceType: "goal"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)

	var out bytes.Buffer
	if err := VoiceoverStatus(planID, &out, VoiceoverStatusOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "missing_audio") {
		t.Fatalf("expected missing_audio readiness: %s", text)
	}
}

func TestVoiceoverStatus_ReadyWhenAudioFound(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	// Write voiceover_text.json
	vt := VoiceoverTextOutput{SchemaVersion: "voiceover_text.v1", WordCount: 10, Text: "hello", Source: VoiceoverTextSource{SourceType: "goal"}}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)
	// Create dummy audio file
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover.wav"), []byte("RIFF...."), 0o644)

	var out bytes.Buffer
	if err := VoiceoverStatus(planID, &out, VoiceoverStatusOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "ready") {
		t.Fatalf("expected ready readiness: %s", text)
	}
}

func TestVoiceoverStatus_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)

	var out bytes.Buffer
	if err := VoiceoverStatus(planID, &out, VoiceoverStatusOptions{JSON: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var res VoiceoverStatusResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if res.Readiness != VoiceoverMissingTextAndAudio {
		t.Fatalf("readiness=%q, want missing_text_and_audio", res.Readiness)
	}
}

func TestVoiceoverStatus_MissingPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	err := VoiceoverStatus("nonexistent-plan", ioDiscard{}, VoiceoverStatusOptions{})
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention not found: %v", err)
	}
}

// ---- ReviewVoiceover tests ----

func TestReviewVoiceover_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 15,
		Text:   "This is the voiceover text for the product demo.",
		Source: VoiceoverTextSource{SourceType: "script_draft", SourceArtifact: "outputs/script_draft.json"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)

	var out bytes.Buffer
	if err := ReviewVoiceover(planID, &out, ReviewVoiceoverOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Voiceover Review") {
		t.Fatalf("expected 'Voiceover Review' heading: %s", text)
	}
	if !strings.Contains(text, "script_draft") {
		t.Fatalf("expected source type in review: %s", text)
	}
	if _, err := os.Stat(filepath.Join(outputsDir, "voiceover_review.md")); err != nil {
		t.Fatalf("voiceover_review.md not created: %v", err)
	}
}

func TestReviewVoiceover_NoTextShowsNextStep(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)

	var out bytes.Buffer
	if err := ReviewVoiceover(planID, &out, ReviewVoiceoverOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "creative-voiceover-text") {
		t.Fatalf("expected next-step command suggestion when no text: %s", text)
	}
}

// ---- Part C: ValidateVoiceover tests ----

func TestValidateVoiceover_PassesWithValidText(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 10,
		Text:   "Valid voiceover text.",
		Mode:   "local_text_extract",
		Source: VoiceoverTextSource{SourceType: "goal"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)

	if err := ValidateVoiceover(planID, ioDiscard{}, ValidateVoiceoverOptions{}); err != nil {
		t.Fatalf("unexpected error for valid text: %v", err)
	}
}

func TestValidateVoiceover_RequireAudioFailsWhenMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)

	err := ValidateVoiceover(planID, ioDiscard{}, ValidateVoiceoverOptions{RequireAudio: true})
	if err == nil {
		t.Fatal("expected error when --require-audio and no audio file")
	}
	if !strings.Contains(err.Error(), "error") {
		t.Fatalf("error should mention validation failure: %v", err)
	}
}

func TestValidateVoiceover_AcceptsSupportedExtension(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 5,
		Text:   "hello world",
		Source: VoiceoverTextSource{SourceType: "goal"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover.mp3"), []byte("mp3 data"), 0o644)

	if err := ValidateVoiceover(planID, ioDiscard{}, ValidateVoiceoverOptions{RequireAudio: true}); err != nil {
		t.Fatalf("unexpected error for valid mp3: %v", err)
	}
}

func TestValidateVoiceover_EmptyTextIsWarning(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 0,
		Text:   "",
		Source: VoiceoverTextSource{SourceType: "goal"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)

	var out bytes.Buffer
	// Empty text is an error, not just a warning
	err := ValidateVoiceover(planID, &out, ValidateVoiceoverOptions{})
	if err == nil {
		t.Fatal("expected error for empty text field")
	}
}

func TestValidateVoiceover_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	vt := VoiceoverTextOutput{
		SchemaVersion: "voiceover_text.v1", WordCount: 5,
		Text:   "hello",
		Source: VoiceoverTextSource{SourceType: "goal"},
	}
	data, _ := json.Marshal(vt)
	_ = os.WriteFile(filepath.Join(outputsDir, "voiceover_text.json"), data, 0o644)

	var out bytes.Buffer
	_ = ValidateVoiceover(planID, &out, ValidateVoiceoverOptions{JSON: true})
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if res["status"] == nil {
		t.Fatal("JSON output missing status field")
	}
}

// ---- truncateWords tests ----

func TestTruncateWords_BelowLimit(t *testing.T) {
	text, warns := truncateWords("hello world foo", 10)
	if text != "hello world foo" {
		t.Fatalf("text changed when below limit: %q", text)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
}

func TestTruncateWords_AtLimit(t *testing.T) {
	words := strings.Repeat("word ", 5)
	text, warns := truncateWords(strings.TrimSpace(words), 5)
	if len(strings.Fields(text)) != 5 {
		t.Fatalf("expected 5 words, got: %q", text)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
}

func TestTruncateWords_ExceedsLimit(t *testing.T) {
	words := strings.Join(make([]string, 20), "word ")
	long := strings.TrimSpace(fmt.Sprintf("%s", strings.Repeat("word ", 20)))
	text, warns := truncateWords(long, 10)
	if len(strings.Fields(text)) > 10 {
		t.Fatalf("expected <=10 words, got %d: %q", len(strings.Fields(text)), text)
	}
	if len(warns) == 0 {
		t.Fatal("expected truncation warning")
	}
	if !strings.Contains(warns[0], "truncated") {
		t.Fatalf("warning should say truncated: %v", warns)
	}
	_ = words
}

// ---- resolveVoiceoverText tests ----

func TestResolveVoiceoverText_PrefersScriptDraftJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	// Write both json and txt
	sd := CreativeScriptOutput{SchemaVersion: "creative_script.v1", Text: "Script from JSON file."}
	sdData, _ := json.Marshal(sd)
	_ = os.WriteFile(filepath.Join(outputsDir, "script_draft.json"), sdData, 0o644)
	_ = os.WriteFile(filepath.Join(outputsDir, "script_draft.txt"), []byte("Text from TXT file."), 0o644)

	text, srcType, _, _ := resolveVoiceoverText(planDir, "goal text", "auto", 120)
	if srcType != "script_draft" {
		t.Fatalf("source_type=%q, want script_draft", srcType)
	}
	if !strings.Contains(text, "Script from JSON file") {
		t.Fatalf("text=%q, want JSON script content", text)
	}
}

func TestResolveVoiceoverText_FallsBackToTxt(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	// Only txt, no json
	_ = os.WriteFile(filepath.Join(outputsDir, "script_draft.txt"), []byte("Text from TXT."), 0o644)

	text, srcType, artifact, _ := resolveVoiceoverText(planDir, "goal text", "auto", 120)
	if srcType != "script_text" {
		t.Fatalf("source_type=%q, want script_text", srcType)
	}
	if artifact == "" {
		t.Fatal("expected non-empty artifact path")
	}
	if !strings.Contains(text, "Text from TXT") {
		t.Fatalf("text=%q", text)
	}
}

func TestResolveVoiceoverText_GoalSourceMode(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForVoiceover(t)
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	// Write script but request goal source
	writeScriptDraftForVoiceover(t, planID, "Script content.")
	_, srcType, _, _ := resolveVoiceoverText(planDir, "my product goal", "goal", 120)
	if srcType != "goal" {
		t.Fatalf("source_type=%q, want goal when sourceMode=goal", srcType)
	}
}

// ---- make --prepare-voiceover dry-run test ----

func TestMake_PrepareVoiceover_DryRunShows4d(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{
		Goal:             "product demo",
		DryRun:           true,
		Yes:              true,
		PrepareVoiceover: true,
	})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "4d") {
		t.Fatalf("expected step 4d in prepare-voiceover dry-run: %s", text)
	}
}

func TestMake_WithoutPrepareVoiceover_DryRunNoShows4d(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{
		Goal:   "product demo",
		DryRun: true,
		Yes:    true,
	})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	if strings.Contains(out.String(), "4d") {
		t.Fatalf("step 4d should not appear without --prepare-voiceover: %s", out.String())
	}
}

// ---- discoverVoiceoverAudio tests ----

func TestDiscoverVoiceoverAudio_FindsWav(t *testing.T) {
	dir := t.TempDir()
	wavPath := filepath.Join(dir, "voiceover.wav")
	_ = os.WriteFile(wavPath, []byte("RIFF"), 0o644)

	found := discoverVoiceoverAudio(dir)
	if found != wavPath {
		t.Fatalf("found=%q, want %q", found, wavPath)
	}
}

func TestDiscoverVoiceoverAudio_FindsMp3(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "voiceover.mp3")
	_ = os.WriteFile(mp3Path, []byte("mp3data"), 0o644)

	found := discoverVoiceoverAudio(dir)
	if found != mp3Path {
		t.Fatalf("found=%q, want %q", found, mp3Path)
	}
}

func TestDiscoverVoiceoverAudio_NoneFound(t *testing.T) {
	dir := t.TempDir()
	found := discoverVoiceoverAudio(dir)
	if found != "" {
		t.Fatalf("expected empty, got %q", found)
	}
}

func TestDiscoverVoiceoverAudio_PrefersWavOverMp3(t *testing.T) {
	dir := t.TempDir()
	// Both exist — wav should win (first in list)
	_ = os.WriteFile(filepath.Join(dir, "voiceover.mp3"), []byte("mp3"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "voiceover.wav"), []byte("wav"), 0o644)

	found := discoverVoiceoverAudio(dir)
	if !strings.HasSuffix(found, "voiceover.wav") {
		t.Fatalf("expected wav to be preferred, got %q", found)
	}
}

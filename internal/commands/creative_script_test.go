package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/modelrouter"
)

// ---- mock adapter ----

type mockScriptAdapter struct {
	supportsProvider bool
	response         modelrouter.Response
	err              error
}

func (m mockScriptAdapter) Name() string { return "mock-script-adapter" }
func (m mockScriptAdapter) Supports(provider string) bool {
	return m.supportsProvider
}
func (m mockScriptAdapter) BuildRequest(req modelrouter.Request) (modelrouter.Request, error) {
	return req, nil
}
func (m mockScriptAdapter) Execute(req modelrouter.Request) (modelrouter.Response, error) {
	return m.response, m.err
}

// ollamaScriptConfig returns a byom-video.yaml with tools enabled for Ollama script generation.
const ollamaScriptConfig = `tools:
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

// ---- helpers ----

// makePlanWithScriptStep creates a creative plan that has a generate_script step.
// It writes the plan JSON directly so it doesn't depend on the goal keyword rules.
func makePlanWithScriptStep(t *testing.T) string {
	t.Helper()
	if err := os.WriteFile(config.DefaultPath, []byte(ollamaScriptConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	planID := "20260508T000000Z-test-script-plan"
	planDir := filepath.Join(creativePlansRoot, planID)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plan := CreativePlan{
		SchemaVersion: "creative_plan.v1",
		PlanID:        planID,
		InputPath:     inputPath,
		Goal:          "make a cinematic short clip",
		Mode:          "deterministic_planning",
		Steps: []CreativeStep{
			{
				ID:          "step_0001",
				Type:        "generate_script",
				Capability:  "text_generation",
				Route:       "creative.script",
				Backend:     "local_writer",
				Status:      "planned",
				Description: "Generate a short script from the user goal.",
			},
		},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "creative_plan.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return planID
}

func writeScriptConfig(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(config.DefaultPath, []byte(ollamaScriptConfig), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- CreativeGenerateScript tests ----

func TestCreativeGenerateScript_RejectsNonexistentPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	writeScriptConfig(t)
	err := CreativeGenerateScript("nonexistent-plan-id", ioDiscard{}, CreativeGenerateScriptOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}
}

func TestCreativeGenerateScript_RejectsUnsupportedProvider(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	adapter := mockScriptAdapter{supportsProvider: false}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{}, adapter)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "Ollama only") && !strings.Contains(err.Error(), "provider") {
		t.Errorf("error should mention provider; got: %v", err)
	}
}

func TestCreativeGenerateScript_FallbackStub_OnUnsupportedProvider(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	adapter := mockScriptAdapter{supportsProvider: false}
	var out bytes.Buffer
	err := creativeGenerateScriptWithAdapter(planID, &out, CreativeGenerateScriptOptions{
		FallbackStub: true,
	}, adapter)
	if err != nil {
		t.Fatalf("fallback-stub should succeed when provider unsupported: %v", err)
	}
	// script_draft.json should exist
	scriptPath := filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Errorf("expected script_draft.json to exist: %v", err)
	}
	data, _ := os.ReadFile(scriptPath)
	var output CreativeScriptOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("script_draft.json malformed: %v", err)
	}
	if output.Mode != "stub" {
		t.Errorf("expected mode=stub; got %s", output.Mode)
	}
}

func TestCreativeGenerateScript_OllamaCallSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	scriptJSON := `{"title":"Test Title","hook":"Here is the hook.","script":"This is the script body.","notes":["note1"]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	var out bytes.Buffer
	err := creativeGenerateScriptWithAdapter(planID, &out, CreativeGenerateScriptOptions{
		MaxWords: 50,
	}, adapter)
	if err != nil {
		t.Fatalf("expected success; got: %v", err)
	}

	scriptPath := filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("script_draft.json not written: %v", err)
	}
	var output CreativeScriptOutput
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("script_draft.json malformed: %v", err)
	}
	if output.SchemaVersion != "creative_script.v1" {
		t.Errorf("expected schema_version=creative_script.v1; got %s", output.SchemaVersion)
	}
	if output.Mode != "ollama" {
		t.Errorf("expected mode=ollama; got %s", output.Mode)
	}
	if output.Title != "Test Title" {
		t.Errorf("expected title='Test Title'; got '%s'", output.Title)
	}
	if output.Hook != "Here is the hook." {
		t.Errorf("expected hook set; got '%s'", output.Hook)
	}
	if output.Text != "This is the script body." {
		t.Errorf("expected script text; got '%s'", output.Text)
	}
	if len(output.Notes) == 0 {
		t.Errorf("expected notes to be populated")
	}

	// script_draft.txt should also exist
	txtPath := filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.txt")
	if _, err := os.Stat(txtPath); err != nil {
		t.Errorf("expected script_draft.txt to exist: %v", err)
	}
}

func TestCreativeGenerateScript_PlainTextFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	// Adapter returns plain text (not JSON)
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{"This is a plain text script."}},
	}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{}, adapter)
	if err != nil {
		t.Fatalf("should succeed even with plain text response: %v", err)
	}
	scriptPath := filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json")
	data, _ := os.ReadFile(scriptPath)
	var output CreativeScriptOutput
	json.Unmarshal(data, &output)
	if !strings.Contains(output.Text, "plain text script") {
		t.Errorf("expected plain text in script field; got: %s", output.Text)
	}
	// Should have a warning about plain text fallback
	if len(output.Warnings) == 0 {
		t.Errorf("expected warnings about non-JSON response")
	}
}

func TestCreativeGenerateScript_OllamaCallFails_FallbackStub(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	adapter := mockScriptAdapter{
		supportsProvider: true,
		err:              fmt.Errorf("connection refused"),
	}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{
		FallbackStub: true,
	}, adapter)
	if err != nil {
		t.Fatalf("fallback-stub should succeed on Ollama error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json"))
	var output CreativeScriptOutput
	json.Unmarshal(data, &output)
	if output.Mode != "stub" {
		t.Errorf("expected stub mode on Ollama failure; got %s", output.Mode)
	}
}

func TestCreativeGenerateScript_OllamaCallFails_NoFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	adapter := mockScriptAdapter{
		supportsProvider: true,
		err:              fmt.Errorf("connection refused"),
	}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{
		FallbackStub: false,
	}, adapter)
	if err == nil {
		t.Fatal("expected error when Ollama call fails and no fallback")
	}
}

func TestCreativeGenerateScript_WithStylePack(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	// Create a style pack
	styleDir := "teststyle"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(styleDir, "profile.md"), []byte("# Profile\n\nThe creator is a tech reviewer."), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptJSON := `{"title":"Tech Review","hook":"Hook","script":"Script body.","notes":[]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{
		StyleDir: styleDir,
	}, adapter)
	if err != nil {
		t.Fatalf("expected success with style pack: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json"))
	var output CreativeScriptOutput
	json.Unmarshal(data, &output)
	if output.StyleContext == nil {
		t.Fatal("expected StyleContext to be populated")
	}
	if !output.StyleContext.Enabled {
		t.Errorf("expected StyleContext.Enabled=true")
	}
	if len(output.StyleContext.FilesUsed) == 0 {
		t.Errorf("expected FilesUsed to be non-empty")
	}
}

func TestCreativeGenerateScript_NoStyle_SkipsStylePack(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	// Create a style pack but pass --no-style
	styleDir := "nsstest"
	if err := os.MkdirAll(styleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(styleDir, "profile.md"), []byte("# Profile\n\nCreator."), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptJSON := `{"title":"T","hook":"H","script":"S","notes":[]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{
		StyleDir: styleDir,
		NoStyle:  true,
	}, adapter)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json"))
	var output CreativeScriptOutput
	json.Unmarshal(data, &output)
	if output.StyleContext != nil && output.StyleContext.Enabled {
		t.Errorf("expected style context disabled with --no-style")
	}
}

func TestCreativeGenerateScript_OverwriteRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	scriptJSON := `{"title":"T","hook":"H","script":"S","notes":[]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	// First run
	if err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{}, adapter); err != nil {
		t.Fatal(err)
	}
	// Second run without --overwrite should fail
	err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{}, adapter)
	if err == nil {
		t.Fatal("expected error when script already exists without --overwrite")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention already exists; got: %v", err)
	}
}

func TestCreativeGenerateScript_OverwriteSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	scriptJSON := `{"title":"T","hook":"H","script":"S","notes":[]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	if err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{}, adapter); err != nil {
		t.Fatal(err)
	}
	if err := creativeGenerateScriptWithAdapter(planID, ioDiscard{}, CreativeGenerateScriptOptions{Overwrite: true}, adapter); err != nil {
		t.Fatalf("expected success with --overwrite: %v", err)
	}
}

func TestCreativeGenerateScript_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	scriptJSON := `{"title":"T","hook":"H","script":"Body","notes":[]}`
	adapter := mockScriptAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{scriptJSON}},
	}
	var out bytes.Buffer
	err := creativeGenerateScriptWithAdapter(planID, &out, CreativeGenerateScriptOptions{JSON: true}, adapter)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	// JSON output is appended after the progress lines
	// Find the JSON object in the output
	text := out.String()
	jsonStart := strings.Index(text, "{")
	if jsonStart == -1 {
		t.Fatalf("no JSON found in output: %s", text)
	}
	var result CreativeScriptOutput
	if err := json.Unmarshal([]byte(text[jsonStart:]), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, text[jsonStart:])
	}
	if result.SchemaVersion != "creative_script.v1" {
		t.Errorf("expected schema_version=creative_script.v1; got %s", result.SchemaVersion)
	}
}

// ---- buildScriptPrompt tests ----

func TestBuildScriptPrompt_ContainsGoal(t *testing.T) {
	prompt := buildScriptPrompt("make cinematic clip", "input.mov", nil, 100, "")
	if !strings.Contains(prompt, "make cinematic clip") {
		t.Errorf("prompt should contain goal; got: %s", prompt)
	}
}

func TestBuildScriptPrompt_ContainsMaxWords(t *testing.T) {
	prompt := buildScriptPrompt("goal", "", nil, 75, "")
	if !strings.Contains(prompt, "75") {
		t.Errorf("prompt should contain max words; got: %s", prompt)
	}
}

func TestBuildScriptPrompt_ContainsTone(t *testing.T) {
	prompt := buildScriptPrompt("goal", "", nil, 100, "energetic")
	if !strings.Contains(prompt, "energetic") {
		t.Errorf("prompt should contain tone; got: %s", prompt)
	}
}

func TestBuildScriptPrompt_WithStyleContext(t *testing.T) {
	ctx := &StyleContext{
		Enabled:  true,
		content:  "creator: tech reviewer",
		FilesUsed: []string{"profile.md"},
	}
	prompt := buildScriptPrompt("goal", "", ctx, 100, "")
	if !strings.Contains(prompt, "creator: tech reviewer") {
		t.Errorf("prompt should include style context; got: %s", prompt)
	}
}

func TestBuildScriptPrompt_WithoutStyleContext(t *testing.T) {
	prompt := buildScriptPrompt("goal", "", nil, 100, "")
	if strings.Contains(prompt, "STYLE CONTEXT") {
		t.Errorf("prompt should not include STYLE CONTEXT section when no style; got: %s", prompt)
	}
}

func TestBuildScriptPrompt_IncludesJSONFormat(t *testing.T) {
	prompt := buildScriptPrompt("goal", "", nil, 100, "")
	if !strings.Contains(prompt, `"title"`) || !strings.Contains(prompt, `"script"`) {
		t.Errorf("prompt should include JSON format instructions; got: %s", prompt)
	}
}

// ---- parseScriptResponse tests ----

func TestParseScriptResponse_ValidJSON(t *testing.T) {
	raw := `{"title":"My Title","hook":"My hook.","script":"The full script.","notes":["note1","note2"]}`
	title, hook, script, _, notes, warnings := parseScriptResponse(raw)
	if title != "My Title" {
		t.Errorf("title=%q", title)
	}
	if hook != "My hook." {
		t.Errorf("hook=%q", hook)
	}
	if script != "The full script." {
		t.Errorf("script=%q", script)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes; got %d", len(notes))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for valid JSON; got %v", warnings)
	}
}

func TestParseScriptResponse_JSONWithMarkdownWrapper(t *testing.T) {
	raw := "```json\n{\"title\":\"T\",\"hook\":\"H\",\"script\":\"S\",\"notes\":[]}\n```"
	_, _, script, _, _, _ := parseScriptResponse(raw)
	if script != "S" {
		t.Errorf("expected extracted script; got: %q", script)
	}
}

func TestParseScriptResponse_PlainText(t *testing.T) {
	raw := "This is just plain text.\nSecond line."
	title, hook, script, _, _, warnings := parseScriptResponse(raw)
	_ = title
	if script != raw {
		t.Errorf("expected plain text as script; got: %q", script)
	}
	if hook != "This is just plain text." {
		t.Errorf("expected first line as hook; got: %q", hook)
	}
	if len(warnings) == 0 {
		t.Errorf("expected warning for non-JSON response")
	}
}

func TestParseScriptResponse_EmptyScriptField(t *testing.T) {
	raw := `{"title":"T","hook":"H","script":"","notes":[]}`
	_, _, script, _, _, warnings := parseScriptResponse(raw)
	// Should fall back to raw text
	if script != raw {
		t.Errorf("expected fallback to raw text when script field empty; got: %q", script)
	}
	if len(warnings) == 0 {
		t.Errorf("expected warning for empty script field")
	}
}

// ---- extractJSON tests ----

func TestExtractJSON_ValidObject(t *testing.T) {
	s := `some prefix {"key":"value"} suffix`
	got := extractJSON(s)
	if got != `{"key":"value"}` {
		t.Errorf("extractJSON=%q", got)
	}
}

func TestExtractJSON_NoObject(t *testing.T) {
	got := extractJSON("no json here")
	if got != "" {
		t.Errorf("expected empty string; got %q", got)
	}
}

// ---- wordCount tests ----

func TestWordCount(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"one two three", 3},
		{"  spaces  ", 1},
	}
	for _, c := range cases {
		got := wordCount(c.s)
		if got != c.want {
			t.Errorf("wordCount(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

// ---- ReviewScript tests ----

func TestReviewScript_RejectsNonexistentPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	err := ReviewScript("nonexistent", ioDiscard{}, ReviewScriptOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent plan")
	}
}

func TestReviewScript_PrintsScriptContent(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	// Write a script_draft.json directly
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := CreativeScriptOutput{
		SchemaVersion:  "creative_script.v1",
		CreativePlanID: planID,
		StepID:         "step_001",
		Goal:           "make cinematic clip",
		Mode:           "ollama",
		Model:          "qwen2.5:7b",
		Provider:       "ollama",
		Title:          "Great Video",
		Hook:           "You won't believe this.",
		Text:           "This is the full script body.",
	}
	data, _ := json.Marshal(script)
	if err := os.WriteFile(filepath.Join(outputsDir, "script_draft.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := ReviewScript(planID, &out, ReviewScriptOptions{}); err != nil {
		t.Fatalf("ReviewScript failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Great Video") {
		t.Errorf("output should contain title; got: %s", text)
	}
	if !strings.Contains(text, "This is the full script body.") {
		t.Errorf("output should contain script text; got: %s", text)
	}
	if !strings.Contains(text, "You won't believe this.") {
		t.Errorf("output should contain hook; got: %s", text)
	}
}

func TestReviewScript_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := CreativeScriptOutput{
		SchemaVersion:  "creative_script.v1",
		CreativePlanID: planID,
		Mode:           "stub",
		Text:           "Script text.",
	}
	data, _ := json.Marshal(script)
	os.WriteFile(filepath.Join(outputsDir, "script_draft.json"), data, 0o644)

	var out bytes.Buffer
	if err := ReviewScript(planID, &out, ReviewScriptOptions{JSON: true}); err != nil {
		t.Fatalf("ReviewScript --json failed: %v", err)
	}
	var result CreativeScriptOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if result.SchemaVersion != "creative_script.v1" {
		t.Errorf("expected schema_version; got %s", result.SchemaVersion)
	}
}

func TestReviewScript_WriteArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanWithScriptStep(t)

	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := CreativeScriptOutput{
		SchemaVersion:  "creative_script.v1",
		CreativePlanID: planID,
		Mode:           "stub",
		Text:           "Script text.",
	}
	data, _ := json.Marshal(script)
	os.WriteFile(filepath.Join(outputsDir, "script_draft.json"), data, 0o644)

	if err := ReviewScript(planID, ioDiscard{}, ReviewScriptOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("ReviewScript --write-artifact failed: %v", err)
	}
	reviewPath := filepath.Join(outputsDir, "script_review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Errorf("expected script_review.md to exist: %v", err)
	}
}

// ---- resolveScriptBackend tests ----

func TestResolveScriptBackend_NoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	// No config file exists
	_, _, err := resolveScriptBackend("creative.script", "")
	if err == nil {
		t.Fatal("expected error when no config")
	}
}

func TestResolveScriptBackend_ToolsDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(config.DefaultPath, []byte("tools:\n  enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := resolveScriptBackend("creative.script", "")
	if err == nil {
		t.Fatal("expected error when tools disabled")
	}
	if !strings.Contains(err.Error(), "tools.enabled") {
		t.Errorf("error should mention tools.enabled; got: %v", err)
	}
}

func TestResolveScriptBackend_MissingRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg := `tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
`
	if err := os.WriteFile(config.DefaultPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := resolveScriptBackend("creative.script", "")
	if err == nil {
		t.Fatal("expected error when route not configured")
	}
	if !strings.Contains(err.Error(), "route") && !strings.Contains(err.Error(), "creative.script") {
		t.Errorf("error should mention route; got: %v", err)
	}
}

func TestResolveScriptBackend_ValidConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(config.DefaultPath, []byte(ollamaScriptConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	name, backend, err := resolveScriptBackend("creative.script", "")
	if err != nil {
		t.Fatalf("expected success; got: %v", err)
	}
	if name != "local_writer" {
		t.Errorf("expected backend name=local_writer; got %s", name)
	}
	if backend.Provider != "ollama" {
		t.Errorf("expected provider=ollama; got %s", backend.Provider)
	}
}

func TestResolveScriptBackend_DirectModelEntry(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(config.DefaultPath, []byte(ollamaScriptConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	name, backend, err := resolveScriptBackend("creative.script", "local_writer")
	if err != nil {
		t.Fatalf("expected success with direct model entry: %v", err)
	}
	if name != "local_writer" {
		t.Errorf("expected backend name=local_writer; got %s", name)
	}
	if backend.Model != "qwen2.5:7b" {
		t.Errorf("expected model=qwen2.5:7b; got %s", backend.Model)
	}
}

// ---- MakeSummary script fields ----

func TestMakeSummary_ScriptFields(t *testing.T) {
	summary := MakeSummary{
		SchemaVersion: "make_summary.v1",
		ScriptStatus:  "completed",
		ScriptMode:    "ollama",
		ScriptModel:   "qwen2.5:7b",
		StyleUsed:     true,
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(data, &result)
	if result["script_status"] != "completed" {
		t.Errorf("expected script_status; got: %v", result["script_status"])
	}
	if result["script_model"] != "qwen2.5:7b" {
		t.Errorf("expected script_model; got: %v", result["script_model"])
	}
	if result["style_used"] != true {
		t.Errorf("expected style_used=true; got: %v", result["style_used"])
	}
}

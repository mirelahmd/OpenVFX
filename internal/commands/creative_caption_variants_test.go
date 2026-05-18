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

// mockCaptionAdapter mirrors mockScriptAdapter for caption variant tests.
type mockCaptionAdapter struct {
	supportsProvider bool
	response         modelrouter.Response
	err              error
}

func (m mockCaptionAdapter) Name() string { return "mock-caption-adapter" }
func (m mockCaptionAdapter) Supports(provider string) bool {
	return m.supportsProvider
}
func (m mockCaptionAdapter) BuildRequest(req modelrouter.Request) (modelrouter.Request, error) {
	return req, nil
}
func (m mockCaptionAdapter) Execute(req modelrouter.Request) (modelrouter.Response, error) {
	return m.response, m.err
}

const ollamaCaptionConfig = `tools:
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
    creative.captions: local_writer
`

// makePlanForCaptions creates a minimal creative plan directory.
func makePlanForCaptions(t *testing.T) string {
	t.Helper()
	if err := os.WriteFile(config.DefaultPath, []byte(ollamaCaptionConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	planID := "20260508T000000Z-test-caption-plan"
	planDir := filepath.Join(creativePlansRoot, planID)
	if err := os.MkdirAll(filepath.Join(planDir, "outputs"), 0o755); err != nil {
		t.Fatal(err)
	}

	plan := CreativePlan{
		SchemaVersion: "creative_plan.v1",
		PlanID:        planID,
		Goal:          "write captions for a product demo video",
		Steps: []CreativeStep{
			{ID: "step_0001", Type: "generate_script", Capability: "text_generation", Description: "Draft script."},
		},
	}
	if err := writeJSONFile(filepath.Join(planDir, "creative_plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	return planID
}

// ---- tests ----

func TestCaptionVariants_FallbackStub_WritesFile(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	adapter := mockCaptionAdapter{supportsProvider: false}
	opts := CaptionVariantsOptions{FallbackStub: true}
	var out bytes.Buffer
	err := captionVariantsWithAdapter(planID, &out, opts, adapter)
	if err != nil {
		t.Fatalf("unexpected error with fallback stub: %v", err)
	}

	outPath := filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("caption_variants.json not created: %v", err)
	}
	var cv CaptionVariantsOutput
	if err := json.Unmarshal(data, &cv); err != nil {
		t.Fatalf("caption_variants.json invalid JSON: %v", err)
	}
	if cv.SchemaVersion != "caption_variants.v1" {
		t.Fatalf("schema_version = %q, want caption_variants.v1", cv.SchemaVersion)
	}
	if cv.Mode != "stub" {
		t.Fatalf("mode = %q, want stub", cv.Mode)
	}
	if len(cv.Variants) == 0 {
		t.Fatal("expected at least one variant in stub output")
	}
}

func TestCaptionVariants_RejectsUnsupportedProvider(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	adapter := mockCaptionAdapter{supportsProvider: false}
	opts := CaptionVariantsOptions{}
	err := captionVariantsWithAdapter(planID, ioDiscard{}, opts, adapter)
	if err == nil {
		t.Fatal("expected error when provider is not supported")
	}
	if !strings.Contains(err.Error(), "Ollama") {
		t.Fatalf("error should mention Ollama, got: %v", err)
	}
}

func TestCaptionVariants_OllamaCallFailed_ReturnsError(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	adapter := mockCaptionAdapter{
		supportsProvider: true,
		err:              fmt.Errorf("connection refused"),
	}
	opts := CaptionVariantsOptions{}
	err := captionVariantsWithAdapter(planID, ioDiscard{}, opts, adapter)
	if err == nil {
		t.Fatal("expected error when Ollama call fails")
	}
	if !strings.Contains(err.Error(), "Ollama") {
		t.Fatalf("error should mention Ollama call failed, got: %v", err)
	}
}

func TestCaptionVariants_OllamaCallFailed_FallbackStub(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	adapter := mockCaptionAdapter{
		supportsProvider: true,
		err:              fmt.Errorf("connection refused"),
	}
	opts := CaptionVariantsOptions{FallbackStub: true}
	err := captionVariantsWithAdapter(planID, ioDiscard{}, opts, adapter)
	if err != nil {
		t.Fatalf("unexpected error with fallback stub on call failure: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json"))
	if err != nil {
		t.Fatalf("caption_variants.json not created: %v", err)
	}
	var cv CaptionVariantsOutput
	if err := json.Unmarshal(data, &cv); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if cv.Mode != "stub" {
		t.Fatalf("mode = %q, want stub", cv.Mode)
	}
}

func TestCaptionVariants_OllamaSuccess_WritesVariants(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	respJSON := `{"variants": [
		{"text": "Transform your workflow", "style": "hook", "reason": "attention-grabbing"},
		{"text": "10x your productivity", "style": "caption", "reason": "benefit-focused"},
		{"text": "Try it free today", "style": "cta", "reason": "call to action"}
	]}`
	adapter := mockCaptionAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{respJSON}},
	}
	opts := CaptionVariantsOptions{Count: 3}
	var out bytes.Buffer
	if err := captionVariantsWithAdapter(planID, &out, opts, adapter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outPath := filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("caption_variants.json not created: %v", err)
	}
	var cv CaptionVariantsOutput
	if err := json.Unmarshal(data, &cv); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if cv.SchemaVersion != "caption_variants.v1" {
		t.Fatalf("schema_version = %q", cv.SchemaVersion)
	}
	if cv.Mode != "ollama" {
		t.Fatalf("mode = %q, want ollama", cv.Mode)
	}
	if len(cv.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(cv.Variants))
	}
	if cv.Variants[0].Text != "Transform your workflow" {
		t.Fatalf("variant[0].text = %q", cv.Variants[0].Text)
	}
}

func TestCaptionVariants_RejectsIfAlreadyExists(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	// Pre-write output
	outPath := filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json")
	if err := os.WriteFile(outPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := mockCaptionAdapter{supportsProvider: true, response: modelrouter.Response{Texts: []string{}}}
	err := captionVariantsWithAdapter(planID, ioDiscard{}, CaptionVariantsOptions{}, adapter)
	if err == nil {
		t.Fatal("expected error when output already exists without --overwrite")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error should mention already exists, got: %v", err)
	}
}

func TestCaptionVariants_Overwrite_ReplacesFile(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)

	outPath := filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json")
	if err := os.WriteFile(outPath, []byte(`{"old": true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	respJSON := `{"variants": [{"text": "New caption", "style": "hook"}]}`
	adapter := mockCaptionAdapter{
		supportsProvider: true,
		response:         modelrouter.Response{Texts: []string{respJSON}},
	}
	err := captionVariantsWithAdapter(planID, ioDiscard{}, CaptionVariantsOptions{Overwrite: true}, adapter)
	if err != nil {
		t.Fatalf("unexpected error with --overwrite: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var cv CaptionVariantsOutput
	if err := json.Unmarshal(data, &cv); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(cv.Variants) != 1 || cv.Variants[0].Text != "New caption" {
		t.Fatalf("expected overwritten variant, got %+v", cv.Variants)
	}
}

func TestReviewCaptionVariants_MissingFile_ReturnsError(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)
	err := ReviewCaptionVariants(planID, ioDiscard{}, ReviewCaptionVariantsOptions{})
	if err == nil {
		t.Fatal("expected error when caption_variants.json missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention not found, got: %v", err)
	}
}

func TestReviewCaptionVariants_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	cv := CaptionVariantsOutput{
		SchemaVersion:  "caption_variants.v1",
		CreativePlanID: planID,
		Mode:           "stub",
		Source:         CaptionVariantSource{SourceType: "goal"},
		Variants: []CaptionVariant{
			{ID: "v_001", Text: "Transform your workflow", Style: "hook"},
			{ID: "v_002", Text: "Try it free", Style: "cta"},
		},
	}
	data, _ := json.Marshal(cv)
	if err := os.WriteFile(filepath.Join(outputsDir, "caption_variants.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := ReviewCaptionVariants(planID, &out, ReviewCaptionVariantsOptions{WriteArtifact: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Caption Variants Review") {
		t.Fatalf("expected 'Caption Variants Review' heading in output: %s", text)
	}
	if !strings.Contains(text, "Transform your workflow") {
		t.Fatalf("expected first variant text in output: %s", text)
	}
	mdPath := filepath.Join(outputsDir, "caption_variants_review.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("expected caption_variants_review.md to be written: %v", err)
	}
}

func TestParseCaptionVariantsResponse_ValidJSON(t *testing.T) {
	raw := `{"variants": [
		{"text": "Boost your brand", "style": "hook", "reason": "concise"},
		{"text": "Join 10k users", "style": "caption"}
	]}`
	variants, warnings := parseCaptionVariantsResponse(raw, 5, "product demo")
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(variants))
	}
	if variants[0].Text != "Boost your brand" {
		t.Fatalf("variant[0].text = %q", variants[0].Text)
	}
	if variants[0].ID == "" {
		t.Fatal("variant[0].id should be set")
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestParseCaptionVariantsResponse_LineFallback(t *testing.T) {
	raw := "Line one caption\nLine two caption\nLine three"
	variants, _ := parseCaptionVariantsResponse(raw, 5, "product demo")
	if len(variants) != 3 {
		t.Fatalf("expected 3 variants from line fallback, got %d: %+v", len(variants), variants)
	}
	if variants[0].Text != "Line one caption" {
		t.Fatalf("variant[0].text = %q", variants[0].Text)
	}
}

func TestParseCaptionVariantsResponse_EmptyFallback(t *testing.T) {
	variants, warnings := parseCaptionVariantsResponse("", 3, "my goal text")
	if len(variants) == 0 {
		t.Fatal("expected stub variants when response is empty")
	}
	if len(warnings) == 0 {
		t.Fatalf("expected at least one warning for empty/invalid response, got none")
	}
}

func TestResolveCaptionSource_PrefersScriptDraft(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	// Write a script_draft.json
	sd := CreativeScriptOutput{
		SchemaVersion: "creative_script.v1",
		Text:          "This is the draft script text for the product.",
	}
	data, _ := json.Marshal(sd)
	if err := os.WriteFile(filepath.Join(outputsDir, "script_draft.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	text, sourceType, artifact := resolveCaptionSource(planDir, "fallback goal")
	if sourceType != "script_draft" {
		t.Fatalf("sourceType = %q, want script_draft", sourceType)
	}
	if !strings.Contains(text, "draft script text") {
		t.Fatalf("expected script text, got: %q", text)
	}
	if artifact == "" {
		t.Fatal("artifact should not be empty")
	}
}

func TestResolveCaptionSource_FallsBackToGoal(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makePlanForCaptions(t)
	planDir := filepath.Join(creativePlansRoot, planID)

	text, sourceType, _ := resolveCaptionSource(planDir, "my fallback goal")
	if sourceType != "goal" {
		t.Fatalf("sourceType = %q, want goal", sourceType)
	}
	if !strings.Contains(text, "my fallback goal") {
		t.Fatalf("expected goal text, got: %q", text)
	}
}

func TestInferPlatformHint_TikTok(t *testing.T) {
	h := inferPlatformHint("make a TikTok video for my brand")
	if h != "tiktok" {
		t.Fatalf("platform_hint = %q, want tiktok", h)
	}
}

func TestInferPlatformHint_Instagram(t *testing.T) {
	h := inferPlatformHint("create Instagram Reels content")
	if h != "instagram" {
		t.Fatalf("platform_hint = %q, want instagram", h)
	}
}

func TestInferPlatformHint_YoutubShorts(t *testing.T) {
	h := inferPlatformHint("make a youtube shorts clip")
	if h != "youtube_shorts" {
		t.Fatalf("platform_hint = %q, want youtube_shorts", h)
	}
}

func TestInferPlatformHint_General(t *testing.T) {
	h := inferPlatformHint("create a product overview")
	if h != "general" {
		t.Fatalf("platform_hint = %q, want general", h)
	}
}

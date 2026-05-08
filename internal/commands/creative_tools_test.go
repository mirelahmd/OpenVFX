package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/config"
)

func TestToolsCommandPrintsEnvNamesOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ELEVENLABS_API_KEY", "secret-value")
	data := `tools:
  enabled: true
  backends:
    voice_backend:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: voice-model
      endpoint: https://api.example.com
      auth:
        type: header_env
        header: xi-api-key
        env: ELEVENLABS_API_KEY
  routes:
    creative.voiceover: voice_backend
`
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Tools(&out, ToolsOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "env=ELEVENLABS_API_KEY") {
		t.Fatalf("missing env name: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("printed env value: %s", text)
	}
}

func TestToolsValidateMissingRouteTarget(t *testing.T) {
	result := ValidateToolsConfig(config.ToolsConfig{
		Enabled: true,
		Backends: map[string]config.ToolBackendConfig{
			"local_writer": {
				Kind:     "text_generation",
				Provider: "ollama",
				Model:    "qwen2.5:7b",
				Endpoint: "http://localhost:11434",
				Auth:     config.ToolAuthConfig{Type: "none"},
			},
		},
		Routes: map[string]string{
			"creative.script": "missing_backend",
		},
	}, false, false)
	if result.Valid {
		t.Fatal("result.Valid = true, want false")
	}
	if len(result.Errors) == 0 || !strings.Contains(strings.Join(result.Errors, "\n"), "missing backend") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestToolsValidateUnknownKindWarningAndStrictError(t *testing.T) {
	tools := config.ToolsConfig{
		Enabled: true,
		Backends: map[string]config.ToolBackendConfig{
			"mystery": {
				Kind:     "future_magic",
				Provider: "custom-http",
				Endpoint: "https://example.com",
				Auth:     config.ToolAuthConfig{Type: "none"},
			},
		},
	}
	result := ValidateToolsConfig(tools, false, false)
	if !result.Valid {
		t.Fatalf("non-strict validation failed: %#v", result)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "unknown kind") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	result = ValidateToolsConfig(tools, true, false)
	if result.Valid {
		t.Fatalf("strict validation passed: %#v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0], "unknown kind") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestToolsValidateCheckEnvWarningAndStrictError(t *testing.T) {
	t.Setenv("SET_ENV_ONLY", "present")
	tools := config.ToolsConfig{
		Enabled: true,
		Backends: map[string]config.ToolBackendConfig{
			"voice_backend": {
				Kind:     "voice_generation",
				Provider: "custom-http",
				Endpoint: "https://example.com",
				Auth: config.ToolAuthConfig{
					Type:   "bearer_env",
					Env:    "MISSING_ENV",
					Header: "",
				},
			},
		},
	}
	result := ValidateToolsConfig(tools, false, true)
	if !result.Valid {
		t.Fatalf("non-strict validation failed: %#v", result)
	}
	if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, "\n"), "MISSING_ENV") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
	result = ValidateToolsConfig(tools, true, true)
	if result.Valid {
		t.Fatalf("strict validation passed: %#v", result)
	}
	if len(result.Errors) == 0 || !strings.Contains(strings.Join(result.Errors, "\n"), "MISSING_ENV") {
		t.Fatalf("errors = %#v", result.Errors)
	}
}

func TestToolsRequirementsDetectCapabilities(t *testing.T) {
	tools := config.ToolsConfig{
		Enabled: true,
		Backends: map[string]config.ToolBackendConfig{
			"writer": {
				Kind:     "text_generation",
				Provider: "ollama",
				Model:    "qwen2.5:7b",
				Endpoint: "http://localhost:11434",
				Auth:     config.ToolAuthConfig{Type: "none"},
			},
			"voice": {
				Kind:     "voice_generation",
				Provider: "custom-http",
				Endpoint: "https://voice.example.com",
				Auth:     config.ToolAuthConfig{Type: "none"},
			},
		},
		Routes: map[string]string{
			"creative.script":    "writer",
			"creative.voiceover": "voice",
		},
	}
	reqs := detectCapabilityRequirements("make a cinematic short with narration and AI b-roll captions and translate to Spanish", tools)
	text := strings.Join(func() []string {
		items := make([]string, 0, len(reqs))
		for _, req := range reqs {
			items = append(items, req.Capability+"="+req.Status)
		}
		return items
	}(), "\n")
	for _, want := range []string{
		"text_generation=satisfied",
		"voice_generation=satisfied",
		"render_composition=missing",
		"video_generation or image_generation=missing",
		"caption_generation or text_generation=satisfied",
		"translation=missing",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing requirement %q in %s", want, text)
		}
	}
	reqs = detectCapabilityRequirements("remove object", tools)
	if len(reqs) == 0 || !strings.Contains(reqs[0].Capability, "object_removal") {
		t.Fatalf("object removal reqs = %#v", reqs)
	}
}

func TestCreativePlanWritesArtifactWithSatisfiedAndMissingCapabilities(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `tools:
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
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := CreativePlanCommand(inputPath, &out, CreativePlanOptions{
		Goal:          "make a cinematic short with narration and AI b-roll",
		WriteArtifact: true,
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("creative plan count = %d", len(entries))
	}
	planPath := filepath.Join(creativePlansRoot, entries[0].Name(), "creative_plan.json")
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("missing creative_plan.json: %v", err)
	}
	plan, err := readCreativePlan(entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.RequiredCapabilities) == 0 {
		t.Fatal("required capabilities empty")
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("warnings empty, want missing capability warnings")
	}
}

func TestCreativePlanStrictFailsOnMissingRequiredRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `tools:
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
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CreativePlanCommand(inputPath, ioDiscard{}, CreativePlanOptions{
		Goal:          "make a cinematic short with narration",
		WriteArtifact: true,
		Strict:        true,
	})
	if err == nil {
		t.Fatal("CreativePlanCommand returned nil error")
	}
	if !strings.Contains(err.Error(), "missing capabilities") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativePlansInspectAndReview(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `tools:
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
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreativePlanCommand(inputPath, ioDiscard{}, CreativePlanOptions{
		Goal:          "make captions",
		WriteArtifact: true,
	}); err != nil {
		t.Fatal(err)
	}
	var plansOut bytes.Buffer
	if err := CreativePlans(&plansOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plansOut.String(), "PLAN ID") {
		t.Fatalf("creative plans output = %s", plansOut.String())
	}
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil {
		t.Fatal(err)
	}
	planID := entries[0].Name()
	var inspectOut bytes.Buffer
	if err := InspectCreativePlan(planID, &inspectOut, InspectCreativePlanOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspectOut.String(), `"schema_version": "creative_plan.v1"`) {
		t.Fatalf("inspect json = %s", inspectOut.String())
	}
	var reviewOut bytes.Buffer
	if err := ReviewCreativePlan(planID, &reviewOut, ReviewCreativePlanOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	reviewPath := filepath.Join(creativePlansRoot, planID, "creative_plan_review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatalf("missing review artifact: %v", err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

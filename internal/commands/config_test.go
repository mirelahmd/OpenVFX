package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/config"
)

func TestConfigShowRedactsSecretValuesAndPrintsEnvNames(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("OPENAI_API_KEY", "secret-value")
	data := `project:
  name: test
python:
  interpreter: python3
models:
  enabled: true
  entries:
    premium_reasoner:
      provider: openai
      model: gpt-4.1
      role: reasoner
      api_key_env: OPENAI_API_KEY
  routes:
    verification: premium_reasoner
`
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ConfigShow(&out, ConfigShowOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "api_key_env=OPENAI_API_KEY") {
		t.Fatalf("missing env name: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("printed secret value: %s", text)
	}
}

func TestModelsCommandDisabledAndEnabled(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(config.DefaultPath, []byte("models:\n  enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Models(&out, ModelsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "models are disabled") {
		t.Fatalf("disabled output = %s", out.String())
	}
	data := `models:
  enabled: true
  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434
  routes:
    caption_expansion: local_qwen
`
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Models(&out, ModelsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"local_qwen"`) || !strings.Contains(out.String(), `"base_url": "http://localhost:11434"`) {
		t.Fatalf("enabled json = %s", out.String())
	}
}

func TestModelsValidateStructuralRules(t *testing.T) {
	t.Chdir(t.TempDir())
	data := `models:
  enabled: true
  entries:
    custom_model:
      provider: made-up-provider
      model: custom-1
      role: general
      options:
        temperature: 0.2
  routes:
    caption_expansion: custom_model
`
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ModelsValidate(&out, ModelsValidateOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"valid": true`) {
		t.Fatalf("validation json = %s", out.String())
	}

	data = `models:
  enabled: true
  entries:
    broken:
      provider:
      model:
      role: thinker
  routes:
    verification: missing
`
	if err := os.WriteFile(config.DefaultPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err := ModelsValidate(&out, ModelsValidateOptions{})
	if err == nil {
		t.Fatal("ModelsValidate returned nil error")
	}
	if !strings.Contains(out.String(), "provider is required") || !strings.Contains(out.String(), "missing model entry") {
		t.Fatalf("validation output = %s", out.String())
	}
}

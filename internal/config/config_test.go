package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, []byte(DefaultContent()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Project.Name != "byom-video-project" {
		t.Fatalf("Project.Name = %q", cfg.Project.Name)
	}
	if cfg.Python.Interpreter != ".venv/bin/python" {
		t.Fatalf("Python.Interpreter = %q", cfg.Python.Interpreter)
	}
	if !cfg.Transcription.Enabled || cfg.Transcription.ModelSize != "tiny" {
		t.Fatalf("Transcription = %#v", cfg.Transcription)
	}
	if !cfg.Report.Enabled {
		t.Fatal("Report.Enabled = false, want true")
	}
	if cfg.Models.Enabled {
		t.Fatal("Models.Enabled = true, want false")
	}
	if cfg.Models.Entries["premium_reasoner"].APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("Models entries = %#v", cfg.Models.Entries)
	}
	if cfg.Models.Routes["highlight_reasoning"] != "premium_reasoner" {
		t.Fatalf("Models routes = %#v", cfg.Models.Routes)
	}
}

func TestLoadIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte("project:\n  name: test\n  unknown: value\nunknown_section:\n  enabled: true\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Project.Name != "test" {
		t.Fatalf("Project.Name = %q", cfg.Project.Name)
	}
}

func TestLoadConfigModelsEntriesRoutesSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`project:
  name: model-test
models:
  enabled: true
  entries:
    local_qwen:
      provider: custom-http
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434
      options:
        temperature: 0.2
    premium_reasoner:
      provider: openai
      model: gpt-4.1
      role: reasoner
      api_key_env: OPENAI_API_KEY
  routes:
    caption_expansion: local_qwen
    verification: premium_reasoner
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Models.Enabled {
		t.Fatal("models enabled = false, want true")
	}
	if cfg.Models.Entries["local_qwen"].Provider != "custom-http" {
		t.Fatalf("entries = %#v", cfg.Models.Entries)
	}
	if cfg.Models.Entries["local_qwen"].Options["temperature"] != 0.2 {
		t.Fatalf("options = %#v", cfg.Models.Entries["local_qwen"].Options)
	}
	if cfg.Models.Entries["premium_reasoner"].APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("entries = %#v", cfg.Models.Entries)
	}
	if cfg.Models.Routes["verification"] != "premium_reasoner" {
		t.Fatalf("routes = %#v", cfg.Models.Routes)
	}
}

func TestLoadConfigModelsOldShapeBackwardCompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`models:
  enabled: true
  providers:
    local_expander:
      provider: ollama
      model: qwen2.5:7b
  routing:
    caption_expansion: local_expander
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Models.Entries["local_expander"].Provider != "ollama" {
		t.Fatalf("entries = %#v", cfg.Models.Entries)
	}
	if cfg.Models.Routes["caption_expansion"] != "local_expander" {
		t.Fatalf("routes = %#v", cfg.Models.Routes)
	}
}

func TestLoadConfigModelsNewShapeWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`models:
  enabled: true
  entries:
    local_qwen:
      provider: qwen-local
      model: qwen2.5:7b
  providers:
    old_local:
      provider: ollama
      model: old
  routes:
    caption_expansion: local_qwen
  routing:
    caption_expansion: old_local
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if _, ok := cfg.Models.Entries["old_local"]; ok {
		t.Fatalf("old provider leaked into normalized entries: %#v", cfg.Models.Entries)
	}
	if cfg.Models.Routes["caption_expansion"] != "local_qwen" {
		t.Fatalf("routes = %#v", cfg.Models.Routes)
	}
}

func TestLoadConfigToolsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: made-up-provider
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: header_env
        header: x-api-key
        env: TEST_API_KEY
      options:
        temperature: 0.2
      response_mapping:
        output_text: $.data.output
  routes:
    creative.script: local_writer
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Tools.Enabled {
		t.Fatal("tools enabled = false, want true")
	}
	backend := cfg.Tools.Backends["local_writer"]
	if backend.Provider != "made-up-provider" {
		t.Fatalf("backend provider = %q", backend.Provider)
	}
	if backend.Auth.Env != "TEST_API_KEY" || backend.Auth.Header != "x-api-key" {
		t.Fatalf("backend auth = %#v", backend.Auth)
	}
	if backend.Options["temperature"] != 0.2 {
		t.Fatalf("backend options = %#v", backend.Options)
	}
	if backend.ResponseMapping["output_text"] != "$.data.output" {
		t.Fatalf("response mapping = %#v", backend.ResponseMapping)
	}
	if cfg.Tools.Routes["creative.script"] != "local_writer" {
		t.Fatalf("tools routes = %#v", cfg.Tools.Routes)
	}
}

func TestLoadConfigToolsUnknownProviderAccepted(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	data := []byte(`tools:
  enabled: false
  backends:
    custom_backend:
      kind: custom
      provider: future-provider
      endpoint: https://example.com
      auth:
        type: none
  routes:
    creative.anything: custom_backend
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Tools.Backends["custom_backend"].Provider != "future-provider" {
		t.Fatalf("backend provider = %#v", cfg.Tools.Backends["custom_backend"])
	}
}

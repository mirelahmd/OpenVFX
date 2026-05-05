package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const DefaultPath = "byom-video.yaml"

type Config struct {
	Project       ProjectConfig
	Python        PythonConfig
	Transcription TranscriptionConfig
	Captions      EnabledConfig
	Chunks        ChunksConfig
	Highlights    HighlightsConfig
	Roughcut      RoughcutConfig
	FFmpegScript  FFmpegScriptConfig
	Report        EnabledConfig
	Models        ModelsConfig
}

type ProjectConfig struct {
	Name string
}

type PythonConfig struct {
	Interpreter string
}

type TranscriptionConfig struct {
	Enabled   bool
	ModelSize string
}

type EnabledConfig struct {
	Enabled bool
}

type ChunksConfig struct {
	Enabled       bool
	TargetSeconds float64
	MaxGapSeconds float64
}

type HighlightsConfig struct {
	Enabled            bool
	TopK               int
	MinDurationSeconds float64
	MaxDurationSeconds float64
}

type RoughcutConfig struct {
	Enabled  bool
	MaxClips int
}

type FFmpegScriptConfig struct {
	Enabled      bool
	OutputFormat string
	Mode         string
}

type ModelsConfig struct {
	Present   bool
	Enabled   bool
	Entries   map[string]ModelEntryConfig
	Routes    map[string]string
	Providers map[string]ModelEntryConfig
	Routing   map[string]string
}

type ModelEntryConfig struct {
	Provider  string         `json:"provider"`
	Model     string         `json:"model"`
	Role      string         `json:"role,omitempty"`
	APIKeyEnv string         `json:"api_key_env,omitempty"`
	BaseURL   string         `json:"base_url,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

func DefaultContent() string {
	return `project:
  name: byom-video-project

python:
  interpreter: .venv/bin/python

transcription:
  enabled: true
  model_size: tiny

captions:
  enabled: true

chunks:
  enabled: true
  target_seconds: 30
  max_gap_seconds: 2.0

highlights:
  enabled: true
  top_k: 10
  min_duration_seconds: 3
  max_duration_seconds: 90

roughcut:
  enabled: true
  max_clips: 5

ffmpeg_script:
  enabled: true
  output_format: mp4
  mode: stream-copy

report:
  enabled: true

models:
  enabled: false

  entries:
    local_qwen:
      provider: ollama
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
      options:
        temperature: 0.1
        max_tokens: 1200

  routes:
    highlight_reasoning: premium_reasoner
    caption_expansion: local_qwen
    timeline_labeling: local_qwen
    verification: premium_reasoner
`
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	cfg := Config{
		Models: ModelsConfig{
			Entries:   map[string]ModelEntryConfig{},
			Routes:    map[string]string{},
			Providers: map[string]ModelEntryConfig{},
			Routing:   map[string]string{},
		},
	}
	pathByIndent := map[int]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, ":") {
			continue
		}
		indent := leadingSpaces(raw)
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		parentPath := parentPathForIndent(pathByIndent, indent)
		currentPath := joinPath(parentPath, key)
		if value == "" {
			pathByIndent[indent] = currentPath
			continue
		}
		setValue(&cfg, parentPath, key, value)
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	normalizeModels(&cfg.Models)
	return cfg, nil
}

func setValue(cfg *Config, section string, key string, value string) {
	switch section {
	case "project":
		if key == "name" {
			cfg.Project.Name = value
		}
	case "python":
		if key == "interpreter" {
			cfg.Python.Interpreter = value
		}
	case "transcription":
		switch key {
		case "enabled":
			cfg.Transcription.Enabled = parseBool(value)
		case "model_size":
			cfg.Transcription.ModelSize = value
		}
	case "captions":
		if key == "enabled" {
			cfg.Captions.Enabled = parseBool(value)
		}
	case "chunks":
		switch key {
		case "enabled":
			cfg.Chunks.Enabled = parseBool(value)
		case "target_seconds":
			cfg.Chunks.TargetSeconds = parseFloat(value)
		case "max_gap_seconds":
			cfg.Chunks.MaxGapSeconds = parseFloat(value)
		}
	case "highlights":
		switch key {
		case "enabled":
			cfg.Highlights.Enabled = parseBool(value)
		case "top_k":
			cfg.Highlights.TopK = parseInt(value)
		case "min_duration_seconds":
			cfg.Highlights.MinDurationSeconds = parseFloat(value)
		case "max_duration_seconds":
			cfg.Highlights.MaxDurationSeconds = parseFloat(value)
		}
	case "roughcut":
		switch key {
		case "enabled":
			cfg.Roughcut.Enabled = parseBool(value)
		case "max_clips":
			cfg.Roughcut.MaxClips = parseInt(value)
		}
	case "ffmpeg_script":
		switch key {
		case "enabled":
			cfg.FFmpegScript.Enabled = parseBool(value)
		case "output_format":
			cfg.FFmpegScript.OutputFormat = value
		case "mode":
			cfg.FFmpegScript.Mode = value
		}
	case "report":
		if key == "enabled" {
			cfg.Report.Enabled = parseBool(value)
		}
	case "models":
		cfg.Models.Present = true
		if key == "enabled" {
			cfg.Models.Enabled = parseBool(value)
		}
	case "models.routing":
		cfg.Models.Present = true
		if cfg.Models.Routing == nil {
			cfg.Models.Routing = map[string]string{}
		}
		cfg.Models.Routing[key] = value
	case "models.routes":
		cfg.Models.Present = true
		if cfg.Models.Routes == nil {
			cfg.Models.Routes = map[string]string{}
		}
		cfg.Models.Routes[key] = value
	default:
		if entryName, optionKey, ok := modelOptionPath(section, "models.entries.", key); ok {
			cfg.Models.Present = true
			if cfg.Models.Entries == nil {
				cfg.Models.Entries = map[string]ModelEntryConfig{}
			}
			entry := cfg.Models.Entries[entryName]
			if entry.Options == nil {
				entry.Options = map[string]any{}
			}
			entry.Options[optionKey] = parseScalar(value)
			cfg.Models.Entries[entryName] = entry
			return
		}
		if entryName, ok := strings.CutPrefix(section, "models.entries."); ok && entryName != "" {
			cfg.Models.Present = true
			if cfg.Models.Entries == nil {
				cfg.Models.Entries = map[string]ModelEntryConfig{}
			}
			entry := cfg.Models.Entries[entryName]
			setModelEntryValue(&entry, key, value)
			cfg.Models.Entries[entryName] = entry
			return
		}
		if providerName, optionKey, ok := modelOptionPath(section, "models.providers.", key); ok {
			cfg.Models.Present = true
			if cfg.Models.Providers == nil {
				cfg.Models.Providers = map[string]ModelEntryConfig{}
			}
			provider := cfg.Models.Providers[providerName]
			if provider.Options == nil {
				provider.Options = map[string]any{}
			}
			provider.Options[optionKey] = parseScalar(value)
			cfg.Models.Providers[providerName] = provider
			return
		}
		if providerName, ok := strings.CutPrefix(section, "models.providers."); ok && providerName != "" {
			cfg.Models.Present = true
			if cfg.Models.Providers == nil {
				cfg.Models.Providers = map[string]ModelEntryConfig{}
			}
			provider := cfg.Models.Providers[providerName]
			setModelEntryValue(&provider, key, value)
			cfg.Models.Providers[providerName] = provider
		}
	}
}

func setModelEntryValue(entry *ModelEntryConfig, key string, value string) {
	switch key {
	case "provider":
		entry.Provider = value
	case "model":
		entry.Model = value
	case "role":
		entry.Role = value
	case "api_key_env":
		entry.APIKeyEnv = value
	case "base_url":
		entry.BaseURL = value
	}
}

func modelOptionPath(section string, prefix string, key string) (string, string, bool) {
	rest, ok := strings.CutPrefix(section, prefix)
	if !ok {
		return "", "", false
	}
	name, suffix, ok := strings.Cut(rest, ".options")
	if !ok || name == "" || suffix != "" {
		return "", "", false
	}
	optionKey := key
	if optionKey == "" {
		return "", "", false
	}
	return name, optionKey, true
}

func normalizeModels(models *ModelsConfig) {
	if models.Entries == nil {
		models.Entries = map[string]ModelEntryConfig{}
	}
	if models.Routes == nil {
		models.Routes = map[string]string{}
	}
	if len(models.Entries) == 0 && len(models.Providers) > 0 {
		models.Entries = cloneModelEntries(models.Providers)
	}
	if len(models.Routes) == 0 && len(models.Routing) > 0 {
		models.Routes = cloneStringMap(models.Routing)
	}
}

func cloneModelEntries(in map[string]ModelEntryConfig) map[string]ModelEntryConfig {
	out := map[string]ModelEntryConfig{}
	for key, value := range in {
		if value.Options != nil {
			value.Options = cloneAnyMap(value.Options)
		}
		out[key] = value
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func leadingSpaces(value string) int {
	return len(value) - len(strings.TrimLeft(value, " "))
}

func parentPathForIndent(paths map[int]string, indent int) string {
	parentIndent := -1
	parent := ""
	for candidateIndent, candidatePath := range paths {
		if candidateIndent < indent && candidateIndent > parentIndent {
			parentIndent = candidateIndent
			parent = candidatePath
		}
	}
	return parent
}

func joinPath(parent string, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

func parseBool(value string) bool {
	return strings.EqualFold(value, "true")
}

func parseScalar(value string) any {
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	return value
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

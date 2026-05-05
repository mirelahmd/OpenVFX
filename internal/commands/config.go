package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"byom-video/internal/config"
	"byom-video/internal/modelrouter"
)

type ConfigShowOptions struct{ JSON bool }
type ModelsOptions struct {
	JSON     bool
	Validate bool
	Doctor   bool
}
type ModelsValidateOptions struct{ JSON bool }
type ModelsDoctorOptions struct{ JSON bool }

type ConfigSummary struct {
	Project       string                     `json:"project"`
	Python        string                     `json:"python"`
	Transcription config.TranscriptionConfig `json:"transcription"`
	Captions      config.EnabledConfig       `json:"captions"`
	Chunks        config.ChunksConfig        `json:"chunks"`
	Highlights    config.HighlightsConfig    `json:"highlights"`
	Roughcut      config.RoughcutConfig      `json:"roughcut"`
	FFmpegScript  config.FFmpegScriptConfig  `json:"ffmpeg_script"`
	Report        config.EnabledConfig       `json:"report"`
	Models        ModelsSummary              `json:"models"`
}

type ModelsSummary struct {
	Enabled bool                               `json:"enabled"`
	Entries map[string]config.ModelEntryConfig `json:"entries,omitempty"`
	Routes  map[string]string                  `json:"routes,omitempty"`
}

type ModelsValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type ModelsDoctorEntry struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

type ModelsDoctorResult struct {
	Enabled  bool                `json:"enabled"`
	Entries  []ModelsDoctorEntry `json:"entries"`
	Warnings []string            `json:"warnings,omitempty"`
}

func ConfigShow(stdout io.Writer, opts ConfigShowOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	summary := buildConfigSummary(cfg)
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Config")
	fmt.Fprintf(stdout, "  project: %s\n", emptyDash(summary.Project))
	fmt.Fprintf(stdout, "  python:  %s\n", emptyDash(summary.Python))
	fmt.Fprintln(stdout, "  pipeline:")
	fmt.Fprintf(stdout, "    transcription: enabled=%t model_size=%s\n", cfg.Transcription.Enabled, emptyDash(cfg.Transcription.ModelSize))
	fmt.Fprintf(stdout, "    captions:      enabled=%t\n", cfg.Captions.Enabled)
	fmt.Fprintf(stdout, "    chunks:        enabled=%t target=%g max_gap=%g\n", cfg.Chunks.Enabled, cfg.Chunks.TargetSeconds, cfg.Chunks.MaxGapSeconds)
	fmt.Fprintf(stdout, "    highlights:    enabled=%t top_k=%d min=%g max=%g\n", cfg.Highlights.Enabled, cfg.Highlights.TopK, cfg.Highlights.MinDurationSeconds, cfg.Highlights.MaxDurationSeconds)
	fmt.Fprintf(stdout, "    roughcut:      enabled=%t max_clips=%d\n", cfg.Roughcut.Enabled, cfg.Roughcut.MaxClips)
	fmt.Fprintf(stdout, "    ffmpeg_script: enabled=%t output_format=%s\n", cfg.FFmpegScript.Enabled, emptyDash(cfg.FFmpegScript.OutputFormat))
	fmt.Fprintf(stdout, "    report:        enabled=%t\n", cfg.Report.Enabled)
	printModelsSummary(stdout, summary.Models, true)
	return nil
}

func Models(stdout io.Writer, opts ModelsOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	summary := buildModelsSummary(cfg.Models)
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printModelsSummary(stdout, summary, false)
	return nil
}

func ModelsValidate(stdout io.Writer, opts ModelsValidateOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	result := ValidateModelsConfig(cfg.Models)
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		if !result.Valid {
			return fmt.Errorf("model config validation failed")
		}
		return nil
	}
	fmt.Fprintln(stdout, "Model config validation")
	if result.Valid {
		fmt.Fprintln(stdout, "  status: ok")
	} else {
		fmt.Fprintln(stdout, "  status: failed")
	}
	if len(result.Errors) > 0 {
		fmt.Fprintln(stdout, "  errors:")
		for _, err := range result.Errors {
			fmt.Fprintf(stdout, "    - %s\n", err)
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(stdout, "  warnings:")
		for _, warning := range result.Warnings {
			fmt.Fprintf(stdout, "    - %s\n", warning)
		}
	}
	if !result.Valid {
		return fmt.Errorf("model config validation failed")
	}
	return nil
}

func ModelsDoctor(stdout io.Writer, opts ModelsDoctorOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	result := ModelsDoctorResult{
		Enabled: cfg.Models.Enabled,
		Entries: []ModelsDoctorEntry{},
	}
	for _, name := range sortedEntryNames(cfg.Models.Entries) {
		entry := cfg.Models.Entries[name]
		item := ModelsDoctorEntry{
			Name:     name,
			Provider: entry.Provider,
			Model:    entry.Model,
			BaseURL:  entry.BaseURL,
			Status:   "not_checked",
		}
		if !cfg.Models.Enabled {
			item.Status = "models_disabled"
			item.Message = "models.enabled is false"
		} else if entry.Provider == "ollama" || entry.Provider == "ollama-local" {
			if err := modelrouter.CheckOllama(entry.BaseURL, 10*time.Second); err != nil {
				item.Status = "unavailable"
				item.Message = err.Error()
			} else {
				item.Status = "ok"
				item.Message = "ollama reachable"
			}
		} else {
			item.Status = "unsupported_provider"
			item.Message = "doctor only checks local Ollama providers in this version"
		}
		result.Entries = append(result.Entries, item)
	}
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		for _, item := range result.Entries {
			if item.Status == "unavailable" {
				return fmt.Errorf("models doctor found unavailable providers")
			}
		}
		return nil
	}
	fmt.Fprintln(stdout, "Models doctor")
	fmt.Fprintf(stdout, "  enabled: %t\n", result.Enabled)
	for _, item := range result.Entries {
		fmt.Fprintf(stdout, "  - %s: provider=%s model=%s status=%s\n", item.Name, emptyDash(item.Provider), emptyDash(item.Model), item.Status)
		if item.BaseURL != "" {
			fmt.Fprintf(stdout, "    base_url: %s\n", item.BaseURL)
		}
		if item.Message != "" {
			fmt.Fprintf(stdout, "    message:  %s\n", item.Message)
		}
	}
	for _, item := range result.Entries {
		if item.Status == "unavailable" {
			return fmt.Errorf("models doctor found unavailable providers")
		}
	}
	return nil
}

func buildConfigSummary(cfg config.Config) ConfigSummary {
	return ConfigSummary{
		Project:       cfg.Project.Name,
		Python:        cfg.Python.Interpreter,
		Transcription: cfg.Transcription,
		Captions:      cfg.Captions,
		Chunks:        cfg.Chunks,
		Highlights:    cfg.Highlights,
		Roughcut:      cfg.Roughcut,
		FFmpegScript:  cfg.FFmpegScript,
		Report:        cfg.Report,
		Models:        buildModelsSummary(cfg.Models),
	}
}

func buildModelsSummary(models config.ModelsConfig) ModelsSummary {
	return ModelsSummary{
		Enabled: models.Enabled,
		Entries: cloneEntries(models.Entries),
		Routes:  cloneRouting(models.Routes),
	}
}

func printModelsSummary(stdout io.Writer, summary ModelsSummary, includeConfigured bool) {
	fmt.Fprintln(stdout, "  models:")
	fmt.Fprintf(stdout, "    enabled: %t\n", summary.Enabled)
	if !summary.Enabled && !includeConfigured {
		fmt.Fprintln(stdout, "    status:  models are disabled")
		return
	}
	if !summary.Enabled {
		fmt.Fprintln(stdout, "    status:  models are disabled")
	}
	if len(summary.Entries) > 0 {
		fmt.Fprintln(stdout, "    entries:")
		for _, name := range sortedEntryNames(summary.Entries) {
			entry := summary.Entries[name]
			fmt.Fprintf(stdout, "      - %s: provider=%s model=%s", name, emptyDash(entry.Provider), emptyDash(entry.Model))
			if entry.Role != "" {
				fmt.Fprintf(stdout, " role=%s", entry.Role)
			}
			if entry.BaseURL != "" {
				fmt.Fprintf(stdout, " base_url=%s", entry.BaseURL)
			}
			if entry.APIKeyEnv != "" {
				fmt.Fprintf(stdout, " api_key_env=%s", entry.APIKeyEnv)
			}
			fmt.Fprintln(stdout)
		}
	}
	if len(summary.Routes) > 0 {
		fmt.Fprintln(stdout, "    routes:")
		for _, key := range sortedRoutingKeys(summary.Routes) {
			fmt.Fprintf(stdout, "      - %s: %s\n", key, summary.Routes[key])
		}
	}
}

func ValidateModelsConfig(models config.ModelsConfig) ModelsValidationResult {
	result := ModelsValidationResult{Valid: true}
	for name, entry := range models.Entries {
		if name == "" {
			result.Errors = append(result.Errors, "model entry logical name must be non-empty")
		}
		if entry.Provider == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("model entry %q provider is required", name))
		}
		if entry.Model == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("model entry %q model is required", name))
		}
		if entry.Role != "" && !allowedModelRole(entry.Role) {
			result.Errors = append(result.Errors, fmt.Sprintf("model entry %q role %q is invalid", name, entry.Role))
		}
	}
	for route, entryName := range models.Routes {
		if route == "" {
			result.Errors = append(result.Errors, "route name must be non-empty")
		}
		if _, ok := models.Entries[entryName]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("route %q points to missing model entry %q", route, entryName))
		}
	}
	if len(models.Entries) == 0 && len(models.Routes) > 0 {
		result.Errors = append(result.Errors, "routes require at least one model entry")
	}
	result.Valid = len(result.Errors) == 0
	return result
}

func allowedModelRole(role string) bool {
	switch role {
	case "reasoner", "expander", "verifier", "general":
		return true
	default:
		return false
	}
}

func cloneEntries(in map[string]config.ModelEntryConfig) map[string]config.ModelEntryConfig {
	out := map[string]config.ModelEntryConfig{}
	for key, value := range in {
		if value.Options != nil {
			value.Options = cloneAnyMap(value.Options)
		}
		out[key] = value
	}
	return out
}

func cloneRouting(in map[string]string) map[string]string {
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

func sortedEntryNames(entries map[string]config.ModelEntryConfig) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRoutingKeys(routing map[string]string) []string {
	keys := make([]string, 0, len(routing))
	for key := range routing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

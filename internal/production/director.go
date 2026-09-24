package production

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DirectorOptions selects how the Creative Director should reason.
type DirectorOptions struct {
	// Mode is "llm" to request model reasoning, or "deterministic" to request
	// the rules-based path explicitly.
	Mode string
	// Provider/Model/Backend/Route override config resolution.
	Provider string
	Model    string
	Backend  string
	Route    string

	TimeoutSeconds int
	Temperature    float64

	PythonInterpreter string
	WorkersDir        string
}

// DirectorResult reports what the sidecar actually did.
type DirectorResult struct {
	ProductionID   string   `json:"production_id"`
	Treatment      string   `json:"treatment"`
	GraphTrace     string   `json:"graph_trace"`
	RequestedMode  string   `json:"requested_mode"`
	EffectiveMode  string   `json:"effective_mode"`
	LLMCalls       int      `json:"llm_calls"`
	FallbackUsed   bool     `json:"fallback_used"`
	FallbackReason string   `json:"fallback_reason"`
	Segments       int      `json:"segments"`
	Gaps           int      `json:"gaps"`
	Warnings       []string `json:"warnings"`
}

// DirectorRunner invokes the Python creative sidecar. The seam keeps the Go
// tests free of any Python or LangGraph dependency.
type DirectorRunner func(python, workersDir string, args []string) ([]byte, error)

// DefaultDirectorRunner shells out to the sidecar.
func DefaultDirectorRunner(python, workersDir string, args []string) ([]byte, error) {
	cmd := exec.Command(python, append([]string{"-m", "openvfx_agent_graph"}, args...)...)
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+workersDir+string(os.PathListSeparator)+os.Getenv("PYTHONPATH"))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return out, nil
}

// directorConfig is the resolved routing handed to the sidecar as JSON. Go owns
// configuration parsing so the Python side never reads byom-video.yaml and
// never chooses a provider on its own.
type directorConfig struct {
	Mode           string  `json:"mode"`
	Provider       string  `json:"provider,omitempty"`
	Model          string  `json:"model,omitempty"`
	BaseURL        string  `json:"base_url,omitempty"`
	Route          string  `json:"route,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	APIKey         string  `json:"api_key,omitempty"`
}

// RunDirector executes the Creative Director graph for a production.
//
// It returns ReasoningUnavailable (with a reason, and no error) when the
// sidecar cannot run at all. That is not a production failure: the runtime
// falls back to intent-only planning and records that no treatment exists.
func RunDirector(
	l Layout,
	productionID string,
	brief string,
	opts DirectorOptions,
	runner DirectorRunner,
) (DirectorResult, string, error) {

	python := opts.PythonInterpreter
	if python == "" {
		python = resolvePython()
	}
	if python == "" {
		return DirectorResult{EffectiveMode: ReasoningUnavailable},
			"no python interpreter available for the creative sidecar", nil
	}

	workersDir := opts.WorkersDir
	if workersDir == "" {
		workersDir = ResolveWorkersDir()
	}
	if workersDir == "" {
		return DirectorResult{EffectiveMode: ReasoningUnavailable},
			"agent sidecar not found; reinstall OpenVFX or set OPENVFX_WORKERS_DIR", nil
	}

	cfg := directorConfig{
		Mode:           strings.ToLower(strings.TrimSpace(opts.Mode)),
		Provider:       opts.Provider,
		Model:          opts.Model,
		BaseURL:        opts.Backend,
		Route:          opts.Route,
		TimeoutSeconds: opts.TimeoutSeconds,
		Temperature:    opts.Temperature,
	}
	if cfg.Mode == "" {
		cfg.Mode = ReasoningDeterministic
	}
	if err := WriteJSON(l.DirectorConfigPath(), cfg); err != nil {
		return DirectorResult{}, "", err
	}

	// The brief travels through a file rather than argv: creative requests are
	// long, contain quotes and newlines, and must not be mangled by a shell.
	briefPath := filepath.Join(filepath.Dir(l.DirectorConfigPath()), "brief.txt")
	if err := os.WriteFile(briefPath, []byte(brief), 0o644); err != nil {
		return DirectorResult{}, "", err
	}

	out, err := runner(python, workersDir, []string{
		"creative-treatment",
		"--production-id", productionID,
		"--production-dir", l.Root,
		"--brief-file", briefPath,
		"--config", l.DirectorConfigPath(),
	})
	if err != nil {
		return DirectorResult{EffectiveMode: ReasoningUnavailable},
			fmt.Sprintf("creative sidecar unavailable: %v", err), nil
	}

	var result DirectorResult
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		return DirectorResult{EffectiveMode: ReasoningUnavailable},
			fmt.Sprintf("creative sidecar produced invalid JSON: %v", jsonErr), nil
	}
	return result, "", nil
}

// LoadValidatedTreatment reads and validates the treatment the sidecar wrote.
//
// The Creative Director may be a language model, so its output is untrusted
// input. A treatment that fails validation is discarded with a reason rather
// than partially applied — a half-trusted creative plan is worse than none.
func LoadValidatedTreatment(l Layout, obs Observation) (CreativeTreatment, string) {
	t, err := ReadTreatment(l)
	if err != nil {
		return CreativeTreatment{}, fmt.Sprintf("no treatment written: %v", err)
	}
	if err := t.Validate(obs); err != nil {
		return CreativeTreatment{}, fmt.Sprintf("treatment rejected: %v", err)
	}
	return t, ""
}

// DirectorConfigFromModels resolves a model route from byom-video.yaml into
// director options. Route keys follow the existing models.routes convention.
func DirectorConfigFromModels(
	routes map[string]string,
	entries map[string]ModelEntry,
	requestedRoute string,
) (provider, model, baseURL, route string) {

	route = requestedRoute
	if route == "" {
		route = "creative_director"
	}
	name, ok := routes[route]
	if !ok || name == "" {
		return "", "", "", route
	}
	entry, ok := entries[name]
	if !ok {
		return "", "", "", route
	}
	return entry.Provider, entry.Model, entry.BaseURL, route
}

// ModelEntry mirrors the fields this package needs from config.ModelEntryConfig,
// so the production package does not depend on the config package's shape.
type ModelEntry struct {
	Provider string
	Model    string
	BaseURL  string
}

// DirectorSummaryLine renders a one-line honest description of what reasoned.
func DirectorSummaryLine(t CreativeTreatment) string {
	r := t.Reasoning
	switch r.EffectiveMode {
	case ReasoningLLM:
		return fmt.Sprintf("llm (%s %s) — semantic reasoning", r.Provider, r.Model)
	case ReasoningDeterministic:
		if r.FallbackUsed && r.RequestedMode == ReasoningLLM {
			return fmt.Sprintf("deterministic (requested llm; fell back: %s)", r.FallbackReason)
		}
		return "deterministic (rules-based; no semantic reasoning)"
	default:
		return r.EffectiveMode
	}
}

// treatmentAge is used by the diagnostic command to show staleness.
func treatmentAge(t CreativeTreatment) time.Duration {
	if t.CreatedAt.IsZero() {
		return 0
	}
	return time.Since(t.CreatedAt)
}

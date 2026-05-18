package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
)

// PlannerDiagnoseOptions controls the agent-planner-diagnose command.
type PlannerDiagnoseOptions struct {
	PlannerName           string
	PlannerModel          string
	PlannerBackend        string
	PlannerRoute          string
	PlannerTimeoutSeconds int
	Check                 bool // --check: test Ollama connectivity
	JSON                  bool
}

// PlannerDiagnoseResult is the structured output of agent-planner-diagnose.
type PlannerDiagnoseResult struct {
	RequestedMode   string `json:"requested_mode"`
	EffectiveMode   string `json:"effective_mode"`
	ResolvedModel   string `json:"resolved_model"`
	ResolvedBackend string `json:"resolved_backend"`
	ResolvedRoute   string `json:"resolved_route"`
	ConfigEnabled   bool   `json:"config_models_enabled"`
	ConfigRouteKey  string `json:"config_route_key"`
	ConfigEntryName string `json:"config_entry_name,omitempty"`

	// OllamaReachable is only set when --check is passed and planner is ollama.
	OllamaReachable *bool  `json:"ollama_reachable,omitempty"`
	OllamaCheckError string `json:"ollama_check_error,omitempty"`
}

// AgentPlannerDiagnoseCommand resolves planner configuration and optionally
// tests connectivity. It never writes artifacts.
func AgentPlannerDiagnoseCommand(stdout io.Writer, opts PlannerDiagnoseOptions) error {
	requestedMode := strings.ToLower(opts.PlannerName)
	if requestedMode == "" {
		requestedMode = "deterministic"
	}

	result := PlannerDiagnoseResult{
		RequestedMode: requestedMode,
		EffectiveMode: requestedMode,
	}

	routeKey := opts.PlannerRoute
	if routeKey == "" && requestedMode == "ollama" {
		routeKey = "agent.planning"
	}
	result.ResolvedRoute = routeKey
	result.ConfigRouteKey = routeKey

	model := opts.PlannerModel
	baseURL := opts.PlannerBackend

	cfg, cfgErr := config.Load(config.DefaultPath)
	if cfgErr == nil {
		result.ConfigEnabled = cfg.Models.Enabled
		if cfg.Models.Enabled && routeKey != "" {
			entryName, ok := cfg.Models.Routes[routeKey]
			if ok && entryName != "" {
				result.ConfigEntryName = entryName
				if entry, ok := cfg.Models.Entries[entryName]; ok {
					if model == "" {
						model = entry.Model
					}
					if baseURL == "" {
						baseURL = entry.BaseURL
					}
				}
			}
		}
	}

	result.ResolvedModel = model
	result.ResolvedBackend = baseURL

	if opts.Check && requestedMode == "ollama" {
		reachable, checkErr := checkOllamaReachable(baseURL, opts.PlannerTimeoutSeconds)
		result.OllamaReachable = &reachable
		if checkErr != nil {
			result.OllamaCheckError = checkErr.Error()
		}
	}

	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Planner diagnosis")
	fmt.Fprintf(stdout, "  requested:    %s\n", result.RequestedMode)
	fmt.Fprintf(stdout, "  effective:    %s\n", result.EffectiveMode)
	if result.ResolvedModel != "" {
		fmt.Fprintf(stdout, "  model:        %s\n", result.ResolvedModel)
	} else {
		fmt.Fprintln(stdout, "  model:        (not set)")
	}
	if result.ResolvedBackend != "" {
		fmt.Fprintf(stdout, "  backend:      %s\n", result.ResolvedBackend)
	} else if requestedMode == "ollama" {
		fmt.Fprintln(stdout, "  backend:      (not set)")
	}
	if result.ResolvedRoute != "" {
		fmt.Fprintf(stdout, "  route:        %s\n", result.ResolvedRoute)
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Config resolution")
	fmt.Fprintf(stdout, "  models.enabled: %v\n", result.ConfigEnabled)
	if result.ConfigRouteKey != "" {
		if result.ConfigEntryName != "" {
			fmt.Fprintf(stdout, "  route %q -> %q\n", result.ConfigRouteKey, result.ConfigEntryName)
		} else {
			fmt.Fprintf(stdout, "  route %q -> (not configured)\n", result.ConfigRouteKey)
		}
	}

	if opts.Check && requestedMode == "ollama" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Ollama connectivity")
		if result.OllamaReachable != nil && *result.OllamaReachable {
			fmt.Fprintf(stdout, "  status: PASS (%s is reachable)\n", result.ResolvedBackend)
		} else {
			errMsg := "unreachable"
			if result.OllamaCheckError != "" {
				errMsg = result.OllamaCheckError
			}
			fmt.Fprintf(stdout, "  status: FAIL (%s)\n", errMsg)
		}
	} else if requestedMode == "ollama" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "Ollama connectivity: (not checked — use --check to test)")
	}

	return nil
}

// checkOllamaReachable pings the Ollama server's /api/tags endpoint.
func checkOllamaReachable(baseURL string, timeoutSeconds int) (bool, error) {
	if strings.TrimSpace(baseURL) == "" {
		return false, fmt.Errorf("no backend URL configured")
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	url := strings.TrimRight(baseURL, "/") + "/api/tags"
	resp, err := client.Get(url)
	if err != nil {
		return false, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500, nil
}

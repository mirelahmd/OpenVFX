package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/production"
)

// AgentGraphRunOptions controls the agent-graph-run command.
type AgentGraphRunOptions struct {
	JSON       bool
	DryRun     bool
	WorkersDir string // path to workers/ directory; auto-detected if empty
}

// agentGraphSidecarResult is the JSON output from the Python sidecar.
type agentGraphSidecarResult struct {
	RunID             string   `json:"run_id"`
	PlanID            string   `json:"plan_id"`
	Decision          string   `json:"decision"`
	DecisionReason    string   `json:"decision_reason"`
	PolicyStatus      string   `json:"policy_status"`
	PlanIssues        []string `json:"plan_issues"`
	RepairSuggestions []string `json:"repair_suggestions"`
	Warnings          []string `json:"warnings"`
	GraphTrace        string   `json:"graph_trace"`
	AgentDecision     string   `json:"agent_decision"`
}

// agentGraphRunnerFunc is the injectable sidecar invocation — swapped in tests.
type agentGraphRunnerFunc func(pythonPath, workersDir, planDir, planID, runID string) ([]byte, error)

func defaultAgentGraphRunner(pythonPath, workersDir, planDir, planID, runID string) ([]byte, error) {
	cmd := exec.Command(pythonPath, "-m", "openvfx_agent_graph", "run",
		"--plan-id", planID,
		"--plan-dir", planDir,
		"--run-id", runID,
	)
	// Ensure the workers directory is on PYTHONPATH so openvfx_agent_graph is importable.
	cmd.Env = append(os.Environ(), "PYTHONPATH="+workersDir+string(os.PathListSeparator)+os.Getenv("PYTHONPATH"))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := []byte(strings.TrimSpace(stderr.String()))
		return detail, err
	}
	return out, nil
}

// AgentGraphRunCommand invokes the LangGraph sidecar against an existing agent plan.
func AgentGraphRunCommand(stdout io.Writer, planID string, opts AgentGraphRunOptions) error {
	return agentGraphRunCommandWithRunner(stdout, planID, opts, defaultAgentGraphRunner)
}

func agentGraphRunCommandWithRunner(
	stdout io.Writer,
	planID string,
	opts AgentGraphRunOptions,
	runner agentGraphRunnerFunc,
) error {
	if strings.TrimSpace(planID) == "" {
		return fmt.Errorf("plan id is required")
	}

	planDir := filepath.Join(agentPlansV1Root, planID)
	if _, err := os.Stat(filepath.Join(planDir, "agent_plan.json")); err != nil {
		return fmt.Errorf("plan %q not found: %w", planID, err)
	}

	runID := "graphrun-" + time.Now().UTC().Format("20060102T150405.000000000Z")

	if opts.DryRun {
		if opts.JSON {
			payload := map[string]any{
				"plan_id":  planID,
				"run_id":   runID,
				"dry_run":  true,
				"plan_dir": planDir,
			}
			data, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Agent graph run (dry-run)")
		fmt.Fprintf(stdout, "  plan id:  %s\n", planID)
		fmt.Fprintf(stdout, "  run id:   %s\n", runID)
		fmt.Fprintf(stdout, "  plan dir: %s\n", planDir)
		fmt.Fprintf(stdout, "  sidecar:  python3 -m openvfx_agent_graph run\n")
		return nil
	}

	// Resolve Python interpreter.
	pythonPath, _ := resolvePythonWithSource()
	if pythonPath == "" {
		return fmt.Errorf("python3 not found; set BYOM_VIDEO_PYTHON or install python3")
	}
	resolvedPython, err := exec.LookPath(pythonPath)
	if err != nil {
		return fmt.Errorf("python interpreter %q not found: %w", pythonPath, err)
	}

	// Resolve workers directory.
	workersDir := opts.WorkersDir
	if workersDir == "" {
		workersDir = resolveWorkersDir()
	}

	out, err := runner(resolvedPython, workersDir, planDir, planID, runID)
	if err != nil {
		// Include stderr if available (exec.ExitError).
		detail := string(out)
		if strings.TrimSpace(detail) == "" {
			detail = err.Error()
		}
		return fmt.Errorf("agent graph sidecar failed: %s", strings.TrimSpace(detail))
	}

	var result agentGraphSidecarResult
	if jsonErr := json.Unmarshal(out, &result); jsonErr != nil {
		return fmt.Errorf("sidecar produced invalid JSON: %w; raw: %s", jsonErr, strings.TrimSpace(string(out)))
	}

	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Agent graph run")
	fmt.Fprintf(stdout, "  plan id:       %s\n", result.PlanID)
	fmt.Fprintf(stdout, "  run id:        %s\n", result.RunID)
	fmt.Fprintf(stdout, "  decision:      %s\n", result.Decision)
	fmt.Fprintf(stdout, "  policy status: %s\n", result.PolicyStatus)
	fmt.Fprintf(stdout, "  path:          %s\n", planDir)
	if result.DecisionReason != "" {
		fmt.Fprintf(stdout, "  reason:        %s\n", result.DecisionReason)
	}
	for _, issue := range result.PlanIssues {
		fmt.Fprintf(stdout, "  issue:         %s\n", issue)
	}
	for _, sug := range result.RepairSuggestions {
		fmt.Fprintf(stdout, "  repair:        %s\n", sug)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(stdout, "  warning:       %s\n", w)
	}
	if result.GraphTrace != "" {
		fmt.Fprintf(stdout, "  graph trace:   %s\n", result.GraphTrace)
	}
	return nil
}

// resolveWorkersDir delegates to the shared installed-first resolver so the
// agent sidecar is found identically from a release install and a checkout.
func resolveWorkersDir() string {
	return production.ResolveWorkersDir()
}

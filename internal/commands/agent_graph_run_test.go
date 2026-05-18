package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockRunner returns the given bytes/error without invoking Python.
func mockRunner(out []byte, err error) agentGraphRunnerFunc {
	return func(_, _, _, _, _ string) ([]byte, error) {
		return out, err
	}
}

func validSidecarResult(planID, runID string) agentGraphSidecarResult {
	return agentGraphSidecarResult{
		RunID:          runID,
		PlanID:         planID,
		Decision:       "approve",
		DecisionReason: "policy allowed, plan is valid",
		PolicyStatus:   "allowed",
		PlanIssues:     []string{},
		Warnings:       []string{},
		GraphTrace:     "graph_trace.json",
		AgentDecision:  "agent_decision.json",
	}
}

func mustMarshalGraph(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// setupGraphPlanDir changes CWD to a temp dir and writes a minimal
// agent_plan.json so agentPlansV1Root (relative) resolves inside it.
func setupGraphPlanDir(t *testing.T, planID string) {
	t.Helper()
	t.Chdir(t.TempDir())
	planDir := filepath.Join(agentPlansV1Root, planID)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "agent_plan.json"), []byte(`{"plan_id":"`+planID+`"}`), 0o644); err != nil {
		t.Fatalf("write agent_plan.json: %v", err)
	}
}

// ---- basic happy-path ----

func TestAgentGraphRun_HumanOutput(t *testing.T) {
	planID := "plan-graph-001"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Agent graph run") {
		t.Errorf("expected header; got: %s", out)
	}
	if !strings.Contains(out, planID) {
		t.Errorf("expected plan id in output; got: %s", out)
	}
	if !strings.Contains(out, "approve") {
		t.Errorf("expected decision in output; got: %s", out)
	}
}

func TestAgentGraphRun_JSONOutput(t *testing.T) {
	planID := "plan-graph-002"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{JSON: true}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got agentGraphSidecarResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Decision != "approve" {
		t.Errorf("expected approve; got %q", got.Decision)
	}
	if got.PlanID != planID {
		t.Errorf("expected plan id %q; got %q", planID, got.PlanID)
	}
}

// ---- dry-run ----

func TestAgentGraphRun_DryRun_Human(t *testing.T) {
	planID := "plan-graph-dry"
	setupGraphPlanDir(t, planID)

	called := false
	runner := func(_, _, _, _, _ string) ([]byte, error) {
		called = true
		return nil, nil
	}

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{DryRun: true}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("runner should not be called in dry-run mode")
	}
	out := buf.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run indicator; got: %s", out)
	}
	if !strings.Contains(out, planID) {
		t.Errorf("expected plan id; got: %s", out)
	}
}

func TestAgentGraphRun_DryRun_JSON(t *testing.T) {
	planID := "plan-graph-dry-json"
	setupGraphPlanDir(t, planID)

	runner := func(_, _, _, _, _ string) ([]byte, error) {
		return nil, nil
	}

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{DryRun: true, JSON: true}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got); err != nil {
		t.Fatalf("not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got["plan_id"] != planID {
		t.Errorf("expected plan_id %q; got %v", planID, got["plan_id"])
	}
	if got["dry_run"] != true {
		t.Errorf("expected dry_run=true; got %v", got["dry_run"])
	}
}

// ---- error cases ----

func TestAgentGraphRun_EmptyPlanID(t *testing.T) {
	runner := mockRunner(nil, nil)
	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, "   ", AgentGraphRunOptions{}, runner)
	if err == nil {
		t.Fatal("expected error for empty plan id")
	}
	if !strings.Contains(err.Error(), "plan id is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentGraphRun_PlanNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	runner := mockRunner(nil, nil)
	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, "no-such-plan", AgentGraphRunOptions{}, runner)
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentGraphRun_SidecarFails(t *testing.T) {
	planID := "plan-graph-fail"
	setupGraphPlanDir(t, planID)

	runner := mockRunner([]byte("ModuleNotFoundError: No module named 'langgraph'"), fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner)
	if err == nil {
		t.Fatal("expected error from failed sidecar")
	}
	if !strings.Contains(err.Error(), "agent graph sidecar failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "langgraph") {
		t.Errorf("expected stderr detail in error; got: %v", err)
	}
}

func TestAgentGraphRun_SidecarInvalidJSON(t *testing.T) {
	planID := "plan-graph-badjson"
	setupGraphPlanDir(t, planID)

	runner := mockRunner([]byte("not json at all"), nil)

	var buf bytes.Buffer
	err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- sidecar result fields ----

func TestAgentGraphRun_DisplaysDecisionReason(t *testing.T) {
	planID := "plan-graph-reason"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	result.DecisionReason = "unique-reason-xyz"
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "unique-reason-xyz") {
		t.Errorf("decision reason not displayed; got: %s", buf.String())
	}
}

func TestAgentGraphRun_DisplaysPlanIssues(t *testing.T) {
	planID := "plan-graph-issues"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	result.Decision = "reject"
	result.PlanIssues = []string{"action type launch_rockets is not allowed"}
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "launch_rockets") {
		t.Errorf("plan issue not displayed; got: %s", buf.String())
	}
}

func TestAgentGraphRun_DisplaysRepairSuggestions(t *testing.T) {
	planID := "plan-graph-repair"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	result.Decision = "repair"
	result.PolicyStatus = "blocked"
	result.RepairSuggestions = []string{"byom-video approve-agent-plan plan-graph-repair --allow-provider-calls"}
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "allow-provider-calls") {
		t.Errorf("repair suggestion not displayed; got: %s", buf.String())
	}
}

func TestAgentGraphRun_DisplaysWarnings(t *testing.T) {
	planID := "plan-graph-warn"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	result.Warnings = []string{"planner used fallback: ollama failed"}
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ollama failed") {
		t.Errorf("warning not displayed; got: %s", buf.String())
	}
}

func TestAgentGraphRun_DisplaysGraphTrace(t *testing.T) {
	planID := "plan-graph-trace"
	setupGraphPlanDir(t, planID)

	result := validSidecarResult(planID, "graphrun-test")
	result.GraphTrace = "graph_trace.json"
	runner := mockRunner(mustMarshalGraph(t, result), nil)

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "graph_trace.json") {
		t.Errorf("graph trace not displayed; got: %s", buf.String())
	}
}

// ---- run id format ----

func TestAgentGraphRun_RunIDFormat(t *testing.T) {
	planID := "plan-graph-runid"
	setupGraphPlanDir(t, planID)

	var capturedRunID string
	runner := func(_, _, _, _, runID string) ([]byte, error) {
		capturedRunID = runID
		result := validSidecarResult(planID, runID)
		return mustMarshalGraph(t, result), nil
	}

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(capturedRunID, "graphrun-") {
		t.Errorf("run id should start with graphrun-; got %q", capturedRunID)
	}
}

// ---- workers dir passthrough ----

func TestAgentGraphRun_WorkersDirPassthrough(t *testing.T) {
	planID := "plan-graph-wdir"
	setupGraphPlanDir(t, planID)

	customDir := "/custom/workers"
	var capturedWorkersDir string
	runner := func(_, workersDir, _, _, _ string) ([]byte, error) {
		capturedWorkersDir = workersDir
		result := validSidecarResult(planID, "graphrun-test")
		return mustMarshalGraph(t, result), nil
	}

	var buf bytes.Buffer
	if err := agentGraphRunCommandWithRunner(&buf, planID, AgentGraphRunOptions{WorkersDir: customDir}, runner); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedWorkersDir != customDir {
		t.Errorf("expected workers dir %q; got %q", customDir, capturedWorkersDir)
	}
}

// ---- resolve workers dir ----

func TestResolveWorkersDir_EnvOverride(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("BYOM_VIDEO_WORKERS_DIR", customDir)
	got := resolveWorkersDir()
	if got != customDir {
		t.Errorf("expected %q; got %q", customDir, got)
	}
}

func TestResolveWorkersDir_CWDAncestor(t *testing.T) {
	base := t.TempDir()
	workersPath := filepath.Join(base, "workers", "openvfx_agent_graph")
	if err := os.MkdirAll(workersPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	sub := filepath.Join(base, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	t.Chdir(sub)

	t.Setenv("BYOM_VIDEO_WORKERS_DIR", "")
	got := resolveWorkersDir()
	if got != filepath.Join(base, "workers") {
		t.Errorf("expected %q; got %q", filepath.Join(base, "workers"), got)
	}
}

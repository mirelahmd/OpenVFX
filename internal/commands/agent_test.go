package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/agent"
)

func TestPlanDryRunWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: "make 2 shorts", DryRun: true}); err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Agent plan") || !strings.Contains(out.String(), "run_pipeline") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	entries, err := os.ReadDir(filepath.Join(".byom-video", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("plan entries = %d", len(entries))
	}
}

func TestInspectPlanJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: "metadata only", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".byom-video", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := InspectPlan(entries[0].Name(), &out, InspectPlanOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"schema_version": "agent_plan.v1"`) {
		t.Fatalf("unexpected JSON: %s", out.String())
	}
}

func TestInspectPlanIncludesCommandPreview(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: "make 2 shorts", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".byom-video", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := InspectPlan(entries[0].Name(), &out, InspectPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "command: ./byom-video run") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestPlanArtifactsOutputAndJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: "make 2 shorts", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := PlanArtifacts(plans[0].PlanID, &out, PlanArtifactsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agent_plan.json") || !strings.Contains(out.String(), "actions.jsonl") {
		t.Fatalf("unexpected artifacts output: %s", out.String())
	}
	out.Reset()
	if err := PlanArtifacts(plans[0].PlanID, &out, PlanArtifactsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"agent_plan"`) || !strings.Contains(out.String(), `"actions_log"`) {
		t.Fatalf("unexpected artifacts json: %s", out.String())
	}
}

func TestInspectPlanIncludesArtifactPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: "make 5 shorts", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	planID := plans[0].PlanID
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make 3 shorts"}); err != nil {
		t.Fatal(err)
	}
	if err := ReviewPlan(planID, &bytes.Buffer{}, ReviewPlanOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if err := DiffSnapshot(planID, "snapshot_0001", &bytes.Buffer{}, DiffSnapshotOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := InspectPlan(planID, &out, InspectPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "actions.jsonl") || !strings.Contains(text, "review.md") || !strings.Contains(text, "diff_current_vs_snapshot_0001.md") || !strings.Contains(text, "snapshots:  1") {
		t.Fatalf("unexpected inspect output: %s", text)
	}
}

func TestWatchPlanExecutionRequiresOnce(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := t.TempDir()
	var out bytes.Buffer
	if err := Plan(dir, &out, PlanOptions{Goal: "watch this folder for shorts", Mode: "watch", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(".byom-video", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	err = ExecutePlan(entries[0].Name(), &out)
	if err == nil || !strings.Contains(err.Error(), "watch plan execution requires --once") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanGoalAwareCreatesActionPreviews(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{
		Goal:                      "make a short clip under 60 seconds",
		DryRun:                    true,
		GoalAware:                 true,
		GoalUseOllama:             true,
		GoalFallbackDeterministic: true,
	}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "goal_rerank") || !strings.Contains(text, "goal_roughcut") {
		t.Fatalf("missing goal-aware actions: %s", text)
	}
	if !strings.Contains(text, "goal-rerank <run_id> --goal ") || !strings.Contains(text, "--use-ollama") || !strings.Contains(text, "--fallback-deterministic") {
		t.Fatalf("missing goal-aware preview flags: %s", text)
	}
}

func TestReviewPlanShowsGoalAwareInfo(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{
		Goal:                      "make a short clip under 60 seconds",
		DryRun:                    true,
		GoalAware:                 true,
		GoalUseOllama:             true,
		GoalFallbackDeterministic: true,
	}); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := ReviewPlan(plans[0].PlanID, &out, ReviewPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"goal aware:", "goal ollama:", "goal fallback:", "goal-rerank <run_id>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("review missing %q: %s", want, text)
		}
	}
}

func TestExecuteActionRunsGoalAwarePostProcessing(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"plan":           map[string]any{"title": "Rough Cut Plan", "intent": "test", "total_duration_seconds": 28.4},
		"clips": []map[string]any{
			{"id": "clip_0001", "highlight_id": "hl_0001", "source_chunk_id": "chunk_0001", "start": 0, "end": 28.4, "duration_seconds": 28.4, "order": 1, "score": 0.72, "text": "cinematic opening shot with strong visual moment", "edit_intent": "hook"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	plan := agent.Plan{
		PlanID:    "plan-1",
		InputPath: "media/input.mov",
		Goal:      "make a short clip under 60 seconds",
		Preset:    "shorts",
	}
	currentRunID := runID
	var batchID string
	rerank := &agent.Action{
		ID:      "act_0002",
		Type:    "goal_rerank",
		Status:  "planned",
		Options: map[string]any{"goal_text": "make a short clip under 60 seconds"},
	}
	if err := executeAction(rerank, plan, &currentRunID, &batchID, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "goal_rerank.json")); err != nil {
		t.Fatalf("missing goal_rerank.json: %v", err)
	}
	roughcut := &agent.Action{ID: "act_0003", Type: "goal_roughcut", Status: "planned", Options: map[string]any{"goal_aware": true}}
	if err := executeAction(roughcut, plan, &currentRunID, &batchID, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "goal_roughcut.json")); err != nil {
		t.Fatalf("missing goal_roughcut.json: %v", err)
	}
}

func TestExecutePlanSuccessSummaryIncludesNextCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := os.WriteFile(filepath.Join(runDir, "report.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "ffmpeg_commands.sh"), []byte("#!/usr/bin/env bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_rerank.json"), map[string]any{
		"schema_version": "goal_rerank.v1", "created_at": time.Now().UTC(), "run_id": runID, "goal": "make a short clip", "mode": "deterministic",
		"source":            map[string]any{"highlights_artifact": "highlights.json"},
		"constraints":       map[string]any{"max_total_duration_seconds": 60, "max_clips": 3, "preferred_style": "shorts"},
		"ranked_highlights": []map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_roughcut.json"), map[string]any{
		"schema_version": "goal_roughcut.v1", "created_at": time.Now().UTC(), "run_id": runID, "goal": "make a short clip",
		"source": map[string]any{"goal_rerank_artifact": "goal_rerank.json"},
		"plan":   map[string]any{"title": "Goal-Aware Roughcut Plan", "intent": "test", "total_duration_seconds": 0},
		"clips":  []map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	printExecutePlanSuccessSummary(&out, agent.Plan{PlanID: "plan-1", Status: "completed"}, runID, "")
	text := out.String()
	for _, want := range []string{"run id:", "run dir:", "report path:", "byom-video inspect " + runID, "byom-video goal-handoff " + runID + " --overwrite", "byom-video export " + runID} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q: %s", want, text)
		}
	}
}

func TestAgentResultWritesArtifactAndAppearsInPlanArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	plan, err := agent.NewPlan(input, "make a short clip under 60 seconds", agent.GoalOptions{GoalAware: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	plan.Status = "completed"
	plan.ApprovalStatus = "approved"
	plan.Actions[0].Status = "completed"
	plan.Actions[0].RunID = "run-goal-aware"
	plan.Actions[1].Status = "completed"
	plan.Actions[1].RunID = "run-goal-aware"
	plan.Actions[2].Status = "completed"
	plan.Actions[2].RunID = "run-goal-aware"
	if err := agent.WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AgentResultCommand(plan.PlanID, &out, AgentResultOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "run ids:") {
		t.Fatalf("unexpected agent result output: %s", out.String())
	}
	artifactPath := filepath.Join(agent.PlanDir(plan.PlanID), "agent_result.md")
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("missing agent_result.md: %v", err)
	}
	out.Reset()
	if err := PlanArtifacts(plan.PlanID, &out, PlanArtifactsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agent_result.md") {
		t.Fatalf("plan artifacts missing agent result: %s", out.String())
	}
	out.Reset()
	if err := InspectPlan(plan.PlanID, &out, InspectPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agent_result.md") {
		t.Fatalf("inspect plan missing agent result: %s", out.String())
	}
}

func TestExecutePlanFailureSummaryShowsNextCommands(t *testing.T) {
	var out bytes.Buffer
	printExecutePlanFailureSummary(&out, agent.Plan{PlanID: "plan-1"}, agent.Action{
		ID:    "act_0002",
		Type:  "goal_rerank",
		Error: "ollama unavailable",
	})
	text := out.String()
	for _, want := range []string{"Plan execution failed", "act_0002 (goal_rerank)", "ollama unavailable", "byom-video inspect-plan plan-1", "byom-video review-plan plan-1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("failure summary missing %q: %s", want, text)
		}
	}
}

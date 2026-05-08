package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/agent"
)

func TestReviewPlanShowsCommandPreview(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeTestPlan(t, "make 2 shorts")
	var out bytes.Buffer
	if err := ReviewPlan(planID, &out, ReviewPlanOptions{}); err != nil {
		t.Fatalf("ReviewPlan returned error: %v", err)
	}
	if !strings.Contains(out.String(), "command: ./byom-video run") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestReviewPlanWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeTestPlan(t, "make 2 shorts")
	var out bytes.Buffer
	if err := ReviewPlan(planID, &out, ReviewPlanOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("ReviewPlan returned error: %v", err)
	}
	path := filepath.Join(agent.PlansRoot, planID, "review.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Plan Review") || !strings.Contains(string(data), "byom-video run") {
		t.Fatalf("unexpected review artifact: %s", string(data))
	}
	logData, err := os.ReadFile(filepath.Join(agent.PlansRoot, planID, "actions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "PLAN_REVIEW_ARTIFACT_WRITTEN") {
		t.Fatalf("missing review artifact event: %s", string(logData))
	}
}

func TestApprovePlanWritesApprovalMetadata(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeTestPlan(t, "make 2 shorts")
	var out bytes.Buffer
	if err := ApprovePlan(planID, &out); err != nil {
		t.Fatalf("ApprovePlan returned error: %v", err)
	}
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ApprovalStatus != "approved" || plan.ApprovalMode != "manual" || plan.ApprovedAt == nil {
		t.Fatalf("plan approval = %#v", plan)
	}
}

func TestExecutePlanRejectsUnapprovedPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeTestPlan(t, "metadata only")
	var out bytes.Buffer
	err := ExecuteSavedPlan(planID, &out, ExecutePlanOptions{})
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("err = %v", err)
	}
}

func TestExecutePlanYesBypassesApprovalAndLogs(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := t.TempDir()
	var out bytes.Buffer
	if err := Plan(dir, &out, PlanOptions{Goal: "watch this folder for shorts", Mode: "watch", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	planID := plans[0].PlanID
	out.Reset()
	err = ExecuteSavedPlan(planID, &out, ExecutePlanOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "watch plan execution requires --once") {
		t.Fatalf("err = %v", err)
	}
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ApprovalMode != "yes_flag" {
		t.Fatalf("approval mode = %q", plan.ApprovalMode)
	}
	data, err := os.ReadFile(filepath.Join(agent.PlansRoot, planID, "actions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "PLAN_APPROVED") {
		t.Fatalf("actions log missing PLAN_APPROVED: %s", string(data))
	}
}

func TestExecutePlanValidatesBeforeExecution(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeTestPlan(t, "metadata only")
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].Type = "bogus"
	plan.ApprovalStatus = "approved"
	if err := agent.WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = ExecuteSavedPlan(planID, &out, ExecutePlanOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "plan validation failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestDiffPlanDetectsChangedGoalAndOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	a := writeTestPlan(t, "make 2 shorts")
	b := writeTestPlan(t, "make 3 shorts")
	planA, err := agent.ReadPlan(a)
	if err != nil {
		t.Fatal(err)
	}
	planB, err := agent.ReadPlan(b)
	if err != nil {
		t.Fatal(err)
	}
	diff := BuildPlanDiff(planA, planB)
	fields := []string{}
	for _, d := range diff.Differences {
		fields = append(fields, d.Field)
	}
	joined := strings.Join(fields, ",")
	if !strings.Contains(joined, "goal") || !strings.Contains(joined, "actions[0].options") {
		t.Fatalf("diff fields = %s", joined)
	}
}

func TestDiffPlanWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	a := writeTestPlan(t, "make 2 shorts")
	b := writeTestPlan(t, "make 3 shorts")
	var out bytes.Buffer
	if err := DiffPlan(a, b, &out, DiffPlanOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("DiffPlan returned error: %v", err)
	}
	path := filepath.Join(agent.PlansRoot, a, "diffs", "diff_"+a+"_vs_"+b+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Plan Diff") || !strings.Contains(string(data), "actions[0].options") {
		t.Fatalf("unexpected diff artifact: %s", string(data))
	}
}

func TestInlineExecuteMarksApprovalMode(t *testing.T) {
	t.Chdir(t.TempDir())
	input := t.TempDir()
	var out bytes.Buffer
	err := Plan(input, &out, PlanOptions{Goal: "watch this folder for shorts", Mode: "watch", Execute: true})
	if err == nil || !strings.Contains(err.Error(), "watch plan execution requires --once") {
		t.Fatalf("err = %v", err)
	}
	entries, err := os.ReadDir(agent.PlansRoot)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := agent.ReadPlan(entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	if plan.ApprovalMode != "inline_execute" {
		t.Fatalf("approval mode = %q", plan.ApprovalMode)
	}
}

func writeTestPlan(t *testing.T, goal string) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), strings.ReplaceAll(goal, " ", "_")+".mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Plan(input, &out, PlanOptions{Goal: goal, DryRun: true}); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	return plans[0].PlanID
}

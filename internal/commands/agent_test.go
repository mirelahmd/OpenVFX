package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/OpenVFX/internal/agent"
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

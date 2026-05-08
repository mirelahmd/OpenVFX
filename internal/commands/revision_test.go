package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/agent"
)

func TestReviseMakeShorterAndNClips(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlan(t, "make 5 shorts")
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make it shorter"}); err != nil {
		t.Fatal(err)
	}
	plan := readRevisionPlan(t, planID)
	if got := intActionOption(plan.Actions[0].Options, "roughcut_max_clips"); got != 4 {
		t.Fatalf("max clips = %d", got)
	}
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make 3 clips"}); err != nil {
		t.Fatal(err)
	}
	plan = readRevisionPlan(t, planID)
	if got := intActionOption(plan.Actions[0].Options, "roughcut_max_clips"); got != 3 {
		t.Fatalf("max clips = %d", got)
	}
}

func TestReviseAddValidationRemoveExportCaptionsOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlanWithOptions(t, "make 5 shorts", PlanOptions{WithExport: true})
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "add validation"}); err != nil {
		t.Fatal(err)
	}
	plan := readRevisionPlan(t, planID)
	if !hasAction(plan, "validate_run") {
		t.Fatalf("missing validate action: %#v", plan.Actions)
	}
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "remove export"}); err != nil {
		t.Fatal(err)
	}
	plan = readRevisionPlan(t, planID)
	if hasAction(plan, "export_run") {
		t.Fatalf("export action still present: %#v", plan.Actions)
	}
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "captions only"}); err != nil {
		t.Fatal(err)
	}
	plan = readRevisionPlan(t, planID)
	if plan.Actions[0].Options["with_captions"] != true || plan.Actions[0].Options["with_highlights"] == true {
		t.Fatalf("options = %#v", plan.Actions[0].Options)
	}
	if !strings.Contains(plan.Actions[0].CommandPreview, "--with-captions") || strings.Contains(plan.Actions[0].CommandPreview, "--with-highlights") {
		t.Fatalf("command preview = %q", plan.Actions[0].CommandPreview)
	}
}

func TestUnknownRevisionDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlan(t, "make 5 shorts")
	before := readRevisionPlan(t, planID)
	err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make it sparkle"})
	if err == nil {
		t.Fatal("RevisePlan returned nil error")
	}
	after := readRevisionPlan(t, planID)
	if before.ApprovalStatus != after.ApprovalStatus || intActionOption(after.Actions[0].Options, "roughcut_max_clips") != 5 {
		t.Fatalf("plan mutated: before=%#v after=%#v", before, after)
	}
}

func TestRevisionResetsApprovalAndDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlan(t, "make 5 shorts")
	if err := ApprovePlan(planID, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make 3 clips", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	plan := readRevisionPlan(t, planID)
	if plan.ApprovalStatus != "approved" {
		t.Fatalf("dry-run reset approval: %#v", plan)
	}
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make 3 clips"}); err != nil {
		t.Fatal(err)
	}
	plan = readRevisionPlan(t, planID)
	if plan.ApprovalStatus != "pending" {
		t.Fatalf("approval not reset: %#v", plan)
	}
}

func TestRevisionShowDiffAndSnapshotInspection(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlan(t, "make 5 shorts")
	var out bytes.Buffer
	if err := RevisePlan(planID, &out, RevisePlanOptions{Request: "make 3 shorts", ShowDiff: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Plan diff") {
		t.Fatalf("output missing diff: %s", out.String())
	}
	out.Reset()
	if err := Snapshots(planID, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "snapshot_0001") {
		t.Fatalf("snapshots output = %s", out.String())
	}
	out.Reset()
	if err := InspectSnapshot(planID, "snapshot_0001", &out, InspectSnapshotOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"snapshot_id": "snapshot_0001"`) {
		t.Fatalf("snapshot json = %s", out.String())
	}
}

func TestDiffSnapshotWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := writeRevisionPlan(t, "make 5 shorts")
	if err := RevisePlan(planID, &bytes.Buffer{}, RevisePlanOptions{Request: "make 3 shorts"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := DiffSnapshot(planID, "snapshot_0001", &out, DiffSnapshotOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(agent.PlansRoot, planID, "diffs", "diff_current_vs_snapshot_0001.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Plan Diff") || !strings.Contains(string(data), "roughcut_max_clips") {
		t.Fatalf("unexpected diff snapshot artifact: %s", string(data))
	}
}

func writeRevisionPlan(t *testing.T, goal string) string {
	return writeRevisionPlanWithOptions(t, goal, PlanOptions{})
}

func writeRevisionPlanWithOptions(t *testing.T, goal string, opts PlanOptions) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.Goal = goal
	opts.DryRun = true
	var out bytes.Buffer
	if err := Plan(input, &out, opts); err != nil {
		t.Fatal(err)
	}
	plans, err := agent.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	return plans[0].PlanID
}

func readRevisionPlan(t *testing.T, planID string) agent.Plan {
	t.Helper()
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func hasAction(plan agent.Plan, actionType string) bool {
	for _, action := range plan.Actions {
		if action.Type == actionType {
			return true
		}
	}
	return false
}

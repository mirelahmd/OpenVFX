package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApproveAgentPlanUpdatesStatusAndWritesEvent(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createQueueAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	plan, err := readAgentPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != agentPlanStatusApproved || plan.ApprovedAt == nil || plan.ApprovalMode != "manual" {
		t.Fatalf("plan = %#v", plan)
	}
	assertAgentPlanEvent(t, planID, "AGENT_PLAN_APPROVED")
}

func TestRejectAgentPlanUpdatesStatusReasonAndWritesEvent(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createQueueAgentPlan(t)
	if err := RejectAgentPlan(planID, ioDiscard{}, RejectAgentPlanOptions{Reason: "not now"}); err != nil {
		t.Fatal(err)
	}
	plan, err := readAgentPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != agentPlanStatusRejected || plan.RejectedAt == nil || plan.RejectionReason != "not now" {
		t.Fatalf("plan = %#v", plan)
	}
	assertAgentPlanEvent(t, planID, "AGENT_PLAN_REJECTED")
}

func TestRejectedPlanCannotConvert(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createQueueAgentPlan(t)
	if err := RejectAgentPlan(planID, ioDiscard{}, RejectAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentPlanToJobDryRunWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "linked_jobs.json")); !os.IsNotExist(err) {
		t.Fatalf("linked_jobs should not exist: %v", err)
	}
	if entries, err := os.ReadDir(jobsRoot); err == nil && len(entries) > 0 {
		t.Fatalf("jobs should not exist: %d", len(entries))
	}
}

func TestConversionRefusesUnapprovedPlanUnlessYes(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{})
	if err == nil || !strings.Contains(err.Error(), "must be approved") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{Yes: true}); err != nil {
		t.Fatal(err)
	}
	plan, _ := readAgentPlan(planID)
	if plan.Status != agentPlanStatusConverted || plan.ApprovalMode != "yes_flag" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestMakeActionConvertsToMakeJob(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	if len(linked.Jobs) != 1 || linked.Jobs[0].JobType != JobActionMake {
		t.Fatalf("linked = %#v", linked)
	}
	job, err := readJob(linked.Jobs[0].JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ActionType != JobActionMake || inputString(job.Input, "goal") == "" || inputString(job.Input, "input_path") == "" {
		t.Fatalf("job = %#v", job)
	}
	if job.ApprovalStatus != JobApprovalPending {
		t.Fatalf("approval = %s", job.ApprovalStatus)
	}
}

func TestReviseMakeActionConvertsToReviseMakeJob(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createReviseAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	job, err := readJob(linked.Jobs[0].JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ActionType != JobActionReviseMake || inputString(job.Input, "make_id") != "mk_123" || inputString(job.Input, "request") == "" {
		t.Fatalf("job = %#v", job)
	}
}

func TestValidateActionConvertsToNotRequiredValidationJob(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createValidateAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	job, err := readJob(linked.Jobs[0].JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ActionType != JobActionValidateCreativeAssemble || job.ApprovalStatus != JobApprovalNotRequired {
		t.Fatalf("job = %#v", job)
	}
}

func TestQueueHealthActionSkippedWithWarning(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createQueueAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	if len(linked.Jobs) != 0 || len(linked.Warnings) == 0 {
		t.Fatalf("linked = %#v", linked)
	}
}

func TestAgentPlanJobsListsLinkedJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AgentPlanJobs(planID, &out, AgentPlanJobsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), JobActionMake) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestConversionRefusesBlockedPolicy(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "make a short", InputPath: "missing.mov"}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConversionRefusesProviderRequiredWithoutAllow(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	plan, err := readAgentPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].RequiresProvider = true
	if err := writeAgentPlan(plan); err != nil {
		t.Fatal(err)
	}
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err = AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{})
	if err == nil || !strings.Contains(err.Error(), "requires provider calls") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConversionWithApproveJobsMarksMakeJobApproved(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{ApproveJobs: true}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	job, err := readJob(linked.Jobs[0].JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ApprovalStatus != JobApprovalApproved {
		t.Fatalf("approval = %s", job.ApprovalStatus)
	}
}

func TestConversionRefusesAlreadyConvertedUnlessForce(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{})
	if err == nil || !strings.Contains(err.Error(), "already has linked jobs") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
}

func TestReviewAndInspectShowLinkedJobAfterConversion(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewAgentPlan(planID, &out, ReviewAgentPlanCommandOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Linked Jobs") {
		t.Fatalf("review = %s", out.String())
	}
	out.Reset()
	if err := InspectAgentPlan(planID, &out, InspectAgentPlanCommandOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "linked jobs: 1") {
		t.Fatalf("inspect = %s", out.String())
	}
}

func TestConversionEventsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"AGENT_PLAN_CONVERSION_STARTED", "AGENT_PLAN_JOB_CREATED", "AGENT_PLAN_CONVERSION_COMPLETED"} {
		assertAgentPlanEvent(t, planID, eventType)
	}
}

func createMakeAgentPlan(t *testing.T) string {
	t.Helper()
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "make a vertical short with captions", InputPath: input}); err != nil {
		t.Fatal(err)
	}
	return latestAgentPlanID(t)
}

func createReviseAgentPlan(t *testing.T) string {
	t.Helper()
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "make it shorter and reassemble", MakeID: "mk_123"}); err != nil {
		t.Fatal(err)
	}
	return latestAgentPlanID(t)
}

func createValidateAgentPlan(t *testing.T) string {
	t.Helper()
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "validate creative assemble", CreativePlanID: "creative_123"}); err != nil {
		t.Fatal(err)
	}
	return latestAgentPlanID(t)
}

func createQueueAgentPlan(t *testing.T) string {
	t.Helper()
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{Goal: "queue health"}); err != nil {
		t.Fatal(err)
	}
	return latestAgentPlanID(t)
}

func readLinkedJobsForTest(t *testing.T, planID string) AgentLinkedJobs {
	t.Helper()
	linked, err := readAgentLinkedJobs(planID)
	if err != nil {
		t.Fatal(err)
	}
	if linked.SchemaVersion != linkedJobsV1Schema {
		t.Fatalf("schema = %s", linked.SchemaVersion)
	}
	return linked
}

func assertAgentPlanEvent(t *testing.T, planID string, eventType string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), eventType) {
		t.Fatalf("missing %s in %s", eventType, string(data))
	}
}

func TestAgentPlanJobsJSONValidWhenMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := AgentPlanJobs("missing", &out, AgentPlanJobsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}

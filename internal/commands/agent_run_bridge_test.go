package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentResultWithNoLinkedJobsShowsConvertCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AgentPlanResult(planID, &out, AgentResultOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agent-plan-to-job") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestAgentResultWithLinkedCompletedJobShowsOutputsAndWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{ApproveJobs: true}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	job, _ := readJob(linked.Jobs[0].JobID)
	job.Status = JobStatusCompleted
	job.Output = map[string]any{"make_id": "mk_123", "run_id": "run_123"}
	if err := writeJob(job); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := AgentPlanResult(planID, &out, AgentResultOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mk_123") {
		t.Fatalf("output = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "agent_result.md")); err != nil {
		t.Fatal(err)
	}
}

func TestAgentPlanJobsRefreshesLiveJobStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	_ = ApproveAgentPlan(planID, ioDiscard{}, ApproveAgentPlanOptions{})
	_ = AgentPlanToJob(planID, ioDiscard{}, AgentPlanToJobOptions{ApproveJobs: true})
	linked := readLinkedJobsForTest(t, planID)
	job, _ := readJob(linked.Jobs[0].JobID)
	job.Status = JobStatusCompleted
	_ = writeJob(job)
	var out bytes.Buffer
	if err := AgentPlanJobs(planID, &out, AgentPlanJobsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var refreshed AgentLinkedJobs
	if err := json.Unmarshal(out.Bytes(), &refreshed); err != nil {
		t.Fatal(err)
	}
	if refreshed.Jobs[0].Status != JobStatusCompleted {
		t.Fatalf("status = %s", refreshed.Jobs[0].Status)
	}
}

func TestAgentRunDryRunAndDefaultWriteNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := AgentRun(planID, ioDiscard{}, AgentRunOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "linked_jobs.json")); !os.IsNotExist(err) {
		t.Fatalf("linked_jobs should not exist: %v", err)
	}
	if err := AgentRun(planID, ioDiscard{}, AgentRunOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "linked_jobs.json")); !os.IsNotExist(err) {
		t.Fatalf("linked_jobs should not exist: %v", err)
	}
}

func TestAgentRunYesConvertApproveJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true, ApproveJobs: true}); err != nil {
		t.Fatal(err)
	}
	linked := readLinkedJobsForTest(t, planID)
	job, _ := readJob(linked.Jobs[0].JobID)
	if job.ApprovalStatus != JobApprovalApproved {
		t.Fatalf("approval = %s", job.ApprovalStatus)
	}
	if _, err := os.Stat(filepath.Join(agentPlansV1Root, planID, "agent_run_summary.json")); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRunRunsLinkedJobsWithFakeDeps(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	_ = AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true, ApproveJobs: true})
	ran := []string{}
	deps := fakeAgentRunDeps(func(jobID string, _ JobRunOptions) error {
		ran = append(ran, jobID)
		job, _ := readJob(jobID)
		job.Status = JobStatusCompleted
		job.Output = map[string]any{"make_id": "mk_fake"}
		return writeJob(job)
	})
	if err := agentRunWithDeps(planID, ioDiscard{}, AgentRunOptions{RunJobs: true}, deps); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 {
		t.Fatalf("ran = %#v", ran)
	}
}

func TestAgentRunRejectsRunJobsAndWorkerOnce(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	err := AgentRun(planID, ioDiscard{}, AgentRunOptions{RunJobs: true, WorkerOnce: true})
	if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentRunStartDaemonUsesFakeStarter(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	started := false
	deps := fakeAgentRunDeps(nil)
	deps.startDaemon = func(io.Writer, DaemonStartOptions) error {
		started = true
		return nil
	}
	if err := agentRunWithDeps(planID, ioDiscard{}, AgentRunOptions{StartDaemon: true}, deps); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("daemon not started")
	}
}

func TestJobResultShowsSourceAgentPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	_ = AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true})
	linked := readLinkedJobsForTest(t, planID)
	var out bytes.Buffer
	if err := JobResult(linked.Jobs[0].JobID, &out, JobResultOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), planID) || !strings.Contains(out.String(), "inspect-agent-plan") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestQueueJSONIncludesSourceAgentPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	_ = AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true})
	var out bytes.Buffer
	if err := Queue(&out, QueueOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "source_agent_plan_id") || !strings.Contains(out.String(), planID) {
		t.Fatalf("queue json = %s", out.String())
	}
}

func TestInspectAndReviewShowAgentRunAdditions(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	_ = AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true, WriteSummary: true})
	var out bytes.Buffer
	if err := InspectAgentPlan(planID, &out, InspectAgentPlanCommandOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "linked jobs") || !strings.Contains(out.String(), "agent_run_summary") {
		t.Fatalf("inspect = %s", out.String())
	}
	out.Reset()
	if err := ReviewAgentPlan(planID, &out, ReviewAgentPlanCommandOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "agent-result") {
		t.Fatalf("review = %s", out.String())
	}
}

func TestAgentRunEventsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := createMakeAgentPlan(t)
	if err := AgentRun(planID, ioDiscard{}, AgentRunOptions{Yes: true, Convert: true}); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"AGENT_RUN_STARTED", "AGENT_RUN_APPROVED_PLAN", "AGENT_RUN_CONVERTED_PLAN", "AGENT_RUN_COMPLETED"} {
		assertAgentPlanEvent(t, planID, eventType)
	}
}

func fakeAgentRunDeps(run func(string, JobRunOptions) error) agentRunDeps {
	if run == nil {
		run = func(string, JobRunOptions) error { return nil }
	}
	return agentRunDeps{
		runJob: func(jobID string, _ io.Writer, opts JobRunOptions) error {
			return run(jobID, opts)
		},
		runWorker:   func(io.Writer, JobWorkerOptions) error { return nil },
		startDaemon: func(io.Writer, DaemonStartOptions) error { return nil },
	}
}

package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobWorkerRequiresMode(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	err := JobWorker(&out, JobWorkerOptions{})
	if err == nil || !strings.Contains(err.Error(), "requires --once, --loop, or --status") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJobWorkerStatusWithoutStateFile(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := JobWorker(&out, JobWorkerOptions{Status: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status:          idle") {
		t.Fatalf("status output = %s", out.String())
	}
	out.Reset()
	if err := JobWorker(&out, JobWorkerOptions{Status: true, JSON: true}); err != nil {
		t.Fatal(err)
	}
	var payload WorkerStatusOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json output invalid: %v", err)
	}
	if payload.State.Status != "idle" {
		t.Fatalf("status = %s", payload.State.Status)
	}
}

func TestJobWorkerDryRunListsEligibleJobsAndRunsNone(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-a", JobStatusPending, JobApprovalApproved, time.Unix(10, 0))
	createWorkerTestJob(t, "job-b", JobStatusPending, JobApprovalPending, time.Unix(20, 0))
	runCount := 0
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		runCount++
		return nil
	})
	var out bytes.Buffer
	if err := jobWorkerWithDeps(&out, JobWorkerOptions{Once: true, DryRun: true, JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	if runCount != 0 {
		t.Fatalf("runCount = %d, want 0", runCount)
	}
	var summary jobWorkerSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(summary.EligibleJobs) != 1 || summary.EligibleJobs[0] != "job-a" {
		t.Fatalf("eligible jobs = %#v", summary.EligibleJobs)
	}
}

func TestJobWorkerSelectsOldestEligibleJob(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-new", JobStatusPending, JobApprovalApproved, time.Unix(30, 0))
	createWorkerTestJob(t, "job-old", JobStatusPending, JobApprovalApproved, time.Unix(10, 0))
	selected := []string{}
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		selected = append(selected, jobID)
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	var out bytes.Buffer
	if err := jobWorkerWithDeps(&out, JobWorkerOptions{Once: true}, deps); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "job-old" {
		t.Fatalf("selected = %#v", selected)
	}
}

func TestJobWorkerSkipsIneligibleJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "pending-approval", JobStatusPending, JobApprovalPending, time.Unix(1, 0))
	createWorkerTestJob(t, "rejected", JobStatusPending, JobApprovalRejected, time.Unix(2, 0))
	createWorkerTestJob(t, "cancelled", JobStatusCancelled, JobApprovalApproved, time.Unix(3, 0))
	createWorkerTestJob(t, "completed", JobStatusCompleted, JobApprovalApproved, time.Unix(4, 0))
	createWorkerTestJob(t, "failed", JobStatusFailed, JobApprovalApproved, time.Unix(5, 0))
	createWorkerTestJob(t, "running", JobStatusRunning, JobApprovalApproved, time.Unix(6, 0))
	createWorkerTestJob(t, "eligible", JobStatusPending, JobApprovalApproved, time.Unix(7, 0))
	selected := []string{}
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		selected = append(selected, jobID)
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	if err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Once: true}, deps); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "eligible" {
		t.Fatalf("selected = %#v", selected)
	}
}

func TestJobWorkerRunsNotRequiredValidationJob(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "validate-job", JobStatusPending, JobApprovalNotRequired, time.Unix(1, 0))
	selected := []string{}
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		selected = append(selected, jobID)
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	if err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Once: true}, deps); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0] != "validate-job" {
		t.Fatalf("selected = %#v", selected)
	}
}

func TestJobWorkerMaxJobsLimitAndLoop(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-1", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	createWorkerTestJob(t, "job-2", JobStatusPending, JobApprovalApproved, time.Unix(2, 0))
	createWorkerTestJob(t, "job-3", JobStatusPending, JobApprovalApproved, time.Unix(3, 0))
	selected := []string{}
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		selected = append(selected, jobID)
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	var out bytes.Buffer
	if err := jobWorkerWithDeps(&out, JobWorkerOptions{Loop: true, MaxJobs: 2, Interval: time.Millisecond}, deps); err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2 (%#v)", len(selected), selected)
	}
	if selected[0] != "job-1" || selected[1] != "job-2" {
		t.Fatalf("selected order = %#v", selected)
	}
}

func TestJobWorkerFailureIncrementsJobsFailed(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-fail", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		markJobTerminal(t, jobID, JobStatusFailed)
		return errors.New("boom")
	})
	var out bytes.Buffer
	if err := jobWorkerWithDeps(&out, JobWorkerOptions{Once: true}, deps); err != nil {
		t.Fatal(err)
	}
	state, err := readWorkerState()
	if err != nil {
		t.Fatal(err)
	}
	if state.JobsFailed != 1 {
		t.Fatalf("jobs_failed = %d", state.JobsFailed)
	}
}

func TestJobWorkerFailFastStopsAfterFirstFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-fail", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	createWorkerTestJob(t, "job-next", JobStatusPending, JobApprovalApproved, time.Unix(2, 0))
	selected := []string{}
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		selected = append(selected, jobID)
		markJobTerminal(t, jobID, JobStatusFailed)
		return errors.New("boom")
	})
	err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Loop: true, MaxJobs: 2, FailFast: true, Interval: time.Millisecond}, deps)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("selected = %#v", selected)
	}
}

func TestJobWorkerLockAcquiredAndReleased(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-1", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	if err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Once: true}, deps); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workerLockPath()); !os.IsNotExist(err) {
		t.Fatalf("worker lock still exists: %v", err)
	}
	evs, err := readWorkerEvents()
	if err != nil {
		t.Fatal(err)
	}
	types := []string{}
	for _, e := range evs {
		types = append(types, e.Type)
	}
	joined := strings.Join(types, ",")
	if !strings.Contains(joined, WorkerEventLockAcquire) || !strings.Contains(joined, WorkerEventLockRelease) {
		t.Fatalf("worker events = %s", joined)
	}
}

func TestJobWorkerExistingLockBlocks(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := WorkerLock{WorkerID: "other", PID: 123, CreatedAt: time.Now().UTC()}
	data, _ := json.Marshal(lock)
	if err := os.WriteFile(workerLockPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Once: true}, fakeWorkerDeps(nil))
	if err == nil || !strings.Contains(err.Error(), "worker lock already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJobWorkerForceLockOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := WorkerLock{WorkerID: "other", PID: 123, CreatedAt: time.Now().UTC()}
	data, _ := json.Marshal(lock)
	if err := os.WriteFile(workerLockPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "job-1", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	deps := fakeWorkerDeps(func(jobID string, _ JobRunOptions) error {
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	if err := jobWorkerWithDeps(ioDiscard{}, JobWorkerOptions{Once: true, ForceLock: true}, deps); err != nil {
		t.Fatal(err)
	}
}

func TestJobWorkerWritesStateAndJSONSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	createWorkerTestJob(t, "job-1", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	deps := fakeWorkerDeps(func(jobID string, opts JobRunOptions) error {
		if !opts.AllowProviderCalls || !opts.AllowOverwrite {
			t.Fatalf("override flags not passed: %#v", opts)
		}
		markJobTerminal(t, jobID, JobStatusCompleted)
		return nil
	})
	var out bytes.Buffer
	if err := jobWorkerWithDeps(&out, JobWorkerOptions{
		Once:               true,
		JSON:               true,
		AllowProviderCalls: true,
		AllowOverwrite:     true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	var summary jobWorkerSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if summary.JobsRun != 1 || summary.JobsSucceeded != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	state, err := readWorkerState()
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != workerStateSchema || state.JobsRun != 1 {
		t.Fatalf("worker state = %#v", state)
	}
	if _, err := os.Stat(workerEventsPath()); err != nil {
		t.Fatalf("worker events missing: %v", err)
	}
}

func createWorkerTestJob(t *testing.T, jobID string, status string, approval string, createdAt time.Time) {
	t.Helper()
	job := &Job{
		SchemaVersion:  jobSchemaVersion,
		JobID:          jobID,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		ActionType:     JobActionValidateCreativeAssemble,
		Status:         status,
		ApprovalStatus: approval,
		Policy:         JobPolicy{AllowMediaWrites: true},
		Input:          map[string]any{"plan_id": "plan-" + jobID},
	}
	if err := writeJob(job); err != nil {
		t.Fatalf("writeJob: %v", err)
	}
	// restore created_at after writeJob updates UpdatedAt only
	path := filepath.Join(jobDir(jobID), "job.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read job file: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal job file: %v", err)
	}
	payload["created_at"] = createdAt.Format(time.RFC3339Nano)
	updated := payload["updated_at"]
	_ = updated
	data, _ = json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("rewrite job file: %v", err)
	}
}

func markJobTerminal(t *testing.T, jobID string, status string) {
	t.Helper()
	job, err := readJob(jobID)
	if err != nil {
		t.Fatalf("readJob(%s): %v", jobID, err)
	}
	job.Status = status
	if err := writeJob(job); err != nil {
		t.Fatalf("writeJob(%s): %v", jobID, err)
	}
}

func fakeWorkerDeps(run func(jobID string, opts JobRunOptions) error) jobWorkerDeps {
	if run == nil {
		run = func(string, JobRunOptions) error { return nil }
	}
	return jobWorkerDeps{
		now:   func() time.Time { return time.Unix(100, 0).UTC() },
		sleep: func(time.Duration) {},
		pid:   func() int { return 4242 },
		runJob: func(jobID string, _ io.Writer, opts JobRunOptions) error {
			return run(jobID, opts)
		},
		readJob: func(jobID string) (*Job, error) {
			return readJob(jobID)
		},
	}
}

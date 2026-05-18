package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// ---- helpers ----

func makeFakeJobDeps(makeErr, reviseErr, validateErr error) jobRunDeps {
	return jobRunDeps{
		runMake: func(inputPath string, stdout io.Writer, opts MakeOptions) error {
			return makeErr
		},
		runReviseMake: func(makeID string, stdout io.Writer, opts ReviseMakeOptions) error {
			return reviseErr
		},
		runValidateAssemble: func(planID string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error {
			return validateErr
		},
	}
}

// ---- job-create tests ----

func TestJobCreate_MissingType(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{})
	if err == nil || !strings.Contains(err.Error(), "--type is required") {
		t.Fatalf("expected --type error, got: %v", err)
	}
}

func TestJobCreate_UnknownType(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{ActionType: "unknown_type"})
	if err == nil || !strings.Contains(err.Error(), "unknown action type") {
		t.Fatalf("expected unknown action type error, got: %v", err)
	}
}

func TestJobCreate_MakeType_MissingGoal(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{ActionType: JobActionMake})
	if err == nil || !strings.Contains(err.Error(), "--goal is required") {
		t.Fatalf("expected --goal error, got: %v", err)
	}
}

func TestJobCreate_ReviseMakeType_MissingMakeID(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{ActionType: JobActionReviseMake, Request: "switch to square"})
	if err == nil || !strings.Contains(err.Error(), "--make-id is required") {
		t.Fatalf("expected --make-id error, got: %v", err)
	}
}

func TestJobCreate_ReviseMakeType_MissingRequest(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{ActionType: JobActionReviseMake, MakeID: "make-abc"})
	if err == nil || !strings.Contains(err.Error(), "--request is required") {
		t.Fatalf("expected --request error, got: %v", err)
	}
}

func TestJobCreate_ValidateType_MissingPlanID(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble})
	if err == nil || !strings.Contains(err.Error(), "--plan-id is required") {
		t.Fatalf("expected --plan-id error, got: %v", err)
	}
}

func TestJobCreate_Make_CreatesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionMake,
		Goal:       "product launch demo",
		Preset:     "shorts",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should mention job ID and pending status
	out := buf.String()
	if !strings.Contains(out, "Job created:") {
		t.Errorf("expected 'Job created:' in output, got: %s", out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("expected 'pending' in output, got: %s", out)
	}
}

func TestJobCreate_Make_ApprovalPending(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionMake,
		Goal:       "product launch",
	})

	// Find the job that was created
	j := findFirstJob(t)
	if j.ApprovalStatus != JobApprovalPending {
		t.Errorf("expected approval_status pending, got: %s", j.ApprovalStatus)
	}
	if j.Status != JobStatusPending {
		t.Errorf("expected status pending, got: %s", j.Status)
	}
	if j.ActionType != JobActionMake {
		t.Errorf("expected action_type make, got: %s", j.ActionType)
	}
	if j.Input["goal"] != "product launch" {
		t.Errorf("expected input goal 'product launch', got: %v", j.Input["goal"])
	}
	if j.Policy.AllowMediaWrites != true {
		t.Error("expected allow_media_writes: true")
	}
}

func TestJobCreate_ValidateAssemble_NotRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionValidateCreativeAssemble,
		PlanID:     "plan-abc-001",
	})

	j := findFirstJob(t)
	if j.ApprovalStatus != JobApprovalNotRequired {
		t.Errorf("expected approval_status not_required, got: %s", j.ApprovalStatus)
	}
	if j.Input["plan_id"] != "plan-abc-001" {
		t.Errorf("expected input plan_id 'plan-abc-001', got: %v", j.Input["plan_id"])
	}
}

func TestJobCreate_ReviseMake_Input(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionReviseMake,
		MakeID:     "make-xyz",
		Request:    "switch to square",
		Reassemble: true,
	})

	j := findFirstJob(t)
	if j.Input["make_id"] != "make-xyz" {
		t.Errorf("expected make_id 'make-xyz', got: %v", j.Input["make_id"])
	}
	if j.Input["request"] != "switch to square" {
		t.Errorf("expected request 'switch to square', got: %v", j.Input["request"])
	}
	if v, _ := j.Input["reassemble"].(bool); !v {
		t.Error("expected reassemble: true in input")
	}
}

func TestJobCreate_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionMake,
		Goal:       "test goal",
		JSON:       true,
	})

	var j Job
	if err := json.Unmarshal(buf.Bytes(), &j); err != nil {
		t.Fatalf("expected JSON output, got: %s — err: %v", buf.String(), err)
	}
	if j.SchemaVersion != jobSchemaVersion {
		t.Errorf("expected schema_version %q, got %q", jobSchemaVersion, j.SchemaVersion)
	}
}

func TestJobCreate_EventsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType: JobActionMake,
		Goal:       "demo",
	})

	j := findFirstJob(t)
	evs, _ := readJobEvents(j.JobID)
	if len(evs) == 0 {
		t.Fatal("expected at least one event, got none")
	}
	if evs[0].Type != JobEventCreated {
		t.Errorf("expected first event %s, got %s", JobEventCreated, evs[0].Type)
	}
}

func TestJobCreate_Policy(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	_ = JobCreate(&buf, JobCreateOptions{
		ActionType:           JobActionMake,
		Goal:                 "demo",
		AllowProviderCalls:   true,
		AllowExternalNetwork: true,
		AllowOverwrite:       true,
	})

	j := findFirstJob(t)
	if !j.Policy.AllowProviderCalls {
		t.Error("expected allow_provider_calls: true")
	}
	if !j.Policy.AllowExternalNetwork {
		t.Error("expected allow_external_network: true")
	}
	if !j.Policy.AllowOverwrite {
		t.Error("expected allow_overwrite: true")
	}
}

// ---- jobs (list) tests ----

func TestJobs_Empty(t *testing.T) {
	t.Chdir(t.TempDir())
	var buf bytes.Buffer
	err := Jobs(&buf, JobsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No jobs found") {
		t.Errorf("expected 'No jobs found', got: %s", buf.String())
	}
}

func TestJobs_List(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-001"})

	var buf bytes.Buffer
	err := Jobs(&buf, JobsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "make") {
		t.Errorf("expected 'make' in output, got: %s", out)
	}
	if !strings.Contains(out, "validate_creative_assemble") {
		t.Errorf("expected 'validate_creative_assemble' in output, got: %s", out)
	}
}

func TestJobs_FilterByStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})

	var buf bytes.Buffer
	_ = Jobs(&buf, JobsOptions{Filter: "completed"})
	if strings.Contains(buf.String(), "make") {
		t.Error("pending job should be filtered out when filter=completed")
	}
}

// ---- job-inspect tests ----

func TestJobInspect_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := JobInspect("nonexistent-job", io.Discard, JobInspectOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestJobInspect_HumanReadable(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	err := JobInspect(j.JobID, &buf, JobInspectOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, j.JobID) {
		t.Errorf("expected job ID in output, got: %s", out)
	}
	if !strings.Contains(out, "make") {
		t.Errorf("expected action_type in output, got: %s", out)
	}
}

func TestJobInspect_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	_ = JobInspect(j.JobID, &buf, JobInspectOptions{JSON: true})

	var got Job
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON, got: %s — %v", buf.String(), err)
	}
	if got.JobID != j.JobID {
		t.Errorf("expected job_id %q, got %q", j.JobID, got.JobID)
	}
}

// ---- job-events tests ----

func TestJobEvents_Empty(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	err := JobEvents(j.JobID, &buf, JobEventsOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "JOB_CREATED") {
		t.Errorf("expected JOB_CREATED event, got: %s", buf.String())
	}
}

func TestJobEvents_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := JobEvents("nonexistent-job", io.Discard, JobEventsOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

// ---- job-approve tests ----

func TestJobApprove_Success(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	err := JobApprove(j.JobID, &buf, JobApproveOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	j2, _ := readJob(j.JobID)
	if j2.ApprovalStatus != JobApprovalApproved {
		t.Errorf("expected approval_status approved, got: %s", j2.ApprovalStatus)
	}
}

func TestJobApprove_AlreadyApproved(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobApprove(j.JobID, io.Discard, JobApproveOptions{})

	err := JobApprove(j.JobID, io.Discard, JobApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "already approved") {
		t.Fatalf("expected already approved error, got: %v", err)
	}
}

func TestJobApprove_NotRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-001"})
	j := findFirstJob(t)

	err := JobApprove(j.JobID, io.Discard, JobApproveOptions{})
	if err == nil || !strings.Contains(err.Error(), "not_required") {
		t.Fatalf("expected not_required error, got: %v", err)
	}
}

// ---- job-reject tests ----

func TestJobReject_Success(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	err := JobReject(j.JobID, io.Discard, JobRejectOptions{Reason: "not needed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	j2, _ := readJob(j.JobID)
	if j2.ApprovalStatus != JobApprovalRejected {
		t.Errorf("expected rejected, got: %s", j2.ApprovalStatus)
	}
}

func TestJobReject_AlreadyRejected(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobReject(j.JobID, io.Discard, JobRejectOptions{})

	err := JobReject(j.JobID, io.Discard, JobRejectOptions{})
	if err == nil || !strings.Contains(err.Error(), "already rejected") {
		t.Fatalf("expected already rejected error, got: %v", err)
	}
}

// ---- job-cancel tests ----

func TestJobCancel_Success(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	err := JobCancel(j.JobID, io.Discard, JobCancelOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	j2, _ := readJob(j.JobID)
	if j2.Status != JobStatusCancelled {
		t.Errorf("expected cancelled, got: %s", j2.Status)
	}
}

func TestJobCancel_AlreadyCancelled(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobCancel(j.JobID, io.Discard, JobCancelOptions{})

	err := JobCancel(j.JobID, io.Discard, JobCancelOptions{})
	if err == nil || !strings.Contains(err.Error(), "already cancelled") {
		t.Fatalf("expected already cancelled error, got: %v", err)
	}
}

// ---- job-run tests ----

func TestJobRun_ApprovalGate_PendingBlocked(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected approval gate error, got: %v", err)
	}
}

func TestJobRun_ApprovalGate_Bypass(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{Yes: true}, deps)
	if err != nil {
		t.Fatalf("unexpected error with --yes bypass: %v", err)
	}
}

func TestJobRun_Approved_Runs(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobApprove(j.JobID, io.Discard, JobApproveOptions{})

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	j2, _ := readJob(j.JobID)
	if j2.Status != JobStatusCompleted {
		t.Errorf("expected completed, got: %s", j2.Status)
	}
}

func TestJobRun_ValidateAssemble_NotRequired_Runs(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-001"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)
	if err != nil {
		t.Fatalf("expected validate job to run without approval, got: %v", err)
	}

	j2, _ := readJob(j.JobID)
	if j2.Status != JobStatusCompleted {
		t.Errorf("expected completed, got: %s", j2.Status)
	}
}

func TestJobRun_Failed_SetsFailed(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(fmt.Errorf("ffmpeg not found"), nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{Yes: true}, deps)
	if err == nil {
		t.Fatal("expected error from failed handler")
	}

	j2, _ := readJob(j.JobID)
	if j2.Status != JobStatusFailed {
		t.Errorf("expected failed, got: %s", j2.Status)
	}
	if len(j2.Errors) == 0 {
		t.Error("expected errors to be recorded")
	}
}

func TestJobRun_Cancelled_Blocked(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobCancel(j.JobID, io.Discard, JobCancelOptions{})

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{Yes: true}, deps)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled error, got: %v", err)
	}
}

func TestJobRun_Rejected_Blocked(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)
	_ = JobReject(j.JobID, io.Discard, JobRejectOptions{})

	deps := makeFakeJobDeps(nil, nil, nil)
	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{Yes: true}, deps)
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("expected rejected error, got: %v", err)
	}
}

func TestJobRun_AlreadyCompleted_Blocked(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-001"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	_ = jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)

	err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "already completed") {
		t.Fatalf("expected already completed error, got: %v", err)
	}
}

func TestJobRun_ReviseMake_PassesOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{
		ActionType:         JobActionReviseMake,
		MakeID:             "make-999",
		Request:            "captions top",
		AllowProviderCalls: true,
	})
	j := findFirstJob(t)
	_ = JobApprove(j.JobID, io.Discard, JobApproveOptions{})

	var gotMakeID string
	var gotRequest string
	deps := jobRunDeps{
		runReviseMake: func(makeID string, stdout io.Writer, opts ReviseMakeOptions) error {
			gotMakeID = makeID
			gotRequest = opts.Request
			return nil
		},
		runMake:             func(string, io.Writer, MakeOptions) error { return nil },
		runValidateAssemble: func(string, io.Writer, ValidateCreativeAssembleOptions) error { return nil },
	}

	if err := jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMakeID != "make-999" {
		t.Errorf("expected make_id 'make-999', got: %s", gotMakeID)
	}
	if gotRequest != "captions top" {
		t.Errorf("expected request 'captions top', got: %s", gotRequest)
	}
}

func TestJobRun_EventsRecorded(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-x"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	_ = jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)

	evs, _ := readJobEvents(j.JobID)
	types := make(map[string]bool)
	for _, e := range evs {
		types[e.Type] = true
	}
	for _, expected := range []string{JobEventCreated, JobEventRunStarted, JobEventActionStarted, JobEventActionCompleted, JobEventRunCompleted} {
		if !types[expected] {
			t.Errorf("expected event %s not found; got events: %v", expected, types)
		}
	}
}

// ---- job-validate tests ----

func TestJobValidate_Valid(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	err := JobValidate(j.JobID, &buf, JobValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "valid") {
		t.Errorf("expected 'valid' in output, got: %s", buf.String())
	}
}

func TestJobValidate_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := JobValidate("nonexistent", io.Discard, JobValidateOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

// ---- job-result tests ----

func TestJobResult_Pending(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionMake, Goal: "demo"})
	j := findFirstJob(t)

	var buf bytes.Buffer
	err := JobResult(j.JobID, &buf, JobResultOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "pending") {
		t.Errorf("expected 'pending' in output, got: %s", buf.String())
	}
}

func TestJobResult_Completed(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = JobCreate(io.Discard, JobCreateOptions{ActionType: JobActionValidateCreativeAssemble, PlanID: "plan-x"})
	j := findFirstJob(t)

	deps := makeFakeJobDeps(nil, nil, nil)
	_ = jobRunWithDeps(j.JobID, io.Discard, JobRunOptions{}, deps)

	var buf bytes.Buffer
	err := JobResult(j.JobID, &buf, JobResultOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "completed") {
		t.Errorf("expected 'completed' in output, got: %s", buf.String())
	}
}

// ---- helper ----

func findFirstJob(t *testing.T) *Job {
	t.Helper()
	entries, err := os.ReadDir(jobsRoot)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no jobs found in %s", jobsRoot)
	}
	var jobID string
	for _, e := range entries {
		if e.IsDir() {
			jobID = e.Name()
			break
		}
	}
	if jobID == "" {
		t.Fatalf("no job directories found in %s", jobsRoot)
	}
	j, err := readJob(jobID)
	if err != nil {
		t.Fatalf("readJob: %v", err)
	}
	return j
}

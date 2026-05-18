package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQueueWithNoJobsPrintsEmptySummary(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := queueWithDeps(&out, QueueOptions{}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Queue") || !strings.Contains(text, "total:             0") {
		t.Fatalf("output = %s", text)
	}
}

func TestQueueCountsJobsByStatusAndApproval(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "pending-approved", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	createWorkerTestJob(t, "pending-approval", JobStatusPending, JobApprovalPending, time.Unix(2, 0))
	createWorkerTestJob(t, "running-job", JobStatusRunning, JobApprovalApproved, time.Unix(3, 0))
	createWorkerTestJob(t, "failed-job", JobStatusFailed, JobApprovalApproved, time.Unix(4, 0))
	createWorkerTestJob(t, "completed-job", JobStatusCompleted, JobApprovalNotRequired, time.Unix(5, 0))

	var out bytes.Buffer
	if err := queueWithDeps(&out, QueueOptions{JSON: true}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if summary.Jobs.Total != 5 {
		t.Fatalf("total = %d", summary.Jobs.Total)
	}
	if summary.Jobs.ByStatus[JobStatusPending] != 2 || summary.Jobs.ByStatus[JobStatusRunning] != 1 || summary.Jobs.ByStatus[JobStatusFailed] != 1 {
		t.Fatalf("by_status = %#v", summary.Jobs.ByStatus)
	}
	if summary.Jobs.ByApproval[JobApprovalApproved] != 3 || summary.Jobs.ByApproval[JobApprovalPending] != 1 || summary.Jobs.ByApproval[JobApprovalNotRequired] != 1 {
		t.Fatalf("by_approval = %#v", summary.Jobs.ByApproval)
	}
}

func TestQueueListsApprovalNeededFailedAndRunning(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "approval-job", JobStatusPending, JobApprovalPending, time.Unix(1, 0))
	createWorkerTestJob(t, "failed-job", JobStatusFailed, JobApprovalApproved, time.Unix(2, 0))
	createWorkerTestJob(t, "running-job", JobStatusRunning, JobApprovalApproved, time.Unix(3, 0))

	var out bytes.Buffer
	if err := queueWithDeps(&out, QueueOptions{JSON: true}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(summary.Jobs.ApprovalNeeded) != 1 || summary.Jobs.ApprovalNeeded[0].JobID != "approval-job" {
		t.Fatalf("approval-needed = %#v", summary.Jobs.ApprovalNeeded)
	}
	if len(summary.Jobs.Failed) != 1 || summary.Jobs.Failed[0].JobID != "failed-job" {
		t.Fatalf("failed = %#v", summary.Jobs.Failed)
	}
	if len(summary.Jobs.Running) != 1 || summary.Jobs.Running[0].JobID != "running-job" {
		t.Fatalf("running = %#v", summary.Jobs.Running)
	}
}

func TestQueueDetectsDaemonRunningAndStalePID(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeDaemonState(&DaemonState{
		SchemaVersion: daemonStateSchema,
		DaemonID:      "d1",
		Status:        "running",
		PID:           777,
	}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	deps := fakeQueueDeps()
	deps.processAlive = func(pid int) bool { return true }
	if err := queueWithDeps(&out, QueueOptions{JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !summary.Daemon.PIDAlive || summary.Daemon.Status != "running" {
		t.Fatalf("daemon = %#v", summary.Daemon)
	}

	out.Reset()
	deps.processAlive = func(pid int) bool { return false }
	if err := queueWithDeps(&out, QueueOptions{JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	found := false
	for _, check := range summary.HealthChecks {
		if check.ID == "daemon_pid" && check.Status == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("health checks = %#v", summary.HealthChecks)
	}
}

func TestQueueDetectsWorkerLock(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(workerLockPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := WorkerLock{WorkerID: "w1", PID: 999, CreatedAt: time.Now().UTC()}
	data, _ := json.Marshal(lock)
	if err := os.WriteFile(workerLockPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	deps := fakeQueueDeps()
	deps.processAlive = func(pid int) bool { return false }
	if err := queueWithDeps(&out, QueueOptions{JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !summary.Worker.LockPresent {
		t.Fatalf("worker = %#v", summary.Worker)
	}
}

func TestQueueHealthOKWithCleanState(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := queueHealthWithDeps(&out, QueueHealthOptions{JSON: true}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if summary.Status != "ok" {
		t.Fatalf("status = %s", summary.Status)
	}
}

func TestQueueHealthWarnsForFailedAndApprovalNeededJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "approval-job", JobStatusPending, JobApprovalPending, time.Unix(1, 0))
	createWorkerTestJob(t, "failed-job", JobStatusFailed, JobApprovalApproved, time.Unix(2, 0))

	var out bytes.Buffer
	if err := queueHealthWithDeps(&out, QueueHealthOptions{JSON: true}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if summary.Status != "warning" {
		t.Fatalf("status = %s", summary.Status)
	}
}

func TestQueueHealthStrictFailsOnWarnings(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "approval-job", JobStatusPending, JobApprovalPending, time.Unix(1, 0))
	err := queueHealthWithDeps(ioDiscard{}, QueueHealthOptions{Strict: true}, fakeQueueDeps())
	if err == nil || !strings.Contains(err.Error(), "queue health failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueueHealthDetectsStaleRunningJobs(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "running-job", JobStatusRunning, JobApprovalApproved, time.Unix(1, 0))
	setJobUpdatedAt(t, "running-job", time.Unix(1, 0))
	var out bytes.Buffer
	deps := fakeQueueDeps()
	deps.now = func() time.Time { return time.Unix(3600, 0).UTC() }
	if err := queueHealthWithDeps(&out, QueueHealthOptions{JSON: true, StaleAfter: 30 * time.Minute}, deps); err != nil {
		t.Fatal(err)
	}
	var summary QueueSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(summary.Jobs.StaleRunning) != 1 || summary.Jobs.StaleRunning[0].JobID != "running-job" {
		t.Fatalf("stale running = %#v", summary.Jobs.StaleRunning)
	}
}

func TestQueueHealthWriteReportWritesArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := queueHealthWithDeps(&out, QueueHealthOptions{WriteReport: true}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(queueRoot, "queue_summary.json"),
		filepath.Join(queueRoot, "queue_health.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing artifact %s: %v", path, err)
		}
	}
}

func TestQueueNextCommandSuggestionsAppear(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(".byom-video", 0o755); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "approval-job", JobStatusPending, JobApprovalPending, time.Unix(1, 0))
	createWorkerTestJob(t, "failed-job", JobStatusFailed, JobApprovalApproved, time.Unix(2, 0))
	var out bytes.Buffer
	if err := queueWithDeps(&out, QueueOptions{}, fakeQueueDeps()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"byom-video daemon start --interval 10s",
		"byom-video job-approve approval-job",
		"byom-video job-result failed-job",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output %s", want, text)
		}
	}
}

func fakeQueueDeps() queueDeps {
	return queueDeps{
		now:          func() time.Time { return time.Unix(7200, 0).UTC() },
		processAlive: func(int) bool { return false },
		readDaemon:   readDaemonState,
		readWorker:   readWorkerState,
		readJob: func(jobID string) (*Job, error) {
			return readJob(jobID)
		},
	}
}

func setJobUpdatedAt(t *testing.T, jobID string, updatedAt time.Time) {
	t.Helper()
	path := filepath.Join(jobDir(jobID), "job.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read job file: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal job file: %v", err)
	}
	payload["updated_at"] = updatedAt.Format(time.RFC3339Nano)
	data, err = json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal job file: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("rewrite job file: %v", err)
	}
}

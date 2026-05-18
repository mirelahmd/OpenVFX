package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDaemonStartWritesStateAndPID(t *testing.T) {
	t.Chdir(t.TempDir())
	startArgs := []string{}
	deps := fakeDaemonDeps()
	deps.startProcess = func(args []string, resetLog bool) (int, error) {
		startArgs = append([]string{}, args...)
		return 4242, nil
	}
	var out bytes.Buffer
	if err := daemonStartWithDeps(&out, DaemonStartOptions{Interval: 2 * time.Second, JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	state, err := readDaemonState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "running" || state.PID != 4242 {
		t.Fatalf("daemon state = %#v", state)
	}
	if pid, ok := readDaemonPID(); !ok || pid != 4242 {
		t.Fatalf("daemon pid = %d %t", pid, ok)
	}
	if len(startArgs) == 0 || startArgs[1] != "job-worker" {
		t.Fatalf("start args = %#v", startArgs)
	}
}

func TestDaemonStartRefusesExistingLivePID(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := writeDaemonPID(999); err != nil {
		t.Fatal(err)
	}
	deps := fakeDaemonDeps()
	deps.processAlive = func(pid int) bool { return true }
	err := daemonStartWithDeps(ioDiscard{}, DaemonStartOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaemonStartStalePIDWithoutForce(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := writeDaemonPID(999); err != nil {
		t.Fatal(err)
	}
	deps := fakeDaemonDeps()
	deps.processAlive = func(pid int) bool { return false }
	err := daemonStartWithDeps(ioDiscard{}, DaemonStartOptions{}, deps)
	if err == nil || !strings.Contains(err.Error(), "stale daemon pid found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaemonStartForceClearsStalePIDAndPassesFlags(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := writeDaemonPID(999); err != nil {
		t.Fatal(err)
	}
	var gotArgs []string
	deps := fakeDaemonDeps()
	deps.processAlive = func(pid int) bool { return false }
	deps.startProcess = func(args []string, resetLog bool) (int, error) {
		gotArgs = append([]string{}, args...)
		if !resetLog {
			t.Fatal("expected resetLog=true")
		}
		return 111, nil
	}
	if err := daemonStartWithDeps(ioDiscard{}, DaemonStartOptions{
		Interval:           5 * time.Second,
		MaxJobs:            3,
		AllowProviderCalls: true,
		AllowOverwrite:     true,
		FailFast:           true,
		Force:              true,
		ResetLog:           true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"job-worker", "--loop", "--interval", "5s", "--max-jobs", "3", "--allow-provider-calls", "--allow-overwrite", "--fail-fast", "--force-lock"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in args %q", want, joined)
		}
	}
}

func TestDaemonStopUpdatesStateAndRemovesPID(t *testing.T) {
	t.Chdir(t.TempDir())
	state := DaemonState{SchemaVersion: daemonStateSchema, DaemonID: "d1", Status: "running", PID: 777}
	if err := writeDaemonState(&state); err != nil {
		t.Fatal(err)
	}
	if err := writeDaemonPID(777); err != nil {
		t.Fatal(err)
	}
	deps := fakeDaemonDeps()
	deps.processAlive = func(pid int) bool { return true }
	stopped := false
	deps.stopProcess = func(pid int, force bool) error {
		stopped = true
		return nil
	}
	if err := daemonStopWithDeps(ioDiscard{}, DaemonStopOptions{}, deps); err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Fatal("stopProcess not called")
	}
	if _, ok := readDaemonPID(); ok {
		t.Fatal("daemon pid still present")
	}
	state, err := readDaemonState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "stopped" {
		t.Fatalf("state = %#v", state)
	}
}

func TestDaemonStopHandlesMissingPIDGracefully(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := daemonStopWithDeps(&out, DaemonStopOptions{}, fakeDaemonDeps()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already stopped") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestDaemonStatusShowsStoppedWhenNoPID(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := daemonStatusWithDeps(&out, DaemonStatusOptions{}, fakeDaemonDeps()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status:                 stopped") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestDaemonStatusDetectsAlivePIDAndQueueSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	state := DaemonState{SchemaVersion: daemonStateSchema, DaemonID: "d1", Status: "running", PID: 777, WorkerIntervalSeconds: 10}
	if err := writeDaemonState(&state); err != nil {
		t.Fatal(err)
	}
	if err := writeDaemonPID(777); err != nil {
		t.Fatal(err)
	}
	createWorkerTestJob(t, "approved", JobStatusPending, JobApprovalApproved, time.Unix(1, 0))
	createWorkerTestJob(t, "needs-approval", JobStatusPending, JobApprovalPending, time.Unix(2, 0))
	createWorkerTestJob(t, "running-job", JobStatusRunning, JobApprovalApproved, time.Unix(3, 0))
	createWorkerTestJob(t, "failed-job", JobStatusFailed, JobApprovalApproved, time.Unix(4, 0))
	deps := fakeDaemonDeps()
	deps.processAlive = func(pid int) bool { return true }
	var out bytes.Buffer
	if err := daemonStatusWithDeps(&out, DaemonStatusOptions{JSON: true}, deps); err != nil {
		t.Fatal(err)
	}
	var payload daemonStatusOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !payload.PIDAlive || payload.JobQueueSummary["pending_approved"] != 1 || payload.JobQueueSummary["pending_approval"] != 1 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDaemonLogsHandlesMissingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := DaemonLogs(&out, DaemonLogsOptions{Lines: 20}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Daemon log not found") {
		t.Fatalf("output = %s", out.String())
	}
}

func TestDaemonLogsReturnsLastNLines(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "1\n2\n3\n4\n5\n"
	if err := os.WriteFile(daemonLogPath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := DaemonLogs(&out, DaemonLogsOptions{Lines: 2, JSON: true}); err != nil {
		t.Fatal(err)
	}
	var payload daemonLogsOutput
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(payload.Lines) != 2 || payload.Lines[0] != "4" || payload.Lines[1] != "5" {
		t.Fatalf("lines = %#v", payload.Lines)
	}
}

func TestDaemonEventsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	deps := fakeDaemonDeps()
	deps.startProcess = func(args []string, resetLog bool) (int, error) { return 123, nil }
	if err := daemonStartWithDeps(ioDiscard{}, DaemonStartOptions{}, deps); err != nil {
		t.Fatal(err)
	}
	state := DaemonState{SchemaVersion: daemonStateSchema, DaemonID: "d1", Status: "running", PID: 123}
	if err := writeDaemonState(&state); err != nil {
		t.Fatal(err)
	}
	if err := writeDaemonPID(123); err != nil {
		t.Fatal(err)
	}
	deps.processAlive = func(pid int) bool { return true }
	deps.stopProcess = func(pid int, force bool) error { return nil }
	if err := daemonStopWithDeps(ioDiscard{}, DaemonStopOptions{}, deps); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(daemonEventsPath())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{DaemonEventStarted, DaemonEventStopped} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing event %s in %s", want, text)
		}
	}
}

func TestDaemonStartFailedWritesState(t *testing.T) {
	t.Chdir(t.TempDir())
	deps := fakeDaemonDeps()
	deps.startProcess = func(args []string, resetLog bool) (int, error) { return 0, errors.New("start failed") }
	err := daemonStartWithDeps(ioDiscard{}, DaemonStartOptions{}, deps)
	if err == nil {
		t.Fatal("expected error")
	}
	state, readErr := readDaemonState()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if state.Status != "failed" || !strings.Contains(state.LastError, "start failed") {
		t.Fatalf("state = %#v", state)
	}
}

func fakeDaemonDeps() daemonDeps {
	return daemonDeps{
		now:        func() time.Time { return time.Unix(200, 0).UTC() },
		executable: func() (string, error) { return "/tmp/byom-video", nil },
		processAlive: func(pid int) bool {
			return false
		},
		startProcess: func(args []string, resetLog bool) (int, error) { return 1234, nil },
		stopProcess:  func(pid int, force bool) error { return nil },
		readWorker: func() (WorkerState, error) {
			return WorkerState{SchemaVersion: workerStateSchema, Status: "idle"}, nil
		},
		readJob: func(jobID string) (*Job, error) {
			return readJob(jobID)
		},
	}
}

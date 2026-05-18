package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
)

const (
	daemonRoot           = ".byom-video/daemon"
	daemonStateSchema    = "openvfx_daemon.v1"
	daemonStateFileName  = "daemon_state.json"
	daemonEventsFileName = "daemon_events.jsonl"
	daemonLogFileName    = "daemon.log"
	daemonPIDFileName    = "daemon.pid"

	DaemonEventStarted        = "DAEMON_STARTED"
	DaemonEventStartFailed    = "DAEMON_START_FAILED"
	DaemonEventStopped        = "DAEMON_STOPPED"
	DaemonEventStopFailed     = "DAEMON_STOP_FAILED"
	DaemonEventStatusChecked  = "DAEMON_STATUS_CHECKED"
	DaemonEventAlreadyRunning = "DAEMON_ALREADY_RUNNING"
	DaemonEventStalePID       = "DAEMON_STALE_PID"
	DaemonEventLogRead        = "DAEMON_LOG_READ"
)

type DaemonState struct {
	SchemaVersion         string    `json:"schema_version"`
	DaemonID              string    `json:"daemon_id"`
	Status                string    `json:"status"`
	PID                   int       `json:"pid"`
	StartedAt             time.Time `json:"started_at,omitempty"`
	StoppedAt             time.Time `json:"stopped_at,omitempty"`
	UpdatedAt             time.Time `json:"updated_at"`
	WorkerMode            string    `json:"worker_mode"`
	WorkerIntervalSeconds int       `json:"worker_interval_seconds"`
	WorkerMaxJobs         int       `json:"worker_max_jobs"`
	AllowProviderCalls    bool      `json:"allow_provider_calls"`
	AllowOverwrite        bool      `json:"allow_overwrite"`
	FailFast              bool      `json:"fail_fast,omitempty"`
	LastError             string    `json:"last_error,omitempty"`
	Warnings              []string  `json:"warnings,omitempty"`
	Errors                []string  `json:"errors,omitempty"`
}

type DaemonStartOptions struct {
	Interval           time.Duration
	MaxJobs            int
	AllowProviderCalls bool
	AllowOverwrite     bool
	FailFast           bool
	Force              bool
	ResetLog           bool
	JSON               bool
}

type DaemonStopOptions struct {
	Force bool
	JSON  bool
}

type DaemonStatusOptions struct {
	JSON bool
}

type DaemonLogsOptions struct {
	Lines int
	JSON  bool
}

type daemonStatusOutput struct {
	State           DaemonState    `json:"state"`
	PIDAlive        bool           `json:"pid_alive"`
	LogPath         string         `json:"log_path"`
	WorkerState     *WorkerState   `json:"worker_state,omitempty"`
	JobQueueSummary map[string]int `json:"job_queue_summary,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

type daemonLogsOutput struct {
	Path  string   `json:"path"`
	Lines []string `json:"lines"`
}

type daemonDeps struct {
	now          func() time.Time
	executable   func() (string, error)
	processAlive func(int) bool
	startProcess func([]string, bool) (int, error)
	stopProcess  func(int, bool) error
	readWorker   func() (WorkerState, error)
	readJob      func(string) (*Job, error)
}

var defaultDaemonDeps = daemonDeps{
	now:        func() time.Time { return time.Now().UTC() },
	executable: os.Executable,
	processAlive: func(pid int) bool {
		return processExists(pid)
	},
	startProcess: func(args []string, resetLog bool) (int, error) {
		return startDaemonProcess(args, resetLog)
	},
	stopProcess: func(pid int, force bool) error {
		return stopDaemonProcess(pid, force)
	},
	readWorker: readWorkerState,
	readJob: func(jobID string) (*Job, error) {
		return readJob(jobID)
	},
}

func DaemonStart(stdout io.Writer, opts DaemonStartOptions) error {
	return daemonStartWithDeps(stdout, opts, defaultDaemonDeps)
}

func DaemonStop(stdout io.Writer, opts DaemonStopOptions) error {
	return daemonStopWithDeps(stdout, opts, defaultDaemonDeps)
}

func DaemonStatus(stdout io.Writer, opts DaemonStatusOptions) error {
	return daemonStatusWithDeps(stdout, opts, defaultDaemonDeps)
}

func DaemonLogs(stdout io.Writer, opts DaemonLogsOptions) error {
	if opts.Lines <= 0 {
		opts.Lines = 80
	}
	lines, err := tailFileLines(daemonLogPath(), opts.Lines)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.JSON {
				data, _ := json.MarshalIndent(daemonLogsOutput{Path: daemonLogPath(), Lines: []string{}}, "", "  ")
				fmt.Fprintln(stdout, string(data))
				return nil
			}
			fmt.Fprintf(stdout, "Daemon log not found: %s\n", daemonLogPath())
			return nil
		}
		return err
	}
	appendDaemonEvent(DaemonEventLogRead, map[string]any{"path": daemonLogPath(), "lines": opts.Lines})
	if opts.JSON {
		data, _ := json.MarshalIndent(daemonLogsOutput{Path: daemonLogPath(), Lines: lines}, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Daemon log: %s\n", daemonLogPath())
	for _, line := range lines {
		fmt.Fprintln(stdout, line)
	}
	return nil
}

func daemonStartWithDeps(stdout io.Writer, opts DaemonStartOptions, deps daemonDeps) error {
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}
	if opts.MaxJobs < 0 {
		return fmt.Errorf("--max-jobs must be zero or positive")
	}
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		return fmt.Errorf("create daemon dir: %w", err)
	}

	if pid, ok := readDaemonPID(); ok {
		if deps.processAlive(pid) {
			appendDaemonEvent(DaemonEventAlreadyRunning, map[string]any{"pid": pid})
			return fmt.Errorf("daemon already running with pid %d", pid)
		}
		appendDaemonEvent(DaemonEventStalePID, map[string]any{"pid": pid})
		if !opts.Force {
			return fmt.Errorf("stale daemon pid found; rerun with --force to clear")
		}
		_ = removeDaemonPID()
	}

	exe, err := deps.executable()
	if err != nil || strings.TrimSpace(exe) == "" {
		exe = "byom-video"
	}
	args := []string{
		exe,
		"job-worker",
		"--loop",
		"--interval", opts.Interval.String(),
	}
	if opts.MaxJobs > 0 {
		args = append(args, "--max-jobs", strconv.Itoa(opts.MaxJobs))
	}
	if opts.AllowProviderCalls {
		args = append(args, "--allow-provider-calls")
	}
	if opts.AllowOverwrite {
		args = append(args, "--allow-overwrite")
	}
	if opts.FailFast {
		args = append(args, "--fail-fast")
	}
	if opts.Force {
		args = append(args, "--force-lock")
	}

	now := deps.now()
	state := DaemonState{
		SchemaVersion:         daemonStateSchema,
		DaemonID:              now.Format("20060102T150405Z") + "-daemon",
		Status:                "starting",
		StartedAt:             now,
		UpdatedAt:             now,
		WorkerMode:            "loop",
		WorkerIntervalSeconds: int(opts.Interval / time.Second),
		WorkerMaxJobs:         opts.MaxJobs,
		AllowProviderCalls:    opts.AllowProviderCalls,
		AllowOverwrite:        opts.AllowOverwrite,
		FailFast:              opts.FailFast,
	}
	_ = writeDaemonState(&state)

	pid, err := deps.startProcess(args, opts.ResetLog)
	if err != nil {
		state.Status = "failed"
		state.LastError = err.Error()
		state.Errors = append(state.Errors, err.Error())
		_ = writeDaemonState(&state)
		appendDaemonEvent(DaemonEventStartFailed, map[string]any{"error": err.Error()})
		return err
	}
	state.Status = "running"
	state.PID = pid
	state.UpdatedAt = deps.now()
	if err := writeDaemonPID(pid); err != nil {
		return err
	}
	if err := writeDaemonState(&state); err != nil {
		return err
	}
	appendDaemonEvent(DaemonEventStarted, map[string]any{"daemon_id": state.DaemonID, "pid": pid})

	if opts.JSON {
		data, _ := json.MarshalIndent(state, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Daemon started")
	fmt.Fprintf(stdout, "  daemon_id: %s\n", state.DaemonID)
	fmt.Fprintf(stdout, "  pid:       %d\n", state.PID)
	fmt.Fprintf(stdout, "  log:       %s\n", daemonLogPath())
	fmt.Fprintf(stdout, "  status:    %s\n", state.Status)
	return nil
}

func daemonStopWithDeps(stdout io.Writer, opts DaemonStopOptions, deps daemonDeps) error {
	state, _ := readDaemonState()
	pid, ok := readDaemonPID()
	if !ok || pid <= 0 {
		state.Status = "stopped"
		state.StoppedAt = deps.now()
		state.UpdatedAt = deps.now()
		_ = writeDaemonState(&state)
		appendDaemonEvent(DaemonEventStopped, map[string]any{"reason": "missing_pid"})
		if opts.JSON {
			data, _ := json.MarshalIndent(state, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Daemon already stopped")
		return nil
	}
	if !deps.processAlive(pid) {
		state.Status = "stopped"
		state.PID = pid
		state.StoppedAt = deps.now()
		state.UpdatedAt = deps.now()
		_ = removeDaemonPID()
		_ = writeDaemonState(&state)
		appendDaemonEvent(DaemonEventStalePID, map[string]any{"pid": pid})
		if opts.JSON {
			data, _ := json.MarshalIndent(state, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Daemon not running; cleared stale pid %d\n", pid)
		return nil
	}
	if err := deps.stopProcess(pid, opts.Force); err != nil {
		state.Status = "failed"
		state.LastError = err.Error()
		state.Errors = append(state.Errors, err.Error())
		state.UpdatedAt = deps.now()
		_ = writeDaemonState(&state)
		appendDaemonEvent(DaemonEventStopFailed, map[string]any{"pid": pid, "error": err.Error()})
		return err
	}
	_ = removeDaemonPID()
	state.Status = "stopped"
	state.PID = pid
	state.StoppedAt = deps.now()
	state.UpdatedAt = deps.now()
	if err := writeDaemonState(&state); err != nil {
		return err
	}
	appendDaemonEvent(DaemonEventStopped, map[string]any{"pid": pid})
	if opts.JSON {
		data, _ := json.MarshalIndent(state, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Daemon stopped")
	fmt.Fprintf(stdout, "  pid:    %d\n", pid)
	fmt.Fprintf(stdout, "  status: %s\n", state.Status)
	return nil
}

func daemonStatusWithDeps(stdout io.Writer, opts DaemonStatusOptions, deps daemonDeps) error {
	state, _ := readDaemonState()
	pid, ok := readDaemonPID()
	pidAlive := ok && pid > 0 && deps.processAlive(pid)
	warnings := []string{}
	if ok && pid > 0 && !pidAlive {
		warnings = append(warnings, fmt.Sprintf("pid %d is not running", pid))
		if state.Status == "running" {
			state.Status = "unknown"
		}
	}
	var workerState *WorkerState
	if ws, err := deps.readWorker(); err == nil && ws.SchemaVersion != "" {
		workerState = &ws
	}
	queueSummary, _ := summarizeJobQueue(deps.readJob)
	appendDaemonEvent(DaemonEventStatusChecked, map[string]any{"pid_alive": pidAlive})
	out := daemonStatusOutput{
		State:           state,
		PIDAlive:        pidAlive,
		LogPath:         daemonLogPath(),
		WorkerState:     workerState,
		JobQueueSummary: queueSummary,
		Warnings:        warnings,
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Daemon status")
	fmt.Fprintf(stdout, "  status:                 %s\n", state.Status)
	fmt.Fprintf(stdout, "  pid:                    %d\n", pid)
	fmt.Fprintf(stdout, "  pid_alive:              %t\n", pidAlive)
	fmt.Fprintf(stdout, "  started_at:             %s\n", formatTimeOrDash(state.StartedAt))
	fmt.Fprintf(stdout, "  worker_interval_seconds:%d\n", state.WorkerIntervalSeconds)
	fmt.Fprintf(stdout, "  worker_max_jobs:        %d\n", state.WorkerMaxJobs)
	fmt.Fprintf(stdout, "  allow_provider_calls:   %t\n", state.AllowProviderCalls)
	fmt.Fprintf(stdout, "  allow_overwrite:        %t\n", state.AllowOverwrite)
	fmt.Fprintf(stdout, "  log_path:               %s\n", daemonLogPath())
	if workerState != nil {
		fmt.Fprintf(stdout, "  worker_status:          %s\n", workerState.Status)
		fmt.Fprintf(stdout, "  worker_last_job_id:     %s\n", emptyDash(workerState.LastJobID))
	}
	if len(queueSummary) > 0 {
		fmt.Fprintln(stdout, "  job_queue:")
		keys := make([]string, 0, len(queueSummary))
		for key := range queueSummary {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(stdout, "    %s: %d\n", key, queueSummary[key])
		}
	}
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "  warning:                %s\n", warning)
	}
	return nil
}

func daemonLogPath() string    { return filepath.Join(daemonRoot, daemonLogFileName) }
func daemonStatePath() string  { return filepath.Join(daemonRoot, daemonStateFileName) }
func daemonPIDPath() string    { return filepath.Join(daemonRoot, daemonPIDFileName) }
func daemonEventsPath() string { return filepath.Join(daemonRoot, daemonEventsFileName) }

func readDaemonState() (DaemonState, error) {
	data, err := os.ReadFile(daemonStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return DaemonState{SchemaVersion: daemonStateSchema, Status: "stopped"}, nil
		}
		return DaemonState{}, err
	}
	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return DaemonState{}, err
	}
	if state.SchemaVersion == "" {
		state.SchemaVersion = daemonStateSchema
	}
	if state.Status == "" {
		state.Status = "unknown"
	}
	return state, nil
}

func writeDaemonState(state *DaemonState) error {
	state.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		return err
	}
	return writeJSONFile(daemonStatePath(), state)
}

func appendDaemonEvent(eventType string, details map[string]any) {
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		return
	}
	log, err := events.Open(daemonEventsPath())
	if err != nil {
		return
	}
	defer log.Close()
	_ = log.Write(eventType, details)
}

func readDaemonPID() (int, bool) {
	data, err := os.ReadFile(daemonPIDPath())
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func writeDaemonPID(pid int) error {
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		return err
	}
	return os.WriteFile(daemonPIDPath(), []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

func removeDaemonPID() error {
	if err := os.Remove(daemonPIDPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return process.Signal(syscall.Signal(0)) == nil
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func startDaemonProcess(args []string, resetLog bool) (int, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("missing daemon executable")
	}
	if err := os.MkdirAll(daemonRoot, 0o755); err != nil {
		return 0, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if resetLog {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	logFile, err := os.OpenFile(daemonLogPath(), flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open daemon log: %w", err)
	}
	defer logFile.Close()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start daemon process: %w", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("release daemon process: %w", err)
	}
	return pid, nil
}

func stopDaemonProcess(pid int, force bool) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	sig := os.Interrupt
	if runtime.GOOS == "windows" {
		if force {
			sig = os.Kill
		}
	} else {
		sig = syscall.SIGTERM
		if force {
			sig = os.Kill
		}
	}
	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("stop daemon process: %w", err)
	}
	for i := 0; i < 20; i++ {
		if !processExists(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if force {
		return fmt.Errorf("daemon process %d did not stop after force signal", pid)
	}
	return fmt.Errorf("daemon process %d did not stop cleanly", pid)
}

func tailFileLines(path string, lines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	all := []string{}
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}
	if lines > 0 && len(all) > lines {
		all = all[len(all)-lines:]
	}
	return all, nil
}

func summarizeJobQueue(readJobFn func(string) (*Job, error)) (map[string]int, error) {
	summary := map[string]int{
		"pending_approved": 0,
		"pending_approval": 0,
		"running":          0,
		"failed":           0,
	}
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return summary, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		job, err := readJobFn(entry.Name())
		if err != nil {
			continue
		}
		switch {
		case job.Status == JobStatusPending && (job.ApprovalStatus == JobApprovalApproved || job.ApprovalStatus == JobApprovalNotRequired):
			summary["pending_approved"]++
		case job.Status == JobStatusPending && job.ApprovalStatus == JobApprovalPending:
			summary["pending_approval"]++
		case job.Status == JobStatusRunning:
			summary["running"]++
		case job.Status == JobStatusFailed:
			summary["failed"]++
		}
	}
	return summary, nil
}

func formatTimeOrDash(ts time.Time) string {
	if ts.IsZero() {
		return "-"
	}
	return ts.Format(time.RFC3339)
}

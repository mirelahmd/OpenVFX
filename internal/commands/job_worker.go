package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
)

const (
	workerRoot              = ".byom-video/worker"
	workerStateSchema       = "openvfx_worker.v1"
	workerStateFileName     = "worker_state.json"
	workerEventsFileName    = "worker_events.jsonl"
	workerLockFileName      = "worker.lock"
	WorkerEventStarted      = "WORKER_STARTED"
	WorkerEventStopped      = "WORKER_STOPPED"
	WorkerEventFailed       = "WORKER_FAILED"
	WorkerEventScanStarted  = "WORKER_SCAN_STARTED"
	WorkerEventScanComplete = "WORKER_SCAN_COMPLETED"
	WorkerEventJobSelected  = "WORKER_JOB_SELECTED"
	WorkerEventJobSkipped   = "WORKER_JOB_SKIPPED"
	WorkerEventJobStarted   = "WORKER_JOB_STARTED"
	WorkerEventJobComplete  = "WORKER_JOB_COMPLETED"
	WorkerEventJobFailed    = "WORKER_JOB_FAILED"
	WorkerEventLockAcquire  = "WORKER_LOCK_ACQUIRED"
	WorkerEventLockRelease  = "WORKER_LOCK_RELEASED"
	WorkerEventLockBusy     = "WORKER_LOCK_BUSY"
)

type WorkerState struct {
	SchemaVersion   string    `json:"schema_version"`
	WorkerID        string    `json:"worker_id"`
	Status          string    `json:"status"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastScanAt      time.Time `json:"last_scan_at,omitempty"`
	LastJobID       string    `json:"last_job_id,omitempty"`
	JobsSeen        int       `json:"jobs_seen"`
	JobsRun         int       `json:"jobs_run"`
	JobsSucceeded   int       `json:"jobs_succeeded"`
	JobsFailed      int       `json:"jobs_failed"`
	Mode            string    `json:"mode"`
	IntervalSeconds int       `json:"interval_seconds"`
	MaxJobs         int       `json:"max_jobs"`
	Warnings        []string  `json:"warnings,omitempty"`
	Errors          []string  `json:"errors,omitempty"`
}

type WorkerLock struct {
	WorkerID  string    `json:"worker_id"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkerStatusOutput struct {
	State       WorkerState `json:"state"`
	LockPresent bool        `json:"lock_present"`
	Lock        *WorkerLock `json:"lock,omitempty"`
	LockError   string      `json:"lock_error,omitempty"`
}

type JobWorkerOptions struct {
	Once               bool
	Loop               bool
	Status             bool
	JSON               bool
	DryRun             bool
	AllowProviderCalls bool
	AllowOverwrite     bool
	FailFast           bool
	ForceLock          bool
	Interval           time.Duration
	MaxJobs            int
}

type jobWorkerSummary struct {
	WorkerID      string   `json:"worker_id"`
	Mode          string   `json:"mode"`
	EligibleJobs  []string `json:"eligible_jobs,omitempty"`
	SelectedJobs  []string `json:"selected_jobs,omitempty"`
	JobsSeen      int      `json:"jobs_seen"`
	JobsRun       int      `json:"jobs_run"`
	JobsSucceeded int      `json:"jobs_succeeded"`
	JobsFailed    int      `json:"jobs_failed"`
	Status        string   `json:"status"`
	Warnings      []string `json:"warnings,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type workerJobCandidate struct {
	JobID          string
	Status         string
	ApprovalStatus string
	CreatedAt      time.Time
}

type jobWorkerDeps struct {
	now     func() time.Time
	sleep   func(time.Duration)
	pid     func() int
	runJob  func(string, io.Writer, JobRunOptions) error
	readJob func(string) (*Job, error)
}

var defaultJobWorkerDeps = jobWorkerDeps{
	now:     func() time.Time { return time.Now().UTC() },
	sleep:   time.Sleep,
	pid:     os.Getpid,
	runJob:  JobRun,
	readJob: func(jobID string) (*Job, error) { return readJob(jobID) },
}

func JobWorker(stdout io.Writer, opts JobWorkerOptions) error {
	return jobWorkerWithDeps(stdout, opts, defaultJobWorkerDeps)
}

func JobWorkerStatus(stdout io.Writer, opts JobWorkerOptions) error {
	state, lockPresent, lock, lockErr := readWorkerStatus()
	if opts.JSON {
		data, err := json.MarshalIndent(WorkerStatusOutput{
			State:       state,
			LockPresent: lockPresent,
			Lock:        lock,
			LockError:   lockErr,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Worker status")
	fmt.Fprintf(stdout, "  status:          %s\n", state.Status)
	fmt.Fprintf(stdout, "  worker_id:       %s\n", emptyDash(state.WorkerID))
	if !state.UpdatedAt.IsZero() {
		fmt.Fprintf(stdout, "  updated_at:      %s\n", state.UpdatedAt.Format(time.RFC3339))
	}
	if !state.LastScanAt.IsZero() {
		fmt.Fprintf(stdout, "  last_scan_at:    %s\n", state.LastScanAt.Format(time.RFC3339))
	}
	fmt.Fprintf(stdout, "  last_job_id:     %s\n", emptyDash(state.LastJobID))
	fmt.Fprintf(stdout, "  jobs_seen:       %d\n", state.JobsSeen)
	fmt.Fprintf(stdout, "  jobs_run:        %d\n", state.JobsRun)
	fmt.Fprintf(stdout, "  jobs_succeeded:  %d\n", state.JobsSucceeded)
	fmt.Fprintf(stdout, "  jobs_failed:     %d\n", state.JobsFailed)
	fmt.Fprintf(stdout, "  lock_present:    %t\n", lockPresent)
	if lockPresent && lock != nil {
		fmt.Fprintf(stdout, "  lock_worker_id:  %s\n", lock.WorkerID)
		fmt.Fprintf(stdout, "  lock_pid:        %d\n", lock.PID)
		if !lock.CreatedAt.IsZero() {
			fmt.Fprintf(stdout, "  lock_created_at: %s\n", lock.CreatedAt.Format(time.RFC3339))
		}
	}
	if lockErr != "" {
		fmt.Fprintf(stdout, "  lock_error:      %s\n", lockErr)
	}
	return nil
}

func jobWorkerWithDeps(stdout io.Writer, opts JobWorkerOptions, deps jobWorkerDeps) error {
	if opts.Status {
		return JobWorkerStatus(stdout, opts)
	}
	if !opts.Once && !opts.Loop {
		return fmt.Errorf("job-worker requires --once, --loop, or --status")
	}
	mode := "once"
	if opts.Loop {
		mode = "loop"
	}
	maxJobs := opts.MaxJobs
	if maxJobs == 0 && !opts.Loop {
		maxJobs = 1
	}
	if maxJobs < 0 {
		return fmt.Errorf("--max-jobs must be zero or positive")
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	workerID := deps.now().Format("20060102T150405Z") + "-worker"
	state := WorkerState{
		SchemaVersion:   workerStateSchema,
		WorkerID:        workerID,
		Status:          "idle",
		StartedAt:       deps.now(),
		UpdatedAt:       deps.now(),
		Mode:            mode,
		IntervalSeconds: int(interval / time.Second),
		MaxJobs:         maxJobs,
	}
	if opts.DryRun {
		state.Warnings = append(state.Warnings, "dry-run mode: no jobs will be executed")
	}

	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		return fmt.Errorf("create worker dir: %w", err)
	}

	if err := acquireWorkerLock(workerID, opts.ForceLock, deps); err != nil {
		appendWorkerEvent(WorkerEventLockBusy, map[string]any{"worker_id": workerID, "error": err.Error()})
		return err
	}
	defer releaseWorkerLock(workerID)
	appendWorkerEvent(WorkerEventLockAcquire, map[string]any{"worker_id": workerID})

	state.Status = "running"
	writeWorkerState(&state)
	appendWorkerEvent(WorkerEventStarted, map[string]any{
		"worker_id":        workerID,
		"mode":             mode,
		"interval_seconds": state.IntervalSeconds,
		"max_jobs":         state.MaxJobs,
		"dry_run":          opts.DryRun,
	})

	summary := jobWorkerSummary{
		WorkerID: workerID,
		Mode:     mode,
		Status:   state.Status,
	}

	for {
		state.LastScanAt = deps.now()
		state.UpdatedAt = deps.now()
		writeWorkerState(&state)
		appendWorkerEvent(WorkerEventScanStarted, map[string]any{"worker_id": workerID})

		candidates, err := eligibleJobs(deps.readJob)
		if err != nil {
			state.Status = "failed"
			state.Errors = append(state.Errors, err.Error())
			writeWorkerState(&state)
			appendWorkerEvent(WorkerEventFailed, map[string]any{"worker_id": workerID, "error": err.Error()})
			return err
		}
		state.JobsSeen += len(candidates)
		for _, candidate := range candidates {
			summary.EligibleJobs = append(summary.EligibleJobs, candidate.JobID)
		}
		appendWorkerEvent(WorkerEventScanComplete, map[string]any{
			"worker_id": workerID,
			"eligible":  len(candidates),
		})

		if opts.DryRun {
			state.Status = "stopped"
			state.UpdatedAt = deps.now()
			writeWorkerState(&state)
			appendWorkerEvent(WorkerEventStopped, map[string]any{"worker_id": workerID, "dry_run": true})
			summary.Status = state.Status
			summary.JobsSeen = state.JobsSeen
			printWorkerSummary(stdout, summary, opts.JSON)
			return nil
		}

		runCountThisScan := 0
		for _, candidate := range candidates {
			current, err := deps.readJob(candidate.JobID)
			if err != nil {
				appendWorkerEvent(WorkerEventJobSkipped, map[string]any{"worker_id": workerID, "job_id": candidate.JobID, "reason": err.Error()})
				continue
			}
			if !isWorkerEligible(current) {
				appendWorkerEvent(WorkerEventJobSkipped, map[string]any{"worker_id": workerID, "job_id": candidate.JobID, "reason": "job no longer eligible"})
				continue
			}
			state.LastJobID = current.JobID
			state.UpdatedAt = deps.now()
			writeWorkerState(&state)
			appendWorkerEvent(WorkerEventJobSelected, map[string]any{"worker_id": workerID, "job_id": current.JobID})
			appendWorkerEvent(WorkerEventJobStarted, map[string]any{"worker_id": workerID, "job_id": current.JobID})
			summary.SelectedJobs = append(summary.SelectedJobs, current.JobID)

			err = deps.runJob(current.JobID, io.Discard, JobRunOptions{
				AllowProviderCalls: opts.AllowProviderCalls,
				AllowOverwrite:     opts.AllowOverwrite,
			})
			state.JobsRun++
			runCountThisScan++
			if err != nil {
				state.JobsFailed++
				state.Errors = append(state.Errors, fmt.Sprintf("%s: %s", current.JobID, err.Error()))
				appendWorkerEvent(WorkerEventJobFailed, map[string]any{"worker_id": workerID, "job_id": current.JobID, "error": err.Error()})
				if opts.FailFast {
					state.Status = "failed"
					state.UpdatedAt = deps.now()
					writeWorkerState(&state)
					appendWorkerEvent(WorkerEventFailed, map[string]any{"worker_id": workerID, "job_id": current.JobID, "error": err.Error()})
					summary.Status = state.Status
					summary.Errors = append(summary.Errors, err.Error())
					summary.JobsSeen = state.JobsSeen
					summary.JobsRun = state.JobsRun
					summary.JobsSucceeded = state.JobsSucceeded
					summary.JobsFailed = state.JobsFailed
					printWorkerSummary(stdout, summary, opts.JSON)
					return err
				}
				summary.Warnings = append(summary.Warnings, fmt.Sprintf("job %s failed: %s", current.JobID, err.Error()))
			} else {
				state.JobsSucceeded++
				appendWorkerEvent(WorkerEventJobComplete, map[string]any{"worker_id": workerID, "job_id": current.JobID})
			}
			state.UpdatedAt = deps.now()
			writeWorkerState(&state)
			if maxJobs > 0 && state.JobsRun >= maxJobs {
				state.Status = "stopped"
				state.UpdatedAt = deps.now()
				writeWorkerState(&state)
				appendWorkerEvent(WorkerEventStopped, map[string]any{"worker_id": workerID, "reason": "max_jobs_reached"})
				summary.Status = state.Status
				summary.JobsSeen = state.JobsSeen
				summary.JobsRun = state.JobsRun
				summary.JobsSucceeded = state.JobsSucceeded
				summary.JobsFailed = state.JobsFailed
				summary.Errors = append(summary.Errors, state.Errors...)
				printWorkerSummary(stdout, summary, opts.JSON)
				return nil
			}
			if opts.Once {
				state.Status = "stopped"
				state.UpdatedAt = deps.now()
				writeWorkerState(&state)
				appendWorkerEvent(WorkerEventStopped, map[string]any{"worker_id": workerID, "reason": "once_completed"})
				summary.Status = state.Status
				summary.JobsSeen = state.JobsSeen
				summary.JobsRun = state.JobsRun
				summary.JobsSucceeded = state.JobsSucceeded
				summary.JobsFailed = state.JobsFailed
				summary.Errors = append(summary.Errors, state.Errors...)
				printWorkerSummary(stdout, summary, opts.JSON)
				return nil
			}
		}

		if !opts.Loop {
			state.Status = "stopped"
			state.UpdatedAt = deps.now()
			writeWorkerState(&state)
			appendWorkerEvent(WorkerEventStopped, map[string]any{"worker_id": workerID, "reason": "scan_completed"})
			summary.Status = state.Status
			summary.JobsSeen = state.JobsSeen
			summary.JobsRun = state.JobsRun
			summary.JobsSucceeded = state.JobsSucceeded
			summary.JobsFailed = state.JobsFailed
			summary.Errors = append(summary.Errors, state.Errors...)
			printWorkerSummary(stdout, summary, opts.JSON)
			return nil
		}
		if maxJobs > 0 && state.JobsRun >= maxJobs {
			state.Status = "stopped"
			state.UpdatedAt = deps.now()
			writeWorkerState(&state)
			appendWorkerEvent(WorkerEventStopped, map[string]any{"worker_id": workerID, "reason": "max_jobs_reached"})
			summary.Status = state.Status
			summary.JobsSeen = state.JobsSeen
			summary.JobsRun = state.JobsRun
			summary.JobsSucceeded = state.JobsSucceeded
			summary.JobsFailed = state.JobsFailed
			summary.Errors = append(summary.Errors, state.Errors...)
			printWorkerSummary(stdout, summary, opts.JSON)
			return nil
		}
		if runCountThisScan == 0 {
			deps.sleep(interval)
		}
	}
}

func eligibleJobs(readJobFn func(string) (*Job, error)) ([]workerJobCandidate, error) {
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read jobs dir: %w", err)
	}
	candidates := []workerJobCandidate{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		job, err := readJobFn(entry.Name())
		if err != nil {
			continue
		}
		if !isWorkerEligible(job) {
			continue
		}
		candidates = append(candidates, workerJobCandidate{
			JobID:          job.JobID,
			Status:         job.Status,
			ApprovalStatus: job.ApprovalStatus,
			CreatedAt:      job.CreatedAt,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) || candidates[i].CreatedAt.IsZero() || candidates[j].CreatedAt.IsZero() {
			return candidates[i].JobID < candidates[j].JobID
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	return candidates, nil
}

func isWorkerEligible(job *Job) bool {
	if job == nil {
		return false
	}
	if job.Status != JobStatusPending {
		return false
	}
	return job.ApprovalStatus == JobApprovalApproved || job.ApprovalStatus == JobApprovalNotRequired
}

func workerStatePath() string  { return filepath.Join(workerRoot, workerStateFileName) }
func workerEventsPath() string { return filepath.Join(workerRoot, workerEventsFileName) }
func workerLockPath() string   { return filepath.Join(workerRoot, workerLockFileName) }

func writeWorkerState(state *WorkerState) error {
	state.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		return err
	}
	return writeJSONFile(workerStatePath(), state)
}

func readWorkerState() (WorkerState, error) {
	data, err := os.ReadFile(workerStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return WorkerState{
				SchemaVersion: workerStateSchema,
				Status:        "idle",
			}, nil
		}
		return WorkerState{}, err
	}
	var state WorkerState
	if err := json.Unmarshal(data, &state); err != nil {
		return WorkerState{}, err
	}
	if state.SchemaVersion == "" {
		state.SchemaVersion = workerStateSchema
	}
	if state.Status == "" {
		state.Status = "idle"
	}
	return state, nil
}

func appendWorkerEvent(eventType string, details map[string]any) {
	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		return
	}
	log, err := events.Open(workerEventsPath())
	if err != nil {
		return
	}
	defer log.Close()
	_ = log.Write(eventType, details)
}

func readWorkerEvents() ([]events.Event, error) {
	data, err := os.ReadFile(workerEventsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []events.Event
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event events.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		out = append(out, event)
	}
	return out, nil
}

func acquireWorkerLock(workerID string, force bool, deps jobWorkerDeps) error {
	if err := os.MkdirAll(workerRoot, 0o755); err != nil {
		return err
	}
	if force {
		_ = os.Remove(workerLockPath())
	}
	lock := WorkerLock{
		WorkerID:  workerID,
		PID:       deps.pid(),
		CreatedAt: deps.now(),
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(workerLockPath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("worker lock already exists; another worker may be running")
		}
		return fmt.Errorf("create worker lock: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write worker lock: %w", err)
	}
	return nil
}

func releaseWorkerLock(workerID string) {
	_ = os.Remove(workerLockPath())
	appendWorkerEvent(WorkerEventLockRelease, map[string]any{"worker_id": workerID})
}

func readWorkerStatus() (WorkerState, bool, *WorkerLock, string) {
	state, err := readWorkerState()
	if err != nil {
		return WorkerState{
			SchemaVersion: workerStateSchema,
			Status:        "failed",
			Errors:        []string{err.Error()},
		}, false, nil, err.Error()
	}
	lockData, err := os.ReadFile(workerLockPath())
	if err != nil {
		if os.IsNotExist(err) {
			return state, false, nil, ""
		}
		return state, false, nil, err.Error()
	}
	var lock WorkerLock
	if err := json.Unmarshal(lockData, &lock); err != nil {
		return state, true, nil, err.Error()
	}
	return state, true, &lock, ""
}

func printWorkerSummary(stdout io.Writer, summary jobWorkerSummary, asJSON bool) {
	if asJSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return
	}
	fmt.Fprintln(stdout, "Job worker")
	fmt.Fprintf(stdout, "  worker_id:      %s\n", summary.WorkerID)
	fmt.Fprintf(stdout, "  mode:           %s\n", summary.Mode)
	fmt.Fprintf(stdout, "  status:         %s\n", summary.Status)
	fmt.Fprintf(stdout, "  jobs_seen:      %d\n", summary.JobsSeen)
	fmt.Fprintf(stdout, "  jobs_run:       %d\n", summary.JobsRun)
	fmt.Fprintf(stdout, "  jobs_succeeded: %d\n", summary.JobsSucceeded)
	fmt.Fprintf(stdout, "  jobs_failed:    %d\n", summary.JobsFailed)
	if len(summary.EligibleJobs) > 0 {
		fmt.Fprintf(stdout, "  eligible_jobs:  %s\n", strings.Join(summary.EligibleJobs, ", "))
	}
	if len(summary.SelectedJobs) > 0 {
		fmt.Fprintf(stdout, "  selected_jobs:  %s\n", strings.Join(summary.SelectedJobs, ", "))
	}
	for _, warning := range summary.Warnings {
		fmt.Fprintf(stdout, "  warning:        %s\n", warning)
	}
	for _, item := range summary.Errors {
		fmt.Fprintf(stdout, "  error:          %s\n", item)
	}
}

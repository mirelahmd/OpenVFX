package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	queueRoot          = ".byom-video/queue"
	queueSummarySchema = "openvfx_queue_summary.v1"
)

type QueueOptions struct {
	JSON           bool
	Limit          int
	FailedOnly     bool
	ApprovalNeeded bool
	RunningOnly    bool
}

type QueueHealthOptions struct {
	JSON        bool
	Strict      bool
	StaleAfter  time.Duration
	WriteReport bool
}

type QueueHealthCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type QueueRuntimeStatus struct {
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	PIDAlive  bool   `json:"pid_alive"`
	StatePath string `json:"state_path"`
}

type QueueWorkerStatus struct {
	Status      string `json:"status"`
	LockPresent bool   `json:"lock_present"`
	StatePath   string `json:"state_path"`
}

type QueueJobView struct {
	JobID             string    `json:"job_id"`
	Status            string    `json:"status"`
	Approval          string    `json:"approval"`
	ActionType        string    `json:"action_type"`
	SourceAgentPlanID string    `json:"source_agent_plan_id,omitempty"`
	Preview           string    `json:"preview"`
	UpdatedAt         time.Time `json:"updated_at"`
	CreatedAt         time.Time `json:"created_at"`
	Warnings          []string  `json:"warnings,omitempty"`
}

type QueueJobsSummary struct {
	Total          int            `json:"total"`
	ByStatus       map[string]int `json:"by_status"`
	ByApproval     map[string]int `json:"by_approval"`
	ApprovalNeeded []QueueJobView `json:"approval_needed,omitempty"`
	Failed         []QueueJobView `json:"failed,omitempty"`
	Running        []QueueJobView `json:"running,omitempty"`
	StaleRunning   []QueueJobView `json:"stale_running,omitempty"`
	Recent         []QueueJobView `json:"recent,omitempty"`
}

type QueueSummary struct {
	SchemaVersion string             `json:"schema_version"`
	CreatedAt     time.Time          `json:"created_at"`
	Status        string             `json:"status"`
	Daemon        QueueRuntimeStatus `json:"daemon"`
	Worker        QueueWorkerStatus  `json:"worker"`
	Jobs          QueueJobsSummary   `json:"jobs"`
	HealthChecks  []QueueHealthCheck `json:"health_checks,omitempty"`
	NextCommands  []string           `json:"next_commands,omitempty"`
}

type queueDeps struct {
	now          func() time.Time
	processAlive func(int) bool
	readDaemon   func() (DaemonState, error)
	readWorker   func() (WorkerState, error)
	readJob      func(string) (*Job, error)
}

var defaultQueueDeps = queueDeps{
	now:          func() time.Time { return time.Now().UTC() },
	processAlive: processExists,
	readDaemon:   readDaemonState,
	readWorker:   readWorkerState,
	readJob: func(jobID string) (*Job, error) {
		return readJob(jobID)
	},
}

func Queue(stdout io.Writer, opts QueueOptions) error {
	return queueWithDeps(stdout, opts, defaultQueueDeps)
}

func QueueHealth(stdout io.Writer, opts QueueHealthOptions) error {
	return queueHealthWithDeps(stdout, opts, defaultQueueDeps)
}

func queueWithDeps(stdout io.Writer, opts QueueOptions, deps queueDeps) error {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}
	summary, err := buildQueueSummary(opts.Limit, 30*time.Minute, deps)
	if err != nil {
		return err
	}
	summary.Status = deriveQueueHealthStatus(summary, false)
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Queue")
	fmt.Fprintln(stdout, "  Runtime:")
	fmt.Fprintf(stdout, "    daemon:       %s\n", summary.Daemon.Status)
	fmt.Fprintf(stdout, "    daemon pid:   %s\n", pidStatusText(summary.Daemon.PID, summary.Daemon.PIDAlive))
	fmt.Fprintf(stdout, "    worker:       %s\n", summary.Worker.Status)
	lockStatus := "missing"
	if summary.Worker.LockPresent {
		lockStatus = "present"
	}
	fmt.Fprintf(stdout, "    worker lock:  %s\n", lockStatus)

	fmt.Fprintln(stdout, "  Queue:")
	fmt.Fprintf(stdout, "    total:             %d\n", summary.Jobs.Total)
	for _, key := range []string{JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed, JobStatusCancelled} {
		fmt.Fprintf(stdout, "    %-17s %d\n", key+":", summary.Jobs.ByStatus[key])
	}
	for _, key := range []string{JobApprovalPending, JobApprovalApproved, JobApprovalRejected, JobApprovalNotRequired} {
		fmt.Fprintf(stdout, "    %-17s %d\n", key+":", summary.Jobs.ByApproval[key])
	}

	renderedAttention := false
	if !opts.FailedOnly && !opts.RunningOnly {
		if len(summary.Jobs.ApprovalNeeded) > 0 || len(summary.Jobs.StaleRunning) > 0 || len(summary.HealthChecks) > 0 {
			fmt.Fprintln(stdout, "  Needs attention:")
			renderedAttention = true
		}
		if len(summary.Jobs.ApprovalNeeded) > 0 {
			fmt.Fprintln(stdout, "    approval-needed:")
			for _, item := range summary.Jobs.ApprovalNeeded {
				fmt.Fprintf(stdout, "      - %s %s %s\n", item.JobID, item.ActionType, item.Preview)
			}
		}
	}
	if len(summary.Jobs.Failed) > 0 && !opts.ApprovalNeeded && !opts.RunningOnly {
		if !renderedAttention {
			fmt.Fprintln(stdout, "  Needs attention:")
			renderedAttention = true
		}
		fmt.Fprintln(stdout, "    failed:")
		for _, item := range summary.Jobs.Failed {
			fmt.Fprintf(stdout, "      - %s %s %s\n", item.JobID, item.ActionType, item.Preview)
		}
	}
	if len(summary.Jobs.Running) > 0 && !opts.ApprovalNeeded && !opts.FailedOnly {
		if !renderedAttention {
			fmt.Fprintln(stdout, "  Needs attention:")
			renderedAttention = true
		}
		fmt.Fprintln(stdout, "    running:")
		for _, item := range summary.Jobs.Running {
			fmt.Fprintf(stdout, "      - %s %s %s\n", item.JobID, item.ActionType, item.Preview)
		}
	}
	if len(summary.Jobs.StaleRunning) > 0 && !opts.FailedOnly && !opts.ApprovalNeeded {
		if !renderedAttention {
			fmt.Fprintln(stdout, "  Needs attention:")
			renderedAttention = true
		}
		fmt.Fprintln(stdout, "    stale running:")
		for _, item := range summary.Jobs.StaleRunning {
			fmt.Fprintf(stdout, "      - %s %s %s\n", item.JobID, item.ActionType, item.Preview)
		}
	}
	for _, check := range summary.HealthChecks {
		if check.Status != "ok" {
			if !renderedAttention {
				fmt.Fprintln(stdout, "  Needs attention:")
				renderedAttention = true
			}
			fmt.Fprintf(stdout, "    - [%s] %s\n", check.Status, check.Message)
		}
	}

	if !opts.FailedOnly && !opts.ApprovalNeeded && !opts.RunningOnly {
		fmt.Fprintln(stdout, "  Recent jobs:")
		for _, item := range summary.Jobs.Recent {
			fmt.Fprintf(stdout, "    - %s  %s  %s  %s  %s\n",
				item.JobID,
				item.Status,
				item.Approval,
				item.ActionType,
				item.Preview,
			)
		}
	}

	if len(summary.NextCommands) > 0 {
		fmt.Fprintln(stdout, "  Next commands:")
		for _, command := range summary.NextCommands {
			fmt.Fprintf(stdout, "    - %s\n", command)
		}
	}
	return nil
}

func queueHealthWithDeps(stdout io.Writer, opts QueueHealthOptions, deps queueDeps) error {
	if opts.StaleAfter <= 0 {
		opts.StaleAfter = 30 * time.Minute
	}
	summary, err := buildQueueSummary(10, opts.StaleAfter, deps)
	if err != nil {
		return err
	}
	status := deriveQueueHealthStatus(summary, opts.Strict)
	summary.Status = status
	if opts.WriteReport {
		if err := writeQueueArtifacts(summary); err != nil {
			return err
		}
	}
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		if status == "failed" {
			return fmt.Errorf("queue health failed")
		}
		return nil
	}
	fmt.Fprintln(stdout, "Queue health")
	fmt.Fprintf(stdout, "  status:       %s\n", status)
	for _, check := range summary.HealthChecks {
		fmt.Fprintf(stdout, "  - %s: %s (%s)\n", check.ID, check.Status, check.Message)
	}
	if opts.WriteReport {
		fmt.Fprintf(stdout, "  report:       %s\n", filepath.Join(queueRoot, "queue_health.md"))
		fmt.Fprintf(stdout, "  summary:      %s\n", filepath.Join(queueRoot, "queue_summary.json"))
	}
	if len(summary.NextCommands) > 0 {
		fmt.Fprintln(stdout, "  next commands:")
		for _, command := range summary.NextCommands {
			fmt.Fprintf(stdout, "    - %s\n", command)
		}
	}
	if status == "failed" {
		return fmt.Errorf("queue health failed")
	}
	return nil
}

func buildQueueSummary(limit int, staleAfter time.Duration, deps queueDeps) (QueueSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	summary := QueueSummary{
		SchemaVersion: queueSummarySchema,
		CreatedAt:     deps.now(),
		Status:        "ok",
		Daemon: QueueRuntimeStatus{
			Status:    "stopped",
			StatePath: daemonStatePath(),
		},
		Worker: QueueWorkerStatus{
			Status:    "idle",
			StatePath: workerStatePath(),
		},
		Jobs: QueueJobsSummary{
			ByStatus:   map[string]int{},
			ByApproval: map[string]int{},
		},
	}

	if _, err := os.Stat(".byom-video"); err != nil {
		if os.IsNotExist(err) {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
				ID:      "root_dir",
				Status:  "warning",
				Message: ".byom-video directory is missing",
			})
		} else {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
				ID:      "root_dir",
				Status:  "failed",
				Message: fmt.Sprintf("read .byom-video: %v", err),
			})
		}
	} else {
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
			ID:      "root_dir",
			Status:  "ok",
			Message: ".byom-video directory readable",
		})
	}
	if _, err := os.Stat(jobsRoot); err != nil {
		if os.IsNotExist(err) {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
				ID:      "jobs_dir",
				Status:  "ok",
				Message: "jobs directory not created yet",
			})
		} else {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
				ID:      "jobs_dir",
				Status:  "failed",
				Message: fmt.Sprintf("read jobs dir: %v", err),
			})
		}
	} else {
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
			ID:      "jobs_dir",
			Status:  "ok",
			Message: "jobs directory readable",
		})
	}

	if state, err := deps.readDaemon(); err == nil {
		summary.Daemon.Status = state.Status
		summary.Daemon.PID = state.PID
		if state.PID > 0 {
			summary.Daemon.PIDAlive = deps.processAlive(state.PID)
		}
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
			ID:      "daemon_state",
			Status:  "ok",
			Message: "daemon state readable",
		})
	} else {
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{ID: "daemon_state", Status: "warning", Message: err.Error()})
	}
	if ws, err := deps.readWorker(); err == nil {
		summary.Worker.Status = ws.Status
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{
			ID:      "worker_state",
			Status:  "ok",
			Message: "worker state readable",
		})
	} else {
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{ID: "worker_state", Status: "warning", Message: err.Error()})
	}
	if lockPresent, lock, lockErr := readWorkerLockStatus(); lockPresent {
		summary.Worker.LockPresent = true
		if lockErr != "" {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{ID: "worker_lock", Status: "warning", Message: lockErr})
		} else if lock != nil && lock.PID > 0 && !deps.processAlive(lock.PID) {
			summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{ID: "worker_lock", Status: "warning", Message: fmt.Sprintf("worker lock is stale for pid %d", lock.PID)})
		}
	}
	if summary.Daemon.PID > 0 && !summary.Daemon.PIDAlive {
		summary.HealthChecks = append(summary.HealthChecks, QueueHealthCheck{ID: "daemon_pid", Status: "warning", Message: fmt.Sprintf("daemon pid %d is not alive", summary.Daemon.PID)})
	}

	jobs, err := collectQueueJobs(limit, staleAfter, deps.readJob, deps.now())
	if err != nil {
		return summary, err
	}
	summary.Jobs = jobs
	summary.NextCommands = buildQueueNextCommands(summary)
	return summary, nil
}

func collectQueueJobs(limit int, staleAfter time.Duration, readJobFn func(string) (*Job, error), now time.Time) (QueueJobsSummary, error) {
	summary := QueueJobsSummary{
		ByStatus:   map[string]int{},
		ByApproval: map[string]int{},
	}
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return summary, nil
		}
		return summary, fmt.Errorf("read jobs dir: %w", err)
	}
	items := []QueueJobView{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		job, err := readJobFn(entry.Name())
		if err != nil {
			continue
		}
		summary.Total++
		summary.ByStatus[job.Status]++
		summary.ByApproval[job.ApprovalStatus]++
		view := QueueJobView{
			JobID:             job.JobID,
			Status:            job.Status,
			Approval:          job.ApprovalStatus,
			ActionType:        job.ActionType,
			SourceAgentPlanID: inputString(job.Input, "agent_plan_id"),
			Preview:           queueJobPreview(job),
			UpdatedAt:         job.UpdatedAt,
			CreatedAt:         job.CreatedAt,
			Warnings:          append([]string{}, job.Warnings...),
		}
		items = append(items, view)
		switch {
		case job.Status == JobStatusPending && job.ApprovalStatus == JobApprovalPending:
			summary.ApprovalNeeded = append(summary.ApprovalNeeded, view)
		case job.Status == JobStatusFailed:
			summary.Failed = append(summary.Failed, view)
		case job.Status == JobStatusRunning:
			summary.Running = append(summary.Running, view)
			if !job.UpdatedAt.IsZero() && now.Sub(job.UpdatedAt) > staleAfter {
				summary.StaleRunning = append(summary.StaleRunning, view)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) || items[i].UpdatedAt.IsZero() || items[j].UpdatedAt.IsZero() {
			return items[i].JobID > items[j].JobID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	summary.Recent = items
	return summary, nil
}

func queueJobPreview(job *Job) string {
	if job == nil {
		return ""
	}
	for _, key := range []string{"goal", "request", "make_id", "plan_id"} {
		if value := inputString(job.Input, key); value != "" {
			return truncate(value, 48)
		}
	}
	return "-"
}

func readWorkerLockStatus() (bool, *WorkerLock, string) {
	data, err := os.ReadFile(workerLockPath())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, ""
		}
		return true, nil, err.Error()
	}
	var lock WorkerLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return true, nil, err.Error()
	}
	return true, &lock, ""
}

func deriveQueueHealthStatus(summary QueueSummary, strict bool) string {
	failed := false
	warning := false
	for _, check := range summary.HealthChecks {
		switch check.Status {
		case "failed":
			failed = true
		case "warning":
			warning = true
		}
	}
	if len(summary.Jobs.StaleRunning) > 0 {
		warning = true
	}
	if len(summary.Jobs.Failed) > 0 {
		warning = true
	}
	if len(summary.Jobs.ApprovalNeeded) > 0 {
		warning = true
	}
	if strict && warning {
		failed = true
	}
	switch {
	case failed:
		return "failed"
	case warning:
		return "warning"
	default:
		return "ok"
	}
}

func buildQueueNextCommands(summary QueueSummary) []string {
	next := []string{}
	if summary.Daemon.Status == "stopped" || summary.Daemon.Status == "unknown" {
		next = append(next, "byom-video daemon start --interval 10s")
	}
	if len(summary.Jobs.ApprovalNeeded) > 0 {
		next = append(next, fmt.Sprintf("byom-video job-approve %s", summary.Jobs.ApprovalNeeded[0].JobID))
	}
	if len(summary.Jobs.Failed) > 0 {
		next = append(next, fmt.Sprintf("byom-video job-result %s", summary.Jobs.Failed[0].JobID))
	}
	if summary.Worker.LockPresent && !summary.Daemon.PIDAlive {
		next = append(next, "byom-video job-worker --once --force-lock")
	}
	if summary.Jobs.Total == 0 {
		next = append(next, "byom-video job-create --type make --goal \"...\"")
	}
	return dedupeStrings(next)
}

func writeQueueArtifacts(summary QueueSummary) error {
	if err := os.MkdirAll(queueRoot, 0o755); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(queueRoot, "queue_summary.json"), summary); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Queue Health\n\n")
	fmt.Fprintf(&b, "- Status: `%s`\n", summary.Status)
	fmt.Fprintf(&b, "- Created: `%s`\n", summary.CreatedAt.Format(time.RFC3339))
	b.WriteString("\n## Runtime\n\n")
	fmt.Fprintf(&b, "- Daemon: `%s` (pid `%d`, alive=`%t`)\n", summary.Daemon.Status, summary.Daemon.PID, summary.Daemon.PIDAlive)
	fmt.Fprintf(&b, "- Worker: `%s` (lock_present=`%t`)\n", summary.Worker.Status, summary.Worker.LockPresent)
	b.WriteString("\n## Jobs\n\n")
	fmt.Fprintf(&b, "- Total: `%d`\n", summary.Jobs.Total)
	for _, key := range []string{JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed, JobStatusCancelled} {
		fmt.Fprintf(&b, "- %s: `%d`\n", key, summary.Jobs.ByStatus[key])
	}
	if len(summary.HealthChecks) > 0 {
		b.WriteString("\n## Health Checks\n\n")
		for _, check := range summary.HealthChecks {
			fmt.Fprintf(&b, "- `%s` `%s` — %s\n", check.ID, check.Status, check.Message)
		}
	}
	if len(summary.NextCommands) > 0 {
		b.WriteString("\n## Next Commands\n\n")
		for _, command := range summary.NextCommands {
			fmt.Fprintf(&b, "- `%s`\n", command)
		}
	}
	return os.WriteFile(filepath.Join(queueRoot, "queue_health.md"), []byte(b.String()), 0o644)
}

func pidStatusText(pid int, alive bool) string {
	switch {
	case pid <= 0:
		return "missing"
	case alive:
		return fmt.Sprintf("%d alive", pid)
	default:
		return fmt.Sprintf("%d dead", pid)
	}
}

package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const agentRunSummarySchema = "openvfx_agent_run_summary.v1"

type AgentRunOptions struct {
	Yes                bool
	Convert            bool
	ApproveJobs        bool
	RunJobs            bool
	WorkerOnce         bool
	StartDaemon        bool
	DryRun             bool
	JSON               bool
	AllowProviderCalls bool
	AllowOverwrite     bool
	Force              bool
	FailFast           bool
	WriteSummary       bool
}

type AgentRunSummary struct {
	SchemaVersion string               `json:"schema_version"`
	CreatedAt     time.Time            `json:"created_at"`
	AgentPlanID   string               `json:"agent_plan_id"`
	Status        string               `json:"status"`
	Steps         []AgentRunStep       `json:"steps"`
	LinkedJobs    []AgentLinkedJobItem `json:"linked_jobs,omitempty"`
	Warnings      []string             `json:"warnings,omitempty"`
	Errors        []string             `json:"errors,omitempty"`
	NextCommands  []string             `json:"next_commands,omitempty"`
}

type AgentRunStep struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type AgentPlanResultSummary struct {
	PlanID           string               `json:"plan_id"`
	Status           string               `json:"status"`
	Intent           string               `json:"intent"`
	PolicyStatus     string               `json:"policy_status"`
	ApprovalStatus   string               `json:"approval_status"`
	ConversionStatus string               `json:"conversion_status"`
	LinkedJobs       []AgentLinkedJobItem `json:"linked_jobs,omitempty"`
	Warnings         []string             `json:"warnings,omitempty"`
	Errors           []string             `json:"errors,omitempty"`
	NextCommands     []string             `json:"next_commands,omitempty"`
	GeneratedAt      time.Time            `json:"generated_at"`
}

type agentRunDeps struct {
	runJob      func(string, io.Writer, JobRunOptions) error
	runWorker   func(io.Writer, JobWorkerOptions) error
	startDaemon func(io.Writer, DaemonStartOptions) error
}

var defaultAgentRunDeps = agentRunDeps{
	runJob:      JobRun,
	runWorker:   JobWorker,
	startDaemon: DaemonStart,
}

func AgentRun(planID string, stdout io.Writer, opts AgentRunOptions) error {
	return agentRunWithDeps(planID, stdout, opts, defaultAgentRunDeps)
}

func AgentPlanResult(planID string, stdout io.Writer, opts AgentResultOptions) error {
	summary, err := buildAgentPlanResultSummary(planID)
	if err != nil {
		return err
	}
	artifactPath := ""
	if opts.WriteArtifact {
		artifactPath = filepath.Join(agentPlansV1Root, planID, "agent_result.md")
		if err := os.WriteFile(artifactPath, []byte(renderAgentPlanResultMarkdown(summary)), 0o644); err != nil {
			return err
		}
		appendAgentPlanEvent(planID, "AGENT_RESULT_ARTIFACT_WRITTEN", map[string]any{"path": artifactPath})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printAgentPlanResult(stdout, summary)
	if artifactPath != "" {
		fmt.Fprintf(stdout, "  artifact: %s\n", artifactPath)
	}
	return nil
}

func agentRunWithDeps(planID string, stdout io.Writer, opts AgentRunOptions, deps agentRunDeps) error {
	if opts.RunJobs && opts.WorkerOnce {
		return fmt.Errorf("--run-jobs and --worker-once cannot be used together")
	}
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	summary := AgentRunSummary{
		SchemaVersion: agentRunSummarySchema,
		CreatedAt:     time.Now().UTC(),
		AgentPlanID:   planID,
		Status:        "planned",
	}
	addStep := func(stepType, status, message string) {
		summary.Steps = append(summary.Steps, AgentRunStep{
			ID:      fmt.Sprintf("step_%04d", len(summary.Steps)+1),
			Type:    stepType,
			Status:  status,
			Message: message,
		})
	}
	if opts.DryRun || (!opts.Yes && !opts.Convert && !opts.RunJobs && !opts.WorkerOnce && !opts.StartDaemon) {
		addStep("review", "skipped", "no mutating flags supplied")
		summary.NextCommands = buildAgentRunNextCommands(planID)
		return printOrWriteAgentRunSummary(stdout, opts, summary)
	}
	appendAgentPlanEvent(planID, "AGENT_RUN_STARTED", map[string]any{"plan_id": planID})
	if opts.Yes && plan.Status != agentPlanStatusApproved && plan.Status != agentPlanStatusConverted {
		if err := ApproveAgentPlan(planID, io.Discard, ApproveAgentPlanOptions{}); err != nil {
			summary.Status = "failed"
			summary.Errors = append(summary.Errors, err.Error())
			addStep("approve", "failed", err.Error())
			appendAgentPlanEvent(planID, "AGENT_RUN_FAILED", map[string]any{"error": err.Error()})
			return printOrWriteAgentRunSummary(stdout, opts, summary)
		}
		addStep("approve", "completed", "agent plan approved")
		appendAgentPlanEvent(planID, "AGENT_RUN_APPROVED_PLAN", map[string]any{"plan_id": planID})
	}
	if opts.Convert {
		var out bytes.Buffer
		err := AgentPlanToJob(planID, &out, AgentPlanToJobOptions{
			Yes:                opts.Yes,
			AllowProviderCalls: opts.AllowProviderCalls,
			AllowOverwrite:     opts.AllowOverwrite,
			ApproveJobs:        opts.ApproveJobs,
			Force:              opts.Force,
		})
		if err != nil {
			summary.Status = "failed"
			summary.Errors = append(summary.Errors, err.Error())
			addStep("convert", "failed", err.Error())
			appendAgentPlanEvent(planID, "AGENT_RUN_FAILED", map[string]any{"error": err.Error()})
			return printOrWriteAgentRunSummary(stdout, opts, summary)
		}
		addStep("convert", "completed", "agent plan converted to jobs")
		appendAgentPlanEvent(planID, "AGENT_RUN_CONVERTED_PLAN", map[string]any{"plan_id": planID})
	}
	linked, linkedErr := readAgentLinkedJobs(planID)
	if linkedErr == nil {
		linked = refreshAgentLinkedJobsInMemory(linked)
		summary.LinkedJobs = linked.Jobs
	}
	if opts.RunJobs {
		appendAgentPlanEvent(planID, "AGENT_RUN_STARTED_JOBS", map[string]any{"plan_id": planID})
		for _, item := range summary.LinkedJobs {
			job, err := readJob(item.JobID)
			if err != nil {
				summary.Errors = append(summary.Errors, err.Error())
				addStep("run_job", "failed", err.Error())
				if opts.FailFast {
					break
				}
				continue
			}
			if job.Status != JobStatusPending || (job.ApprovalStatus != JobApprovalApproved && job.ApprovalStatus != JobApprovalNotRequired) {
				addStep("run_job", "skipped", fmt.Sprintf("%s not eligible", job.JobID))
				continue
			}
			err = deps.runJob(job.JobID, stdout, JobRunOptions{
				Yes:                true,
				AllowProviderCalls: opts.AllowProviderCalls,
				AllowOverwrite:     opts.AllowOverwrite,
			})
			if err != nil {
				summary.Errors = append(summary.Errors, err.Error())
				addStep("run_job", "failed", fmt.Sprintf("%s: %s", job.JobID, err.Error()))
				appendAgentPlanEvent(planID, "AGENT_RUN_FAILED_JOB", map[string]any{"job_id": job.JobID, "error": err.Error()})
				if opts.FailFast {
					break
				}
				continue
			}
			addStep("run_job", "completed", job.JobID)
			appendAgentPlanEvent(planID, "AGENT_RUN_COMPLETED_JOB", map[string]any{"job_id": job.JobID})
		}
	}
	if opts.WorkerOnce {
		if err := deps.runWorker(stdout, JobWorkerOptions{Once: true, AllowProviderCalls: opts.AllowProviderCalls, AllowOverwrite: opts.AllowOverwrite, FailFast: opts.FailFast}); err != nil {
			summary.Errors = append(summary.Errors, err.Error())
			addStep("worker_once", "failed", err.Error())
		} else {
			addStep("worker_once", "completed", "job-worker --once completed")
		}
	}
	if opts.StartDaemon {
		if err := deps.startDaemon(stdout, DaemonStartOptions{Interval: 10 * time.Second, AllowProviderCalls: opts.AllowProviderCalls, AllowOverwrite: opts.AllowOverwrite, FailFast: opts.FailFast}); err != nil {
			summary.Errors = append(summary.Errors, err.Error())
			addStep("start_daemon", "failed", err.Error())
		} else {
			addStep("start_daemon", "completed", "daemon started")
			appendAgentPlanEvent(planID, "AGENT_RUN_STARTED_DAEMON", map[string]any{"plan_id": planID})
		}
	}
	if linked, err := readAgentLinkedJobs(planID); err == nil {
		linked = refreshAgentLinkedJobsInMemory(linked)
		summary.LinkedJobs = linked.Jobs
		_ = writeAgentLinkedJobs(linked)
	}
	if len(summary.Errors) > 0 {
		summary.Status = "failed"
		appendAgentPlanEvent(planID, "AGENT_RUN_FAILED", map[string]any{"errors": summary.Errors})
	} else {
		summary.Status = "completed"
		appendAgentPlanEvent(planID, "AGENT_RUN_COMPLETED", map[string]any{"plan_id": planID})
	}
	summary.NextCommands = buildAgentRunNextCommands(planID)
	if opts.WriteSummary || opts.Convert || opts.RunJobs || opts.WorkerOnce || opts.StartDaemon {
		if err := writeAgentRunSummary(planID, summary); err != nil {
			return err
		}
	}
	return printOrWriteAgentRunSummary(stdout, opts, summary)
}

func buildAgentPlanResultSummary(planID string) (AgentPlanResultSummary, error) {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return AgentPlanResultSummary{}, err
	}
	policy, _ := readAgentPlanPolicy(planID)
	var linked AgentLinkedJobs
	if existing, err := readAgentLinkedJobs(planID); err == nil {
		linked = refreshAgentLinkedJobsInMemory(existing)
	}
	summary := AgentPlanResultSummary{
		PlanID:           plan.PlanID,
		Status:           plan.Status,
		Intent:           plan.Intent,
		PolicyStatus:     policy.Status,
		ApprovalStatus:   plan.Status,
		ConversionStatus: agentConversionStatus(plan, linked),
		LinkedJobs:       linked.Jobs,
		Warnings:         append(append([]string{}, plan.Warnings...), linked.Warnings...),
		Errors:           append(append([]string{}, plan.Errors...), linked.Errors...),
		GeneratedAt:      time.Now().UTC(),
	}
	summary.NextCommands = buildAgentResultNextCommands(plan, linked)
	return summary, nil
}

func printOrWriteAgentRunSummary(stdout io.Writer, opts AgentRunOptions, summary AgentRunSummary) error {
	if opts.WriteSummary && len(summary.Steps) == 1 && summary.Steps[0].Status == "skipped" {
		if err := writeAgentRunSummary(summary.AgentPlanID, summary); err != nil {
			return err
		}
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Agent run")
	fmt.Fprintf(stdout, "  plan id: %s\n", summary.AgentPlanID)
	fmt.Fprintf(stdout, "  status:  %s\n", summary.Status)
	for _, step := range summary.Steps {
		fmt.Fprintf(stdout, "  - %s: %s %s\n", step.Type, step.Status, step.Message)
	}
	if len(summary.LinkedJobs) > 0 {
		fmt.Fprintln(stdout, "  linked jobs:")
		for _, job := range summary.LinkedJobs {
			fmt.Fprintf(stdout, "    - %s %s approval=%s\n", job.JobID, job.Status, job.ApprovalStatus)
		}
	}
	for _, command := range summary.NextCommands {
		fmt.Fprintf(stdout, "  next: %s\n", command)
	}
	return nil
}

func printAgentPlanResult(stdout io.Writer, summary AgentPlanResultSummary) {
	fmt.Fprintln(stdout, "Agent result")
	fmt.Fprintf(stdout, "  plan id:       %s\n", summary.PlanID)
	fmt.Fprintf(stdout, "  status:        %s\n", summary.Status)
	fmt.Fprintf(stdout, "  intent:        %s\n", summary.Intent)
	fmt.Fprintf(stdout, "  policy:        %s\n", summary.PolicyStatus)
	fmt.Fprintf(stdout, "  conversion:    %s\n", summary.ConversionStatus)
	for _, job := range summary.LinkedJobs {
		fmt.Fprintf(stdout, "  job:           %s %s approval=%s type=%s\n", job.JobID, job.Status, job.ApprovalStatus, job.JobType)
		for key, value := range job.Output {
			fmt.Fprintf(stdout, "    output %s: %v\n", key, value)
		}
	}
	for _, command := range summary.NextCommands {
		fmt.Fprintf(stdout, "  next:          %s\n", command)
	}
}

func renderAgentPlanResultMarkdown(summary AgentPlanResultSummary) string {
	var b strings.Builder
	b.WriteString("# Agent Result\n\n")
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", summary.PlanID)
	fmt.Fprintf(&b, "- Status: `%s`\n", summary.Status)
	fmt.Fprintf(&b, "- Intent: %s\n", summary.Intent)
	fmt.Fprintf(&b, "- Policy: `%s`\n", summary.PolicyStatus)
	fmt.Fprintf(&b, "- Conversion: `%s`\n", summary.ConversionStatus)
	if len(summary.LinkedJobs) > 0 {
		b.WriteString("\n## Jobs\n\n")
		for _, job := range summary.LinkedJobs {
			fmt.Fprintf(&b, "- `%s` `%s` approval=`%s` type=`%s`\n", job.JobID, job.Status, job.ApprovalStatus, job.JobType)
		}
	}
	if len(summary.NextCommands) > 0 {
		b.WriteString("\n## Next Commands\n\n")
		for _, command := range summary.NextCommands {
			fmt.Fprintf(&b, "- `%s`\n", command)
		}
	}
	return b.String()
}

func writeAgentRunSummary(planID string, summary AgentRunSummary) error {
	return writeJSONFile(filepath.Join(agentPlansV1Root, planID, "agent_run_summary.json"), summary)
}

func buildAgentRunNextCommands(planID string) []string {
	return []string{
		fmt.Sprintf("byom-video agent-result %s", planID),
		fmt.Sprintf("byom-video agent-plan-jobs %s", planID),
		"byom-video queue",
	}
}

func buildAgentResultNextCommands(plan AgentPlanV1, linked AgentLinkedJobs) []string {
	next := []string{}
	if plan.Status != agentPlanStatusApproved && plan.Status != agentPlanStatusConverted {
		next = append(next, fmt.Sprintf("byom-video approve-agent-plan %s", plan.PlanID))
	}
	if len(linked.Jobs) == 0 && plan.Status == agentPlanStatusApproved {
		next = append(next, fmt.Sprintf("byom-video agent-plan-to-job %s", plan.PlanID))
	}
	for _, job := range linked.Jobs {
		switch {
		case job.Status == JobStatusPending && job.ApprovalStatus == JobApprovalPending:
			next = append(next, fmt.Sprintf("byom-video job-approve %s", job.JobID))
		case job.Status == JobStatusPending && (job.ApprovalStatus == JobApprovalApproved || job.ApprovalStatus == JobApprovalNotRequired):
			next = append(next, "byom-video job-worker --once")
			next = append(next, "byom-video daemon start --interval 10s")
		case job.Status == JobStatusFailed:
			next = append(next, fmt.Sprintf("byom-video job-result %s", job.JobID))
		case job.Status == JobStatusCompleted:
			next = append(next, fmt.Sprintf("byom-video job-result %s", job.JobID))
		}
	}
	next = append(next, "byom-video queue")
	return dedupeStrings(next)
}

func agentConversionStatus(plan AgentPlanV1, linked AgentLinkedJobs) string {
	if plan.Status == agentPlanStatusConverted {
		return "converted"
	}
	if len(linked.Jobs) > 0 {
		return "linked"
	}
	return "not_converted"
}

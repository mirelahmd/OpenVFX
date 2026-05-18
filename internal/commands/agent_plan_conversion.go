package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
)

const linkedJobsV1Schema = "openvfx_agent_linked_jobs.v1"

type ApproveAgentPlanOptions struct{ JSON bool }

type RejectAgentPlanOptions struct {
	Reason string
	JSON   bool
}

type AgentPlanJobsOptions struct{ JSON bool }

type AgentPlanToJobOptions struct {
	DryRun             bool
	Yes                bool
	JSON               bool
	AllowProviderCalls bool
	AllowOverwrite     bool
	ApproveJobs        bool
	Force              bool
}

type AgentLinkedJobs struct {
	SchemaVersion string               `json:"schema_version"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	AgentPlanID   string               `json:"agent_plan_id"`
	Jobs          []AgentLinkedJobItem `json:"jobs"`
	Warnings      []string             `json:"warnings,omitempty"`
	Errors        []string             `json:"errors,omitempty"`
}

type AgentLinkedJobItem struct {
	JobID          string         `json:"job_id"`
	ActionID       string         `json:"action_id"`
	ActionType     string         `json:"action_type"`
	JobType        string         `json:"job_type"`
	CreatedAt      time.Time      `json:"created_at"`
	Status         string         `json:"status"`
	ApprovalStatus string         `json:"approval_status"`
	JobPath        string         `json:"job_path"`
	Output         map[string]any `json:"output,omitempty"`
	Errors         []string       `json:"errors,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type agentConversionPreview struct {
	AgentPlanID string                `json:"agent_plan_id"`
	DryRun      bool                  `json:"dry_run"`
	Jobs        []AgentLinkedJobItem  `json:"jobs,omitempty"`
	Skipped     []map[string]string   `json:"skipped,omitempty"`
	Warnings    []string              `json:"warnings,omitempty"`
	Errors      []string              `json:"errors,omitempty"`
	Policy      AgentPlanPolicyReview `json:"policy_review"`
}

func ApproveAgentPlan(planID string, stdout io.Writer, opts ApproveAgentPlanOptions) error {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	if plan.Status == agentPlanStatusRejected {
		return fmt.Errorf("agent plan %s is rejected and cannot be approved", planID)
	}
	now := time.Now().UTC()
	plan.Status = agentPlanStatusApproved
	plan.ApprovedAt = &now
	plan.ApprovalMode = "manual"
	if err := writeAgentPlan(plan); err != nil {
		return err
	}
	appendAgentPlanEvent(planID, "AGENT_PLAN_APPROVED", map[string]any{"plan_id": planID, "approved_at": now})
	if opts.JSON {
		data, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Agent plan approved: %s\n", planID)
	fmt.Fprintf(stdout, "  approved_at: %s\n", now.Format(time.RFC3339))
	return nil
}

func RejectAgentPlan(planID string, stdout io.Writer, opts RejectAgentPlanOptions) error {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	plan.Status = agentPlanStatusRejected
	plan.RejectedAt = &now
	plan.RejectionReason = opts.Reason
	if err := writeAgentPlan(plan); err != nil {
		return err
	}
	appendAgentPlanEvent(planID, "AGENT_PLAN_REJECTED", map[string]any{"plan_id": planID, "rejected_at": now, "reason": opts.Reason})
	if opts.JSON {
		data, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Agent plan rejected: %s\n", planID)
	if opts.Reason != "" {
		fmt.Fprintf(stdout, "  reason: %s\n", opts.Reason)
	}
	return nil
}

func AgentPlanJobs(planID string, stdout io.Writer, opts AgentPlanJobsOptions) error {
	linked, err := readAgentLinkedJobs(planID)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.JSON {
				fmt.Fprintln(stdout, `{"jobs":[]}`)
			} else {
				fmt.Fprintf(stdout, "No linked jobs found for agent plan %s.\n", planID)
			}
			return nil
		}
		return err
	}
	linked = refreshAgentLinkedJobsInMemory(linked)
	if opts.JSON {
		data, _ := json.MarshalIndent(linked, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Agent plan jobs")
	fmt.Fprintf(stdout, "  plan id: %s\n", linked.AgentPlanID)
	for _, job := range linked.Jobs {
		fmt.Fprintf(stdout, "  - %s action=%s type=%s status=%s approval=%s path=%s\n", job.JobID, job.ActionID, job.JobType, job.Status, job.ApprovalStatus, job.JobPath)
	}
	for _, warning := range linked.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
	return nil
}

func AgentPlanToJob(planID string, stdout io.Writer, opts AgentPlanToJobOptions) error {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	policy, err := readAgentPlanPolicy(planID)
	if err != nil {
		return err
	}
	if plan.Status == agentPlanStatusRejected {
		return fmt.Errorf("agent plan %s is rejected and cannot be converted", planID)
	}
	if plan.Status != agentPlanStatusApproved && plan.Status != agentPlanStatusConverted {
		if !opts.Yes {
			return fmt.Errorf("agent plan %s must be approved before conversion; use approve-agent-plan or --yes", planID)
		}
		now := time.Now().UTC()
		plan.Status = agentPlanStatusApproved
		plan.ApprovedAt = &now
		plan.ApprovalMode = "yes_flag"
	}
	if policy.Status == "blocked" && !opts.Force {
		return fmt.Errorf("agent plan policy is blocked; use --force only after review")
	}
	if existing, err := readAgentLinkedJobs(planID); err == nil && len(existing.Jobs) > 0 && !opts.Force {
		return fmt.Errorf("agent plan already has linked jobs; use --force to convert again")
	}
	if opts.DryRun {
		preview, err := previewAgentJobConversion(plan, policy, opts)
		if err != nil {
			return err
		}
		preview.DryRun = true
		if opts.JSON {
			data, _ := json.MarshalIndent(preview, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		printAgentConversionPreview(stdout, preview)
		return nil
	}

	appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_STARTED", map[string]any{"plan_id": planID})
	preview, err := previewAgentJobConversion(plan, policy, opts)
	if err != nil {
		appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_FAILED", map[string]any{"plan_id": planID, "error": err.Error()})
		return err
	}
	linked := AgentLinkedJobs{
		SchemaVersion: linkedJobsV1Schema,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		AgentPlanID:   planID,
		Warnings:      append([]string{}, preview.Warnings...),
		Errors:        append([]string{}, preview.Errors...),
	}
	if existing, err := readAgentLinkedJobs(planID); err == nil && opts.Force {
		linked.CreatedAt = existing.CreatedAt
		linked.Jobs = append(linked.Jobs, existing.Jobs...)
		linked.Warnings = append(linked.Warnings, "force conversion appended additional jobs")
	}
	for _, action := range plan.Actions {
		item, job, skip, err := convertAgentActionToJob(plan, action, opts)
		if err != nil {
			appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_FAILED", map[string]any{"plan_id": planID, "error": err.Error()})
			return err
		}
		if skip != "" {
			continue
		}
		if err := writeJob(job); err != nil {
			appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_FAILED", map[string]any{"plan_id": planID, "error": err.Error()})
			return err
		}
		appendJobEvent(job.JobID, JobEventCreated, map[string]any{
			"job_id":          job.JobID,
			"action_type":     job.ActionType,
			"agent_plan_id":   planID,
			"agent_action_id": action.ID,
			"approval_status": job.ApprovalStatus,
		})
		appendAgentPlanEvent(planID, "AGENT_PLAN_JOB_CREATED", map[string]any{"plan_id": planID, "job_id": job.JobID, "action_id": action.ID})
		linked.Jobs = append(linked.Jobs, item)
	}
	if err := writeAgentLinkedJobs(linked); err != nil {
		appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_FAILED", map[string]any{"plan_id": planID, "error": err.Error()})
		return err
	}
	plan.Status = agentPlanStatusConverted
	plan.NextCommands = append([]string{
		fmt.Sprintf("byom-video agent-plan-jobs %s", planID),
		"byom-video jobs",
	}, plan.NextCommands...)
	plan.NextCommands = dedupeStrings(plan.NextCommands)
	if err := writeAgentPlan(plan); err != nil {
		appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_FAILED", map[string]any{"plan_id": planID, "error": err.Error()})
		return err
	}
	appendAgentPlanEvent(planID, "AGENT_PLAN_CONVERSION_COMPLETED", map[string]any{"plan_id": planID, "jobs": len(linked.Jobs)})
	if opts.JSON {
		data, _ := json.MarshalIndent(linked, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Agent plan converted: %s\n", planID)
	fmt.Fprintf(stdout, "  jobs: %d\n", len(linked.Jobs))
	for _, warning := range linked.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
	return nil
}

func previewAgentJobConversion(plan AgentPlanV1, policy AgentPlanPolicyReview, opts AgentPlanToJobOptions) (agentConversionPreview, error) {
	preview := agentConversionPreview{AgentPlanID: plan.PlanID, Policy: policy}
	if policy.Status == "blocked" && !opts.Force {
		preview.Errors = append(preview.Errors, "policy is blocked")
		return preview, fmt.Errorf("agent plan policy is blocked; use --force only after review")
	}
	for _, action := range plan.Actions {
		item, _, skip, err := convertAgentActionToJob(plan, action, opts)
		if err != nil {
			preview.Errors = append(preview.Errors, err.Error())
			return preview, err
		}
		if skip != "" {
			preview.Skipped = append(preview.Skipped, map[string]string{
				"action_id":   action.ID,
				"action_type": action.Type,
				"reason":      skip,
			})
			preview.Warnings = append(preview.Warnings, skip)
			continue
		}
		preview.Jobs = append(preview.Jobs, item)
	}
	return preview, nil
}

func convertAgentActionToJob(plan AgentPlanV1, action AgentActionV1, opts AgentPlanToJobOptions) (AgentLinkedJobItem, *Job, string, error) {
	if action.Type == agentActionTypeQueue {
		return AgentLinkedJobItem{}, nil, "queue_health action is informational and is not converted to a job in v1", nil
	}
	if action.RequiresProvider && !opts.AllowProviderCalls {
		return AgentLinkedJobItem{}, nil, "", fmt.Errorf("action %s requires provider calls; pass --allow-provider-calls to convert", action.ID)
	}
	if action.RequiresOverwrite && !opts.AllowOverwrite {
		return AgentLinkedJobItem{}, nil, "", fmt.Errorf("action %s requires overwrite permission; pass --allow-overwrite to convert", action.ID)
	}
	jobType := ""
	input := map[string]any{
		"agent_plan_id":   plan.PlanID,
		"agent_action_id": action.ID,
	}
	switch action.Type {
	case agentActionTypeMake:
		jobType = JobActionMake
		copyAgentActionKeys(input, action.Input, []string{
			"input_path", "goal", "platform", "preset", "burn_captions", "allow_missing_captions",
			"generate_script", "generate_captions", "prepare_voiceover", "generate_voiceover",
			"mix_voiceover", "caption_position", "caption_style", "overwrite", "skip_pipeline",
		})
		if inputString(input, "goal") == "" {
			input["goal"] = plan.Input.Goal
		}
	case agentActionTypeReviseMake:
		jobType = JobActionReviseMake
		copyAgentActionKeys(input, action.Input, []string{"make_id", "request", "reassemble", "validate", "overwrite", "fallback_stub"})
	case agentActionTypeValidate:
		jobType = JobActionValidateCreativeAssemble
		if value := inputString(action.Input, "creative_plan_id"); value != "" {
			input["plan_id"] = value
		}
	default:
		return AgentLinkedJobItem{}, nil, "", fmt.Errorf("unsupported agent action type %s for job conversion", action.Type)
	}
	approval := JobApprovalPending
	if jobType == JobActionValidateCreativeAssemble {
		approval = JobApprovalNotRequired
	} else if opts.ApproveJobs {
		approval = JobApprovalApproved
	}
	now := time.Now().UTC()
	jobID := fmt.Sprintf("%s-%s-%s", now.Format("20060102T150405.000000000Z"), strings.ReplaceAll(jobType, "_", "-"), action.ID)
	job := &Job{
		SchemaVersion:  jobSchemaVersion,
		JobID:          jobID,
		CreatedAt:      now,
		UpdatedAt:      now,
		ActionType:     jobType,
		Status:         JobStatusPending,
		ApprovalStatus: approval,
		Policy: JobPolicy{
			AllowOverwrite:       opts.AllowOverwrite || inputBool(action.Input, "overwrite"),
			AllowProviderCalls:   opts.AllowProviderCalls,
			AllowExternalNetwork: opts.AllowProviderCalls,
			AllowMediaWrites:     true,
		},
		Input: input,
		NextCommands: []string{
			fmt.Sprintf("byom-video job-inspect %s", jobID),
		},
	}
	if approval == JobApprovalPending {
		job.NextCommands = append([]string{fmt.Sprintf("byom-video job-approve %s", jobID)}, job.NextCommands...)
	}
	item := AgentLinkedJobItem{
		JobID:          jobID,
		ActionID:       action.ID,
		ActionType:     action.Type,
		JobType:        jobType,
		CreatedAt:      now,
		Status:         job.Status,
		ApprovalStatus: job.ApprovalStatus,
		JobPath:        jobFilePath(jobID),
	}
	return item, job, "", nil
}

func copyAgentActionKeys(dst map[string]any, src map[string]any, keys []string) {
	for _, key := range keys {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}

func printAgentConversionPreview(stdout io.Writer, preview agentConversionPreview) {
	fmt.Fprintln(stdout, "Agent plan conversion preview")
	fmt.Fprintf(stdout, "  plan id: %s\n", preview.AgentPlanID)
	for _, job := range preview.Jobs {
		fmt.Fprintf(stdout, "  job:     action=%s type=%s approval=%s\n", job.ActionID, job.JobType, job.ApprovalStatus)
	}
	for _, skipped := range preview.Skipped {
		fmt.Fprintf(stdout, "  skip:    action=%s type=%s reason=%s\n", skipped["action_id"], skipped["action_type"], skipped["reason"])
	}
	for _, warning := range preview.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
}

func readAgentLinkedJobs(planID string) (AgentLinkedJobs, error) {
	var linked AgentLinkedJobs
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "linked_jobs.json"))
	if err != nil {
		return linked, err
	}
	if err := json.Unmarshal(data, &linked); err != nil {
		return linked, err
	}
	return linked, nil
}

func refreshAgentLinkedJobsInMemory(linked AgentLinkedJobs) AgentLinkedJobs {
	for i := range linked.Jobs {
		job, err := readJob(linked.Jobs[i].JobID)
		if err != nil {
			linked.Jobs[i].Errors = append(linked.Jobs[i].Errors, err.Error())
			continue
		}
		linked.Jobs[i].Status = job.Status
		linked.Jobs[i].ApprovalStatus = job.ApprovalStatus
		linked.Jobs[i].Output = job.Output
		linked.Jobs[i].Errors = append([]string{}, job.Errors...)
		linked.Jobs[i].Warnings = append([]string{}, job.Warnings...)
	}
	return linked
}

func writeAgentLinkedJobs(linked AgentLinkedJobs) error {
	linked.UpdatedAt = time.Now().UTC()
	return writeJSONFile(filepath.Join(agentPlansV1Root, linked.AgentPlanID, "linked_jobs.json"), linked)
}

func appendAgentPlanEvent(planID string, eventType string, details map[string]any) {
	log, err := events.Open(filepath.Join(agentPlansV1Root, planID, "events.jsonl"))
	if err != nil {
		return
	}
	defer log.Close()
	_ = log.Write(eventType, details)
}

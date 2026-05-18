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
	jobsRoot         = ".byom-video/jobs"
	jobSchemaVersion = "openvfx_job.v1"

	// action types
	JobActionMake                     = "make"
	JobActionReviseMake               = "revise_make"
	JobActionValidateCreativeAssemble = "validate_creative_assemble"

	// job status
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"

	// approval status
	JobApprovalPending     = "pending"
	JobApprovalApproved    = "approved"
	JobApprovalRejected    = "rejected"
	JobApprovalNotRequired = "not_required"

	// event names
	JobEventCreated         = "JOB_CREATED"
	JobEventApproved        = "JOB_APPROVED"
	JobEventRejected        = "JOB_REJECTED"
	JobEventCancelled       = "JOB_CANCELLED"
	JobEventRunStarted      = "JOB_RUN_STARTED"
	JobEventRunCompleted    = "JOB_RUN_COMPLETED"
	JobEventRunFailed       = "JOB_RUN_FAILED"
	JobEventActionStarted   = "JOB_ACTION_STARTED"
	JobEventActionCompleted = "JOB_ACTION_COMPLETED"
	JobEventActionFailed    = "JOB_ACTION_FAILED"
	JobEventPolicyBlocked   = "JOB_POLICY_BLOCKED"
)

// ---- schema types ----

type JobPolicy struct {
	AllowOverwrite       bool `json:"allow_overwrite"`
	AllowProviderCalls   bool `json:"allow_provider_calls"`
	AllowExternalNetwork bool `json:"allow_external_network"`
	AllowMediaWrites     bool `json:"allow_media_writes"`
}

type Job struct {
	SchemaVersion  string         `json:"schema_version"`
	JobID          string         `json:"job_id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	ActionType     string         `json:"action_type"`
	Status         string         `json:"status"`
	ApprovalStatus string         `json:"approval_status"`
	Policy         JobPolicy      `json:"policy"`
	Input          map[string]any `json:"input"`
	Output         map[string]any `json:"output,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
	Errors         []string       `json:"errors,omitempty"`
	NextCommands   []string       `json:"next_commands"`
}

// ---- options structs ----

type JobCreateOptions struct {
	// common
	ActionType           string
	AllowOverwrite       bool
	AllowProviderCalls   bool
	AllowExternalNetwork bool
	JSON                 bool

	// make-specific
	Goal   string
	Preset string

	// revise_make-specific
	MakeID     string
	Request    string
	Reassemble bool

	// validate_creative_assemble-specific
	PlanID string
}

type JobsOptions struct {
	JSON   bool
	Filter string // filter by status
	Limit  int
}

type JobInspectOptions struct{ JSON bool }

type JobEventsOptions struct {
	JSON  bool
	Limit int
}

type JobApproveOptions struct{ JSON bool }

type JobRejectOptions struct {
	Reason string
	JSON   bool
}

type JobCancelOptions struct {
	Reason string
	JSON   bool
}

type JobResultOptions struct{ JSON bool }

type JobValidateOptions struct{ JSON bool }

// ---- helpers ----

func newJobID(actionType string) string {
	slug := strings.ReplaceAll(actionType, "_", "-")
	return time.Now().UTC().Format("20060102T150405Z") + "-" + slug
}

func jobDir(jobID string) string {
	return filepath.Join(jobsRoot, jobID)
}

func jobFilePath(jobID string) string {
	return filepath.Join(jobsRoot, jobID, "job.json")
}

func jobEventsPath(jobID string) string {
	return filepath.Join(jobsRoot, jobID, "events.jsonl")
}

func readJob(jobID string) (*Job, error) {
	data, err := os.ReadFile(jobFilePath(jobID))
	if err != nil {
		return nil, fmt.Errorf("job %q not found: %w", jobID, err)
	}
	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("job.json is malformed: %w", err)
	}
	return &j, nil
}

func writeJob(j *Job) error {
	j.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(jobDir(j.JobID), 0o755); err != nil {
		return fmt.Errorf("create job dir: %w", err)
	}
	return writeJSONFile(jobFilePath(j.JobID), j)
}

func appendJobEvent(jobID string, eventType string, details map[string]any) {
	log, err := events.Open(jobEventsPath(jobID))
	if err != nil {
		return
	}
	defer log.Close()
	_ = log.Write(eventType, details)
}

func readJobEvents(jobID string) ([]events.Event, error) {
	path := jobEventsPath(jobID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read events: %w", err)
	}
	var out []events.Event
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e events.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func inputString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	v, _ := input[key].(string)
	return v
}

func inputBool(input map[string]any, key string) bool {
	if input == nil {
		return false
	}
	v, _ := input[key].(bool)
	return v
}

// ---- job-create ----

func JobCreate(stdout io.Writer, opts JobCreateOptions) error {
	if opts.ActionType == "" {
		return fmt.Errorf("--type is required (make|revise_make|validate_creative_assemble)")
	}

	switch opts.ActionType {
	case JobActionMake, JobActionReviseMake, JobActionValidateCreativeAssemble:
		// valid
	default:
		return fmt.Errorf("unknown action type %q; supported: make, revise_make, validate_creative_assemble", opts.ActionType)
	}

	// Validate type-specific required inputs
	switch opts.ActionType {
	case JobActionMake:
		if strings.TrimSpace(opts.Goal) == "" {
			return fmt.Errorf("--goal is required for action type 'make'")
		}
	case JobActionReviseMake:
		if strings.TrimSpace(opts.MakeID) == "" {
			return fmt.Errorf("--make-id is required for action type 'revise_make'")
		}
		if strings.TrimSpace(opts.Request) == "" {
			return fmt.Errorf("--request is required for action type 'revise_make'")
		}
	case JobActionValidateCreativeAssemble:
		if strings.TrimSpace(opts.PlanID) == "" {
			return fmt.Errorf("--plan-id is required for action type 'validate_creative_assemble'")
		}
	}

	jobID := newJobID(opts.ActionType)
	now := time.Now().UTC()

	approvalStatus := JobApprovalPending
	if opts.ActionType == JobActionValidateCreativeAssemble {
		approvalStatus = JobApprovalNotRequired
	}

	policy := JobPolicy{
		AllowOverwrite:       opts.AllowOverwrite,
		AllowProviderCalls:   opts.AllowProviderCalls,
		AllowExternalNetwork: opts.AllowExternalNetwork,
		AllowMediaWrites:     true, // always true; v1 does not expose --no-media-writes
	}

	input := buildJobInput(opts)

	j := &Job{
		SchemaVersion:  jobSchemaVersion,
		JobID:          jobID,
		CreatedAt:      now,
		UpdatedAt:      now,
		ActionType:     opts.ActionType,
		Status:         JobStatusPending,
		ApprovalStatus: approvalStatus,
		Policy:         policy,
		Input:          input,
		NextCommands:   []string{},
	}

	if err := os.MkdirAll(jobDir(jobID), 0o755); err != nil {
		return fmt.Errorf("create job dir: %w", err)
	}
	if err := writeJSONFile(jobFilePath(jobID), j); err != nil {
		return fmt.Errorf("write job.json: %w", err)
	}

	appendJobEvent(jobID, JobEventCreated, map[string]any{
		"job_id":          jobID,
		"action_type":     opts.ActionType,
		"approval_status": approvalStatus,
	})

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "Job created: %s\n", jobID)
	fmt.Fprintf(stdout, "  action_type:     %s\n", j.ActionType)
	fmt.Fprintf(stdout, "  status:          %s\n", j.Status)
	fmt.Fprintf(stdout, "  approval_status: %s\n", j.ApprovalStatus)
	printJobNextSteps(stdout, j)
	return nil
}

func buildJobInput(opts JobCreateOptions) map[string]any {
	switch opts.ActionType {
	case JobActionMake:
		input := map[string]any{
			"goal": opts.Goal,
		}
		if opts.Preset != "" {
			input["preset"] = opts.Preset
		}
		return input
	case JobActionReviseMake:
		input := map[string]any{
			"make_id": opts.MakeID,
			"request": opts.Request,
		}
		if opts.Reassemble {
			input["reassemble"] = true
		}
		return input
	case JobActionValidateCreativeAssemble:
		return map[string]any{
			"plan_id": opts.PlanID,
		}
	}
	return map[string]any{}
}

func printJobNextSteps(stdout io.Writer, j *Job) {
	fmt.Fprintln(stdout)
	if j.ApprovalStatus == JobApprovalPending {
		fmt.Fprintf(stdout, "  To approve:  byom-video job-approve %s\n", j.JobID)
		fmt.Fprintf(stdout, "  To reject:   byom-video job-reject %s\n", j.JobID)
	}
	if j.ApprovalStatus == JobApprovalApproved || j.ApprovalStatus == JobApprovalNotRequired {
		fmt.Fprintf(stdout, "  To run:      byom-video job-run %s\n", j.JobID)
	}
	fmt.Fprintf(stdout, "  To inspect:  byom-video job-inspect %s\n", j.JobID)
}

// ---- jobs (list) ----

func Jobs(stdout io.Writer, opts JobsOptions) error {
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "No jobs found.")
			return nil
		}
		return fmt.Errorf("read jobs dir: %w", err)
	}

	type jobRow struct {
		JobID          string    `json:"job_id"`
		ActionType     string    `json:"action_type"`
		Status         string    `json:"status"`
		ApprovalStatus string    `json:"approval_status"`
		CreatedAt      time.Time `json:"created_at"`
	}

	var rows []jobRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		j, err := readJob(e.Name())
		if err != nil {
			continue
		}
		if opts.Filter != "" && j.Status != opts.Filter {
			continue
		}
		rows = append(rows, jobRow{
			JobID:          j.JobID,
			ActionType:     j.ActionType,
			Status:         j.Status,
			ApprovalStatus: j.ApprovalStatus,
			CreatedAt:      j.CreatedAt,
		})
	}

	// newest first
	sort.Slice(rows, func(i, k int) bool {
		return rows[i].CreatedAt.After(rows[k].CreatedAt)
	})

	if opts.Limit > 0 && len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No jobs found.")
		return nil
	}

	fmt.Fprintf(stdout, "%-40s %-28s %-12s %-14s\n", "JOB_ID", "ACTION_TYPE", "STATUS", "APPROVAL")
	fmt.Fprintf(stdout, "%-40s %-28s %-12s %-14s\n",
		strings.Repeat("-", 39), strings.Repeat("-", 27), strings.Repeat("-", 11), strings.Repeat("-", 13))
	for _, r := range rows {
		fmt.Fprintf(stdout, "%-40s %-28s %-12s %-14s\n", r.JobID, r.ActionType, r.Status, r.ApprovalStatus)
	}
	return nil
}

// ---- job-inspect ----

func JobInspect(jobID string, stdout io.Writer, opts JobInspectOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "Job: %s\n", j.JobID)
	fmt.Fprintf(stdout, "  action_type:     %s\n", j.ActionType)
	fmt.Fprintf(stdout, "  status:          %s\n", j.Status)
	fmt.Fprintf(stdout, "  approval_status: %s\n", j.ApprovalStatus)
	fmt.Fprintf(stdout, "  created_at:      %s\n", j.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "  updated_at:      %s\n", j.UpdatedAt.Format(time.RFC3339))

	fmt.Fprintln(stdout, "  policy:")
	fmt.Fprintf(stdout, "    allow_overwrite:        %v\n", j.Policy.AllowOverwrite)
	fmt.Fprintf(stdout, "    allow_provider_calls:   %v\n", j.Policy.AllowProviderCalls)
	fmt.Fprintf(stdout, "    allow_external_network: %v\n", j.Policy.AllowExternalNetwork)
	fmt.Fprintf(stdout, "    allow_media_writes:     %v\n", j.Policy.AllowMediaWrites)

	if len(j.Input) > 0 {
		fmt.Fprintln(stdout, "  input:")
		keys := sortedKeys(j.Input)
		for _, k := range keys {
			fmt.Fprintf(stdout, "    %s: %v\n", k, j.Input[k])
		}
	}

	if len(j.Output) > 0 {
		fmt.Fprintln(stdout, "  output:")
		keys := sortedKeys(j.Output)
		for _, k := range keys {
			fmt.Fprintf(stdout, "    %s: %v\n", k, j.Output[k])
		}
	}

	if len(j.Warnings) > 0 {
		fmt.Fprintln(stdout, "  warnings:")
		for _, w := range j.Warnings {
			fmt.Fprintf(stdout, "    - %s\n", w)
		}
	}

	if len(j.Errors) > 0 {
		fmt.Fprintln(stdout, "  errors:")
		for _, e := range j.Errors {
			fmt.Fprintf(stdout, "    - %s\n", e)
		}
	}

	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---- job-events ----

func JobEvents(jobID string, stdout io.Writer, opts JobEventsOptions) error {
	// Verify job exists
	if _, err := readJob(jobID); err != nil {
		return err
	}

	evs, err := readJobEvents(jobID)
	if err != nil {
		return err
	}

	if opts.Limit > 0 && len(evs) > opts.Limit {
		evs = evs[len(evs)-opts.Limit:]
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(evs, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	if len(evs) == 0 {
		fmt.Fprintln(stdout, "No events found.")
		return nil
	}

	for _, e := range evs {
		ts := e.Time.Format("2006-01-02T15:04:05Z")
		detailStr := ""
		if e.Details != nil {
			if b, err := json.Marshal(e.Details); err == nil {
				detailStr = " " + string(b)
			}
		}
		fmt.Fprintf(stdout, "%s  %s%s\n", ts, e.Type, detailStr)
	}
	return nil
}

// ---- job-approve ----

func JobApprove(jobID string, stdout io.Writer, opts JobApproveOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	if j.ApprovalStatus == JobApprovalNotRequired {
		return fmt.Errorf("job %q does not require approval (approval_status: not_required)", jobID)
	}
	if j.ApprovalStatus == JobApprovalApproved {
		return fmt.Errorf("job %q is already approved", jobID)
	}
	if j.ApprovalStatus == JobApprovalRejected {
		return fmt.Errorf("job %q has already been rejected", jobID)
	}
	if j.Status == JobStatusCancelled {
		return fmt.Errorf("job %q is cancelled and cannot be approved", jobID)
	}

	j.ApprovalStatus = JobApprovalApproved
	if err := writeJob(j); err != nil {
		return fmt.Errorf("write job: %w", err)
	}

	appendJobEvent(jobID, JobEventApproved, map[string]any{"job_id": jobID})

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "Job approved: %s\n", jobID)
	fmt.Fprintf(stdout, "  To run: byom-video job-run %s\n", jobID)
	return nil
}

// ---- job-reject ----

func JobReject(jobID string, stdout io.Writer, opts JobRejectOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	if j.ApprovalStatus == JobApprovalNotRequired {
		return fmt.Errorf("job %q does not require approval (approval_status: not_required)", jobID)
	}
	if j.ApprovalStatus == JobApprovalRejected {
		return fmt.Errorf("job %q is already rejected", jobID)
	}
	if j.Status == JobStatusCancelled {
		return fmt.Errorf("job %q is cancelled and cannot be rejected", jobID)
	}
	if j.Status == JobStatusCompleted || j.Status == JobStatusRunning {
		return fmt.Errorf("job %q is %s and cannot be rejected", jobID, j.Status)
	}

	j.ApprovalStatus = JobApprovalRejected
	if opts.Reason != "" {
		j.Warnings = append(j.Warnings, "rejected: "+opts.Reason)
	}
	if err := writeJob(j); err != nil {
		return fmt.Errorf("write job: %w", err)
	}

	appendJobEvent(jobID, JobEventRejected, map[string]any{"job_id": jobID, "reason": opts.Reason})

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "Job rejected: %s\n", jobID)
	return nil
}

// ---- job-cancel ----

func JobCancel(jobID string, stdout io.Writer, opts JobCancelOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	if j.Status == JobStatusCancelled {
		return fmt.Errorf("job %q is already cancelled", jobID)
	}
	if j.Status == JobStatusCompleted || j.Status == JobStatusFailed {
		return fmt.Errorf("job %q is %s and cannot be cancelled", jobID, j.Status)
	}

	j.Status = JobStatusCancelled
	if opts.Reason != "" {
		j.Warnings = append(j.Warnings, "cancelled: "+opts.Reason)
	}
	if err := writeJob(j); err != nil {
		return fmt.Errorf("write job: %w", err)
	}

	appendJobEvent(jobID, JobEventCancelled, map[string]any{"job_id": jobID, "reason": opts.Reason})

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "Job cancelled: %s\n", jobID)
	return nil
}

// ---- job-result ----

func JobResult(jobID string, stdout io.Writer, opts JobResultOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(j, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		return nil
	}

	fmt.Fprintf(stdout, "# Job Result: %s\n\n", j.JobID)
	fmt.Fprintf(stdout, "- **Action type:** %s\n", j.ActionType)
	fmt.Fprintf(stdout, "- **Status:** %s\n", j.Status)
	fmt.Fprintf(stdout, "- **Approval:** %s\n", j.ApprovalStatus)
	fmt.Fprintf(stdout, "- **Created:** %s\n", j.CreatedAt.Format(time.RFC3339))
	if planID := inputString(j.Input, "agent_plan_id"); planID != "" {
		fmt.Fprintf(stdout, "- **Source agent plan:** %s\n", planID)
		if actionID := inputString(j.Input, "agent_action_id"); actionID != "" {
			fmt.Fprintf(stdout, "- **Source agent action:** %s\n", actionID)
		}
		fmt.Fprintf(stdout, "- **Inspect source:** `byom-video inspect-agent-plan %s`\n", planID)
	}

	if len(j.Output) > 0 {
		fmt.Fprintln(stdout, "\n## Output")
		keys := sortedKeys(j.Output)
		for _, k := range keys {
			fmt.Fprintf(stdout, "- **%s:** %v\n", k, j.Output[k])
		}
	}

	if len(j.Warnings) > 0 {
		fmt.Fprintln(stdout, "\n## Warnings")
		for _, w := range j.Warnings {
			fmt.Fprintf(stdout, "- %s\n", w)
		}
	}

	if len(j.Errors) > 0 {
		fmt.Fprintln(stdout, "\n## Errors")
		for _, e := range j.Errors {
			fmt.Fprintf(stdout, "- %s\n", e)
		}
	}

	if len(j.NextCommands) > 0 {
		fmt.Fprintln(stdout, "\n## Next Steps")
		for _, cmd := range j.NextCommands {
			fmt.Fprintf(stdout, "- `byom-video %s`\n", cmd)
		}
	}

	return nil
}

// ---- job-validate ----

func JobValidate(jobID string, stdout io.Writer, opts JobValidateOptions) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	type validationResult struct {
		JobID    string   `json:"job_id"`
		Valid    bool     `json:"valid"`
		Warnings []string `json:"warnings,omitempty"`
		Errors   []string `json:"errors,omitempty"`
	}

	var errs []string
	var warns []string

	if j.SchemaVersion != jobSchemaVersion {
		errs = append(errs, fmt.Sprintf("unexpected schema_version: %q (expected %q)", j.SchemaVersion, jobSchemaVersion))
	}
	if j.JobID == "" {
		errs = append(errs, "job_id is empty")
	}
	if j.ActionType == "" {
		errs = append(errs, "action_type is empty")
	}
	switch j.ActionType {
	case JobActionMake, JobActionReviseMake, JobActionValidateCreativeAssemble:
	default:
		errs = append(errs, fmt.Sprintf("unknown action_type: %q", j.ActionType))
	}
	switch j.Status {
	case JobStatusPending, JobStatusRunning, JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
	default:
		errs = append(errs, fmt.Sprintf("unknown status: %q", j.Status))
	}
	switch j.ApprovalStatus {
	case JobApprovalPending, JobApprovalApproved, JobApprovalRejected, JobApprovalNotRequired:
	default:
		errs = append(errs, fmt.Sprintf("unknown approval_status: %q", j.ApprovalStatus))
	}

	// Validate events.jsonl
	eventsPath := jobEventsPath(jobID)
	if data, err := os.ReadFile(eventsPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var e map[string]any
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				warns = append(warns, fmt.Sprintf("events.jsonl line %d is malformed", lineNum))
			}
		}
	}

	result := validationResult{
		JobID:    jobID,
		Valid:    len(errs) == 0,
		Warnings: warns,
		Errors:   errs,
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintf(stdout, "%s\n", data)
		if !result.Valid {
			return fmt.Errorf("job %q failed validation", jobID)
		}
		return nil
	}

	if result.Valid {
		fmt.Fprintf(stdout, "job %q: valid\n", jobID)
	} else {
		fmt.Fprintf(stdout, "job %q: INVALID\n", jobID)
		for _, e := range errs {
			fmt.Fprintf(stdout, "  error: %s\n", e)
		}
	}
	for _, w := range warns {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}

	if !result.Valid {
		return fmt.Errorf("job %q failed validation", jobID)
	}
	return nil
}

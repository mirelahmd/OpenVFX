package commands

import (
	"fmt"
	"io"
)

// ---- options ----

type JobRunOptions struct {
	Yes                bool // bypass approval gate
	JSON               bool
	AllowProviderCalls bool
	AllowOverwrite     bool
}

// ---- injectable deps ----

type jobRunDeps struct {
	runMake             func(inputPath string, stdout io.Writer, opts MakeOptions) error
	runReviseMake       func(makeID string, stdout io.Writer, opts ReviseMakeOptions) error
	runValidateAssemble func(planID string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error
}

var defaultJobRunDeps = jobRunDeps{
	runMake:             Make,
	runReviseMake:       ReviseMake,
	runValidateAssemble: ValidateCreativeAssemble,
}

// ---- job-run ----

func JobRun(jobID string, stdout io.Writer, opts JobRunOptions) error {
	return jobRunWithDeps(jobID, stdout, opts, defaultJobRunDeps)
}

func jobRunWithDeps(jobID string, stdout io.Writer, opts JobRunOptions, deps jobRunDeps) error {
	j, err := readJob(jobID)
	if err != nil {
		return err
	}

	// Guard: terminal-state jobs cannot be re-run
	switch j.Status {
	case JobStatusCancelled:
		return fmt.Errorf("job %q is cancelled and cannot be run", jobID)
	case JobStatusRunning:
		return fmt.Errorf("job %q is already running", jobID)
	case JobStatusCompleted:
		return fmt.Errorf("job %q is already completed", jobID)
	case JobStatusFailed:
		return fmt.Errorf("job %q has already failed; create a new job to retry", jobID)
	}

	// Approval gate
	if j.ApprovalStatus == JobApprovalRejected {
		return fmt.Errorf("job %q has been rejected and cannot be run", jobID)
	}
	if j.ApprovalStatus == JobApprovalPending && !opts.Yes {
		appendJobEvent(jobID, JobEventPolicyBlocked, map[string]any{
			"job_id": jobID,
			"reason": "approval_status is pending",
		})
		return fmt.Errorf("job %q requires approval before running\n  To approve: byom-video job-approve %s\n  To bypass:  byom-video job-run %s --yes",
			jobID, jobID, jobID)
	}

	// Mark running
	j.Status = JobStatusRunning
	if err := writeJob(j); err != nil {
		return fmt.Errorf("write job: %w", err)
	}
	appendJobEvent(jobID, JobEventRunStarted, map[string]any{"job_id": jobID})
	appendJobEvent(jobID, JobEventActionStarted, map[string]any{
		"job_id":      jobID,
		"action_type": j.ActionType,
	})

	// Dispatch to action handler
	var output map[string]any
	var runErr error

	switch j.ActionType {
	case JobActionMake:
		output, runErr = handleJobMake(j, stdout, opts, deps)
	case JobActionReviseMake:
		output, runErr = handleJobReviseMake(j, stdout, opts, deps)
	case JobActionValidateCreativeAssemble:
		output, runErr = handleJobValidateAssemble(j, stdout, opts, deps)
	default:
		runErr = fmt.Errorf("unsupported action type: %q", j.ActionType)
	}

	// Re-read to pick up any writes done by the handler
	if j2, err2 := readJob(jobID); err2 == nil {
		j = j2
	}

	if runErr != nil {
		appendJobEvent(jobID, JobEventActionFailed, map[string]any{
			"job_id":      jobID,
			"action_type": j.ActionType,
			"error":       runErr.Error(),
		})
		appendJobEvent(jobID, JobEventRunFailed, map[string]any{
			"job_id": jobID,
			"error":  runErr.Error(),
		})
		j.Status = JobStatusFailed
		j.Errors = append(j.Errors, runErr.Error())
		_ = writeJob(j)
		return runErr
	}

	appendJobEvent(jobID, JobEventActionCompleted, map[string]any{
		"job_id":      jobID,
		"action_type": j.ActionType,
	})
	appendJobEvent(jobID, JobEventRunCompleted, map[string]any{"job_id": jobID})

	j.Status = JobStatusCompleted
	j.Output = output
	j.NextCommands = buildJobNextCommands(j)
	_ = writeJob(j)

	if opts.JSON {
		fmt.Fprintf(stdout, `{"job_id":%q,"status":"completed"}`, jobID)
		fmt.Fprintln(stdout)
		return nil
	}

	fmt.Fprintf(stdout, "Job completed: %s\n", jobID)
	fmt.Fprintf(stdout, "  To view result: byom-video job-result %s\n", jobID)
	return nil
}

// ---- action handlers ----

func handleJobMake(j *Job, stdout io.Writer, runOpts JobRunOptions, deps jobRunDeps) (map[string]any, error) {
	goal := inputString(j.Input, "goal")
	if goal == "" {
		return nil, fmt.Errorf("job input missing 'goal'")
	}
	inputPath := inputString(j.Input, "input_path")
	preset := inputString(j.Input, "preset")

	opts := MakeOptions{
		Goal:                           goal,
		Preset:                         preset,
		Yes:                            true,
		AllowMissingCaptions:           true,
		AllowMissingVoiceover:          true,
		AllowMissingGeneratedVoiceover: true,
		Overwrite:                      j.Policy.AllowOverwrite || runOpts.AllowOverwrite,
	}

	if err := deps.runMake(inputPath, stdout, opts); err != nil {
		return nil, err
	}

	return map[string]any{"status": "completed", "goal": goal}, nil
}

func handleJobReviseMake(j *Job, stdout io.Writer, runOpts JobRunOptions, deps jobRunDeps) (map[string]any, error) {
	makeID := inputString(j.Input, "make_id")
	request := inputString(j.Input, "request")
	if makeID == "" {
		return nil, fmt.Errorf("job input missing 'make_id'")
	}
	if request == "" {
		return nil, fmt.Errorf("job input missing 'request'")
	}

	opts := ReviseMakeOptions{
		Request:            request,
		Yes:                true,
		Overwrite:          j.Policy.AllowOverwrite || runOpts.AllowOverwrite,
		AllowProviderCalls: j.Policy.AllowProviderCalls || runOpts.AllowProviderCalls,
		Reassemble:         inputBool(j.Input, "reassemble"),
	}

	if err := deps.runReviseMake(makeID, stdout, opts); err != nil {
		return nil, err
	}

	return map[string]any{"make_id": makeID, "status": "completed"}, nil
}

func handleJobValidateAssemble(j *Job, stdout io.Writer, _ JobRunOptions, deps jobRunDeps) (map[string]any, error) {
	planID := inputString(j.Input, "plan_id")
	if planID == "" {
		return nil, fmt.Errorf("job input missing 'plan_id'")
	}

	if err := deps.runValidateAssemble(planID, stdout, ValidateCreativeAssembleOptions{}); err != nil {
		return nil, err
	}

	return map[string]any{"plan_id": planID, "status": "completed"}, nil
}

func buildJobNextCommands(j *Job) []string {
	var cmds []string
	switch j.ActionType {
	case JobActionMake:
		cmds = append(cmds, "makes")
	case JobActionReviseMake:
		if makeID := inputString(j.Input, "make_id"); makeID != "" {
			cmds = append(cmds, fmt.Sprintf("make-result %s", makeID))
		}
	case JobActionValidateCreativeAssemble:
		if planID := inputString(j.Input, "plan_id"); planID != "" {
			cmds = append(cmds, fmt.Sprintf("inspect-creative-plan %s", planID))
		}
	}
	return cmds
}

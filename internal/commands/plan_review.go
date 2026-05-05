package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"byom-video/internal/agent"
)

type ReviewPlanOptions struct {
	JSON          bool
	WriteArtifact bool
}
type ExecutePlanOptions struct {
	Yes          bool
	DryRun       bool
	WithExport   bool
	WithValidate bool
}
type DiffPlanOptions struct {
	JSON          bool
	WriteArtifact bool
}

type PlanReview struct {
	PlanID             string         `json:"plan_id"`
	Goal               string         `json:"goal"`
	InputPath          string         `json:"input_path"`
	TargetType         string         `json:"target_type"`
	Preset             string         `json:"preset"`
	Safety             agent.Safety   `json:"safety"`
	ValidationStatus   string         `json:"validation_status"`
	ValidationErrors   []string       `json:"validation_errors,omitempty"`
	ApprovalStatus     string         `json:"approval_status"`
	ReviewStatus       string         `json:"review_status"`
	ExportsIncluded    bool           `json:"exports_included"`
	ValidationIncluded bool           `json:"validation_included"`
	WatchMode          string         `json:"watch_mode,omitempty"`
	Actions            []agent.Action `json:"actions"`
}

type PlanDiff struct {
	PlanA       string       `json:"plan_a"`
	PlanB       string       `json:"plan_b"`
	Differences []Difference `json:"differences"`
}

type Difference struct {
	Field string `json:"field"`
	A     any    `json:"a"`
	B     any    `json:"b"`
}

func ReviewPlan(planID string, stdout io.Writer, opts ReviewPlanOptions) error {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	review := BuildPlanReview(plan)
	plan.ReviewStatus = "reviewed"
	if plan.ValidationStatus == "" || plan.ValidationStatus == "failed" {
		errs := agent.ValidatePlan(plan)
		if len(errs) > 0 {
			plan.ValidationStatus = "failed"
			plan.ValidationErrors = errs
		} else {
			plan.ValidationStatus = "completed"
			plan.ValidationErrors = nil
		}
		review.ValidationStatus = plan.ValidationStatus
		review.ValidationErrors = plan.ValidationErrors
	}
	_ = agent.WritePlan(plan)
	wroteReviewPath := ""
	if opts.WriteArtifact {
		path, err := writeReviewArtifact(plan.PlanID, review)
		if err != nil {
			return err
		}
		wroteReviewPath = path
		log, err := agent.OpenActionLog(plan.PlanID)
		if err == nil {
			_ = log.Write("PLAN_REVIEW_ARTIFACT_WRITTEN", map[string]any{"path": path})
			_ = log.Close()
		}
	}
	if opts.JSON {
		data, err := json.MarshalIndent(review, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printReview(stdout, review)
	if wroteReviewPath != "" {
		fmt.Fprintf(stdout, "  artifact: %s\n", wroteReviewPath)
	}
	return nil
}

func ApprovePlan(planID string, stdout io.Writer) error {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	if errs := agent.ValidatePlan(plan); len(errs) > 0 {
		plan.Status = "failed"
		plan.ValidationStatus = "failed"
		plan.ValidationErrors = errs
		_ = agent.WritePlan(plan)
		return fmt.Errorf("plan validation failed: %s", strings.Join(errs, "; "))
	}
	now := time.Now().UTC()
	_, _ = agent.CreateSnapshot(plan, "before approve-plan", now)
	plan.ValidationStatus = "completed"
	plan.ValidationErrors = nil
	plan.ApprovalStatus = "approved"
	plan.ApprovedAt = &now
	plan.ApprovalMode = "manual"
	if err := agent.WritePlan(plan); err != nil {
		return err
	}
	log, err := agent.OpenActionLog(plan.PlanID)
	if err == nil {
		_ = log.Write("PLAN_APPROVED", map[string]any{"approval_mode": "manual"})
		_ = log.Close()
	}
	fmt.Fprintf(stdout, "Plan approved: %s\n", plan.PlanID)
	return nil
}

func ExecuteSavedPlan(planID string, stdout io.Writer, opts ExecutePlanOptions) error {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	if opts.WithExport {
		return fmt.Errorf("--with-export cannot mutate a saved plan; create a new plan with --with-export")
	}
	if opts.WithValidate {
		return fmt.Errorf("--with-validate cannot mutate a saved plan; create a new plan with --with-validate")
	}
	if opts.DryRun {
		fmt.Fprintln(stdout, "Execute plan dry run")
		printPlan(stdout, plan)
		return nil
	}
	if plan.ApprovalStatus != "approved" {
		if !opts.Yes {
			return fmt.Errorf("plan requires approval; run `byom-video approve-plan %s` or pass --yes", plan.PlanID)
		}
		now := time.Now().UTC()
		_, _ = agent.CreateSnapshot(plan, "before execute-plan --yes", now)
		plan.ApprovalStatus = "approved"
		plan.ApprovedAt = &now
		plan.ApprovalMode = "yes_flag"
		if err := agent.WritePlan(plan); err != nil {
			return err
		}
		log, err := agent.OpenActionLog(plan.PlanID)
		if err == nil {
			_ = log.Write("PLAN_APPROVED", map[string]any{"approval_mode": "yes_flag"})
			_ = log.Close()
		}
	}
	return ExecutePlan(plan.PlanID, stdout)
}

func DiffPlan(a string, b string, stdout io.Writer, opts DiffPlanOptions) error {
	planA, err := agent.ReadPlan(a)
	if err != nil {
		return err
	}
	planB, err := agent.ReadPlan(b)
	if err != nil {
		return err
	}
	diff := BuildPlanDiff(planA, planB)
	wroteDiffPath := ""
	if opts.WriteArtifact {
		path, err := writeDiffArtifact(a, fmt.Sprintf("diff_%s_vs_%s.md", a, b), diff)
		if err != nil {
			return err
		}
		wroteDiffPath = path
		log, err := agent.OpenActionLog(a)
		if err == nil {
			_ = log.Write("PLAN_DIFF_ARTIFACT_WRITTEN", map[string]any{"path": path, "plan_b": b})
			_ = log.Close()
		}
	}
	if opts.JSON {
		data, err := json.MarshalIndent(diff, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintf(stdout, "Plan diff: %s -> %s\n", a, b)
	if len(diff.Differences) == 0 {
		fmt.Fprintln(stdout, "  no differences")
	} else {
		for _, d := range diff.Differences {
			fmt.Fprintf(stdout, "  - %s: %v -> %v\n", d.Field, d.A, d.B)
		}
	}
	if wroteDiffPath != "" {
		fmt.Fprintf(stdout, "  artifact: %s\n", wroteDiffPath)
	}
	return nil
}

func BuildPlanReview(plan agent.Plan) PlanReview {
	review := PlanReview{
		PlanID:           plan.PlanID,
		Goal:             plan.Goal,
		InputPath:        plan.InputPath,
		TargetType:       plan.TargetType,
		Preset:           plan.Preset,
		Safety:           plan.Safety,
		ValidationStatus: plan.ValidationStatus,
		ValidationErrors: plan.ValidationErrors,
		ApprovalStatus:   plan.ApprovalStatus,
		ReviewStatus:     plan.ReviewStatus,
		Actions:          plan.Actions,
	}
	for _, action := range plan.Actions {
		switch action.Type {
		case "export_run":
			review.ExportsIncluded = true
		case "validate_run":
			review.ValidationIncluded = true
		case "watch_pipeline":
			if boolActionOption(action.Options, "once") {
				review.WatchMode = "once"
			} else {
				review.WatchMode = "long-running"
			}
		}
	}
	if review.ValidationStatus == "" {
		review.ValidationStatus = "not_run"
	}
	return review
}

func BuildPlanDiff(a agent.Plan, b agent.Plan) PlanDiff {
	diff := PlanDiff{PlanA: a.PlanID, PlanB: b.PlanID}
	add := func(field string, av any, bv any) {
		if !reflect.DeepEqual(av, bv) {
			diff.Differences = append(diff.Differences, Difference{Field: field, A: av, B: bv})
		}
	}
	add("goal", a.Goal, b.Goal)
	add("input_path", a.InputPath, b.InputPath)
	add("target_type", a.TargetType, b.TargetType)
	add("preset", a.Preset, b.Preset)
	add("safety.exports_require_explicit_execution", a.Safety.ExportsRequireExplicitExecution, b.Safety.ExportsRequireExplicitExecution)
	add("safety.no_input_files_modified", a.Safety.NoInputFilesModified, b.Safety.NoInputFilesModified)
	add("action_count", len(a.Actions), len(b.Actions))
	max := len(a.Actions)
	if len(b.Actions) > max {
		max = len(b.Actions)
	}
	for i := 0; i < max; i++ {
		if i >= len(a.Actions) {
			add(fmt.Sprintf("actions[%d]", i), nil, b.Actions[i])
			continue
		}
		if i >= len(b.Actions) {
			add(fmt.Sprintf("actions[%d]", i), a.Actions[i], nil)
			continue
		}
		add(fmt.Sprintf("actions[%d].type", i), a.Actions[i].Type, b.Actions[i].Type)
		add(fmt.Sprintf("actions[%d].command_preview", i), a.Actions[i].CommandPreview, b.Actions[i].CommandPreview)
		add(fmt.Sprintf("actions[%d].options", i), a.Actions[i].Options, b.Actions[i].Options)
	}
	return diff
}

func printReview(stdout io.Writer, review PlanReview) {
	fmt.Fprintln(stdout, "Plan review")
	fmt.Fprintf(stdout, "  plan id:       %s\n", review.PlanID)
	fmt.Fprintf(stdout, "  goal:          %s\n", review.Goal)
	fmt.Fprintf(stdout, "  input:         %s\n", review.InputPath)
	fmt.Fprintf(stdout, "  target:        %s\n", review.TargetType)
	fmt.Fprintf(stdout, "  preset:        %s\n", review.Preset)
	fmt.Fprintf(stdout, "  approval:      %s\n", review.ApprovalStatus)
	fmt.Fprintf(stdout, "  validation:    %s\n", review.ValidationStatus)
	fmt.Fprintf(stdout, "  export:        %t\n", review.ExportsIncluded)
	fmt.Fprintf(stdout, "  validate:      %t\n", review.ValidationIncluded)
	if review.WatchMode != "" {
		fmt.Fprintf(stdout, "  watch mode:    %s\n", review.WatchMode)
	}
	fmt.Fprintf(stdout, "  safety:        exports_explicit=%t no_input_mods=%t\n", review.Safety.ExportsRequireExplicitExecution, review.Safety.NoInputFilesModified)
	if len(review.ValidationErrors) > 0 {
		fmt.Fprintln(stdout, "  validation errors:")
		for _, err := range review.ValidationErrors {
			fmt.Fprintf(stdout, "    - %s\n", err)
		}
	}
	fmt.Fprintln(stdout, "  actions:")
	for _, action := range review.Actions {
		fmt.Fprintf(stdout, "    - %s %s %s\n", action.ID, action.Type, action.Status)
		if action.CommandPreview != "" {
			fmt.Fprintf(stdout, "      command: %s\n", action.CommandPreview)
		}
	}
}

func writeReviewArtifact(planID string, review PlanReview) (string, error) {
	path := filepath.Join(agent.PlanDir(planID), "review.md")
	var builder strings.Builder
	builder.WriteString("# Plan Review\n\n")
	builder.WriteString(fmt.Sprintf("- generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- plan_id: %s\n", review.PlanID))
	builder.WriteString(fmt.Sprintf("- goal: %s\n", review.Goal))
	builder.WriteString(fmt.Sprintf("- input_path: %s\n", review.InputPath))
	builder.WriteString(fmt.Sprintf("- target_type: %s\n", review.TargetType))
	builder.WriteString(fmt.Sprintf("- preset: %s\n", review.Preset))
	builder.WriteString(fmt.Sprintf("- approval_status: %s\n", review.ApprovalStatus))
	builder.WriteString(fmt.Sprintf("- validation_status: %s\n", review.ValidationStatus))
	builder.WriteString(fmt.Sprintf("- exports_included: %t\n", review.ExportsIncluded))
	builder.WriteString(fmt.Sprintf("- validation_included: %t\n", review.ValidationIncluded))
	if review.WatchMode != "" {
		builder.WriteString(fmt.Sprintf("- watch_mode: %s\n", review.WatchMode))
	}
	builder.WriteString(fmt.Sprintf("- safety.exports_require_explicit_execution: %t\n", review.Safety.ExportsRequireExplicitExecution))
	builder.WriteString(fmt.Sprintf("- safety.no_input_files_modified: %t\n", review.Safety.NoInputFilesModified))
	if len(review.ValidationErrors) > 0 {
		builder.WriteString("\n## Validation Errors\n\n")
		for _, err := range review.ValidationErrors {
			builder.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}
	builder.WriteString("\n## Actions\n\n")
	for _, action := range review.Actions {
		builder.WriteString(fmt.Sprintf("- %s `%s` %s\n", action.ID, action.Type, action.Status))
		if action.CommandPreview != "" {
			builder.WriteString(fmt.Sprintf("  - command: `%s`\n", action.CommandPreview))
		}
		if action.Error != "" {
			builder.WriteString(fmt.Sprintf("  - error: %s\n", action.Error))
		}
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write review artifact: %w", err)
	}
	return path, nil
}

func writeDiffArtifact(planID string, filename string, diff PlanDiff) (string, error) {
	dir := filepath.Join(agent.PlanDir(planID), "diffs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create diffs directory: %w", err)
	}
	path := filepath.Join(dir, filename)
	var builder strings.Builder
	builder.WriteString("# Plan Diff\n\n")
	builder.WriteString(fmt.Sprintf("- generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- plan_a: %s\n", diff.PlanA))
	builder.WriteString(fmt.Sprintf("- plan_b: %s\n", diff.PlanB))
	builder.WriteString("\n## Differences\n\n")
	if len(diff.Differences) == 0 {
		builder.WriteString("- no differences\n")
	} else {
		for _, d := range diff.Differences {
			builder.WriteString(fmt.Sprintf("- %s: `%v` -> `%v`\n", d.Field, d.A, d.B))
		}
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write diff artifact: %w", err)
	}
	return path, nil
}

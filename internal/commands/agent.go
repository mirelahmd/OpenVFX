package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/agent"
	"github.com/mirelahmd/byom-video/internal/batch"
)

type PlanOptions struct {
	Goal                      string
	Preset                    string
	MaxClips                  int
	WithExport                bool
	WithValidate              bool
	WithReport                bool
	WithReportSet             bool
	GoalAware                 bool
	GoalUseOllama             bool
	GoalFallbackDeterministic bool
	Execute                   bool
	DryRun                    bool
	Mode                      string
	Recursive                 bool
	Once                      bool
	Limit                     int
}

type InspectPlanOptions struct{ JSON bool }

type PlanArtifactsOptions struct{ JSON bool }

type PlanArtifactsSummary struct {
	PlanID      string   `json:"plan_id"`
	Plan        string   `json:"agent_plan"`
	ActionLog   string   `json:"actions_log"`
	Review      string   `json:"review,omitempty"`
	AgentResult string   `json:"agent_result,omitempty"`
	RunIDs      []string `json:"run_ids,omitempty"`
	BatchIDs    []string `json:"batch_ids,omitempty"`
	Snapshots   []string `json:"snapshots"`
	Diffs       []string `json:"diffs"`
}

func Plan(inputFile string, stdout io.Writer, opts PlanOptions) error {
	plan, err := agent.NewPlan(inputFile, opts.Goal, agent.GoalOptions{
		PresetOverride:            opts.Preset,
		MaxClips:                  opts.MaxClips,
		WithExport:                opts.WithExport,
		WithValidate:              opts.WithValidate,
		WithReport:                opts.WithReport,
		WithReportSet:             opts.WithReportSet,
		GoalAware:                 opts.GoalAware,
		GoalUseOllama:             opts.GoalUseOllama,
		GoalFallbackDeterministic: opts.GoalFallbackDeterministic,
		Mode:                      opts.Mode,
		Recursive:                 opts.Recursive,
		Once:                      opts.Once,
		Limit:                     opts.Limit,
	}, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := agent.WritePlan(plan); err != nil {
		return err
	}
	log, err := agent.OpenActionLog(plan.PlanID)
	if err != nil {
		return err
	}
	_ = log.Write("PLAN_CREATED", map[string]any{"plan_id": plan.PlanID, "goal": plan.Goal})
	_ = log.Close()
	printPlan(stdout, plan)
	fmt.Fprintf(stdout, "  execute:  %t\n", opts.Execute)
	if opts.DryRun || !opts.Execute {
		return nil
	}
	now := time.Now().UTC()
	_, _ = agent.CreateSnapshot(plan, "before inline execute approval", now)
	plan.ApprovalStatus = "approved"
	plan.ApprovedAt = &now
	plan.ApprovalMode = "inline_execute"
	if err := agent.WritePlan(plan); err != nil {
		return err
	}
	log, err = agent.OpenActionLog(plan.PlanID)
	if err == nil {
		_ = log.Write("PLAN_APPROVED", map[string]any{"approval_mode": "inline_execute"})
		_ = log.Close()
	}
	return ExecutePlan(plan.PlanID, stdout)
}

func ExecutePlan(planID string, stdout io.Writer) error {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	log, err := agent.OpenActionLog(plan.PlanID)
	if err != nil {
		return err
	}
	defer log.Close()
	_ = log.Write("PLAN_VALIDATION_STARTED", map[string]any{"plan_id": plan.PlanID})
	if errs := agent.ValidatePlan(plan); len(errs) > 0 {
		plan.Status = "failed"
		plan.ValidationStatus = "failed"
		plan.ValidationErrors = errs
		_ = agent.WritePlan(plan)
		_ = log.Write("PLAN_VALIDATION_FAILED", map[string]any{"errors": errs})
		return fmt.Errorf("plan validation failed: %s", strings.Join(errs, "; "))
	}
	plan.ValidationStatus = "completed"
	plan.ValidationErrors = nil
	_ = agent.WritePlan(plan)
	_ = log.Write("PLAN_VALIDATION_COMPLETED", map[string]any{"plan_id": plan.PlanID})
	plan.Status = "running"
	_ = log.Write("PLAN_EXECUTION_STARTED", map[string]any{"plan_id": plan.PlanID})
	var runID string
	var batchID string
	for i := range plan.Actions {
		action := &plan.Actions[i]
		action.Status = "running"
		_ = agent.WritePlan(plan)
		_ = log.Write("ACTION_STARTED", map[string]any{"id": action.ID, "type": action.Type})
		err := executeAction(action, plan, &runID, &batchID, stdout)
		if err != nil {
			action.Status = "failed"
			action.Error = err.Error()
			plan.Status = "failed"
			_ = agent.WritePlan(plan)
			_ = log.Write("ACTION_FAILED", map[string]any{"id": action.ID, "error": err.Error()})
			_ = log.Write("PLAN_EXECUTION_FAILED", map[string]any{"error": err.Error()})
			printExecutePlanFailureSummary(stdout, plan, *action)
			return err
		}
		action.Status = "completed"
		if runID != "" {
			action.RunID = runID
		}
		if batchID != "" {
			action.BatchID = batchID
		}
		_ = agent.WritePlan(plan)
		_ = log.Write("ACTION_COMPLETED", map[string]any{"id": action.ID, "run_id": runID, "batch_id": batchID})
	}
	plan.Status = "completed"
	_ = agent.WritePlan(plan)
	_ = log.Write("PLAN_EXECUTION_COMPLETED", map[string]any{"plan_id": plan.PlanID, "run_id": runID})
	printExecutePlanSuccessSummary(stdout, plan, runID, batchID)
	return nil
}

func Plans(stdout io.Writer) error {
	plans, err := agent.ListPlans()
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		fmt.Fprintln(stdout, "No plans found. Run `byom-video plan <input-file> --goal \"make 5 shorts\"` first.")
		return nil
	}
	fmt.Fprintf(stdout, "%-28s %-25s %-10s %-10s %-20s %s\n", "PLAN ID", "CREATED AT", "PRESET", "STATUS", "RUN ID", "GOAL")
	for _, plan := range plans {
		fmt.Fprintf(stdout, "%-28s %-25s %-10s %-10s %-20s %s\n", plan.PlanID, plan.CreatedAt.Format(time.RFC3339), plan.Preset, planDefault(plan.Status, "planned"), planRunID(plan), plan.Goal)
	}
	return nil
}

func InspectPlan(planID string, stdout io.Writer, opts InspectPlanOptions) error {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printPlan(stdout, plan)
	printPlanArtifactsSummary(stdout, plan.PlanID)
	return nil
}

func PlanArtifacts(planID string, stdout io.Writer, opts PlanArtifactsOptions) error {
	if _, err := agent.ReadPlan(planID); err != nil {
		return err
	}
	summary, err := collectPlanArtifacts(planID)
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Plan artifacts")
	fmt.Fprintf(stdout, "  plan id:    %s\n", summary.PlanID)
	fmt.Fprintf(stdout, "  plan:       %s\n", summary.Plan)
	fmt.Fprintf(stdout, "  action log: %s\n", summary.ActionLog)
	if summary.Review != "" {
		fmt.Fprintf(stdout, "  review:     %s\n", summary.Review)
	}
	if summary.AgentResult != "" {
		fmt.Fprintf(stdout, "  agent result: %s\n", summary.AgentResult)
	}
	if len(summary.RunIDs) > 0 {
		fmt.Fprintf(stdout, "  run ids:    %s\n", strings.Join(summary.RunIDs, ", "))
	}
	if len(summary.BatchIDs) > 0 {
		fmt.Fprintf(stdout, "  batch ids:  %s\n", strings.Join(summary.BatchIDs, ", "))
	}
	fmt.Fprintf(stdout, "  snapshots:  %d\n", len(summary.Snapshots))
	for _, path := range summary.Snapshots {
		fmt.Fprintf(stdout, "    - %s\n", path)
	}
	fmt.Fprintf(stdout, "  diffs:      %d\n", len(summary.Diffs))
	for _, path := range summary.Diffs {
		fmt.Fprintf(stdout, "    - %s\n", path)
	}
	return nil
}

func executeAction(action *agent.Action, plan agent.Plan, runID *string, batchID *string, stdout io.Writer) error {
	switch action.Type {
	case "run_pipeline":
		opts := runOptionsFromAction(plan.Preset, action.Options)
		var out bytes.Buffer
		if err := Run(plan.InputPath, &out, opts); err != nil {
			return err
		}
		*runID = batch.ParseRunID(out.String())
	case "batch_pipeline":
		opts := BatchOptions{
			Preset:     presetForExecution(plan.Preset),
			RunOptions: runOptionsFromAction(plan.Preset, action.Options),
			Recursive:  boolActionOption(action.Options, "recursive"),
			Limit:      intActionOption(action.Options, "limit"),
		}
		var out bytes.Buffer
		if err := Batch(plan.InputPath, &out, opts); err != nil {
			return err
		}
		*batchID = parseLabel(out.String(), "batch id:")
	case "watch_pipeline":
		if !boolActionOption(action.Options, "once") {
			return fmt.Errorf("watch plan execution requires --once in this version")
		}
		opts := WatchOptions{
			Preset:          presetForExecution(plan.Preset),
			RunOptions:      runOptionsFromAction(plan.Preset, action.Options),
			Recursive:       boolActionOption(action.Options, "recursive"),
			Once:            true,
			Limit:           intActionOption(action.Options, "limit"),
			IntervalSeconds: 5,
		}
		return Watch(plan.InputPath, stdout, opts)
	case "export_run":
		if *runID == "" {
			return fmt.Errorf("export requires completed run action")
		}
		return Export(*runID, stdout)
	case "validate_run":
		if *runID == "" {
			return fmt.Errorf("validate requires completed run action")
		}
		return Validate(*runID, stdout, ValidateOptions{})
	case "goal_rerank":
		if *runID == "" {
			return fmt.Errorf("goal-rerank requires completed run action")
		}
		goal, _ := action.Options["goal_text"].(string)
		return GoalRerankCommand(*runID, stdout, GoalRerankOptions{
			Goal:                  goal,
			UseOllama:             boolActionOption(action.Options, "goal_use_ollama"),
			FallbackDeterministic: boolActionOption(action.Options, "goal_fallback_deterministic"),
		})
	case "goal_roughcut":
		if *runID == "" {
			return fmt.Errorf("goal-roughcut requires completed run action")
		}
		return GoalRoughcutCommand(*runID, stdout, GoalRoughcutOptions{Overwrite: true})
	default:
		return fmt.Errorf("unknown action type %q", action.Type)
	}
	return nil
}

func runOptionsFromAction(preset string, options map[string]any) RunOptions {
	opts := RunOptions{TranscriptModelSize: "tiny", ChunkTargetSeconds: 30, ChunkMaxGapSeconds: 2, HighlightTopK: 10, HighlightMinDuration: 3, HighlightMaxDuration: 90, RoughcutMaxClips: 5, FFmpegOutputFormat: "mp4"}
	if preset == "metadata" {
		return opts
	}
	opts.WithTranscript = boolOption(options, "with_transcript")
	opts.WithCaptions = boolOption(options, "with_captions")
	opts.WithChunks = boolOption(options, "with_chunks")
	opts.WithHighlights = boolOption(options, "with_highlights")
	opts.WithRoughcut = boolOption(options, "with_roughcut")
	opts.WithFFmpegScript = boolOption(options, "with_ffmpeg_script")
	opts.WithReport = boolOption(options, "with_report")
	if v, ok := options["roughcut_max_clips"].(float64); ok && v > 0 {
		opts.RoughcutMaxClips = int(v)
	}
	if v, ok := options["roughcut_max_clips"].(int); ok && v > 0 {
		opts.RoughcutMaxClips = v
	}
	return opts
}

func boolOption(options map[string]any, key string) bool {
	v, _ := options[key].(bool)
	return v
}

func boolActionOption(options map[string]any, key string) bool {
	v, _ := options[key].(bool)
	return v
}

func intActionOption(options map[string]any, key string) int {
	if v, ok := options[key].(int); ok {
		return v
	}
	if v, ok := options[key].(float64); ok {
		return int(v)
	}
	return 0
}

func printPlan(stdout io.Writer, plan agent.Plan) {
	fmt.Fprintln(stdout, "Agent plan")
	fmt.Fprintf(stdout, "  plan id:  %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  goal:     %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  input:    %s\n", plan.InputPath)
	fmt.Fprintf(stdout, "  target:   %s\n", planDefault(plan.TargetType, "file"))
	fmt.Fprintf(stdout, "  preset:   %s\n", plan.Preset)
	fmt.Fprintf(stdout, "  status:   %s\n", planDefault(plan.Status, "planned"))
	fmt.Fprintf(stdout, "  path:     %s\n", filepath.Join(agent.PlanDir(plan.PlanID), "agent_plan.json"))
	fmt.Fprintln(stdout, "  actions:")
	for _, action := range plan.Actions {
		fmt.Fprintf(stdout, "    - %s %s %s", action.ID, action.Type, action.Status)
		if action.RunID != "" {
			fmt.Fprintf(stdout, " run=%s", action.RunID)
		}
		if action.BatchID != "" {
			fmt.Fprintf(stdout, " batch=%s", action.BatchID)
		}
		if action.Error != "" {
			fmt.Fprintf(stdout, " error=%s", action.Error)
		}
		fmt.Fprintln(stdout)
		if action.CommandPreview != "" {
			fmt.Fprintf(stdout, "      command: %s\n", action.CommandPreview)
		}
	}
}

func printPlanArtifactsSummary(stdout io.Writer, planID string) {
	summary, err := collectPlanArtifacts(planID)
	if err != nil {
		return
	}
	fmt.Fprintln(stdout, "  artifacts:")
	fmt.Fprintf(stdout, "    action log: %s\n", summary.ActionLog)
	if summary.Review != "" {
		fmt.Fprintf(stdout, "    review:     %s\n", summary.Review)
	}
	if summary.AgentResult != "" {
		fmt.Fprintf(stdout, "    agent result: %s\n", summary.AgentResult)
	}
	if len(summary.RunIDs) > 0 {
		fmt.Fprintf(stdout, "    run ids:     %s\n", strings.Join(summary.RunIDs, ", "))
	}
	if len(summary.BatchIDs) > 0 {
		fmt.Fprintf(stdout, "    batch ids:   %s\n", strings.Join(summary.BatchIDs, ", "))
	}
	fmt.Fprintf(stdout, "    snapshots:  %d\n", len(summary.Snapshots))
	if len(summary.Diffs) > 0 {
		fmt.Fprintln(stdout, "    diffs:")
		for _, path := range summary.Diffs {
			fmt.Fprintf(stdout, "      - %s\n", path)
		}
	}
}

func collectPlanArtifacts(planID string) (PlanArtifactsSummary, error) {
	dir := agent.PlanDir(planID)
	summary := PlanArtifactsSummary{
		PlanID:    planID,
		Plan:      filepath.Join(dir, "agent_plan.json"),
		ActionLog: filepath.Join(dir, "actions.jsonl"),
	}
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return summary, err
	}
	summary.RunIDs = uniqueSortedRunIDs(plan)
	summary.BatchIDs = uniqueSortedBatchIDs(plan)
	review := filepath.Join(dir, "review.md")
	if _, err := os.Stat(review); err == nil {
		summary.Review = review
	}
	agentResult := filepath.Join(dir, "agent_result.md")
	if _, err := os.Stat(agentResult); err == nil {
		summary.AgentResult = agentResult
	}
	snapshots, err := agent.ListSnapshots(planID)
	if err != nil {
		return summary, err
	}
	for _, snapshot := range snapshots {
		summary.Snapshots = append(summary.Snapshots, filepath.Join(dir, "snapshots", snapshot.SnapshotID+".json"))
	}
	diffsDir := filepath.Join(dir, "diffs")
	entries, err := os.ReadDir(diffsDir)
	if err != nil && !os.IsNotExist(err) {
		return summary, fmt.Errorf("read diffs directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		summary.Diffs = append(summary.Diffs, filepath.Join(diffsDir, entry.Name()))
	}
	sort.Strings(summary.Diffs)
	return summary, nil
}

func planRunID(plan agent.Plan) string {
	for _, action := range plan.Actions {
		if action.RunID != "" {
			return action.RunID
		}
	}
	return "-"
}

func planDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseLabel(output string, label string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, label); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func presetForExecution(preset string) string {
	if preset == "custom" {
		return "shorts"
	}
	return preset
}

func printExecutePlanSuccessSummary(stdout io.Writer, plan agent.Plan, runID string, batchID string) {
	fmt.Fprintln(stdout, "Plan execution result")
	fmt.Fprintf(stdout, "  plan id:      %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  final status: %s\n", planDefault(plan.Status, "completed"))
	if runID != "" {
		runDir := filepath.Join(".byom-video", "runs", runID)
		fmt.Fprintf(stdout, "  run id:       %s\n", runID)
		fmt.Fprintf(stdout, "  run dir:      %s\n", runDir)
		reportPath := filepath.Join(runDir, "report.html")
		if _, err := os.Stat(reportPath); err == nil {
			fmt.Fprintf(stdout, "  report path:  %s\n", reportPath)
		}
		if _, err := os.Stat(filepath.Join(runDir, "goal_rerank.json")); err == nil {
			fmt.Fprintf(stdout, "  goal rerank:  %s\n", filepath.Join(runDir, "goal_rerank.json"))
		}
		if _, err := os.Stat(filepath.Join(runDir, "goal_roughcut.json")); err == nil {
			fmt.Fprintf(stdout, "  goal roughcut:%s\n", " "+filepath.Join(runDir, "goal_roughcut.json"))
		}
		fmt.Fprintln(stdout, "  next commands:")
		for _, cmd := range dedupeStrings(suggestNextCommandsForRun(runID)) {
			fmt.Fprintf(stdout, "    - %s\n", cmd)
		}
		return
	}
	if batchID != "" {
		fmt.Fprintf(stdout, "  batch id:      %s\n", batchID)
		fmt.Fprintln(stdout, "  next commands:")
		fmt.Fprintf(stdout, "    - byom-video inspect-batch %s\n", batchID)
	}
}

func printExecutePlanFailureSummary(stdout io.Writer, plan agent.Plan, action agent.Action) {
	fmt.Fprintln(stdout, "Plan execution failed")
	fmt.Fprintf(stdout, "  plan id:      %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  final status: failed\n")
	fmt.Fprintf(stdout, "  failed action:%s %s (%s)\n", " ", action.ID, action.Type)
	if action.Error != "" {
		fmt.Fprintf(stdout, "  error:        %s\n", action.Error)
	}
	fmt.Fprintln(stdout, "  next commands:")
	fmt.Fprintf(stdout, "    - byom-video inspect-plan %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "    - byom-video review-plan %s\n", plan.PlanID)
}

func suggestNextCommandsForRun(runID string) []string {
	runDir := filepath.Join(".byom-video", "runs", runID)
	next := []string{
		fmt.Sprintf("byom-video inspect %s", runID),
		fmt.Sprintf("byom-video validate %s", runID),
	}
	if _, err := os.Stat(filepath.Join(runDir, "report.html")); err == nil {
		next = append(next, fmt.Sprintf("byom-video open-report %s", runID))
	}
	if _, err := os.Stat(filepath.Join(runDir, "goal_roughcut.json")); err == nil {
		next = append(next, fmt.Sprintf("byom-video goal-handoff %s --overwrite", runID))
	}
	if _, err := os.Stat(filepath.Join(runDir, "ffmpeg_commands.sh")); err == nil {
		next = append(next, fmt.Sprintf("byom-video export %s", runID))
	}
	return next
}

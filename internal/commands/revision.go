package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"byom-video/internal/agent"
)

type RevisePlanOptions struct {
	Request  string
	DryRun   bool
	JSON     bool
	ShowDiff bool
}

type InspectSnapshotOptions struct{ JSON bool }
type DiffSnapshotOptions struct {
	JSON          bool
	WriteArtifact bool
}

type RevisionResult struct {
	PlanID        string       `json:"plan_id"`
	Request       string       `json:"request"`
	Changed       []Difference `json:"changed"`
	ApprovalReset bool         `json:"approval_reset"`
	SnapshotID    string       `json:"snapshot_id,omitempty"`
}

func Snapshots(planID string, stdout io.Writer) error {
	snapshots, err := agent.ListSnapshots(planID)
	if err != nil {
		return err
	}
	if len(snapshots) == 0 {
		fmt.Fprintln(stdout, "No snapshots found.")
		return nil
	}
	fmt.Fprintf(stdout, "%-16s %-25s %s\n", "SNAPSHOT", "CREATED AT", "REASON")
	for _, snapshot := range snapshots {
		fmt.Fprintf(stdout, "%-16s %-25s %s\n", snapshot.SnapshotID, snapshot.CreatedAt.Format(time.RFC3339), snapshot.Reason)
	}
	return nil
}

func InspectSnapshot(planID string, snapshotID string, stdout io.Writer, opts InspectSnapshotOptions) error {
	snapshot, err := agent.ReadSnapshot(planID, snapshotID)
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Plan snapshot")
	fmt.Fprintf(stdout, "  snapshot: %s\n", snapshot.SnapshotID)
	fmt.Fprintf(stdout, "  created:  %s\n", snapshot.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "  reason:   %s\n", snapshot.Reason)
	fmt.Fprintf(stdout, "  plan id:  %s\n", snapshot.Plan.PlanID)
	fmt.Fprintf(stdout, "  goal:     %s\n", snapshot.Plan.Goal)
	fmt.Fprintf(stdout, "  preset:   %s\n", snapshot.Plan.Preset)
	return nil
}

func RevisePlan(planID string, stdout io.Writer, opts RevisePlanOptions) error {
	if strings.TrimSpace(opts.Request) == "" {
		return fmt.Errorf("--request is required")
	}
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	original := clonePlan(plan)
	revised := clonePlan(plan)
	changes, err := applyRevision(&revised, opts.Request)
	if err != nil {
		return err
	}
	result := RevisionResult{PlanID: planID, Request: opts.Request, Changed: changes, ApprovalReset: len(changes) > 0}
	if opts.DryRun {
		printRevision(stdout, result)
		if opts.ShowDiff {
			printDiff(stdout, BuildPlanDiff(original, revised))
		}
		return nil
	}
	snapshot, err := agent.CreateSnapshot(original, "before revise-plan: "+opts.Request, time.Now().UTC())
	if err != nil {
		return err
	}
	result.SnapshotID = snapshot.SnapshotID
	if result.ApprovalReset {
		revised.ApprovalStatus = "pending"
		revised.ApprovedAt = nil
		revised.ApprovalMode = ""
		revised.ValidationStatus = ""
		revised.ValidationErrors = nil
	}
	if err := agent.WritePlan(revised); err != nil {
		return err
	}
	log, err := agent.OpenActionLog(planID)
	if err == nil {
		_ = log.Write("PLAN_REVISED", map[string]any{"request": opts.Request, "snapshot_id": snapshot.SnapshotID})
		_ = log.Close()
	}
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printRevision(stdout, result)
	if opts.ShowDiff {
		printDiff(stdout, BuildPlanDiff(original, revised))
	}
	return nil
}

func DiffSnapshot(planID string, snapshotID string, stdout io.Writer, opts DiffSnapshotOptions) error {
	current, err := agent.ReadPlan(planID)
	if err != nil {
		return err
	}
	snapshot, err := agent.ReadSnapshot(planID, snapshotID)
	if err != nil {
		return err
	}
	diff := BuildPlanDiff(snapshot.Plan, current)
	wroteDiffPath := ""
	if opts.WriteArtifact {
		path, err := writeDiffArtifact(planID, "diff_current_vs_"+snapshotID+".md", diff)
		if err != nil {
			return err
		}
		wroteDiffPath = path
		log, err := agent.OpenActionLog(planID)
		if err == nil {
			_ = log.Write("PLAN_DIFF_ARTIFACT_WRITTEN", map[string]any{"path": path, "snapshot_id": snapshotID})
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
	printDiff(stdout, diff)
	if wroteDiffPath != "" {
		fmt.Fprintf(stdout, "  artifact: %s\n", wroteDiffPath)
	}
	return nil
}

func applyRevision(plan *agent.Plan, request string) ([]Difference, error) {
	before := clonePlan(*plan)
	req := strings.ToLower(strings.TrimSpace(request))
	switch {
	case strings.Contains(req, "shorter"):
		setMaxClips(plan, max(1, currentMaxClips(plan)-1))
	case strings.Contains(req, "longer"):
		setMaxClips(plan, min(20, currentMaxClips(plan)+1))
	case numberForClips(req) > 0:
		setMaxClips(plan, numberForClips(req))
	case strings.Contains(req, "add validation"):
		addValidation(plan)
	case strings.Contains(req, "remove validation"):
		removeAction(plan, "validate_run")
	case strings.Contains(req, "add export"):
		addExport(plan)
	case strings.Contains(req, "remove export"):
		removeAction(plan, "export_run")
	case strings.Contains(req, "caption"):
		setPipelineOptions(plan, "custom", map[string]any{"with_transcript": true, "with_captions": true})
	case strings.Contains(req, "metadata"):
		setPipelineOptions(plan, "metadata", map[string]any{})
	case strings.Contains(req, "highlight"):
		setPipelineOptions(plan, "custom", map[string]any{"with_transcript": true, "with_chunks": true, "with_highlights": true})
	default:
		return nil, fmt.Errorf("unknown revision request %q; examples: make it shorter, make 3 clips, add validation, remove export, captions only, metadata only, find highlights", request)
	}
	refreshPreviews(plan)
	diff := BuildPlanDiff(before, *plan)
	return diff.Differences, nil
}

func clonePlan(plan agent.Plan) agent.Plan {
	data, err := json.Marshal(plan)
	if err != nil {
		return plan
	}
	var out agent.Plan
	if err := json.Unmarshal(data, &out); err != nil {
		return plan
	}
	agent.NormalizePlan(&out)
	return out
}

func currentMaxClips(plan *agent.Plan) int {
	for _, action := range plan.Actions {
		if action.Type == "run_pipeline" || action.Type == "batch_pipeline" || action.Type == "watch_pipeline" {
			if n := intActionOption(action.Options, "roughcut_max_clips"); n > 0 {
				return n
			}
		}
	}
	return 3
}

func setMaxClips(plan *agent.Plan, n int) {
	if n <= 0 {
		n = 1
	}
	for i := range plan.Actions {
		if plan.Actions[i].Type == "run_pipeline" || plan.Actions[i].Type == "batch_pipeline" || plan.Actions[i].Type == "watch_pipeline" {
			if plan.Actions[i].Options == nil {
				plan.Actions[i].Options = map[string]any{}
			}
			plan.Actions[i].Options["roughcut_max_clips"] = n
			plan.Actions[i].Options["with_transcript"] = true
			plan.Actions[i].Options["with_captions"] = true
			plan.Actions[i].Options["with_chunks"] = true
			plan.Actions[i].Options["with_highlights"] = true
			plan.Actions[i].Options["with_roughcut"] = true
			plan.Actions[i].Options["with_ffmpeg_script"] = true
			if _, ok := plan.Actions[i].Options["with_report"]; !ok {
				plan.Actions[i].Options["with_report"] = true
			}
		}
	}
	plan.Preset = "shorts"
}

func setPipelineOptions(plan *agent.Plan, preset string, options map[string]any) {
	plan.Preset = preset
	for i := range plan.Actions {
		if plan.Actions[i].Type == "run_pipeline" || plan.Actions[i].Type == "batch_pipeline" || plan.Actions[i].Type == "watch_pipeline" {
			plan.Actions[i].Options = options
		}
	}
}

func addValidation(plan *agent.Plan) {
	for _, action := range plan.Actions {
		if action.Type == "validate_run" {
			return
		}
	}
	plan.Actions = append(plan.Actions, agent.Action{ID: nextActionID(plan), Type: "validate_run", Status: "planned", Description: "Validate run artifacts", CommandPreview: "./byom-video validate <run_id>", Options: map[string]any{}})
}

func addExport(plan *agent.Plan) {
	for _, action := range plan.Actions {
		if action.Type == "export_run" {
			return
		}
	}
	plan.Actions = append(plan.Actions, agent.Action{ID: nextActionID(plan), Type: "export_run", Status: "planned", Description: "Run explicit export", CommandPreview: "./byom-video export <run_id>", Options: map[string]any{}})
}

func removeAction(plan *agent.Plan, actionType string) {
	filtered := []agent.Action{}
	for _, action := range plan.Actions {
		if action.Type != actionType {
			filtered = append(filtered, action)
		}
	}
	plan.Actions = filtered
}

func refreshPreviews(plan *agent.Plan) {
	for i := range plan.Actions {
		action := &plan.Actions[i]
		switch action.Type {
		case "run_pipeline", "batch_pipeline", "watch_pipeline":
			action.CommandPreview = agent.CommandPreviewForOptions(action.Type, plan.InputPath, plan.Preset, "", action.Options)
		case "export_run":
			action.CommandPreview = "./byom-video export <run_id>"
		case "validate_run":
			action.CommandPreview = "./byom-video validate <run_id>"
		}
	}
}

func nextActionID(plan *agent.Plan) string {
	return fmt.Sprintf("act_%04d", len(plan.Actions)+1)
}

func numberForClips(request string) int {
	re := regexp.MustCompile(`(\d+)\s+(clips|shorts)`)
	match := re.FindStringSubmatch(request)
	if len(match) < 2 {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(match[1], "%d", &n)
	return n
}

func printRevision(stdout io.Writer, result RevisionResult) {
	fmt.Fprintln(stdout, "Plan revision")
	fmt.Fprintf(stdout, "  plan id:        %s\n", result.PlanID)
	fmt.Fprintf(stdout, "  request:        %s\n", result.Request)
	if result.SnapshotID != "" {
		fmt.Fprintf(stdout, "  snapshot:       %s\n", result.SnapshotID)
	}
	fmt.Fprintf(stdout, "  approval reset: %t\n", result.ApprovalReset)
	fmt.Fprintln(stdout, "  changes:")
	for _, change := range result.Changed {
		fmt.Fprintf(stdout, "    - %s: %v -> %v\n", change.Field, change.A, change.B)
	}
}

func printDiff(stdout io.Writer, diff PlanDiff) {
	fmt.Fprintf(stdout, "Plan diff: %s -> %s\n", diff.PlanA, diff.PlanB)
	if len(diff.Differences) == 0 {
		fmt.Fprintln(stdout, "  no differences")
		return
	}
	for _, d := range diff.Differences {
		fmt.Fprintf(stdout, "  - %s: %v -> %v\n", d.Field, d.A, d.B)
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

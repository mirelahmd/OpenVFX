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

	"github.com/mirelahmd/byom-video/internal/agent"
	"github.com/mirelahmd/byom-video/internal/editorartifacts"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/exportartifacts"
	"github.com/mirelahmd/byom-video/internal/goalartifacts"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type AgentResultOptions struct {
	JSON          bool
	WriteArtifact bool
}

type AgentResultSummary struct {
	PlanID          string            `json:"plan_id"`
	Goal            string            `json:"goal"`
	Status          string            `json:"status"`
	ApprovalStatus  string            `json:"approval_status"`
	ExecutionStatus string            `json:"execution_status"`
	RunIDs          []string          `json:"run_ids,omitempty"`
	BatchIDs        []string          `json:"batch_ids,omitempty"`
	Actions         []agent.Action    `json:"actions"`
	Artifacts       []AgentResultPath `json:"artifacts,omitempty"`
	NextCommands    []string          `json:"next_commands"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

type AgentResultPath struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

func AgentResultCommand(planID string, stdout io.Writer, opts AgentResultOptions) error {
	summary, err := BuildAgentResultSummary(planID)
	if err != nil {
		return err
	}
	artifactPath := ""
	if opts.WriteArtifact {
		artifactPath, err = writeAgentResultArtifact(planID, summary)
		if err != nil {
			return err
		}
		log, logErr := agent.OpenActionLog(planID)
		if logErr == nil {
			_ = log.Write("AGENT_RESULT_ARTIFACT_WRITTEN", map[string]any{"path": artifactPath})
			_ = log.Close()
		}
	}
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	printAgentResult(stdout, summary)
	if artifactPath != "" {
		fmt.Fprintf(stdout, "  artifact:         %s\n", artifactPath)
	}
	return nil
}

func BuildAgentResultSummary(planID string) (AgentResultSummary, error) {
	plan, err := agent.ReadPlan(planID)
	if err != nil {
		return AgentResultSummary{}, err
	}
	runIDs := uniqueSortedRunIDs(plan)
	batchIDs := uniqueSortedBatchIDs(plan)
	summary := AgentResultSummary{
		PlanID:          plan.PlanID,
		Goal:            plan.Goal,
		Status:          plan.Status,
		ApprovalStatus:  plan.ApprovalStatus,
		ExecutionStatus: planExecutionStatus(plan),
		RunIDs:          runIDs,
		BatchIDs:        batchIDs,
		Actions:         plan.Actions,
		GeneratedAt:     time.Now().UTC(),
	}
	artifacts, next := summarizePlanResults(plan, runIDs, batchIDs)
	summary.Artifacts = artifacts
	summary.NextCommands = next
	return summary, nil
}

func printAgentResult(stdout io.Writer, summary AgentResultSummary) {
	fmt.Fprintln(stdout, "Agent result")
	fmt.Fprintf(stdout, "  plan id:          %s\n", summary.PlanID)
	fmt.Fprintf(stdout, "  goal:             %s\n", summary.Goal)
	fmt.Fprintf(stdout, "  status:           %s\n", planDefault(summary.Status, "unknown"))
	fmt.Fprintf(stdout, "  approval status:  %s\n", planDefault(summary.ApprovalStatus, "unknown"))
	fmt.Fprintf(stdout, "  execution:        %s\n", planDefault(summary.ExecutionStatus, "not_started"))
	if len(summary.RunIDs) > 0 {
		fmt.Fprintf(stdout, "  run ids:          %s\n", strings.Join(summary.RunIDs, ", "))
	}
	if len(summary.BatchIDs) > 0 {
		fmt.Fprintf(stdout, "  batch ids:        %s\n", strings.Join(summary.BatchIDs, ", "))
	}
	if len(summary.Artifacts) > 0 {
		fmt.Fprintln(stdout, "  artifacts:")
		for _, item := range summary.Artifacts {
			fmt.Fprintf(stdout, "    - %s: %s\n", item.Label, item.Path)
		}
	}
	fmt.Fprintln(stdout, "  next commands:")
	for _, cmd := range summary.NextCommands {
		fmt.Fprintf(stdout, "    - %s\n", cmd)
	}
}

func writeAgentResultArtifact(planID string, summary AgentResultSummary) (string, error) {
	path := filepath.Join(agent.PlanDir(planID), "agent_result.md")
	var b strings.Builder
	b.WriteString("# Agent Result\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", summary.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", summary.PlanID)
	fmt.Fprintf(&b, "- Goal: %s\n", summary.Goal)
	fmt.Fprintf(&b, "- Status: `%s`\n", planDefault(summary.Status, "unknown"))
	fmt.Fprintf(&b, "- Approval status: `%s`\n", planDefault(summary.ApprovalStatus, "unknown"))
	fmt.Fprintf(&b, "- Execution status: `%s`\n", planDefault(summary.ExecutionStatus, "not_started"))
	if len(summary.RunIDs) > 0 {
		fmt.Fprintf(&b, "- Run IDs: `%s`\n", strings.Join(summary.RunIDs, "`, `"))
	}
	if len(summary.BatchIDs) > 0 {
		fmt.Fprintf(&b, "- Batch IDs: `%s`\n", strings.Join(summary.BatchIDs, "`, `"))
	}
	b.WriteString("\n## Actions\n\n")
	for _, action := range summary.Actions {
		fmt.Fprintf(&b, "- `%s` `%s` `%s`\n", action.ID, action.Type, action.Status)
		if action.CommandPreview != "" {
			fmt.Fprintf(&b, "  - command: `%s`\n", action.CommandPreview)
		}
		if action.Error != "" {
			fmt.Fprintf(&b, "  - error: `%s`\n", action.Error)
		}
	}
	if len(summary.Artifacts) > 0 {
		b.WriteString("\n## Artifacts\n\n")
		for _, item := range summary.Artifacts {
			fmt.Fprintf(&b, "- `%s`: `%s`\n", item.Label, item.Path)
		}
	}
	b.WriteString("\n## Next Commands\n\n")
	for _, cmd := range summary.NextCommands {
		fmt.Fprintf(&b, "- `%s`\n", cmd)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write agent_result.md: %w", err)
	}
	return path, nil
}

func summarizePlanResults(plan agent.Plan, runIDs []string, batchIDs []string) ([]AgentResultPath, []string) {
	artifacts := []AgentResultPath{}
	next := []string{}
	if len(runIDs) == 0 {
		next = append(next,
			fmt.Sprintf("byom-video inspect-plan %s", plan.PlanID),
			fmt.Sprintf("byom-video review-plan %s", plan.PlanID),
		)
		if len(batchIDs) > 0 {
			for _, batchID := range batchIDs {
				next = append(next, fmt.Sprintf("byom-video inspect-batch %s", batchID))
			}
		}
		return artifacts, dedupeStrings(next)
	}
	for _, runID := range runIDs {
		runDir := filepath.Join(runstore.RunsRoot, runID)
		artifacts = append(artifacts, AgentResultPath{Label: "run_directory", Path: runDir})
		next = append(next,
			fmt.Sprintf("byom-video inspect %s", runID),
			fmt.Sprintf("byom-video validate %s", runID),
		)
		if _, err := os.Stat(filepath.Join(runDir, "report.html")); err == nil {
			artifacts = append(artifacts, AgentResultPath{Label: "report", Path: filepath.Join(runDir, "report.html")})
			next = append(next, fmt.Sprintf("byom-video open-report %s", runID))
		}
		if _, err := os.Stat(filepath.Join(runDir, "goal_rerank.json")); err == nil {
			artifacts = append(artifacts, AgentResultPath{Label: "goal_rerank", Path: filepath.Join(runDir, "goal_rerank.json")})
		}
		if _, err := os.Stat(filepath.Join(runDir, "goal_roughcut.json")); err == nil {
			artifacts = append(artifacts, AgentResultPath{Label: "goal_roughcut", Path: filepath.Join(runDir, "goal_roughcut.json")})
			next = append(next, fmt.Sprintf("byom-video goal-handoff %s --overwrite", runID))
		}
		if _, err := os.Stat(filepath.Join(runDir, "ffmpeg_commands.sh")); err == nil {
			artifacts = append(artifacts, AgentResultPath{Label: "ffmpeg_script", Path: filepath.Join(runDir, "ffmpeg_commands.sh")})
			next = append(next, fmt.Sprintf("byom-video export %s", runID))
		}
	}
	return artifacts, dedupeStrings(next)
}

func planExecutionStatus(plan agent.Plan) string {
	switch plan.Status {
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "running":
		return "running"
	default:
		for _, action := range plan.Actions {
			if action.Status == "completed" || action.Status == "failed" || action.Status == "running" {
				return "partial"
			}
		}
		return "not_started"
	}
}

func uniqueSortedRunIDs(plan agent.Plan) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, action := range plan.Actions {
		if strings.TrimSpace(action.RunID) == "" || seen[action.RunID] {
			continue
		}
		seen[action.RunID] = true
		out = append(out, action.RunID)
	}
	sort.Strings(out)
	return out
}

func uniqueSortedBatchIDs(plan agent.Plan) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, action := range plan.Actions {
		if strings.TrimSpace(action.BatchID) == "" || seen[action.BatchID] {
			continue
		}
		seen[action.BatchID] = true
		out = append(out, action.BatchID)
	}
	sort.Strings(out)
	return out
}

type GoalReviewBundleOptions struct {
	JSON      bool
	Overwrite bool
}

type GoalReviewBundleSummary struct {
	RunID          string   `json:"run_id"`
	Goal           string   `json:"goal"`
	Mode           string   `json:"goal_rerank_mode"`
	MaxDuration    float64  `json:"max_total_duration_seconds"`
	MaxClips       int      `json:"max_clips"`
	PreferredStyle string   `json:"preferred_style"`
	ClipCount      int      `json:"clip_count"`
	Artifact       string   `json:"artifact"`
	NextCommands   []string `json:"next_commands"`
}

func GoalReviewBundleCommand(runID string, stdout io.Writer, opts GoalReviewBundleOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("GOAL_REVIEW_BUNDLE_STARTED", map[string]any{"run_id": runID})
	}
	outPath := filepath.Join(runDir, "goal_review_bundle.md")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "GOAL_REVIEW_BUNDLE_FAILED", "goal_review_bundle.md already exists; pass --overwrite")
			return fmt.Errorf("goal_review_bundle.md already exists; pass --overwrite")
		}
	}
	summary, markdown, err := buildGoalReviewBundle(runDir, runID, outPath)
	if err != nil {
		writeMaskFailure(log, "GOAL_REVIEW_BUNDLE_FAILED", err.Error())
		return err
	}
	if err := os.WriteFile(outPath, []byte(markdown), 0o644); err != nil {
		writeMaskFailure(log, "GOAL_REVIEW_BUNDLE_FAILED", err.Error())
		return fmt.Errorf("write goal_review_bundle.md: %w", err)
	}
	if err := addManifestArtifact(runDir, "goal_review_bundle", "goal_review_bundle.md"); err != nil {
		writeMaskFailure(log, "GOAL_REVIEW_BUNDLE_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "GOAL_REVIEW_BUNDLE_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("GOAL_REVIEW_BUNDLE_COMPLETED", map[string]any{"path": "goal_review_bundle.md", "clips": summary.ClipCount})
	}
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Goal review bundle created")
	fmt.Fprintf(stdout, "  run id:    %s\n", runID)
	fmt.Fprintf(stdout, "  path:      %s\n", outPath)
	fmt.Fprintf(stdout, "  clips:     %d\n", summary.ClipCount)
	for _, cmd := range summary.NextCommands {
		fmt.Fprintf(stdout, "  next:      %s\n", cmd)
	}
	return nil
}

func buildGoalReviewBundle(runDir string, runID string, outPath string) (GoalReviewBundleSummary, string, error) {
	rerank, err := goalartifacts.ReadGoalRerank(filepath.Join(runDir, "goal_rerank.json"))
	if err != nil {
		return GoalReviewBundleSummary{}, "", err
	}
	roughcut, err := goalartifacts.ReadGoalRoughcut(filepath.Join(runDir, "goal_roughcut.json"))
	if err != nil {
		return GoalReviewBundleSummary{}, "", err
	}
	cardsPath := filepath.Join(runDir, "clip_cards.json")
	selectedPath := filepath.Join(runDir, "selected_clips.json")
	exportManifestPath := filepath.Join(runDir, "export_manifest.json")

	cardTitles := map[string]string{}
	cardCaptions := map[string][]string{}
	if cards, err := editorartifacts.ReadClipCards(cardsPath); err == nil {
		for _, card := range cards.Cards {
			cardTitles[card.ClipID] = card.Title
			cardCaptions[card.ClipID] = card.Captions
		}
	}
	outputNames := map[string]string{}
	if selected, err := exportartifacts.ReadSelectedClips(selectedPath); err == nil {
		for _, clip := range selected.Clips {
			outputNames[clip.ID] = clip.OutputFilename
		}
	}
	exportSummary := ""
	if exportManifest, err := exportartifacts.ReadExportManifest(exportManifestPath); err == nil {
		exportSummary = fmt.Sprintf("planned=%d exported=%d validated=%d missing=%d", exportManifest.Summary.Planned, exportManifest.Summary.Exported, exportManifest.Summary.Validated, exportManifest.Summary.Missing)
	}

	next := []string{
		fmt.Sprintf("byom-video goal-handoff %s --overwrite", runID),
		fmt.Sprintf("byom-video open-report %s", runID),
		fmt.Sprintf("byom-video validate %s", runID),
	}
	if _, err := os.Stat(filepath.Join(runDir, "ffmpeg_commands.sh")); err == nil {
		next = append(next, fmt.Sprintf("byom-video export %s", runID))
	}
	summary := GoalReviewBundleSummary{
		RunID:          runID,
		Goal:           rerank.Goal,
		Mode:           rerank.Mode,
		MaxDuration:    rerank.Constraints.MaxTotalDurationSeconds,
		MaxClips:       rerank.Constraints.MaxClips,
		PreferredStyle: rerank.Constraints.PreferredStyle,
		ClipCount:      len(roughcut.Clips),
		Artifact:       outPath,
		NextCommands:   dedupeStrings(next),
	}

	var b strings.Builder
	b.WriteString("# Goal Review Bundle\n\n")
	fmt.Fprintf(&b, "- Generated at: `%s`\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Run ID: `%s`\n", runID)
	fmt.Fprintf(&b, "- Goal: %s\n", rerank.Goal)
	fmt.Fprintf(&b, "- Goal rerank mode: `%s`\n", rerank.Mode)
	fmt.Fprintf(&b, "- Preferred style: `%s`\n", rerank.Constraints.PreferredStyle)
	fmt.Fprintf(&b, "- Max total duration: `%.0f seconds`\n", rerank.Constraints.MaxTotalDurationSeconds)
	fmt.Fprintf(&b, "- Max clips: `%d`\n", rerank.Constraints.MaxClips)
	b.WriteString("\n## Goal-Aware Clips\n\n")
	for _, clip := range roughcut.Clips {
		fmt.Fprintf(&b, "### %s\n\n", clip.ID)
		fmt.Fprintf(&b, "- Range: `%.3f-%.3f`\n", clip.Start, clip.End)
		fmt.Fprintf(&b, "- Goal score: `%.3f`\n", clip.GoalScore)
		fmt.Fprintf(&b, "- Reason: %s\n", clip.Reason)
		fmt.Fprintf(&b, "- Text: %s\n", clip.Text)
		if title := cardTitles[clip.ID]; title != "" {
			fmt.Fprintf(&b, "- Clip card title: %s\n", title)
		}
		if captions := cardCaptions[clip.ID]; len(captions) > 0 {
			fmt.Fprintf(&b, "- Captions: %s\n", strings.Join(captions, " | "))
		}
		if output := outputNames[clip.ID]; output != "" {
			fmt.Fprintf(&b, "- Output filename: `%s`\n", output)
		}
		b.WriteString("\n")
	}
	if exportSummary != "" {
		b.WriteString("## Export Manifest Summary\n\n")
		fmt.Fprintf(&b, "- %s\n\n", exportSummary)
	}
	b.WriteString("## Next Commands\n\n")
	for _, cmd := range summary.NextCommands {
		fmt.Fprintf(&b, "- `%s`\n", cmd)
	}
	return summary, b.String(), nil
}

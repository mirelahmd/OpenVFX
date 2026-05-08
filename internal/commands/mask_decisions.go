package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

// ── Decision-level mask editing ────────────────────────────────────────────────

type MaskDecisionOptions struct {
	Set    string
	Reason string
	DryRun bool
	JSON   bool
}

type MaskDecisionResult struct {
	RunID      string `json:"run_id"`
	DecisionID string `json:"decision_id"`
	OldValue   string `json:"old_value"`
	NewValue   string `json:"new_value"`
	Reason     string `json:"reason,omitempty"`
	Applied    bool   `json:"applied"`
	DryRun     bool   `json:"dry_run"`
	SnapshotID string `json:"snapshot_id,omitempty"`
}

type MaskRemoveDecisionOptions struct {
	DryRun bool
	JSON   bool
}

type MaskRemoveDecisionResult struct {
	RunID      string `json:"run_id"`
	DecisionID string `json:"decision_id"`
	Applied    bool   `json:"applied"`
	DryRun     bool   `json:"dry_run"`
	SnapshotID string `json:"snapshot_id,omitempty"`
}

type MaskReorderOptions struct {
	Order  string // comma-separated decision IDs
	DryRun bool
	JSON   bool
}

type MaskReorderResult struct {
	RunID      string   `json:"run_id"`
	Order      []string `json:"order"`
	Applied    bool     `json:"applied"`
	DryRun     bool     `json:"dry_run"`
	SnapshotID string   `json:"snapshot_id,omitempty"`
}

type MaskDecisionsOptions struct{ JSON bool }

var validDecisionValues = map[string]bool{
	"keep":           true,
	"reject":         true,
	"candidate_keep": true,
}

func MaskDecisionCommand(runID string, decisionID string, stdout io.Writer, opts MaskDecisionOptions) error {
	if opts.Set == "" {
		return fmt.Errorf("--set is required (keep, reject, candidate_keep)")
	}
	if !validDecisionValues[opts.Set] {
		return fmt.Errorf("invalid --set value %q; supported: keep, reject, candidate_keep", opts.Set)
	}

	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
	}

	maskPath := filepath.Join(runDir, "inference_mask.json")
	original, err := readInferenceMask(maskPath)
	if err != nil {
		writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
		return err
	}

	idx := findDecisionIndex(original.Decisions, decisionID)
	if idx < 0 {
		writeMaskFailure(log, "MASK_DECISION_FAILED", fmt.Sprintf("decision %q not found", decisionID))
		return fmt.Errorf("decision %q not found in inference_mask.json", decisionID)
	}

	oldValue := original.Decisions[idx].Decision
	result := MaskDecisionResult{
		RunID:      runID,
		DecisionID: decisionID,
		OldValue:   oldValue,
		NewValue:   opts.Set,
		Applied:    !opts.DryRun,
		DryRun:     opts.DryRun,
	}

	revised := deepCopyMask(original)
	revised.Decisions[idx].Decision = opts.Set
	if opts.Reason != "" {
		existing := revised.Decisions[idx].Reason
		if existing == "" {
			revised.Decisions[idx].Reason = "Manual note: " + opts.Reason
		} else {
			revised.Decisions[idx].Reason = existing + " | Manual note: " + opts.Reason
		}
		result.Reason = revised.Decisions[idx].Reason
	}

	if err := validateProposedMask(revised); err != nil {
		writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
		return fmt.Errorf("proposed mask failed validation: %w", err)
	}

	if !opts.DryRun {
		snapshotID, err := createMaskSnapshot(runDir, original)
		if err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}
		result.SnapshotID = snapshotID

		if err := writeJSONFile(maskPath, revised); err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}

		if log != nil {
			_ = log.Write("MASK_DECISION_UPDATED", map[string]any{
				"decision_id": decisionID,
				"old":         oldValue,
				"new":         opts.Set,
				"snapshot_id": snapshotID,
			})
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if opts.DryRun {
		fmt.Fprintln(stdout, "Mask decision (dry run)")
	} else {
		fmt.Fprintln(stdout, "Mask decision updated")
	}
	fmt.Fprintf(stdout, "  run id:      %s\n", runID)
	fmt.Fprintf(stdout, "  decision id: %s\n", decisionID)
	fmt.Fprintf(stdout, "  old:         %s\n", oldValue)
	fmt.Fprintf(stdout, "  new:         %s\n", opts.Set)
	if result.Reason != "" {
		fmt.Fprintf(stdout, "  reason:      %s\n", result.Reason)
	}
	if result.SnapshotID != "" {
		fmt.Fprintf(stdout, "  snapshot:    %s\n", result.SnapshotID)
	}
	return nil
}

func MaskRemoveDecisionCommand(runID string, decisionID string, stdout io.Writer, opts MaskRemoveDecisionOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
	}

	maskPath := filepath.Join(runDir, "inference_mask.json")
	original, err := readInferenceMask(maskPath)
	if err != nil {
		writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
		return err
	}

	idx := findDecisionIndex(original.Decisions, decisionID)
	if idx < 0 {
		writeMaskFailure(log, "MASK_DECISION_FAILED", fmt.Sprintf("decision %q not found", decisionID))
		return fmt.Errorf("decision %q not found in inference_mask.json", decisionID)
	}

	result := MaskRemoveDecisionResult{
		RunID:      runID,
		DecisionID: decisionID,
		Applied:    !opts.DryRun,
		DryRun:     opts.DryRun,
	}

	revised := deepCopyMask(original)
	revised.Decisions = append(revised.Decisions[:idx], revised.Decisions[idx+1:]...)

	if !opts.DryRun {
		snapshotID, err := createMaskSnapshot(runDir, original)
		if err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}
		result.SnapshotID = snapshotID

		if err := writeJSONFile(maskPath, revised); err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}

		if log != nil {
			_ = log.Write("MASK_DECISION_REMOVED", map[string]any{
				"decision_id": decisionID,
				"snapshot_id": snapshotID,
			})
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if opts.DryRun {
		fmt.Fprintln(stdout, "Mask remove decision (dry run)")
	} else {
		fmt.Fprintln(stdout, "Mask decision removed")
	}
	fmt.Fprintf(stdout, "  run id:      %s\n", runID)
	fmt.Fprintf(stdout, "  decision id: %s\n", decisionID)
	if result.SnapshotID != "" {
		fmt.Fprintf(stdout, "  snapshot:    %s\n", result.SnapshotID)
	}
	return nil
}

func MaskReorderCommand(runID string, stdout io.Writer, opts MaskReorderOptions) error {
	if strings.TrimSpace(opts.Order) == "" {
		return fmt.Errorf("--order is required (comma-separated decision ids)")
	}

	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
	}

	maskPath := filepath.Join(runDir, "inference_mask.json")
	original, err := readInferenceMask(maskPath)
	if err != nil {
		writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
		return err
	}

	orderIDs := splitAndTrim(opts.Order)
	reordered, err := reorderDecisions(original.Decisions, orderIDs)
	if err != nil {
		writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
		return err
	}

	result := MaskReorderResult{
		RunID:   runID,
		Order:   orderIDs,
		Applied: !opts.DryRun,
		DryRun:  opts.DryRun,
	}

	if !opts.DryRun {
		revised := deepCopyMask(original)
		revised.Decisions = reordered

		snapshotID, err := createMaskSnapshot(runDir, original)
		if err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}
		result.SnapshotID = snapshotID

		if err := writeJSONFile(maskPath, revised); err != nil {
			writeMaskFailure(log, "MASK_DECISION_FAILED", err.Error())
			return err
		}

		if log != nil {
			_ = log.Write("MASK_DECISIONS_REORDERED", map[string]any{
				"order":       orderIDs,
				"snapshot_id": snapshotID,
			})
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if opts.DryRun {
		fmt.Fprintln(stdout, "Mask reorder (dry run)")
	} else {
		fmt.Fprintln(stdout, "Mask decisions reordered")
	}
	fmt.Fprintf(stdout, "  run id:   %s\n", runID)
	fmt.Fprintf(stdout, "  order:    %s\n", strings.Join(orderIDs, ", "))
	if result.SnapshotID != "" {
		fmt.Fprintf(stdout, "  snapshot: %s\n", result.SnapshotID)
	}
	return nil
}

func MaskDecisionsList(runID string, stdout io.Writer, opts MaskDecisionsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		return err
	}

	if opts.JSON {
		data, err := json.MarshalIndent(mask.Decisions, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Mask decisions")
	fmt.Fprintf(stdout, "  run id:    %s\n", runID)
	fmt.Fprintf(stdout, "  decisions: %d\n", len(mask.Decisions))
	for _, d := range mask.Decisions {
		fmt.Fprintf(stdout, "  - %s  %.3f-%.3f  %-15s  %s\n", d.ID, d.Start, d.End, d.Decision, d.Reason)
		if d.TextPreview != "" {
			fmt.Fprintf(stdout, "      text: %s\n", d.TextPreview)
		}
	}
	return nil
}

// ── Route execution preview ────────────────────────────────────────────────────

type RoutePreviewOptions struct {
	JSON          bool
	WriteArtifact bool
}

type RoutePreviewTask struct {
	TaskID         string            `json:"task_id"`
	TaskType       string            `json:"task_type"`
	ModelRoute     string            `json:"model_route"`
	ResolvedEntry  string            `json:"resolved_entry,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	Model          string            `json:"model,omitempty"`
	InputDecisions []DecisionPreview `json:"input_decisions"`
	Constraints    MaskConstraints   `json:"constraints"`
	OutputContract map[string]any    `json:"output_contract"`
	PayloadPreview PayloadPreview    `json:"payload_preview"`
	Status         string            `json:"status"`
	Warnings       []string          `json:"warnings"`
}

type DecisionPreview struct {
	DecisionID  string  `json:"decision_id"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	TextPreview string  `json:"text_preview"`
}

type PayloadPreview struct {
	Instruction string `json:"instruction"`
	Schema      string `json:"schema"`
}

type RoutePreview struct {
	SchemaVersion string             `json:"schema_version"`
	CreatedAt     time.Time          `json:"created_at"`
	RunID         string             `json:"run_id"`
	Tasks         []RoutePreviewTask `json:"tasks"`
	Warnings      []string           `json:"warnings"`
}

func RoutePreviewCommand(runID string, stdout io.Writer, opts RoutePreviewOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("ROUTE_PREVIEW_STARTED", map[string]any{"run_id": runID})
	}

	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		writeMaskFailure(log, "ROUTE_PREVIEW_FAILED", err.Error())
		return err
	}

	expansionPath := filepath.Join(runDir, "expansion_tasks.json")
	data, err := os.ReadFile(expansionPath)
	if err != nil {
		writeMaskFailure(log, "ROUTE_PREVIEW_FAILED", "expansion_tasks.json is required")
		return fmt.Errorf("expansion_tasks.json is required; run expansion-plan first")
	}
	var tasks ExpansionTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		writeMaskFailure(log, "ROUTE_PREVIEW_FAILED", err.Error())
		return fmt.Errorf("decode expansion_tasks.json: %w", err)
	}

	cfg := config.Config{}
	if c, loadErr := config.Load(config.DefaultPath); loadErr == nil {
		cfg = c
	}

	decisionMap := buildDecisionMap(mask.Decisions)
	previewTasks := []RoutePreviewTask{}
	allWarnings := []string{}

	for _, task := range tasks.Tasks {
		pt := buildRoutePreviewTask(task, mask, cfg, decisionMap)
		previewTasks = append(previewTasks, pt)
		allWarnings = append(allWarnings, pt.Warnings...)
	}

	preview := RoutePreview{
		SchemaVersion: "route_preview.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Tasks:         previewTasks,
		Warnings:      dedupeStrings(allWarnings),
	}

	if opts.WriteArtifact {
		path := filepath.Join(runDir, "route_preview.json")
		if err := writeJSONFile(path, preview); err != nil {
			writeMaskFailure(log, "ROUTE_PREVIEW_FAILED", err.Error())
			return err
		}
		if err := addManifestArtifact(runDir, "route_preview", "route_preview.json"); err != nil {
			writeMaskFailure(log, "ROUTE_PREVIEW_FAILED", err.Error())
			return err
		}
	}

	if log != nil {
		_ = log.Write("ROUTE_PREVIEW_COMPLETED", map[string]any{"tasks": len(previewTasks), "warnings": len(preview.Warnings)})
	}

	if opts.JSON {
		out, err := json.MarshalIndent(preview, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}

	printRoutePreview(stdout, preview)
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "route_preview.json"))
	}
	return nil
}

func buildDecisionMap(decisions []MaskDecision) map[string]MaskDecision {
	m := map[string]MaskDecision{}
	for _, d := range decisions {
		m[d.ID] = d
	}
	return m
}

func buildRoutePreviewTask(task ExpansionTask, mask InferenceMask, cfg config.Config, decisionMap map[string]MaskDecision) RoutePreviewTask {
	pt := RoutePreviewTask{
		TaskID:         task.ID,
		TaskType:       task.Type,
		ModelRoute:     task.ModelRoute,
		Constraints:    mask.Constraints,
		OutputContract: task.OutputContract,
		Warnings:       []string{},
	}

	entryName, ok := cfg.Models.Routes[task.ModelRoute]
	if !ok {
		pt.Status = "missing_route"
		pt.Warnings = append(pt.Warnings, fmt.Sprintf("route %q is not configured", task.ModelRoute))
	} else if entry, ok := cfg.Models.Entries[entryName]; !ok {
		pt.ResolvedEntry = entryName
		pt.Status = "missing_entry"
		pt.Warnings = append(pt.Warnings, fmt.Sprintf("entry %q is not configured", entryName))
	} else {
		pt.ResolvedEntry = entryName
		pt.Provider = entry.Provider
		pt.Model = entry.Model
		if cfg.Models.Enabled {
			pt.Status = "configured"
		} else {
			pt.Status = "models_disabled"
		}
	}

	for _, ref := range task.InputRefs {
		if d, ok := decisionMap[ref]; ok {
			pt.InputDecisions = append(pt.InputDecisions, DecisionPreview{
				DecisionID:  d.ID,
				Start:       d.Start,
				End:         d.End,
				TextPreview: d.TextPreview,
			})
		}
	}

	pt.PayloadPreview = buildPayloadPreview(task.Type, mask)
	return pt
}

func buildPayloadPreview(taskType string, mask InferenceMask) PayloadPreview {
	switch taskType {
	case "caption_variants":
		return PayloadPreview{
			Instruction: fmt.Sprintf("Generate up to %d caption variants per clip under %d words. Tone: %s. Avoid: %s.",
				3, mask.Constraints.MaxCaptionWords, nonEmptyString(mask.Constraints.Tone, "neutral"),
				joinOr(mask.Constraints.MustNotInclude, "nothing specified")),
			Schema: "preview_only",
		}
	case "timeline_labels":
		return PayloadPreview{
			Instruction: fmt.Sprintf("Generate a short timeline label per clip. Tone: %s. Avoid: %s.",
				nonEmptyString(mask.Constraints.Tone, "neutral"),
				joinOr(mask.Constraints.MustNotInclude, "nothing specified")),
			Schema: "preview_only",
		}
	case "short_descriptions":
		return PayloadPreview{
			Instruction: fmt.Sprintf("Generate a short description per clip under %d words. Tone: %s.",
				mask.Constraints.MaxCaptionWords*4,
				nonEmptyString(mask.Constraints.Tone, "neutral")),
			Schema: "preview_only",
		}
	default:
		return PayloadPreview{
			Instruction: fmt.Sprintf("Execute task type %q under mask constraints.", taskType),
			Schema:      "preview_only",
		}
	}
}

func printRoutePreview(stdout io.Writer, preview RoutePreview) {
	fmt.Fprintln(stdout, "Route preview")
	fmt.Fprintf(stdout, "  run id: %s\n", preview.RunID)
	fmt.Fprintf(stdout, "  tasks:  %d\n", len(preview.Tasks))
	for _, t := range preview.Tasks {
		fmt.Fprintf(stdout, "  - task:      %s (%s)\n", t.TaskID, t.TaskType)
		fmt.Fprintf(stdout, "    route:     %s\n", t.ModelRoute)
		if t.ResolvedEntry != "" {
			fmt.Fprintf(stdout, "    entry:     %s\n", t.ResolvedEntry)
		}
		if t.Provider != "" {
			fmt.Fprintf(stdout, "    provider:  %s\n", t.Provider)
		}
		if t.Model != "" {
			fmt.Fprintf(stdout, "    model:     %s\n", t.Model)
		}
		fmt.Fprintf(stdout, "    status:    %s\n", t.Status)
		fmt.Fprintf(stdout, "    decisions: %d input(s)\n", len(t.InputDecisions))
		fmt.Fprintf(stdout, "    payload:   %s\n", t.PayloadPreview.Instruction)
		for _, w := range t.Warnings {
			fmt.Fprintf(stdout, "    warning:   %s\n", w)
		}
	}
	for _, w := range preview.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────────

func findDecisionIndex(decisions []MaskDecision, id string) int {
	for i, d := range decisions {
		if d.ID == id {
			return i
		}
	}
	return -1
}

func reorderDecisions(decisions []MaskDecision, orderIDs []string) ([]MaskDecision, error) {
	if len(orderIDs) != len(decisions) {
		return nil, fmt.Errorf("--order must contain all %d decision ids exactly once (got %d)", len(decisions), len(orderIDs))
	}
	byID := map[string]MaskDecision{}
	for _, d := range decisions {
		byID[d.ID] = d
	}
	seen := map[string]bool{}
	result := make([]MaskDecision, 0, len(orderIDs))
	for _, id := range orderIDs {
		if seen[id] {
			return nil, fmt.Errorf("duplicate decision id %q in --order", id)
		}
		d, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("decision id %q not found in inference_mask.json", id)
		}
		seen[id] = true
		result = append(result, d)
	}
	return result, nil
}

func validateProposedMask(mask InferenceMask) error {
	if mask.SchemaVersion != "inference_mask.v1" {
		return fmt.Errorf("schema_version must be inference_mask.v1")
	}
	if strings.TrimSpace(mask.Intent) == "" {
		return fmt.Errorf("intent must be non-empty")
	}
	for i, d := range mask.Decisions {
		if strings.TrimSpace(d.ID) == "" {
			return fmt.Errorf("decisions[%d].id must be non-empty", i)
		}
		if !validDecisionValues[d.Decision] {
			return fmt.Errorf("decisions[%d].decision %q is invalid", i, d.Decision)
		}
		if d.End < d.Start {
			return fmt.Errorf("decisions[%d].end must be >= start", i)
		}
	}
	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func joinOr(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	return strings.Join(items, ", ")
}

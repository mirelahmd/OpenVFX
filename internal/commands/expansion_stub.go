package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
)

// ── Expansion output schema ────────────────────────────────────────────────────

type ExpansionOutputItem struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id"`
	DecisionID string         `json:"decision_id"`
	Text       string         `json:"text"`
	Start      float64        `json:"start"`
	End        float64        `json:"end"`
	Metadata   map[string]any `json:"metadata"`
}

type ExpansionOutputSource struct {
	InferenceMaskArtifact  string   `json:"inference_mask_artifact"`
	ExpansionTasksArtifact string   `json:"expansion_tasks_artifact"`
	TaskIDs                []string `json:"task_ids"`
}

type ExpansionOutput struct {
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	Mode          string                `json:"mode"`
	TaskType      string                `json:"task_type"`
	Source        ExpansionOutputSource `json:"source"`
	Items         []ExpansionOutputItem `json:"items"`
	Warnings      []string              `json:"warnings,omitempty"`
}

// ── expand-stub ────────────────────────────────────────────────────────────────

type ExpandStubOptions struct {
	Overwrite bool
	JSON      bool
	TaskType  string
}

type ExpandStubSummary struct {
	RunID      string         `json:"run_id"`
	Mode       string         `json:"mode"`
	Files      []string       `json:"files"`
	ItemCounts map[string]int `json:"item_counts"`
	Warnings   []string       `json:"warnings,omitempty"`
}

func ExpandStub(runID string, stdout io.Writer, opts ExpandStubOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPAND_STUB_STARTED", map[string]any{"run_id": runID})
	}

	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		writeMaskFailure(log, "EXPAND_STUB_FAILED", err.Error())
		return err
	}

	taskData, err := os.ReadFile(filepath.Join(runDir, "expansion_tasks.json"))
	if err != nil {
		msg := "expansion_tasks.json is required; run expansion-plan first"
		writeMaskFailure(log, "EXPAND_STUB_FAILED", msg)
		return fmt.Errorf("%s", msg)
	}
	var tasks ExpansionTasks
	if err := json.Unmarshal(taskData, &tasks); err != nil {
		writeMaskFailure(log, "EXPAND_STUB_FAILED", err.Error())
		return fmt.Errorf("decode expansion_tasks.json: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(runDir, "expansions"), 0o755); err != nil {
		writeMaskFailure(log, "EXPAND_STUB_FAILED", err.Error())
		return fmt.Errorf("create expansions dir: %w", err)
	}

	decisionMap := buildDecisionMap(mask.Decisions)

	// Build set of rejected decision IDs.
	rejectedIDs := map[string]bool{}
	for _, d := range mask.Decisions {
		if d.Decision == "reject" {
			rejectedIDs[d.ID] = true
		}
	}

	// Group tasks by type, filtering by --task-type when set.
	type taskGroup struct {
		taskType string
		tasks    []ExpansionTask
	}
	seen := map[string]bool{}
	var groups []taskGroup
	for _, t := range tasks.Tasks {
		if opts.TaskType != "" && t.Type != opts.TaskType {
			continue
		}
		if !seen[t.Type] {
			seen[t.Type] = true
			groups = append(groups, taskGroup{taskType: t.Type})
		}
		for i := range groups {
			if groups[i].taskType == t.Type {
				groups[i].tasks = append(groups[i].tasks, t)
				break
			}
		}
	}

	if opts.TaskType != "" && len(groups) == 0 {
		msg := fmt.Sprintf("no tasks found for task type %q", opts.TaskType)
		writeMaskFailure(log, "EXPAND_STUB_FAILED", msg)
		return fmt.Errorf("%s", msg)
	}

	summary := ExpandStubSummary{
		RunID:      runID,
		Mode:       "stub",
		ItemCounts: map[string]int{},
	}

	for _, group := range groups {
		outPath := filepath.Join(runDir, "expansions", group.taskType+".json")
		if !opts.Overwrite {
			if _, err := os.Stat(outPath); err == nil {
				msg := fmt.Sprintf("expansions/%s.json already exists; pass --overwrite", group.taskType)
				writeMaskFailure(log, "EXPAND_STUB_FAILED", msg)
				return fmt.Errorf("%s", msg)
			}
		}

		var taskIDs []string
		for _, t := range group.tasks {
			taskIDs = append(taskIDs, t.ID)
		}

		output, warnings := buildStubOutput(group.taskType, group.tasks, mask, decisionMap, rejectedIDs)
		output.Source = ExpansionOutputSource{
			InferenceMaskArtifact:  "inference_mask.json",
			ExpansionTasksArtifact: "expansion_tasks.json",
			TaskIDs:                taskIDs,
		}

		if err := writeJSONFile(outPath, output); err != nil {
			writeMaskFailure(log, "EXPAND_STUB_FAILED", err.Error())
			return err
		}
		if err := addManifestArtifact(runDir, "expansion_"+group.taskType, filepath.Join("expansions", group.taskType+".json")); err != nil {
			writeMaskFailure(log, "EXPAND_STUB_FAILED", err.Error())
			return err
		}

		relPath := filepath.Join("expansions", group.taskType+".json")
		summary.Files = append(summary.Files, relPath)
		summary.ItemCounts[group.taskType] = len(output.Items)
		summary.Warnings = append(summary.Warnings, warnings...)
	}

	if log != nil {
		_ = log.Write("EXPAND_STUB_COMPLETED", map[string]any{
			"run_id": runID,
			"files":  summary.Files,
		})
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Stub expansion completed")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  mode:   stub\n")
	for _, relPath := range summary.Files {
		taskType := strings.TrimSuffix(filepath.Base(relPath), ".json")
		fmt.Fprintf(stdout, "  - %-25s  %d items\n", relPath, summary.ItemCounts[taskType])
	}
	for _, w := range summary.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	return nil
}

func buildStubOutput(taskType string, tasks []ExpansionTask, mask InferenceMask, decisionMap map[string]MaskDecision, rejectedIDs map[string]bool) (ExpansionOutput, []string) {
	var warnings []string
	items := []ExpansionOutputItem{}

	// Collect ordered unique input decision IDs across all tasks of this type.
	seenRefs := map[string]bool{}
	type refEntry struct {
		decisionID string
		taskID     string
		contract   map[string]any
	}
	var refs []refEntry
	for _, task := range tasks {
		for _, ref := range task.InputRefs {
			key := task.ID + ":" + ref
			if seenRefs[key] {
				continue
			}
			seenRefs[key] = true
			refs = append(refs, refEntry{decisionID: ref, taskID: task.ID, contract: task.OutputContract})
		}
	}

	activeCount := 0
	for _, ref := range refs {
		if rejectedIDs[ref.decisionID] {
			continue
		}
		d, ok := decisionMap[ref.decisionID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("input_ref %q not found in inference_mask.json decisions", ref.decisionID))
			continue
		}
		activeCount++

		switch taskType {
		case "caption_variants":
			maxItems := contractInt(ref.contract, "max_items", 3)
			maxWords := contractInt(ref.contract, "max_words", mask.Constraints.MaxCaptionWords)
			if maxWords <= 0 {
				maxWords = 18
			}
			for n := 1; n <= maxItems; n++ {
				preview := firstNWords(nonEmptyString(d.TextPreview, d.Reason), maxWords)
				items = append(items, ExpansionOutputItem{
					ID:         fmt.Sprintf("cap_%s_%04d", d.ID, n),
					TaskID:     ref.taskID,
					DecisionID: d.ID,
					Text:       fmt.Sprintf("Stub caption %d for %s: %s", n, d.ID, preview),
					Start:      d.Start,
					End:        d.End,
					Metadata: map[string]any{
						"model_route": tasks[0].ModelRoute,
						"stub":        true,
						"variant":     n,
					},
				})
			}
		case "timeline_labels":
			maxWords := contractInt(ref.contract, "max_words", 8)
			preview := firstNWords(nonEmptyString(d.TextPreview, d.Reason), maxWords)
			items = append(items, ExpansionOutputItem{
				ID:         fmt.Sprintf("lbl_%s", d.ID),
				TaskID:     ref.taskID,
				DecisionID: d.ID,
				Text:       fmt.Sprintf("Label: %s", preview),
				Start:      d.Start,
				End:        d.End,
				Metadata: map[string]any{
					"model_route": tasks[0].ModelRoute,
					"stub":        true,
				},
			})
		case "short_descriptions":
			maxWords := contractInt(ref.contract, "max_words", 80)
			reason := nonEmptyString(d.Reason, d.TextPreview)
			body := firstNWords(reason, maxWords)
			items = append(items, ExpansionOutputItem{
				ID:         fmt.Sprintf("desc_%s", d.ID),
				TaskID:     ref.taskID,
				DecisionID: d.ID,
				Text:       fmt.Sprintf("Stub description for %s (%.1fs-%.1fs): %s", d.ID, d.Start, d.End, body),
				Start:      d.Start,
				End:        d.End,
				Metadata: map[string]any{
					"model_route": tasks[0].ModelRoute,
					"stub":        true,
				},
			})
		default:
			items = append(items, ExpansionOutputItem{
				ID:         fmt.Sprintf("item_%s", d.ID),
				TaskID:     ref.taskID,
				DecisionID: d.ID,
				Text:       fmt.Sprintf("Stub item for %s (task type: %s)", d.ID, taskType),
				Start:      d.Start,
				End:        d.End,
				Metadata: map[string]any{
					"model_route": tasks[0].ModelRoute,
					"stub":        true,
				},
			})
		}
	}

	if activeCount == 0 {
		warnings = append(warnings, fmt.Sprintf("all decisions were rejected for task type %q; producing empty items", taskType))
	}

	return ExpansionOutput{
		SchemaVersion: "expansion_output.v1",
		CreatedAt:     time.Now().UTC(),
		Mode:          "stub",
		TaskType:      taskType,
		Items:         items,
		Warnings:      warnings,
	}, warnings
}

func contractInt(contract map[string]any, key string, fallback int) int {
	if v, ok := contract[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case int64:
			return int(n)
		}
	}
	return fallback
}

func firstNWords(s string, n int) string {
	if n <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[:n], " ") + "..."
}

// ── expansion-validate ─────────────────────────────────────────────────────────

type ExpansionValidateOptions struct{ JSON bool }

type ExpansionFileValidation struct {
	TaskType  string   `json:"task_type"`
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	Valid     bool     `json:"valid"`
	ItemCount int      `json:"item_count,omitempty"`
	Errors    []string `json:"errors,omitempty"`
}

type ExpansionValidationResult struct {
	RunID  string                    `json:"run_id"`
	Valid  bool                      `json:"valid"`
	Files  []ExpansionFileValidation `json:"files"`
	Errors []string                  `json:"errors,omitempty"`
}

var knownExpansionTaskTypes = []string{"caption_variants", "timeline_labels", "short_descriptions"}

func ExpansionValidate(runID string, stdout io.Writer, opts ExpansionValidateOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPANSION_VALIDATION_STARTED", map[string]any{"run_id": runID})
	}

	// Load mask for rejected-decision cross-check (optional).
	maskPath := filepath.Join(runDir, "inference_mask.json")
	var mask *InferenceMask
	if m, err := readInferenceMask(maskPath); err == nil {
		mask = &m
	}

	rejectedIDs := map[string]bool{}
	if mask != nil {
		for _, d := range mask.Decisions {
			if d.Decision == "reject" {
				rejectedIDs[d.ID] = true
			}
		}
	}

	expansionsDir := filepath.Join(runDir, "expansions")
	result := ExpansionValidationResult{RunID: runID, Valid: true}

	for _, taskType := range knownExpansionTaskTypes {
		filePath := filepath.Join(expansionsDir, taskType+".json")
		_, statErr := os.Stat(filePath)
		fv := ExpansionFileValidation{
			TaskType: taskType,
			Path:     filePath,
			Exists:   statErr == nil,
		}
		if !fv.Exists {
			fv.Valid = false
			fv.Errors = append(fv.Errors, "file not found")
			result.Files = append(result.Files, fv)
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			fv.Errors = append(fv.Errors, "read error: "+err.Error())
			result.Files = append(result.Files, fv)
			continue
		}

		var out ExpansionOutput
		if err := json.Unmarshal(data, &out); err != nil {
			fv.Errors = append(fv.Errors, "invalid JSON: "+err.Error())
			result.Files = append(result.Files, fv)
			continue
		}

		errs := validateExpansionOutput(out, taskType, rejectedIDs)
		fv.Errors = append(fv.Errors, errs...)
		fv.Valid = len(fv.Errors) == 0
		fv.ItemCount = len(out.Items)

		if !fv.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s validation failed", taskType))
		}
		result.Files = append(result.Files, fv)
	}

	if log != nil {
		if result.Valid {
			_ = log.Write("EXPANSION_VALIDATION_COMPLETED", map[string]any{"run_id": runID, "valid": true})
		} else {
			_ = log.Write("EXPANSION_VALIDATION_FAILED", map[string]any{"run_id": runID, "errors": result.Errors})
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !result.Valid {
			return fmt.Errorf("expansion validation failed")
		}
		return nil
	}

	fmt.Fprintln(stdout, "Expansion validation")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	if result.Valid {
		fmt.Fprintln(stdout, "  status: ok")
	} else {
		fmt.Fprintln(stdout, "  status: failed")
	}
	for _, fv := range result.Files {
		if !fv.Exists {
			fmt.Fprintf(stdout, "  - %-25s  missing\n", fv.TaskType)
			continue
		}
		if fv.Valid {
			fmt.Fprintf(stdout, "  - %-25s  ok  (%d items)\n", fv.TaskType, fv.ItemCount)
		} else {
			fmt.Fprintf(stdout, "  - %-25s  failed\n", fv.TaskType)
			for _, e := range fv.Errors {
				fmt.Fprintf(stdout, "    - %s\n", e)
			}
		}
	}
	if !result.Valid {
		return fmt.Errorf("expansion validation failed")
	}
	return nil
}

func validateExpansionOutput(out ExpansionOutput, expectedTaskType string, rejectedIDs map[string]bool) []string {
	var errs []string
	if out.SchemaVersion != "expansion_output.v1" {
		errs = append(errs, fmt.Sprintf("schema_version must be expansion_output.v1 (got %q)", out.SchemaVersion))
	}
	if out.CreatedAt.IsZero() {
		errs = append(errs, "created_at is missing or zero")
	}
	if strings.TrimSpace(out.Mode) == "" {
		errs = append(errs, "mode is empty")
	}
	if strings.TrimSpace(out.TaskType) == "" {
		errs = append(errs, "task_type is empty")
	} else if out.TaskType != expectedTaskType {
		errs = append(errs, fmt.Sprintf("task_type mismatch: file reports %q but filename implies %q", out.TaskType, expectedTaskType))
	}
	if out.Source.InferenceMaskArtifact == "" {
		errs = append(errs, "source.inference_mask_artifact is empty")
	}
	if out.Source.ExpansionTasksArtifact == "" {
		errs = append(errs, "source.expansion_tasks_artifact is empty")
	}
	if out.Items == nil {
		errs = append(errs, "items array is missing")
	}
	for i, item := range out.Items {
		if strings.TrimSpace(item.ID) == "" {
			errs = append(errs, fmt.Sprintf("items[%d].id is empty", i))
		}
		if strings.TrimSpace(item.TaskID) == "" {
			errs = append(errs, fmt.Sprintf("items[%d].task_id is empty", i))
		}
		if strings.TrimSpace(item.DecisionID) == "" {
			errs = append(errs, fmt.Sprintf("items[%d].decision_id is empty", i))
		}
		if strings.TrimSpace(item.Text) == "" {
			errs = append(errs, fmt.Sprintf("items[%d].text is empty", i))
		}
		if item.End > 0 && item.End < item.Start {
			errs = append(errs, fmt.Sprintf("items[%d].end (%.3f) < start (%.3f)", i, item.End, item.Start))
		}
		if rejectedIDs[item.DecisionID] {
			errs = append(errs, fmt.Sprintf("items[%d].decision_id %q references a rejected decision", i, item.DecisionID))
		}
	}
	return errs
}

// ── review-expansions ──────────────────────────────────────────────────────────

type ReviewExpansionsOptions struct {
	JSON          bool
	WriteArtifact bool
}

type ExpansionReview struct {
	RunID     string                `json:"run_id"`
	CreatedAt time.Time             `json:"created_at"`
	Files     []ExpansionFileReview `json:"files"`
}

type ExpansionFileReview struct {
	TaskType    string   `json:"task_type"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	ItemCount   int      `json:"item_count"`
	DecisionIDs []string `json:"decision_ids"`
	Previews    []string `json:"previews"`
}

func ReviewExpansions(runID string, stdout io.Writer, opts ReviewExpansionsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	expansionsDir := filepath.Join(runDir, "expansions")
	review := ExpansionReview{
		RunID:     runID,
		CreatedAt: time.Now().UTC(),
	}

	for _, taskType := range knownExpansionTaskTypes {
		filePath := filepath.Join(expansionsDir, taskType+".json")
		_, statErr := os.Stat(filePath)
		frev := ExpansionFileReview{
			TaskType:    taskType,
			Path:        filePath,
			Exists:      statErr == nil,
			DecisionIDs: []string{},
			Previews:    []string{},
		}
		if frev.Exists {
			data, readErr := os.ReadFile(filePath)
			if readErr == nil {
				var out ExpansionOutput
				if jsonErr := json.Unmarshal(data, &out); jsonErr == nil {
					frev.ItemCount = len(out.Items)
					seenDIDs := map[string]bool{}
					for _, item := range out.Items {
						if !seenDIDs[item.DecisionID] {
							seenDIDs[item.DecisionID] = true
							frev.DecisionIDs = append(frev.DecisionIDs, item.DecisionID)
						}
						if len(frev.Previews) < 3 {
							frev.Previews = append(frev.Previews, trimPreview(item.Text))
						}
					}
				}
			}
		}
		review.Files = append(review.Files, frev)
	}

	if opts.WriteArtifact {
		artPath := filepath.Join(runDir, "expansions_review.md")
		if err := writeExpansionReviewMarkdown(artPath, review); err != nil {
			return err
		}
		if err := addManifestArtifact(runDir, "expansions_review", "expansions_review.md"); err != nil {
			return err
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(review, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if opts.WriteArtifact {
			fmt.Fprintf(stdout, "artifact: %s\n", filepath.Join(runDir, "expansions_review.md"))
		}
		return nil
	}

	fmt.Fprintln(stdout, "Expansion review")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	for _, frev := range review.Files {
		if !frev.Exists {
			fmt.Fprintf(stdout, "  %-25s  missing\n", frev.TaskType)
			continue
		}
		fmt.Fprintf(stdout, "  %-25s  %d items  decisions: %s\n",
			frev.TaskType, frev.ItemCount, strings.Join(frev.DecisionIDs, ", "))
		for _, p := range frev.Previews {
			fmt.Fprintf(stdout, "    - %s\n", p)
		}
	}
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "expansions_review.md"))
	}
	return nil
}

func writeExpansionReviewMarkdown(path string, review ExpansionReview) error {
	var b strings.Builder
	b.WriteString("# Expansion Review\n\n")
	b.WriteString(fmt.Sprintf("- generated_at: %s\n", review.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- run_id: %s\n\n", review.RunID))
	for _, frev := range review.Files {
		b.WriteString(fmt.Sprintf("## %s\n\n", frev.TaskType))
		if !frev.Exists {
			b.WriteString("_File not found._\n\n")
			continue
		}
		b.WriteString(fmt.Sprintf("- items: %d\n", frev.ItemCount))
		b.WriteString(fmt.Sprintf("- decisions: %s\n\n", strings.Join(frev.DecisionIDs, ", ")))
		if len(frev.Previews) > 0 {
			b.WriteString("### Previews\n\n")
			for _, p := range frev.Previews {
				b.WriteString(fmt.Sprintf("- %s\n", p))
			}
			b.WriteString("\n")
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write expansions review: %w", err)
	}
	return nil
}

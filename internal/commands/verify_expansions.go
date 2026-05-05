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

// ── Verification results schema ────────────────────────────────────────────────

type VerificationResultCheck struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Status  string         `json:"status"` // passed, failed, warning, skipped
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type VerificationResultSummary struct {
	ChecksTotal  int `json:"checks_total"`
	ChecksPassed int `json:"checks_passed"`
	ChecksFailed int `json:"checks_failed"`
	Warnings     int `json:"warnings"`
}

type VerificationResultSource struct {
	InferenceMaskArtifact string   `json:"inference_mask_artifact"`
	VerificationArtifact  string   `json:"verification_artifact"`
	ExpansionArtifacts    []string `json:"expansion_artifacts"`
}

type VerificationResults struct {
	SchemaVersion string                    `json:"schema_version"`
	CreatedAt     time.Time                 `json:"created_at"`
	RunID         string                    `json:"run_id"`
	Mode          string                    `json:"mode"`
	Source        VerificationResultSource  `json:"source"`
	Status        string                    `json:"status"` // passed, failed, warning
	Summary       VerificationResultSummary `json:"summary"`
	Checks        []VerificationResultCheck `json:"checks"`
}

// ── verify-expansions ──────────────────────────────────────────────────────────

type VerifyExpansionsOptions struct {
	JSON             bool
	ToleranceSeconds float64
}

func VerifyExpansions(runID string, stdout io.Writer, opts VerifyExpansionsOptions) error {
	if opts.ToleranceSeconds < 0 {
		return fmt.Errorf("--tolerance-seconds must be non-negative")
	}
	if opts.ToleranceSeconds == 0 {
		opts.ToleranceSeconds = 0.25
	}

	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("VERIFICATION_STARTED", map[string]any{"run_id": runID})
	}

	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		writeMaskFailure(log, "VERIFICATION_FAILED", err.Error())
		return err
	}

	verPlanData, err := os.ReadFile(filepath.Join(runDir, "verification.json"))
	if err != nil {
		msg := "verification.json is required; run verification-plan first"
		writeMaskFailure(log, "VERIFICATION_FAILED", msg)
		return fmt.Errorf("%s", msg)
	}
	var verPlan VerificationPlan
	if err := json.Unmarshal(verPlanData, &verPlan); err != nil {
		writeMaskFailure(log, "VERIFICATION_FAILED", err.Error())
		return fmt.Errorf("decode verification.json: %w", err)
	}

	// Load optional expansion tasks for contract checking.
	var expTasks *ExpansionTasks
	if data, err := os.ReadFile(filepath.Join(runDir, "expansion_tasks.json")); err == nil {
		var et ExpansionTasks
		if json.Unmarshal(data, &et) == nil {
			expTasks = &et
		}
	}

	// Load all present expansion output files.
	expansionsDir := filepath.Join(runDir, "expansions")
	expansionOutputs := map[string]ExpansionOutput{}
	presentArtifacts := []string{}
	for _, taskType := range knownExpansionTaskTypes {
		path := filepath.Join(expansionsDir, taskType+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var out ExpansionOutput
		if json.Unmarshal(data, &out) == nil {
			expansionOutputs[taskType] = out
			presentArtifacts = append(presentArtifacts, filepath.Join("expansions", taskType+".json"))
		}
	}

	results := VerificationResults{
		SchemaVersion: "verification_results.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Mode:          "deterministic",
		Source: VerificationResultSource{
			InferenceMaskArtifact: "inference_mask.json",
			VerificationArtifact:  "verification.json",
			ExpansionArtifacts:    presentArtifacts,
		},
	}

	for _, check := range verPlan.Checks {
		rc := runVerificationCheck(check, mask, expansionOutputs, expTasks, opts.ToleranceSeconds)
		results.Checks = append(results.Checks, rc)
	}

	// Compute summary and overall status.
	summary := VerificationResultSummary{ChecksTotal: len(results.Checks)}
	overallStatus := "passed"
	for _, rc := range results.Checks {
		switch rc.Status {
		case "passed", "skipped":
			summary.ChecksPassed++
		case "failed":
			summary.ChecksFailed++
			overallStatus = "failed"
		case "warning":
			summary.Warnings++
			if overallStatus == "passed" {
				overallStatus = "warning"
			}
		}
	}
	results.Summary = summary
	results.Status = overallStatus

	// Always write verification_results.json.
	outPath := filepath.Join(runDir, "verification_results.json")
	if err := writeJSONFile(outPath, results); err != nil {
		writeMaskFailure(log, "VERIFICATION_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "verification_results", "verification_results.json"); err != nil {
		writeMaskFailure(log, "VERIFICATION_FAILED", err.Error())
		return err
	}

	if log != nil {
		_ = log.Write("VERIFICATION_COMPLETED", map[string]any{
			"run_id":  runID,
			"status":  overallStatus,
			"passed":  summary.ChecksPassed,
			"failed":  summary.ChecksFailed,
			"warning": summary.Warnings,
		})
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	printVerificationResults(stdout, results)
	return nil
}

func runVerificationCheck(check VerificationCheck, mask InferenceMask, expansionOutputs map[string]ExpansionOutput, expTasks *ExpansionTasks, toleranceSecs float64) VerificationResultCheck {
	rc := VerificationResultCheck{
		ID:      check.ID,
		Type:    check.Type,
		Status:  "passed",
		Message: "",
		Details: map[string]any{},
	}

	switch check.Type {
	case "must_not_include":
		runMustNotInclude(&rc, mask, expansionOutputs)
	case "timestamp_drift":
		runTimestampDrift(&rc, mask, expansionOutputs, toleranceSecs)
	case "missing_required_decisions":
		runMissingRequiredDecisions(&rc, mask, expansionOutputs)
	case "output_contract_compliance":
		runOutputContractCompliance(&rc, mask, expansionOutputs, expTasks)
	default:
		rc.Status = "skipped"
		rc.Message = fmt.Sprintf("unknown check type %q; skipped", check.Type)
	}

	return rc
}

func runMustNotInclude(rc *VerificationResultCheck, mask InferenceMask, expansionOutputs map[string]ExpansionOutput) {
	banned := mask.Constraints.MustNotInclude
	if len(banned) == 0 {
		rc.Message = "no must_not_include phrases configured"
		return
	}

	type violation struct {
		itemID string
		phrase string
	}
	var violations []violation

	for _, out := range expansionOutputs {
		for _, item := range out.Items {
			lower := strings.ToLower(item.Text)
			for _, phrase := range banned {
				if strings.Contains(lower, strings.ToLower(phrase)) {
					violations = append(violations, violation{itemID: item.ID, phrase: phrase})
				}
			}
		}
	}

	if len(violations) > 0 {
		rc.Status = "failed"
		msgs := make([]string, 0, len(violations))
		for _, v := range violations {
			msgs = append(msgs, fmt.Sprintf("item %q contains banned phrase %q", v.itemID, v.phrase))
		}
		rc.Message = strings.Join(msgs, "; ")
		rc.Details["violations"] = violations
	} else {
		rc.Message = fmt.Sprintf("checked %d banned phrases; none found", len(banned))
	}
}

func runTimestampDrift(rc *VerificationResultCheck, mask InferenceMask, expansionOutputs map[string]ExpansionOutput, toleranceSecs float64) {
	decisionMap := buildDecisionMap(mask.Decisions)

	type drift struct {
		itemID     string
		decisionID string
		field      string
		got        float64
		expected   float64
	}
	var drifts []drift

	for _, out := range expansionOutputs {
		for _, item := range out.Items {
			if item.DecisionID == "" {
				continue
			}
			d, ok := decisionMap[item.DecisionID]
			if !ok {
				continue
			}
			if item.Start != 0 || item.End != 0 {
				if abs64(item.Start-d.Start) > toleranceSecs {
					drifts = append(drifts, drift{item.ID, item.DecisionID, "start", item.Start, d.Start})
				}
				if abs64(item.End-d.End) > toleranceSecs {
					drifts = append(drifts, drift{item.ID, item.DecisionID, "end", item.End, d.End})
				}
			}
		}
	}

	if len(drifts) > 0 {
		rc.Status = "failed"
		msgs := make([]string, 0, len(drifts))
		for _, dr := range drifts {
			msgs = append(msgs, fmt.Sprintf("item %q %s: got %.3f expected %.3f (tolerance %.3f)", dr.itemID, dr.field, dr.got, dr.expected, toleranceSecs))
		}
		rc.Message = strings.Join(msgs, "; ")
		rc.Details["drifts"] = drifts
	} else {
		rc.Message = fmt.Sprintf("timestamp drift within tolerance %.3fs for all items", toleranceSecs)
	}
}

func runMissingRequiredDecisions(rc *VerificationResultCheck, mask InferenceMask, expansionOutputs map[string]ExpansionOutput) {
	// Collect all decision IDs referenced by any expansion item.
	covered := map[string]bool{}
	for _, out := range expansionOutputs {
		for _, item := range out.Items {
			if item.DecisionID != "" {
				covered[item.DecisionID] = true
			}
		}
	}

	var missing []string
	for _, d := range mask.Decisions {
		if d.Decision == "reject" {
			continue
		}
		if !covered[d.ID] {
			missing = append(missing, d.ID)
		}
	}

	if len(missing) > 0 {
		rc.Status = "failed"
		rc.Message = fmt.Sprintf("non-rejected decisions with no expansion item: %s", strings.Join(missing, ", "))
		rc.Details["missing_decision_ids"] = missing
	} else {
		rc.Message = "all non-rejected decisions have at least one expansion item"
	}
}

func runOutputContractCompliance(rc *VerificationResultCheck, mask InferenceMask, expansionOutputs map[string]ExpansionOutput, expTasks *ExpansionTasks) {
	if expTasks == nil {
		rc.Status = "skipped"
		rc.Message = "expansion_tasks.json not available; skipped"
		return
	}

	// Build contract map per task type from expansion_tasks.
	type contract struct {
		maxWords int
		maxItems int
	}
	contracts := map[string]contract{}
	for _, task := range expTasks.Tasks {
		c := contract{
			maxWords: contractInt(task.OutputContract, "max_words", 0),
			maxItems: contractInt(task.OutputContract, "max_items", 0),
		}
		if _, ok := contracts[task.Type]; !ok {
			contracts[task.Type] = c
		}
	}

	type violation struct {
		taskType string
		itemID   string
		field    string
		got      int
		limit    int
	}
	var violations []violation

	for taskType, out := range expansionOutputs {
		c, ok := contracts[taskType]
		if !ok {
			continue
		}

		// Check max_words per item.
		if c.maxWords > 0 {
			for _, item := range out.Items {
				words := len(strings.Fields(item.Text))
				if words > c.maxWords {
					violations = append(violations, violation{taskType, item.ID, "word_count", words, c.maxWords})
				}
			}
		}

		// Check max_items per decision.
		if c.maxItems > 0 {
			countByDecision := map[string]int{}
			for _, item := range out.Items {
				countByDecision[item.DecisionID]++
			}
			for decID, count := range countByDecision {
				if count > c.maxItems {
					violations = append(violations, violation{taskType, decID, "item_count", count, c.maxItems})
				}
			}
		}
	}

	if len(violations) > 0 {
		rc.Status = "failed"
		msgs := make([]string, 0, len(violations))
		for _, v := range violations {
			msgs = append(msgs, fmt.Sprintf("%s/%s %s: %d > %d", v.taskType, v.itemID, v.field, v.got, v.limit))
		}
		rc.Message = strings.Join(msgs, "; ")
		rc.Details["violations"] = violations
	} else {
		rc.Message = "all expansion items comply with output contract"
	}
}

func printVerificationResults(stdout io.Writer, results VerificationResults) {
	statusSymbol := map[string]string{
		"passed":  "ok",
		"failed":  "FAILED",
		"warning": "warning",
		"skipped": "skipped",
	}
	fmt.Fprintln(stdout, "Verification results")
	fmt.Fprintf(stdout, "  run id:  %s\n", results.RunID)
	fmt.Fprintf(stdout, "  status:  %s\n", results.Status)
	fmt.Fprintf(stdout, "  checks:  %d total  %d passed  %d failed  %d warnings\n",
		results.Summary.ChecksTotal, results.Summary.ChecksPassed, results.Summary.ChecksFailed, results.Summary.Warnings)
	for _, rc := range results.Checks {
		sym := statusSymbol[rc.Status]
		if sym == "" {
			sym = rc.Status
		}
		fmt.Fprintf(stdout, "  - [%-7s] %s (%s)\n", sym, rc.ID, rc.Type)
		if rc.Message != "" {
			fmt.Fprintf(stdout, "             %s\n", rc.Message)
		}
	}
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ── review-verification ────────────────────────────────────────────────────────

type ReviewVerificationOptions struct {
	JSON          bool
	WriteArtifact bool
}

func ReviewVerification(runID string, stdout io.Writer, opts ReviewVerificationOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filepath.Join(runDir, "verification_results.json"))
	if err != nil {
		return fmt.Errorf("verification_results.json not found; run verify-expansions first")
	}
	var results VerificationResults
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("decode verification_results.json: %w", err)
	}

	if opts.WriteArtifact {
		artPath := filepath.Join(runDir, "verification_review.md")
		if err := writeVerificationReviewMarkdown(artPath, results); err != nil {
			return err
		}
		if err := addManifestArtifact(runDir, "verification_review", "verification_review.md"); err != nil {
			return err
		}
	}

	if opts.JSON {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Fprintln(stdout, string(out))
		if opts.WriteArtifact {
			fmt.Fprintf(stdout, "artifact: %s\n", filepath.Join(runDir, "verification_review.md"))
		}
		return nil
	}

	printVerificationResults(stdout, results)
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "verification_review.md"))
	}
	return nil
}

func writeVerificationReviewMarkdown(path string, results VerificationResults) error {
	var b strings.Builder
	b.WriteString("# Verification Review\n\n")
	b.WriteString(fmt.Sprintf("- generated_at: %s\n", results.CreatedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- run_id: %s\n", results.RunID))
	b.WriteString(fmt.Sprintf("- mode: %s\n", results.Mode))
	b.WriteString(fmt.Sprintf("- status: %s\n\n", results.Status))
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- checks_total: %d\n", results.Summary.ChecksTotal))
	b.WriteString(fmt.Sprintf("- checks_passed: %d\n", results.Summary.ChecksPassed))
	b.WriteString(fmt.Sprintf("- checks_failed: %d\n", results.Summary.ChecksFailed))
	b.WriteString(fmt.Sprintf("- warnings: %d\n\n", results.Summary.Warnings))
	b.WriteString("## Checks\n\n")
	for _, rc := range results.Checks {
		b.WriteString(fmt.Sprintf("### %s — %s\n\n", rc.ID, rc.Type))
		b.WriteString(fmt.Sprintf("- status: %s\n", rc.Status))
		if rc.Message != "" {
			b.WriteString(fmt.Sprintf("- message: %s\n", rc.Message))
		}
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write verification review: %w", err)
	}
	return nil
}

// ── validation helper ──────────────────────────────────────────────────────────

func validateVerificationResultsShape(payload map[string]any) []string {
	var errs []string
	requireStringValue(&errs, payload, "schema_version", "verification_results.v1")
	if _, ok := payload["status"]; !ok {
		errs = append(errs, "missing field \"status\"")
	}
	if _, ok := payload["summary"]; !ok {
		errs = append(errs, "missing field \"summary\"")
	}
	checks, ok := payload["checks"].([]any)
	if !ok {
		errs = append(errs, "checks must be an array")
	} else {
		for i, raw := range checks {
			c, ok := raw.(map[string]any)
			if !ok {
				errs = append(errs, fmt.Sprintf("checks[%d] is not an object", i))
				continue
			}
			for _, field := range []string{"id", "type", "status", "message"} {
				if _, ok := c[field]; !ok {
					errs = append(errs, fmt.Sprintf("checks[%d] missing field %q", i, field))
				}
			}
		}
	}
	return errs
}

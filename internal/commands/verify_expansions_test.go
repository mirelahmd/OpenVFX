package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/manifest"
)

// ── Setup helpers ──────────────────────────────────────────────────────────────

// setupVerifyRun creates a run with mask + expansion plan + stub expansions.
func setupVerifyRun(t *testing.T) (string, string) {
	t.Helper()
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := VerificationPlanCommand(runID, &bytes.Buffer{}, VerificationPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	return runID, runDir
}

func readVerificationResults(t *testing.T, path string) VerificationResults {
	t.Helper()
	var r VerificationResults
	readJSON(t, path, &r)
	return r
}

// ── verify-expansions tests ────────────────────────────────────────────────────

func TestVerifyExpansionsPassesValidStubOutputs(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	var out bytes.Buffer
	if err := VerifyExpansions(runID, &out, VerifyExpansionsOptions{}); err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}

	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))
	if results.Status != "passed" {
		t.Errorf("status = %q; want passed", results.Status)
	}
	if results.SchemaVersion != "verification_results.v1" {
		t.Errorf("schema_version = %q", results.SchemaVersion)
	}
	if results.Summary.ChecksFailed != 0 {
		t.Errorf("checks_failed = %d; want 0", results.Summary.ChecksFailed)
	}
}

func TestVerifyExpansionsMustNotIncludeFailsOnBannedPhrase(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Inject a banned phrase into a caption item.
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) == 0 {
		t.Skip("no expansion items to corrupt")
	}
	output.Items[0].Text = "This text contains unsupported claims from a stub"
	if err := writeJSONFile(capPath, output); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	VerifyExpansions(runID, &out, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	var mustNotCheck *VerificationResultCheck
	for i := range results.Checks {
		if results.Checks[i].Type == "must_not_include" {
			mustNotCheck = &results.Checks[i]
			break
		}
	}
	if mustNotCheck == nil {
		t.Fatal("must_not_include check not found")
	}
	if mustNotCheck.Status != "failed" {
		t.Errorf("must_not_include check status = %q; want failed", mustNotCheck.Status)
	}
	if results.Status != "failed" {
		t.Errorf("overall status = %q; want failed", results.Status)
	}
}

func TestVerifyExpansionsTimestampDriftFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Corrupt timing on a caption item significantly.
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) == 0 {
		t.Skip("no expansion items to corrupt")
	}
	output.Items[0].Start = 999.0
	output.Items[0].End = 1000.0
	if err := writeJSONFile(capPath, output); err != nil {
		t.Fatal(err)
	}

	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	var driftCheck *VerificationResultCheck
	for i := range results.Checks {
		if results.Checks[i].Type == "timestamp_drift" {
			driftCheck = &results.Checks[i]
			break
		}
	}
	if driftCheck == nil {
		t.Fatal("timestamp_drift check not found")
	}
	if driftCheck.Status != "failed" {
		t.Errorf("timestamp_drift check status = %q; want failed", driftCheck.Status)
	}
}

func TestVerifyExpansionsMissingRequiredDecisionsFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Empty out all expansion items so decisions are uncovered.
	for _, taskType := range knownExpansionTaskTypes {
		capPath := filepath.Join(runDir, "expansions", taskType+".json")
		output := readExpansionOutput(t, capPath)
		output.Items = []ExpansionOutputItem{}
		if err := writeJSONFile(capPath, output); err != nil {
			t.Fatal(err)
		}
	}

	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	var missingCheck *VerificationResultCheck
	for i := range results.Checks {
		if results.Checks[i].Type == "missing_required_decisions" {
			missingCheck = &results.Checks[i]
			break
		}
	}
	if missingCheck == nil {
		t.Fatal("missing_required_decisions check not found")
	}
	if missingCheck.Status != "failed" {
		t.Errorf("missing_required_decisions status = %q; want failed", missingCheck.Status)
	}
}

func TestVerifyExpansionsOutputContractCatchesMaxWords(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Bloat text of a caption item far beyond max_words.
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) == 0 {
		t.Skip("no expansion items to corrupt")
	}
	// max_words from expansion_tasks is 18 by default; 50 words will exceed it.
	output.Items[0].Text = strings.Repeat("word ", 50)
	if err := writeJSONFile(capPath, output); err != nil {
		t.Fatal(err)
	}

	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	var contractCheck *VerificationResultCheck
	for i := range results.Checks {
		if results.Checks[i].Type == "output_contract_compliance" {
			contractCheck = &results.Checks[i]
			break
		}
	}
	if contractCheck == nil {
		t.Fatal("output_contract_compliance check not found")
	}
	if contractCheck.Status != "failed" {
		t.Errorf("output_contract_compliance status = %q; want failed", contractCheck.Status)
	}
}

func TestVerifyExpansionsOutputContractCatchesMaxItems(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Add extra items beyond max_items (default 3) for a single decision.
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) == 0 {
		t.Skip("no expansion items")
	}
	decID := output.Items[0].DecisionID
	taskID := output.Items[0].TaskID
	// Add 5 extra items for same decision (total will exceed max_items=3).
	for i := 0; i < 5; i++ {
		output.Items = append(output.Items, ExpansionOutputItem{
			ID: fmt.Sprintf("extra_%d", i), TaskID: taskID,
			DecisionID: decID, Text: "extra caption",
			Start: output.Items[0].Start, End: output.Items[0].End,
			Metadata: map[string]any{"stub": true},
		})
	}
	if err := writeJSONFile(capPath, output); err != nil {
		t.Fatal(err)
	}

	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	var contractCheck *VerificationResultCheck
	for i := range results.Checks {
		if results.Checks[i].Type == "output_contract_compliance" {
			contractCheck = &results.Checks[i]
			break
		}
	}
	if contractCheck == nil {
		t.Fatal("output_contract_compliance check not found")
	}
	if contractCheck.Status != "failed" {
		t.Errorf("output_contract_compliance status = %q; want failed", contractCheck.Status)
	}
}

func TestVerifyExpansionsRequiresVerificationPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	// No verification plan.
	err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{})
	if err == nil || !strings.Contains(err.Error(), "verification.json") {
		t.Fatalf("expected verification.json error, got: %v", err)
	}
}

func TestVerifyExpansionsWritesManifestArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	if err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "verification_results.json") {
		t.Error("manifest missing verification_results.json")
	}
}

func TestVerifyExpansionsJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupVerifyRun(t)

	var out bytes.Buffer
	if err := VerifyExpansions(runID, &out, VerifyExpansionsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var results VerificationResults
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if results.RunID != runID {
		t.Errorf("run_id = %q", results.RunID)
	}
	if results.Mode != "deterministic" {
		t.Errorf("mode = %q", results.Mode)
	}
}

func TestVerifyExpansionsRejectedDecisionsNotRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Mark the only decision as rejected.
	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask, _ := readInferenceMask(maskPath)
	for i := range mask.Decisions {
		mask.Decisions[i].Decision = "reject"
	}
	_ = writeJSONFile(maskPath, mask)

	// Clear all expansion items (rejected decisions should not require expansion).
	for _, taskType := range knownExpansionTaskTypes {
		outPath := filepath.Join(runDir, "expansions", taskType+".json")
		output := readExpansionOutput(t, outPath)
		output.Items = []ExpansionOutputItem{}
		_ = writeJSONFile(outPath, output)
	}

	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))

	for _, rc := range results.Checks {
		if rc.Type == "missing_required_decisions" && rc.Status == "failed" {
			t.Errorf("missing_required_decisions should pass when all decisions are rejected; got: %s", rc.Message)
		}
	}
}

// ── review-verification tests ──────────────────────────────────────────────────

func TestReviewVerificationPrintsResults(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupVerifyRun(t)

	if err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewVerification(runID, &out, ReviewVerificationOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "passed") && !strings.Contains(text, "failed") {
		t.Errorf("expected status in review output: %s", text)
	}
	if !strings.Contains(text, "checks") {
		t.Errorf("expected checks summary in output: %s", text)
	}
}

func TestReviewVerificationWritesMarkdownArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	if err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ReviewVerification(runID, &bytes.Buffer{}, ReviewVerificationOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}

	artPath := filepath.Join(runDir, "verification_review.md")
	data, err := os.ReadFile(artPath)
	if err != nil {
		t.Fatalf("missing verification_review.md: %v", err)
	}
	if !strings.Contains(string(data), "Verification Review") {
		t.Errorf("unexpected content: %s", string(data))
	}

	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "verification_review.md") {
		t.Error("manifest missing verification_review.md")
	}
}

func TestReviewVerificationRequiresResults(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupVerifyRun(t)
	// No verify-expansions run.
	err := ReviewVerification(runID, &bytes.Buffer{}, ReviewVerificationOptions{})
	if err == nil || !strings.Contains(err.Error(), "verification_results.json") {
		t.Fatalf("expected error, got: %v", err)
	}
}

func TestReviewVerificationJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupVerifyRun(t)

	if err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewVerification(runID, &out, ReviewVerificationOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var results VerificationResults
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if results.RunID != runID {
		t.Errorf("run_id = %q", results.RunID)
	}
}

// ── inspect-mask integration ───────────────────────────────────────────────────

func TestInspectMaskShowsVerificationResults(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupVerifyRun(t)

	if err := VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := InspectMask(runID, &out, InspectMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "verification_results") {
		t.Errorf("expected verification_results in inspect-mask: %s", out.String())
	}
}

// ── validateVerificationResultsShape ──────────────────────────────────────────

func TestValidateVerificationResultsShapePassesGoodPayload(t *testing.T) {
	payload := map[string]any{
		"schema_version": "verification_results.v1",
		"status":         "passed",
		"summary":        map[string]any{},
		"checks": []any{
			map[string]any{"id": "c1", "type": "must_not_include", "status": "passed", "message": "ok"},
		},
	}
	errs := validateVerificationResultsShape(payload)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateVerificationResultsShapeCatchesBadVersion(t *testing.T) {
	payload := map[string]any{
		"schema_version": "wrong.v1",
		"status":         "passed",
		"summary":        map[string]any{},
		"checks":         []any{},
	}
	errs := validateVerificationResultsShape(payload)
	if len(errs) == 0 {
		t.Error("expected error for wrong schema_version")
	}
}

func TestVerifyExpansionsToleranceSeconds(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupVerifyRun(t)

	// Introduce a small drift (0.1s) that is within default tolerance (0.25s).
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) == 0 {
		t.Skip("no items")
	}
	original := output.Items[0].Start
	output.Items[0].Start = original + 0.1
	_ = writeJSONFile(capPath, output)

	// Should pass with default tolerance.
	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{}) //nolint
	results := readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))
	for _, rc := range results.Checks {
		if rc.Type == "timestamp_drift" && rc.Status == "failed" {
			t.Errorf("0.1s drift should pass at default 0.25s tolerance; got: %s", rc.Message)
		}
	}

	// Now set strict tolerance of 0.05s — should fail.
	VerifyExpansions(runID, &bytes.Buffer{}, VerifyExpansionsOptions{ToleranceSeconds: 0.05}) //nolint
	results = readVerificationResults(t, filepath.Join(runDir, "verification_results.json"))
	for _, rc := range results.Checks {
		if rc.Type == "timestamp_drift" && rc.Status != "failed" {
			t.Errorf("0.1s drift should fail at 0.05s tolerance; got status: %s", rc.Status)
		}
	}
}

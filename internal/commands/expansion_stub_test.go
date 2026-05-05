package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/manifest"
)

// ── Helpers ────────────────────────────────────────────────────────────────────

func setupExpandStubRun(t *testing.T) (string, string) {
	t.Helper()
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	return runID, runDir
}

func readExpansionOutput(t *testing.T, path string) ExpansionOutput {
	t.Helper()
	var out ExpansionOutput
	readJSON(t, path, &out)
	return out
}

// ── expand-stub tests ──────────────────────────────────────────────────────────

func TestExpandStubGeneratesCaptionVariants(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	var out bytes.Buffer
	if err := ExpandStub(runID, &out, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, outPath)

	if output.SchemaVersion != "expansion_output.v1" {
		t.Errorf("schema_version = %q", output.SchemaVersion)
	}
	if output.Mode != "stub" {
		t.Errorf("mode = %q", output.Mode)
	}
	if output.TaskType != "caption_variants" {
		t.Errorf("task_type = %q", output.TaskType)
	}
	if len(output.Items) == 0 {
		t.Fatal("expected items, got none")
	}
	for _, item := range output.Items {
		if item.ID == "" || item.TaskID == "" || item.DecisionID == "" || item.Text == "" {
			t.Errorf("incomplete item: %+v", item)
		}
		stub, _ := item.Metadata["stub"].(bool)
		if !stub {
			t.Errorf("expected metadata.stub=true, got: %v", item.Metadata)
		}
	}
}

func TestExpandStubGeneratesTimelineLabelsAndShortDescriptions(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	lblPath := filepath.Join(runDir, "expansions", "timeline_labels.json")
	lbl := readExpansionOutput(t, lblPath)
	if lbl.TaskType != "timeline_labels" || len(lbl.Items) == 0 {
		t.Errorf("timeline_labels: type=%q items=%d", lbl.TaskType, len(lbl.Items))
	}

	descPath := filepath.Join(runDir, "expansions", "short_descriptions.json")
	desc := readExpansionOutput(t, descPath)
	if desc.TaskType != "short_descriptions" || len(desc.Items) == 0 {
		t.Errorf("short_descriptions: type=%q items=%d", desc.TaskType, len(desc.Items))
	}
}

func TestExpandStubSkipsRejectedDecisions(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	// Mark the only decision as rejected.
	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask, err := readInferenceMask(maskPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(mask.Decisions) == 0 {
		t.Fatal("no decisions to reject")
	}
	mask.Decisions[0].Decision = "reject"
	if err := writeJSONFile(maskPath, mask); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := ExpandStub(runID, &out, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, outPath)
	if len(output.Items) != 0 {
		t.Errorf("expected no items when all decisions rejected, got %d", len(output.Items))
	}
	if len(output.Warnings) == 0 {
		t.Error("expected warning when all decisions rejected")
	}
}

func TestExpandStubRefusesOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got: %v", err)
	}
}

func TestExpandStubOverwriteFlag(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{Overwrite: true}); err != nil {
		t.Fatalf("expected overwrite to succeed, got: %v", err)
	}
}

func TestExpandStubTaskTypeFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{TaskType: "caption_variants"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(runDir, "expansions", "caption_variants.json")); err != nil {
		t.Fatal("caption_variants.json should exist")
	}
	if _, err := os.Stat(filepath.Join(runDir, "expansions", "timeline_labels.json")); err == nil {
		t.Fatal("timeline_labels.json should not exist when filtered")
	}
}

func TestExpandStubJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	var out bytes.Buffer
	if err := ExpandStub(runID, &out, ExpandStubOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var summary ExpandStubSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if summary.RunID != runID {
		t.Errorf("run_id = %q", summary.RunID)
	}
	if summary.Mode != "stub" {
		t.Errorf("mode = %q", summary.Mode)
	}
	if len(summary.Files) == 0 {
		t.Error("expected files in summary")
	}
}

func TestExpandStubRecordsManifestArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"expansions/caption_variants.json",
		"expansions/timeline_labels.json",
		"expansions/short_descriptions.json",
	} {
		if !manifestHasArtifact(m, name) {
			t.Errorf("manifest missing %s", name)
		}
	}
}

func TestExpandStubRequiresExpansionTasks(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	// No expansion-plan run → no expansion_tasks.json.
	err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{})
	if err == nil || !strings.Contains(err.Error(), "expansion_tasks.json") {
		t.Fatalf("expected expansion_tasks error, got: %v", err)
	}
}

// ── expansion-validate tests ───────────────────────────────────────────────────

func TestExpansionValidateAcceptsValidOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ExpansionValidate(runID, &out, ExpansionValidateOptions{}); err != nil {
		t.Fatalf("validate failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "ok") {
		t.Errorf("expected ok in output: %s", out.String())
	}
}

func TestExpansionValidateRejectsBadTiming(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	// Corrupt caption_variants.json with bad timing.
	capPath := filepath.Join(runDir, "expansions", "caption_variants.json")
	output := readExpansionOutput(t, capPath)
	if len(output.Items) > 0 {
		output.Items[0].Start = 99.0
		output.Items[0].End = 1.0 // end < start
		if err := writeJSONFile(capPath, output); err != nil {
			t.Fatal(err)
		}
	}

	err := ExpansionValidate(runID, &bytes.Buffer{}, ExpansionValidateOptions{})
	if err == nil {
		t.Fatal("expected validation to fail on bad timing")
	}
}

func TestExpansionValidateRejectsRejectedDecisionReference(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}

	// Mark decision as rejected in the mask.
	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask, _ := readInferenceMask(maskPath)
	if len(mask.Decisions) > 0 {
		mask.Decisions[0].Decision = "reject"
		_ = writeJSONFile(maskPath, mask)
	}

	err := ExpansionValidate(runID, &bytes.Buffer{}, ExpansionValidateOptions{})
	if err == nil {
		t.Fatal("expected validation to fail on rejected decision reference")
	}
}

func TestExpansionValidateJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ExpansionValidate(runID, &out, ExpansionValidateOptions{JSON: true}) //nolint
	var result ExpansionValidationResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if result.RunID != runID {
		t.Errorf("run_id = %q", result.RunID)
	}
}

func TestExpansionValidateMissingFilesReportsThemNotError(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)
	// No expand-stub run → no expansion files.
	var out bytes.Buffer
	// Should not error — files are simply missing (not invalid format).
	ExpansionValidate(runID, &out, ExpansionValidateOptions{}) //nolint
	text := out.String()
	if !strings.Contains(text, "missing") {
		t.Errorf("expected 'missing' in output: %s", text)
	}
}

// ── review-expansions tests ────────────────────────────────────────────────────

func TestReviewExpansionsPrintsOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewExpansions(runID, &out, ReviewExpansionsOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "caption_variants") {
		t.Errorf("expected caption_variants in review: %s", text)
	}
}

func TestReviewExpansionsWritesMarkdownArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ReviewExpansions(runID, &bytes.Buffer{}, ReviewExpansionsOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}

	artPath := filepath.Join(runDir, "expansions_review.md")
	data, err := os.ReadFile(artPath)
	if err != nil {
		t.Fatalf("missing expansions_review.md: %v", err)
	}
	if !strings.Contains(string(data), "Expansion Review") {
		t.Errorf("unexpected content: %s", string(data))
	}

	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "expansions_review.md") {
		t.Error("manifest missing expansions_review.md")
	}
}

func TestReviewExpansionsJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewExpansions(runID, &out, ReviewExpansionsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var review ExpansionReview
	if err := json.Unmarshal(out.Bytes(), &review); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if review.RunID != runID {
		t.Errorf("run_id = %q", review.RunID)
	}
	if len(review.Files) == 0 {
		t.Error("expected files in review")
	}
}

// ── inspect-mask integration ───────────────────────────────────────────────────

func TestInspectMaskShowsExpansionOutputs(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := InspectMask(runID, &out, InspectMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "caption_variants") {
		t.Errorf("expected caption_variants in inspect-mask: %s", text)
	}
}

// ── source schema ──────────────────────────────────────────────────────────────

func TestExpandStubSourceFields(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandStub(runID, &bytes.Buffer{}, ExpandStubOptions{}); err != nil {
		t.Fatal(err)
	}
	output := readExpansionOutput(t, filepath.Join(runDir, "expansions", "caption_variants.json"))
	if output.Source.InferenceMaskArtifact != "inference_mask.json" {
		t.Errorf("source.inference_mask_artifact = %q", output.Source.InferenceMaskArtifact)
	}
	if output.Source.ExpansionTasksArtifact != "expansion_tasks.json" {
		t.Errorf("source.expansion_tasks_artifact = %q", output.Source.ExpansionTasksArtifact)
	}
	if len(output.Source.TaskIDs) == 0 {
		t.Error("expected task_ids in source")
	}
}

// ── firstNWords helper ─────────────────────────────────────────────────────────

func TestFirstNWords(t *testing.T) {
	cases := []struct {
		input    string
		n        int
		expected string
	}{
		{"one two three four five", 3, "one two three..."},
		{"one two", 5, "one two"},
		{"", 3, ""},
		{"single", 1, "single"},
		{"one two three", 3, "one two three"},
	}
	for _, c := range cases {
		got := firstNWords(c.input, c.n)
		if got != c.expected {
			t.Errorf("firstNWords(%q, %d) = %q; want %q", c.input, c.n, got, c.expected)
		}
	}
}

// ── validateExpansionOutput helper ────────────────────────────────────────────

func TestValidateExpansionOutputPassesGoodOutput(t *testing.T) {
	out := ExpansionOutput{
		SchemaVersion: "expansion_output.v1",
		CreatedAt:     time.Now().UTC(),
		Mode:          "stub",
		TaskType:      "caption_variants",
		Source: ExpansionOutputSource{
			InferenceMaskArtifact:  "inference_mask.json",
			ExpansionTasksArtifact: "expansion_tasks.json",
			TaskIDs:                []string{"task_0001"},
		},
		Items: []ExpansionOutputItem{
			{ID: "cap_d1_0001", TaskID: "task_0001", DecisionID: "decision_0001", Text: "Test caption", Start: 0, End: 5},
		},
	}
	errs := validateExpansionOutput(out, "caption_variants", map[string]bool{})
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateExpansionOutputCatchesBadSchemaVersion(t *testing.T) {
	out := ExpansionOutput{
		SchemaVersion: "wrong.v1",
		CreatedAt:     time.Now().UTC(),
		Mode:          "stub",
		TaskType:      "caption_variants",
		Source:        ExpansionOutputSource{InferenceMaskArtifact: "x", ExpansionTasksArtifact: "y"},
		Items:         []ExpansionOutputItem{},
	}
	errs := validateExpansionOutput(out, "caption_variants", map[string]bool{})
	if len(errs) == 0 {
		t.Error("expected error for bad schema_version")
	}
}

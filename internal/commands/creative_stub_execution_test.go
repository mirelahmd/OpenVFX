package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/byom-video/internal/config"
)

// helper — creates an approved creative plan with a rich set of step types
func makeApprovedStubPlan(t *testing.T, goal string) string {
	t.Helper()
	if err := os.WriteFile(config.DefaultPath, []byte(`tools:
  enabled: true
  backends:
    local_writer:
      kind: text_generation
      provider: ollama
      model: qwen2.5:7b
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.script: local_writer
`), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreativePlanCommand(inputPath, ioDiscard{}, CreativePlanOptions{Goal: goal}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil || len(entries) == 0 {
		t.Fatal("no creative plan created")
	}
	planID := entries[0].Name()
	if err := ApproveCreativePlan(planID, ioDiscard{}, ApproveCreativePlanOptions{}); err != nil {
		t.Fatal(err)
	}
	return planID
}

func TestCreativeExecuteStub_RejectsUnapproved(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t) // unapproved

	err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{})
	if err == nil {
		t.Fatal("expected error for unapproved plan")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeExecuteStub_YesBypassesApproval(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{Yes: true}); err != nil {
		t.Fatalf("CreativeExecuteStub --yes error: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m["execution_status"] != "stub_completed" {
		t.Fatalf("execution_status = %v, want stub_completed", m["execution_status"])
	}
}

func TestCreativeExecuteStub_DryRunWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	var out bytes.Buffer
	if err := CreativeExecuteStub(planID, &out, CreativeExecuteStubOptions{DryRun: true}); err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	if !strings.Contains(out.String(), "no files written") {
		t.Fatalf("expected dry-run note: %s", out.String())
	}
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if _, err := os.Stat(outputsDir); err == nil {
		t.Fatal("outputs dir should not exist after --dry-run")
	}
}

func TestCreativeExecuteStub_GenerateScript(t *testing.T) {
	t.Chdir(t.TempDir())
	// goal triggers script step
	planID := makeApprovedStubPlan(t, "write a script for narration")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{StepType: "generate_script"}); err != nil {
		t.Fatalf("stub error: %v", err)
	}
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if _, err := os.Stat(filepath.Join(outputsDir, "script_draft.txt")); err != nil {
		t.Fatal("missing script_draft.txt")
	}
	data, err := os.ReadFile(filepath.Join(outputsDir, "script_draft.json"))
	if err != nil {
		t.Fatal("missing script_draft.json")
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("script_draft.json invalid JSON: %v", err)
	}
	if out["schema_version"] != "creative_script.v1" {
		t.Fatalf("schema_version = %v", out["schema_version"])
	}
	if !strings.Contains(out["text"].(string), "Stub script draft") {
		t.Fatalf("text = %v", out["text"])
	}
}

func TestCreativeExecuteStub_GenerateVoiceover(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "narration with voiceover")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{StepType: "generate_voiceover"}); err != nil {
		t.Fatalf("stub error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_plan.json"))
	if err != nil {
		t.Fatal("missing voiceover_plan.json")
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out["schema_version"] != "voiceover_plan.v1" {
		t.Fatalf("schema_version = %v", out["schema_version"])
	}
}

func TestCreativeExecuteStub_GenerateVisualAsset(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "cinematic b-roll video generation")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{StepType: "generate_visual_asset"}); err != nil {
		t.Fatalf("stub error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "visual_asset_prompts.json"))
	if err != nil {
		t.Fatal("missing visual_asset_prompts.json")
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out["schema_version"] != "visual_asset_prompts.v1" {
		t.Fatalf("schema_version = %v", out["schema_version"])
	}
	prompts, _ := out["prompts"].([]any)
	if len(prompts) == 0 {
		t.Fatal("expected at least one prompt")
	}
	p0 := prompts[0].(map[string]any)
	if p0["kind"] != "video_generation" {
		t.Fatalf("visual kind = %v, want video_generation", p0["kind"])
	}
}

func TestCreativeExecuteStub_RenderDraft(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "render a cinematic short")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{StepType: "render_draft"}); err != nil {
		t.Fatalf("stub error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "composition_plan.json"))
	if err != nil {
		t.Fatal("missing composition_plan.json")
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	if out["schema_version"] != "composition_plan.v1" {
		t.Fatalf("schema_version = %v", out["schema_version"])
	}
	if out["planned_output"] != "outputs/draft.mp4" {
		t.Fatalf("planned_output = %v", out["planned_output"])
	}
}

func TestCreativeExecuteStub_OutputsIndexListed(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatalf("stub error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json"))
	if err != nil {
		t.Fatal("missing creative_outputs.json")
	}
	var idx CreativeOutputsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("creative_outputs.json invalid: %v", err)
	}
	if idx.SchemaVersion != "creative_outputs.v1" {
		t.Fatalf("schema_version = %v", idx.SchemaVersion)
	}
	if len(idx.Artifacts) == 0 {
		t.Fatal("expected at least one artifact in index")
	}
	for _, a := range idx.Artifacts {
		if a.Status != "created" {
			t.Fatalf("artifact status = %v, want created", a.Status)
		}
	}
}

func TestReviewCreativeOutputs_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ReviewCreativeOutputs(planID, ioDiscard{}, ReviewCreativeOutputsOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("ReviewCreativeOutputs error: %v", err)
	}
	reviewPath := filepath.Join(creativePlansRoot, planID, "creative_outputs_review.md")
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("missing creative_outputs_review.md: %v", err)
	}
	if !strings.Contains(string(data), "# Creative Outputs Review") {
		t.Fatalf("review missing header: %s", string(data))
	}
}

func TestValidateCreativePlan_CatchesMissingListedArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}

	// corrupt the index by pointing to a non-existent file
	indexPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json")
	raw, _ := os.ReadFile(indexPath)
	var idx map[string]any
	_ = json.Unmarshal(raw, &idx)
	idx["artifacts"] = []any{map[string]any{
		"type":    "script",
		"path":    "outputs/nonexistent_file.json",
		"step_id": "step_0001",
		"status":  "created",
	}}
	data, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(indexPath, data, 0o644)

	err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected validation error for missing artifact path")
	}
}

func TestInspectCreativePlan_ShowsOutputCount(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := InspectCreativePlan(planID, &out, InspectCreativePlanOptions{}); err != nil {
		t.Fatalf("InspectCreativePlan error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "output artifacts:") {
		t.Fatalf("expected output artifacts in inspect output; got: %s", text)
	}
	if !strings.Contains(text, "stub_completed") {
		t.Fatalf("expected stub_completed step status; got: %s", text)
	}
}

func TestCreativeResult_IncludesOutputSummary(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions")

	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := CreativeResult(planID, &out, CreativeResultOptions{}); err != nil {
		t.Fatalf("CreativeResult error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "output artifacts:") {
		t.Fatalf("expected output artifacts in creative-result; got: %s", text)
	}
	if !strings.Contains(text, "stub_completed") {
		t.Fatalf("expected stub_completed execution status; got: %s", text)
	}
}

func TestCreativeExecuteStub_StepTypeFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a short with captions and narration")

	// filter to only generate_captions_or_caption_variants
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{
		StepType: "generate_captions_or_caption_variants",
	}); err != nil {
		t.Fatalf("stub filter error: %v", err)
	}
	outputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
	// caption_plan.json should exist
	if _, err := os.Stat(filepath.Join(outputsDir, "caption_plan.json")); err != nil {
		t.Fatal("missing caption_plan.json")
	}
	// script_draft.json should NOT exist (filtered out)
	if _, err := os.Stat(filepath.Join(outputsDir, "script_draft.json")); err == nil {
		t.Fatal("script_draft.json should not exist when step-type filter is active")
	}
}

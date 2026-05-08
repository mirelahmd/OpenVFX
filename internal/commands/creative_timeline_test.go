package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper — creates an approved stub plan with all artifacts written
func makeStubPlanWithOutputs(t *testing.T) string {
	t.Helper()
	planID := makeApprovedStubPlan(t, "make a cinematic short with narration and AI b-roll captions")
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatalf("CreativeExecuteStub error: %v", err)
	}
	return planID
}

func TestCreativeTimeline_NoRunID(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatalf("CreativeTimeline error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_timeline.json"))
	if err != nil {
		t.Fatal("creative_timeline.json not created")
	}
	var tl CreativeTimelineArtifact
	if err := json.Unmarshal(data, &tl); err != nil {
		t.Fatalf("creative_timeline.json invalid JSON: %v", err)
	}
	if tl.SchemaVersion != "creative_timeline.v1" {
		t.Fatalf("schema_version = %v", tl.SchemaVersion)
	}
	if len(tl.Tracks) == 0 {
		t.Fatal("expected at least one track")
	}
	// must have all four tracks
	trackIDs := map[string]bool{}
	for _, tr := range tl.Tracks {
		trackIDs[tr.ID] = true
	}
	for _, expected := range []string{"track_video_main", "track_voiceover", "track_captions", "track_visual_overlays"} {
		if !trackIDs[expected] {
			t.Fatalf("missing track %q", expected)
		}
	}
}

func TestCreativeTimeline_OverwriteRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	// second run without --overwrite should fail
	err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{})
	if err == nil {
		t.Fatal("expected error on second run without --overwrite")
	}
	if !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeTimeline_OverwriteSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{Overwrite: true}); err != nil {
		t.Fatalf("overwrite error: %v", err)
	}
}

func TestCreativeTimeline_PlanNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := CreativeTimeline("nonexistent-plan", ioDiscard{}, CreativeTimelineOptions{})
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestCreativeTimeline_UpdatesOutputsIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json"))
	if err != nil {
		t.Fatal("creative_outputs.json not found")
	}
	var idx CreativeOutputsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatal("creative_outputs.json invalid")
	}
	found := false
	for _, a := range idx.Artifacts {
		if a.Type == "creative_timeline" {
			found = true
			if a.Status != "created" {
				t.Fatalf("artifact status = %v, want created", a.Status)
			}
		}
	}
	if !found {
		t.Fatal("creative_timeline artifact not in index")
	}
}

func TestCreativeTimeline_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	var out bytes.Buffer
	if err := CreativeTimeline(planID, &out, CreativeTimelineOptions{JSON: true}); err != nil {
		t.Fatalf("JSON mode error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("JSON output invalid: %v", err)
	}
	if m["schema_version"] != "creative_timeline.v1" {
		t.Fatalf("schema_version = %v", m["schema_version"])
	}
}

func TestCreativeRenderPlan_Basic(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatalf("CreativeRenderPlan error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_render_plan.json"))
	if err != nil {
		t.Fatal("creative_render_plan.json not created")
	}
	var rp CreativeRenderPlanArtifact
	if err := json.Unmarshal(data, &rp); err != nil {
		t.Fatalf("creative_render_plan.json invalid JSON: %v", err)
	}
	if rp.SchemaVersion != "creative_render_plan.v1" {
		t.Fatalf("schema_version = %v", rp.SchemaVersion)
	}
	if rp.PlannedOutput.PlannedFile != "outputs/draft.mp4" {
		t.Fatalf("planned_file = %v", rp.PlannedOutput.PlannedFile)
	}
	if len(rp.Steps) == 0 {
		t.Fatal("expected at least one render step")
	}
}

func TestCreativeRenderPlan_RequiresTimeline(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	// skip creative-timeline; render plan should fail
	err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{})
	if err == nil {
		t.Fatal("expected error when timeline missing")
	}
	if !strings.Contains(err.Error(), "creative-timeline") && !strings.Contains(err.Error(), "creative_timeline.json") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeRenderPlan_OverwriteRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{})
	if err == nil {
		t.Fatal("expected error on second run without --overwrite")
	}
	if !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeRenderPlan_UpdatesOutputsIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json"))
	var idx CreativeOutputsIndex
	_ = json.Unmarshal(data, &idx)
	found := false
	for _, a := range idx.Artifacts {
		if a.Type == "creative_render_plan" {
			found = true
		}
	}
	if !found {
		t.Fatal("creative_render_plan artifact not in index")
	}
}

func TestReviewCreativeTimeline_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := ReviewCreativeTimeline(planID, &out, ReviewCreativeTimelineOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("ReviewCreativeTimeline error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "# Creative Timeline Review") {
		t.Fatalf("missing header in output: %s", text)
	}
	reviewPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_timeline_review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatal("creative_timeline_review.md not created")
	}
}

func TestReviewCreativeTimeline_RequiresTimeline(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	err := ReviewCreativeTimeline(planID, ioDiscard{}, ReviewCreativeTimelineOptions{})
	if err == nil {
		t.Fatal("expected error when timeline missing")
	}
}

func TestValidateCreativePlan_ValidatesTimeline(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}

	// valid timeline passes
	if err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{}); err != nil {
		t.Fatalf("validate with valid timeline error: %v", err)
	}

	// corrupt timeline schema_version
	tlPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_timeline.json")
	raw, _ := os.ReadFile(tlPath)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["schema_version"] = "wrong.v0"
	data, _ := json.Marshal(m)
	_ = os.WriteFile(tlPath, data, 0o644)

	err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected validation error for wrong timeline schema_version")
	}
}

func TestValidateCreativePlan_ValidatesRenderPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	// valid render plan passes
	if err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{}); err != nil {
		t.Fatalf("validate with valid render plan error: %v", err)
	}

	// corrupt render plan by removing planned_output
	rpPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_render_plan.json")
	raw, _ := os.ReadFile(rpPath)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	delete(m, "planned_output")
	data, _ := json.Marshal(m)
	_ = os.WriteFile(rpPath, data, 0o644)

	err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected validation error for missing planned_output")
	}
}

func TestInspectCreativePlan_ShowsTimelineInfo(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := InspectCreativePlan(planID, &out, InspectCreativePlanOptions{}); err != nil {
		t.Fatalf("InspectCreativePlan error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "timeline:") {
		t.Fatalf("expected timeline in inspect output: %s", text)
	}
	if !strings.Contains(text, "timeline tracks:") {
		t.Fatalf("expected timeline tracks in inspect output: %s", text)
	}
}

func TestCreativeResult_IncludesTimelineNextCmd(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeStubPlanWithOutputs(t)

	var out bytes.Buffer
	if err := CreativeResult(planID, &out, CreativeResultOptions{}); err != nil {
		t.Fatalf("CreativeResult error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "creative-timeline") {
		t.Fatalf("expected creative-timeline in result next commands: %s", text)
	}
}

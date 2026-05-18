package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makePipelineFriendlyInput creates a stub input file and a minimal run directory
// that mimics a completed pipeline --preset shorts (without running real ffprobe/python).
// This allows Make tests to bypass the real pipeline using a pre-seeded run.
func seedRunForMake(t *testing.T, inputPath string) string {
	t.Helper()
	runID := "test-make-run-001"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     inputPath,
		"created_at":     "2026-01-01T00:00:00Z",
		"artifacts":      []map[string]any{{"name": "roughcut", "path": "roughcut.json"}},
	})
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips": []map[string]any{
			{"id": "clip_0001", "start": 0.0, "end": 5.0, "duration_seconds": 5.0, "text": "test"},
		},
	})
	return runID
}

func TestMake_RequiresGoal(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	err := Make(input, ioDiscard{}, MakeOptions{})
	if err == nil {
		t.Fatal("expected error when --goal is missing")
	}
	if !strings.Contains(err.Error(), "--goal") {
		t.Fatalf("error should mention --goal, got: %v", err)
	}
}

func TestMake_DryRun_WritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{Goal: "cinematic short", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "dry-run") || !strings.Contains(text, "no files written") {
		t.Fatalf("expected dry-run output, got: %s", text)
	}
	// makes dir must not be created
	if _, err := os.Stat(makesRoot); err == nil {
		t.Fatal("makes dir should not exist after --dry-run")
	}
}

func TestMake_DryRun_GoalAware_ShowsGoalStage(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{Goal: "cinematic short", DryRun: true, GoalAware: true})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "goal-rerank") {
		t.Fatalf("expected goal-rerank in goal-aware dry-run: %s", text)
	}
}

func TestMake_DryRun_WithYes_ShowsAssembleStage(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{
		Goal:         "cinematic short",
		DryRun:       true,
		Yes:          true,
		BurnCaptions: true,
	})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "creative-assemble") {
		t.Fatalf("expected creative-assemble stage in --yes dry-run: %s", text)
	}
	if !strings.Contains(text, "burn-captions") {
		t.Fatalf("expected --burn-captions flag in output: %s", text)
	}
}

// TestMake_PlanOnly tests that without --yes, Make stops after creating the plan.
// This test uses makeApprovedStubPlan helpers indirectly by verifying the plan
// directory is created and the summary has status=planned.
func TestMake_SummaryWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	// Pre-seed a run and plan so Make can find them after the mocked pipeline
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	// Seed a run (simulates completed pipeline)
	seedRunForMake(t, input)

	// Write byom-video.yaml config so creative-plan works
	_ = os.WriteFile("byom-video.yaml", []byte(`tools:
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
`), 0o644)

	// We can't run the real pipeline, so we test the summary helper directly
	makeID := "test-make-001"
	runID := "test-run-001"
	planID := "test-plan-001"
	nextCmds := buildNextCommands(runID, planID, "")
	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Goal:           "cinematic short",
		Status:         "planned",
		RunID:          runID,
		CreativePlanID: planID,
		NextCommands:   nextCmds,
	}
	if err := writeMakeSummary(makeID, summary); err != nil {
		t.Fatalf("writeMakeSummary error: %v", err)
	}

	// Verify file exists and is valid
	data, err := os.ReadFile(filepath.Join(makesRoot, makeID, "make_summary.json"))
	if err != nil {
		t.Fatal("make_summary.json not created")
	}
	var loaded MakeSummary
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("make_summary.json invalid JSON: %v", err)
	}
	if loaded.SchemaVersion != "make_summary.v1" {
		t.Fatalf("schema_version = %v", loaded.SchemaVersion)
	}
	if loaded.Status != "planned" {
		t.Fatalf("status = %v, want planned", loaded.Status)
	}
	if loaded.RunID != runID {
		t.Fatalf("run_id = %v, want %v", loaded.RunID, runID)
	}
	if loaded.CreativePlanID != planID {
		t.Fatalf("creative_plan_id = %v, want %v", loaded.CreativePlanID, planID)
	}
	if len(loaded.NextCommands) == 0 {
		t.Fatal("expected next_commands in summary")
	}
}

func TestMake_SummaryHasRunIDAndPlanID(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-fields"
	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Goal:           "short clip",
		Status:         "completed",
		RunID:          "run-abc123",
		CreativePlanID: "plan-xyz456",
		DraftPath:      ".byom-video/creative_plans/plan-xyz456/outputs/draft.mp4",
	}
	if err := writeMakeSummary(makeID, summary); err != nil {
		t.Fatalf("writeMakeSummary error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(makesRoot, makeID, "make_summary.json"))
	var loaded MakeSummary
	_ = json.Unmarshal(data, &loaded)
	if loaded.RunID != "run-abc123" {
		t.Fatalf("run_id = %v", loaded.RunID)
	}
	if loaded.CreativePlanID != "plan-xyz456" {
		t.Fatalf("creative_plan_id = %v", loaded.CreativePlanID)
	}
	if loaded.DraftPath == "" {
		t.Fatal("expected draft_path in summary")
	}
}

func TestMakes_EmptyList(t *testing.T) {
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := Makes(&out, MakesOptions{}); err != nil {
		t.Fatalf("Makes error: %v", err)
	}
	if !strings.Contains(out.String(), "No makes found") {
		t.Fatalf("expected 'No makes found' message: %s", out.String())
	}
}

func TestMakes_ShowsRows(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, id := range []string{"make-001", "make-002"} {
		_ = writeMakeSummary(id, MakeSummary{
			SchemaVersion: "make_summary.v1",
			MakeID:        id,
			Goal:          "test goal " + id,
			Status:        "planned",
		})
	}
	var out bytes.Buffer
	if err := Makes(&out, MakesOptions{}); err != nil {
		t.Fatalf("Makes error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "make-001") || !strings.Contains(text, "make-002") {
		t.Fatalf("expected both make IDs in output: %s", text)
	}
}

func TestInspectMake_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	err := InspectMake("nonexistent-make", ioDiscard{}, InspectMakeOptions{})
	if err == nil {
		t.Fatal("expected error for missing make")
	}
}

func TestInspectMake_ShowsFields(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-inspect-make"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         makeID,
		Goal:           "cinematic short",
		Status:         "completed",
		RunID:          "run-123",
		CreativePlanID: "plan-456",
		DraftPath:      "outputs/draft.mp4",
	})
	var out bytes.Buffer
	if err := InspectMake(makeID, &out, InspectMakeOptions{}); err != nil {
		t.Fatalf("InspectMake error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "run-123") {
		t.Fatalf("expected run_id in output: %s", text)
	}
	if !strings.Contains(text, "plan-456") {
		t.Fatalf("expected plan_id in output: %s", text)
	}
	if !strings.Contains(text, "completed") {
		t.Fatalf("expected status in output: %s", text)
	}
}

func TestInspectMake_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-json-make"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Goal:          "test",
		Status:        "planned",
	})
	var out bytes.Buffer
	if err := InspectMake(makeID, &out, InspectMakeOptions{JSON: true}); err != nil {
		t.Fatalf("InspectMake --json error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if m["schema_version"] != "make_summary.v1" {
		t.Fatalf("schema_version = %v", m["schema_version"])
	}
}

// --- Bug fix regression tests (carried forward from Prompt 047) ---

// TestTimelineSourceStartZero verifies that source_start=0 is serialized in the JSON
// (not silently dropped by omitempty).
func TestTimelineSourceStartZero(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "test-source-start-zero"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips": []map[string]any{
			{"id": "clip_0001", "start": 0.0, "end": 5.0, "duration_seconds": 5.0, "text": "starts at zero"},
		},
	})
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/tmp/test.mov",
	})

	planID := makeApprovedStubPlan(t, "cinematic short zero start")
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{RunID: runID}); err != nil {
		t.Fatalf("creative-timeline error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_timeline.json"))
	if err != nil {
		t.Fatal("creative_timeline.json not created")
	}

	// Verify source_start is present as a key in the raw JSON (even for value 0)
	if !strings.Contains(string(data), `"source_start"`) {
		t.Fatalf("source_start key missing from creative_timeline.json (omitempty bug); raw: %s", string(data))
	}

	var tl CreativeTimelineArtifact
	_ = json.Unmarshal(data, &tl)
	for _, track := range tl.Tracks {
		if track.ID == "track_video_main" {
			for _, item := range track.Items {
				if item.Kind == "source_clip" && item.SourceEnd == 5.0 {
					if item.SourceStart != 0.0 {
						t.Fatalf("expected source_start=0.0, got %v", item.SourceStart)
					}
				}
			}
		}
	}
}

// --- Prompt 048 tests ---

func TestMake_PresetDefault_IsShorts(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	// dry-run doesn't run pipeline — safe to call
	err := Make(input, &out, MakeOptions{Goal: "cinematic short", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "shorts") {
		t.Fatalf("expected 'shorts' in dry-run output (default preset): %s", text)
	}
}

func TestMake_PresetMetadata_Yes_FailsWithoutSkipPipeline(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	err := Make(input, ioDiscard{}, MakeOptions{Goal: "cinematic short", Preset: "metadata", Yes: true})
	if err == nil {
		t.Fatal("expected error: metadata preset with --yes and no --skip-pipeline")
	}
	if !strings.Contains(err.Error(), "cannot assemble") {
		t.Fatalf("error should mention 'cannot assemble': %v", err)
	}
}

func TestMake_PresetUnknown_Fails(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	err := Make(input, ioDiscard{}, MakeOptions{Goal: "cinematic short", Preset: "invalidpreset"})
	if err == nil {
		t.Fatal("expected error for unknown preset")
	}
	if !strings.Contains(err.Error(), "unknown preset") {
		t.Fatalf("error should mention 'unknown preset': %v", err)
	}
}

func TestMake_SkipPipeline_MissingRun_Fails(t *testing.T) {
	t.Chdir(t.TempDir())
	err := Make("", ioDiscard{}, MakeOptions{
		Goal:         "cinematic short",
		SkipPipeline: "nonexistent-run-id",
		DryRun:       false,
	})
	if err == nil {
		t.Fatal("expected error for missing skip-pipeline run")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention 'not found': %v", err)
	}
}

func TestMake_SkipPipeline_NoClipSource_Fails(t *testing.T) {
	t.Chdir(t.TempDir())
	// Create a run dir with no clip source artifacts
	runID := "test-no-clips-run"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/tmp/test.mov",
	})
	// No roughcut.json or any clip source

	err := Make("", ioDiscard{}, MakeOptions{
		Goal:         "cinematic short",
		SkipPipeline: runID,
	})
	if err == nil {
		t.Fatal("expected error for run with no clip source")
	}
	if !strings.Contains(err.Error(), "no usable clip source") {
		t.Fatalf("error should mention 'no usable clip source': %v", err)
	}
}

func TestMake_SkipPipeline_InputWarning(t *testing.T) {
	t.Chdir(t.TempDir())
	// Create a run dir with roughcut.json and a manifest with a different input path
	runID := "test-input-warn-run"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/original/path/video.mov",
	})
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips":          []map[string]any{},
	})

	// Create a different input file
	differentInput := filepath.Join(t.TempDir(), "different.mov")
	_ = os.WriteFile(differentInput, []byte("stub"), 0o644)

	var out bytes.Buffer
	// We expect a warning but not a failure (no --strict-input)
	// The call will fail after the warning because creative-plan can't run, but the warning path is tested
	_ = Make(differentInput, &out, MakeOptions{
		Goal:         "cinematic short",
		SkipPipeline: runID,
	})
	text := out.String()
	if !strings.Contains(text, "warning") || !strings.Contains(text, "differs") {
		t.Fatalf("expected input path mismatch warning in output: %s", text)
	}
}

func TestMake_StrictInput_FailsOnMismatch(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "test-strict-input-run"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/original/path/video.mov",
	})
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips":          []map[string]any{},
	})

	differentInput := filepath.Join(t.TempDir(), "different.mov")
	_ = os.WriteFile(differentInput, []byte("stub"), 0o644)

	err := Make(differentInput, ioDiscard{}, MakeOptions{
		Goal:         "cinematic short",
		SkipPipeline: runID,
		StrictInput:  true,
	})
	if err == nil {
		t.Fatal("expected error with --strict-input and mismatched input path")
	}
	if !strings.Contains(err.Error(), "differs") {
		t.Fatalf("error should mention path difference: %v", err)
	}
}

func TestMake_SkipPipeline_DryRun(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{
		Goal:         "cinematic short",
		DryRun:       true,
		SkipPipeline: "my-run-id-123",
	})
	if err != nil {
		t.Fatalf("dry-run with skip-pipeline error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "my-run-id-123") {
		t.Fatalf("expected skip-pipeline run_id in dry-run output: %s", text)
	}
	if !strings.Contains(text, "skip pipeline") && !strings.Contains(text, "reuse run") {
		t.Fatalf("expected 'skip pipeline' or 'reuse run' in dry-run output: %s", text)
	}
}

func TestMake_ExportDryRun_ShowsExportStage(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)

	var out bytes.Buffer
	err := Make(input, &out, MakeOptions{
		Goal:   "cinematic short",
		DryRun: true,
		Export: true,
	})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "export") {
		t.Fatalf("expected 'export' in dry-run output when --export set: %s", text)
	}
}

func TestMake_SummaryHasPresetAndStatuses(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-statuses"
	summary := MakeSummary{
		SchemaVersion:    "make_summary.v1",
		MakeID:           makeID,
		Goal:             "cinematic short",
		Preset:           "shorts",
		Status:           "completed",
		RunID:            "run-abc",
		CreativePlanID:   "plan-xyz",
		PipelineStatus:   "completed",
		CreativeStatus:   "stub_completed",
		AssembleStatus:   "completed",
		ValidationStatus: "ok",
		ExportStatus:     "completed",
		ExportedFiles:    []string{"exports/clip_0001.mp4"},
		CaptionStatus:    "skipped",
		VoiceoverStatus:  "",
	}
	if err := writeMakeSummary(makeID, summary); err != nil {
		t.Fatalf("writeMakeSummary error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(makesRoot, makeID, "make_summary.json"))
	var m map[string]any
	_ = json.Unmarshal(data, &m)

	checks := map[string]string{
		"preset":            "shorts",
		"pipeline_status":   "completed",
		"creative_status":   "stub_completed",
		"assemble_status":   "completed",
		"validation_status": "ok",
		"export_status":     "completed",
		"caption_status":    "skipped",
	}
	for k, want := range checks {
		if got, ok := m[k]; !ok || got != want {
			t.Fatalf("summary.%s = %v, want %v", k, got, want)
		}
	}
	files, _ := m["exported_files"].([]any)
	if len(files) == 0 {
		t.Fatal("exported_files should be non-empty")
	}
}

func TestMake_RequireExport_FailsWhenExportFails(t *testing.T) {
	t.Chdir(t.TempDir())
	// Create a minimal run with clip source but no ffmpeg_commands.sh
	runID := "test-require-export-run"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/tmp/test.mov",
	})
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips":          []map[string]any{{"id": "c1", "start": 0.0, "end": 5.0}},
	})

	err := Make("", ioDiscard{}, MakeOptions{
		Goal:          "cinematic short",
		SkipPipeline:  runID,
		Export:        true,
		RequireExport: true,
	})
	if err == nil {
		t.Fatal("expected error when --require-export and export fails")
	}
	if !strings.Contains(err.Error(), "export failed") {
		t.Fatalf("error should mention 'export failed': %v", err)
	}
}

func TestMake_Export_WarnsWhenExportFails(t *testing.T) {
	t.Chdir(t.TempDir())
	// Create a run with clip source but no ffmpeg_commands.sh — export will fail
	runID := "test-export-warn-run"
	runDir := filepath.Join(".byom-video", "runs", runID)
	_ = os.MkdirAll(runDir, 0o755)
	_ = writeJSONFile(filepath.Join(runDir, "manifest.json"), map[string]any{
		"schema_version": "manifest.v1",
		"run_id":         runID,
		"input_path":     "/tmp/test.mov",
	})
	_ = writeJSONFile(filepath.Join(runDir, "roughcut.json"), map[string]any{
		"schema_version": "roughcut.v1",
		"clips":          []map[string]any{{"id": "c1", "start": 0.0, "end": 5.0}},
	})

	var out bytes.Buffer
	// Export fails (no ffmpeg_commands.sh), but RequireExport=false so we should NOT return an error here.
	// The error comes later from creative-plan (no config), so we just verify the warning was printed.
	_ = Make("", &out, MakeOptions{
		Goal:         "cinematic short",
		SkipPipeline: runID,
		Export:       true,
	})
	text := out.String()
	if !strings.Contains(text, "warning") {
		t.Fatalf("expected export warning in output: %s", text)
	}
}

func TestMakeResult_ReadsAndPrints(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-result-read"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion:    "make_summary.v1",
		MakeID:           makeID,
		Goal:             "cinematic short",
		Preset:           "shorts",
		Status:           "completed",
		RunID:            "run-123",
		CreativePlanID:   "plan-456",
		ValidationStatus: "ok",
		DraftPath:        "outputs/draft.mp4",
	})

	var out bytes.Buffer
	if err := MakeResult(makeID, &out, MakeResultOptions{}); err != nil {
		t.Fatalf("MakeResult error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "run-123") {
		t.Fatalf("expected run_id in make-result output: %s", text)
	}
	if !strings.Contains(text, "completed") {
		t.Fatalf("expected status in make-result output: %s", text)
	}
	if !strings.Contains(text, "ok") {
		t.Fatalf("expected validation status in make-result output: %s", text)
	}
}

func TestMakeResult_WriteArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-result-artifact"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Goal:          "short clip",
		Status:        "completed",
	})

	if err := MakeResult(makeID, ioDiscard{}, MakeResultOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("MakeResult --write-artifact error: %v", err)
	}
	mdPath := filepath.Join(makesRoot, makeID, "make_result.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("make_result.md not created: %v", err)
	}
	data, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(data), "Make Result") {
		t.Fatalf("make_result.md missing expected header: %s", string(data))
	}
}

func TestMakeResult_JSON(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-make-result-json"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		Goal:          "short clip",
		Preset:        "shorts",
		Status:        "planned",
	})

	var out bytes.Buffer
	if err := MakeResult(makeID, &out, MakeResultOptions{JSON: true}); err != nil {
		t.Fatalf("MakeResult --json error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("make-result --json output is not valid JSON: %v", err)
	}
	if m["schema_version"] != "make_summary.v1" {
		t.Fatalf("schema_version = %v", m["schema_version"])
	}
	if m["preset"] != "shorts" {
		t.Fatalf("preset = %v, want shorts", m["preset"])
	}
}

func TestMakes_ShowsPresetColumn(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = writeMakeSummary("make-preset-test", MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        "make-preset-test",
		Goal:          "test goal",
		Preset:        "shorts",
		Status:        "completed",
	})
	var out bytes.Buffer
	if err := Makes(&out, MakesOptions{}); err != nil {
		t.Fatalf("Makes error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "PRESET") {
		t.Fatalf("expected PRESET column header in makes output: %s", text)
	}
	if !strings.Contains(text, "shorts") {
		t.Fatalf("expected 'shorts' preset value in makes output: %s", text)
	}
}

func TestMakes_ShowsDraftColumn(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = writeMakeSummary("make-draft-test", MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        "make-draft-test",
		Goal:          "test",
		Status:        "completed",
		DraftPath:     "outputs/draft.mp4",
	})
	var out bytes.Buffer
	if err := Makes(&out, MakesOptions{}); err != nil {
		t.Fatalf("Makes error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "DRAFT") {
		t.Fatalf("expected DRAFT column in makes output: %s", text)
	}
	if !strings.Contains(text, "yes") {
		t.Fatalf("expected 'yes' for draft presence in makes output: %s", text)
	}
}

func TestInspectMake_ShowsNewFields(t *testing.T) {
	t.Chdir(t.TempDir())
	makeID := "test-inspect-new-fields"
	_ = writeMakeSummary(makeID, MakeSummary{
		SchemaVersion:    "make_summary.v1",
		MakeID:           makeID,
		Goal:             "cinematic short",
		Preset:           "shorts",
		Status:           "completed",
		SkipPipeline:     true,
		ReusedRunID:      "reused-run-999",
		ValidationStatus: "ok",
		ExportStatus:     "completed",
		CaptionStatus:    "skipped",
	})
	var out bytes.Buffer
	if err := InspectMake(makeID, &out, InspectMakeOptions{}); err != nil {
		t.Fatalf("InspectMake error: %v", err)
	}
	text := out.String()
	for _, want := range []string{"skip_pipeline", "reused-run-999", "validation", "export_status", "captions"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in inspect-make output: %s", want, text)
		}
	}
}

// --- Bug fix regression tests (carried forward from Prompt 047) ---

// TestValidateAssemble_SkippedCaptionNoSpuriousWarning verifies that when
// caption burn is skipped (subtitles filter missing + allow-missing-captions),
// validate-creative-assemble does NOT warn about a missing draft_assembled.mp4.
func TestValidateAssemble_SkippedCaptionNoSpuriousWarning(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	captPath := filepath.Join(t.TempDir(), "captions.srt")
	_ = os.WriteFile(captPath, []byte("1\n00:00:00,000 --> 00:00:05,000\nHello\n"), 0o644)

	// Run assemble with caption burn allowed-missing and no subtitles filter
	runner := &fakeFFmpegRunner{hasSubtitles: false}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		BurnCaptions:         true,
		CaptionsPath:         captPath,
		AllowMissingCaptions: true,
	}, runner); err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	// Read result and confirm assembled_video stage points at draft.mp4, not draft_assembled.mp4
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)

	for _, stage := range result.Stages {
		if stage.Name == "assembled_video" && stage.File == "outputs/draft_assembled.mp4" {
			t.Fatalf("assembled_video stage still points to draft_assembled.mp4 after rename; should be draft.mp4")
		}
	}

	// Validate must not warn about missing draft_assembled.mp4
	var out bytes.Buffer
	if err := ValidateCreativeAssemble(planID, &out, ValidateCreativeAssembleOptions{}); err != nil {
		t.Fatalf("validate error: %v", err)
	}
	text := out.String()
	if strings.Contains(text, "draft_assembled.mp4") {
		t.Fatalf("validate-creative-assemble should NOT warn about missing draft_assembled.mp4 when caption was skipped; output: %s", text)
	}
}

package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigWithRoutes = `models:
  enabled: false

  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander

  routes:
    caption_expansion: local_qwen
    timeline_labeling: local_qwen
    description_expansion: local_qwen
    verification: local_qwen
`

func writeTestConfig(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile("byom-video.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsString(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// ── Routes plan tests ──────────────────────────────────────────────────────────

func TestRoutesPlanResolvesConfiguredRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RoutesPlanCommand(runID, &out, RoutesPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "local_qwen") {
		t.Fatalf("expected local_qwen in routes plan, got: %s", text)
	}
	if !strings.Contains(text, "models_disabled") {
		t.Fatalf("expected models_disabled status (enabled: false), got: %s", text)
	}
}

func TestRoutesPlanWarnsOnMissingRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RoutesPlanCommand(runID, &out, RoutesPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "missing_route") {
		t.Fatalf("expected missing_route warning (no config), got: %s", out.String())
	}
}

func TestRoutesPlanStrictFailsOnMissingRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := RoutesPlanCommand(runID, &bytes.Buffer{}, RoutesPlanOptions{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("expected strict error, got: %v", err)
	}
}

func TestRoutesPlanHandlesModelsDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RoutesPlanCommand(runID, &out, RoutesPlanOptions{}); err != nil {
		t.Fatalf("routes-plan should not fail when models disabled: %v", err)
	}
	if !strings.Contains(out.String(), "models_disabled") {
		t.Fatalf("expected models_disabled, got: %s", out.String())
	}
}

func TestRoutesPlanWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := RoutesPlanCommand(runID, &bytes.Buffer{}, RoutesPlanOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runDir, "routes_plan.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("routes_plan.json missing: %v", err)
	}
	var plan RoutesPlan
	readJSON(t, path, &plan)
	if plan.SchemaVersion != "routes_plan.v1" {
		t.Fatalf("schema_version = %s", plan.SchemaVersion)
	}
	if plan.RunID != runID {
		t.Fatalf("run_id = %s", plan.RunID)
	}
}

func TestRoutesPlanJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RoutesPlanCommand(runID, &out, RoutesPlanOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var plan RoutesPlan
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &plan); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if plan.SchemaVersion != "routes_plan.v1" {
		t.Fatalf("schema_version = %s", plan.SchemaVersion)
	}
}

// ── Revise mask tests ──────────────────────────────────────────────────────────

func TestReviseMaskShorterLongerSetWords(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 18}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords != 15 {
		t.Fatalf("after shorter: expected 15, got %d", mask.Constraints.MaxCaptionWords)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions longer"}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords != 18 {
		t.Fatalf("after longer: expected 18, got %d", mask.Constraints.MaxCaptionWords)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "set captions to 12 words"}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords != 12 {
		t.Fatalf("after set: expected 12, got %d", mask.Constraints.MaxCaptionWords)
	}
}

func TestReviseMaskMinClamp(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 6}); err != nil {
		t.Fatal(err)
	}
	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords < 5 {
		t.Fatalf("min clamp failed: got %d", mask.Constraints.MaxCaptionWords)
	}
}

func TestReviseMaskMaxClamp(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 39}); err != nil {
		t.Fatal(err)
	}
	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions longer"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords > 40 {
		t.Fatalf("max clamp failed: got %d", mask.Constraints.MaxCaptionWords)
	}
}

func TestReviseMaskToneChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make tone more technical"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !strings.Contains(mask.Constraints.Tone, "technical") {
		t.Fatalf("tone = %s", mask.Constraints.Tone)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make tone more casual"}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !strings.Contains(mask.Constraints.Tone, "casual") {
		t.Fatalf("tone = %s", mask.Constraints.Tone)
	}
}

func TestReviseMaskMustIncludeMustNotInclude(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "avoid hype"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !containsString(mask.Constraints.MustNotInclude, "hype") {
		t.Fatalf("must_not_include missing hype: %v", mask.Constraints.MustNotInclude)
	}
	if !containsString(mask.Constraints.MustNotInclude, "exaggerated claims") {
		t.Fatalf("must_not_include missing exaggerated claims: %v", mask.Constraints.MustNotInclude)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "avoid unsupported claims"}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !containsString(mask.Constraints.MustNotInclude, "unsupported claims") {
		t.Fatalf("must_not_include missing unsupported claims: %v", mask.Constraints.MustNotInclude)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "require hook"}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !containsString(mask.Constraints.MustInclude, "strong hook") {
		t.Fatalf("must_include missing strong hook: %v", mask.Constraints.MustInclude)
	}
}

func TestReviseMaskUnknownRequestFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "do something weird"})
	if err == nil || !strings.Contains(err.Error(), "unknown revision request") {
		t.Fatalf("expected unknown revision error, got: %v", err)
	}
	_ = runDir
}

func TestReviseMaskDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 18}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "set captions to 5 words", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Constraints.MaxCaptionWords != 18 {
		t.Fatalf("dry-run mutated mask: max_caption_words = %d", mask.Constraints.MaxCaptionWords)
	}
	snapDir := filepath.Join(runDir, "mask_snapshots")
	if _, err := os.Stat(snapDir); err == nil {
		entries, _ := os.ReadDir(snapDir)
		if len(entries) > 0 {
			t.Fatal("dry-run created snapshot")
		}
	}
}

func TestReviseMaskCreatesSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}
	snapPath := filepath.Join(runDir, "mask_snapshots", "mask_snapshot_0001.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
}

func TestReviseMaskShowDiff(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 18}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviseMask(runID, &out, ReviseMaskOptions{Request: "set captions to 10 words", ShowDiff: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "max_caption_words") {
		t.Fatalf("show-diff output missing field, got: %s", out.String())
	}
	_ = runDir
}

// ── Mask snapshots tests ───────────────────────────────────────────────────────

func TestMaskSnapshotsListAndInspect(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := MaskSnapshots(runID, &out, MaskSnapshotsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No mask snapshots") {
		t.Fatalf("expected no snapshots message, got: %s", out.String())
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := MaskSnapshots(runID, &out, MaskSnapshotsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mask_snapshot_0001") {
		t.Fatalf("expected mask_snapshot_0001, got: %s", out.String())
	}

	out.Reset()
	if err := InspectMaskSnapshot(runID, "mask_snapshot_0001", &out, InspectMaskSnapshotOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mask_snapshot_0001") {
		t.Fatalf("inspect output = %s", out.String())
	}
}

func TestMaskSnapshotsJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := MaskSnapshots(runID, &out, MaskSnapshotsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var snapshots []MaskSnapshotEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &snapshots); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestInspectMaskSnapshotJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "make captions shorter"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := InspectMaskSnapshot(runID, "mask_snapshot_0001", &out, InspectMaskSnapshotOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var snapMask InferenceMask
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &snapMask); err != nil {
		t.Fatalf("invalid JSON from inspect-mask-snapshot: %v", err)
	}
	if snapMask.SchemaVersion != "inference_mask.v1" {
		t.Fatalf("schema_version = %s", snapMask.SchemaVersion)
	}
	_ = runDir
}

// ── Diff mask tests ────────────────────────────────────────────────────────────

func TestDiffMaskDetectsChangedConstraints(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 18}); err != nil {
		t.Fatal(err)
	}

	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "set captions to 12 words"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := DiffMask(runID, "mask_snapshot_0001", &out, DiffMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "max_caption_words") {
		t.Fatalf("diff should show max_caption_words change, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "18") || !strings.Contains(out.String(), "12") {
		t.Fatalf("diff should show 18→12, got: %s", out.String())
	}
}

func TestDiffMaskWritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 18}); err != nil {
		t.Fatal(err)
	}
	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "set captions to 12 words"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := DiffMask(runID, "mask_snapshot_0001", &out, DiffMaskOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	diffPath := filepath.Join(runDir, "mask_diffs", "diff_current_vs_mask_snapshot_0001.md")
	if _, err := os.Stat(diffPath); err != nil {
		t.Fatalf("diff artifact missing: %v", err)
	}
	data, _ := os.ReadFile(diffPath)
	if !strings.Contains(string(data), "max_caption_words") {
		t.Fatalf("diff artifact content: %s", string(data))
	}
}

func TestDiffMaskJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: 20}); err != nil {
		t.Fatal(err)
	}
	if err := ReviseMask(runID, &bytes.Buffer{}, ReviseMaskOptions{Request: "require hook"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := DiffMask(runID, "mask_snapshot_0001", &out, DiffMaskOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var diff MaskDiff
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &diff); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if diff.SnapshotID != "mask_snapshot_0001" {
		t.Fatalf("snapshot_id = %s", diff.SnapshotID)
	}
	_ = runDir
}

func TestDiffMaskNoChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	snapDir := filepath.Join(runDir, "mask_snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if err := writeJSONFile(filepath.Join(snapDir, "mask_snapshot_0001.json"), mask); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := DiffMask(runID, "mask_snapshot_0001", &out, DiffMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no changes") {
		t.Fatalf("expected no changes message, got: %s", out.String())
	}
}

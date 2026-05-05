package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"byom-video/internal/manifest"
)

// ── mask-decisions ──────────────────────────────────────────────────────────────

func TestMaskDecisionsListsAll(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := MaskDecisionsList(runID, &out, MaskDecisionsOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "decision_0001") {
		t.Fatalf("decisions list = %s", out.String())
	}
}

func TestMaskDecisionsListJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := MaskDecisionsList(runID, &out, MaskDecisionsOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var decisions []MaskDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &decisions); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	_ = runDir
}

// ── mask-decision (set) ────────────────────────────────────────────────────────

func TestMaskDecisionUpdatesValue(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := MaskDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskDecisionOptions{Set: "reject"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Decisions[0].Decision != "reject" {
		t.Fatalf("decision = %s", mask.Decisions[0].Decision)
	}
}

func TestMaskDecisionAppendsReason(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := MaskDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskDecisionOptions{Set: "keep", Reason: "approved by editor"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if !strings.Contains(mask.Decisions[0].Reason, "Manual note: approved by editor") {
		t.Fatalf("reason = %s", mask.Decisions[0].Reason)
	}
}

func TestMaskDecisionUnknownIDFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	err := MaskDecisionCommand(runID, "decision_9999", &bytes.Buffer{},
		MaskDecisionOptions{Set: "reject"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
	_ = runDir
}

func TestMaskDecisionInvalidSetFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	err := MaskDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskDecisionOptions{Set: "approve"})
	if err == nil || !strings.Contains(err.Error(), "invalid --set") {
		t.Fatalf("expected invalid set error, got: %v", err)
	}
	_ = runDir
}

func TestMaskDecisionDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := MaskDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskDecisionOptions{Set: "reject", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Decisions[0].Decision != "keep" {
		t.Fatalf("dry-run mutated decision: got %s", mask.Decisions[0].Decision)
	}
	snapDir := filepath.Join(runDir, "mask_snapshots")
	if _, err := os.Stat(snapDir); err == nil {
		entries, _ := os.ReadDir(snapDir)
		if len(entries) > 0 {
			t.Fatal("dry-run created snapshot")
		}
	}
}

func TestMaskDecisionCreatesSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := MaskDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskDecisionOptions{Set: "reject"}); err != nil {
		t.Fatal(err)
	}
	snapPath := filepath.Join(runDir, "mask_snapshots", "mask_snapshot_0001.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
}

func TestMaskDecisionJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := MaskDecisionCommand(runID, "decision_0001", &out,
		MaskDecisionOptions{Set: "candidate_keep", JSON: true}); err != nil {
		t.Fatal(err)
	}
	var result MaskDecisionResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if result.NewValue != "candidate_keep" {
		t.Fatalf("new_value = %s", result.NewValue)
	}
	_ = runDir
}

// ── mask-remove-decision ────────────────────────────────────────────────────────

func TestMaskRemoveDecisionRemoves(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if len(mask.Decisions) < 2 {
		t.Fatalf("expected at least 2 decisions, got %d", len(mask.Decisions))
	}

	if err := MaskRemoveDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskRemoveDecisionOptions{}); err != nil {
		t.Fatal(err)
	}
	mask = readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if len(mask.Decisions) != 1 {
		t.Fatalf("expected 1 decision after remove, got %d", len(mask.Decisions))
	}
	for _, d := range mask.Decisions {
		if d.ID == "decision_0001" {
			t.Fatal("decision_0001 still present after remove")
		}
	}
}

func TestMaskRemoveDecisionUnknownIDFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	err := MaskRemoveDecisionCommand(runID, "decision_9999", &bytes.Buffer{},
		MaskRemoveDecisionOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
	_ = runDir
}

func TestMaskRemoveDecisionDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	if err := MaskRemoveDecisionCommand(runID, "decision_0001", &bytes.Buffer{},
		MaskRemoveDecisionOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	found := false
	for _, d := range mask.Decisions {
		if d.ID == "decision_0001" {
			found = true
		}
	}
	if !found {
		t.Fatal("dry-run removed decision_0001")
	}
}

// ── mask-reorder ────────────────────────────────────────────────────────────────

func TestMaskReorderValidOrder(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	if err := MaskReorderCommand(runID, &bytes.Buffer{},
		MaskReorderOptions{Order: "decision_0002,decision_0001"}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Decisions[0].ID != "decision_0002" {
		t.Fatalf("expected decision_0002 first, got %s", mask.Decisions[0].ID)
	}
	if mask.Decisions[1].ID != "decision_0001" {
		t.Fatalf("expected decision_0001 second, got %s", mask.Decisions[1].ID)
	}
}

func TestMaskReorderMissingIDFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	err := MaskReorderCommand(runID, &bytes.Buffer{},
		MaskReorderOptions{Order: "decision_0001,decision_9999"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
	_ = runDir
}

func TestMaskReorderWrongCountFails(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	err := MaskReorderCommand(runID, &bytes.Buffer{},
		MaskReorderOptions{Order: "decision_0001"})
	if err == nil || !strings.Contains(err.Error(), "exactly once") {
		t.Fatalf("expected count error, got: %v", err)
	}
	_ = runDir
}

func TestMaskReorderDryRunDoesNotMutate(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	if err := MaskReorderCommand(runID, &bytes.Buffer{},
		MaskReorderOptions{Order: "decision_0002,decision_0001", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if mask.Decisions[0].ID != "decision_0001" {
		t.Fatalf("dry-run mutated order: first is %s", mask.Decisions[0].ID)
	}
}

func TestMaskReorderCreatesSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{TopK: 2}); err != nil {
		t.Fatal(err)
	}

	if err := MaskReorderCommand(runID, &bytes.Buffer{},
		MaskReorderOptions{Order: "decision_0002,decision_0001"}); err != nil {
		t.Fatal(err)
	}
	snapPath := filepath.Join(runDir, "mask_snapshots", "mask_snapshot_0001.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot missing: %v", err)
	}
}

// ── validate proposed mask ─────────────────────────────────────────────────────

func TestValidateProposedMaskBlocksBadDecision(t *testing.T) {
	mask := InferenceMask{
		SchemaVersion: "inference_mask.v1",
		Intent:        "test",
		Decisions: []MaskDecision{
			{ID: "decision_0001", Decision: "invalid_value", Start: 0, End: 5},
		},
	}
	if err := validateProposedMask(mask); err == nil {
		t.Fatal("expected validation error for invalid decision value")
	}
}

func TestValidateProposedMaskPassesGoodMask(t *testing.T) {
	mask := InferenceMask{
		SchemaVersion: "inference_mask.v1",
		Intent:        "test",
		Decisions: []MaskDecision{
			{ID: "decision_0001", Decision: "keep", Start: 0, End: 5},
		},
	}
	if err := validateProposedMask(mask); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── route-preview ──────────────────────────────────────────────────────────────

func TestRoutePreviewBuildsTaskPayload(t *testing.T) {
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
	if err := RoutePreviewCommand(runID, &out, RoutePreviewOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "task_0001") {
		t.Fatalf("route preview = %s", out.String())
	}
	if !strings.Contains(out.String(), "caption_variants") {
		t.Fatalf("route preview missing task type: %s", out.String())
	}
}

func TestRoutePreviewHandlesMissingModelRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	// No config → all routes missing
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RoutePreviewCommand(runID, &out, RoutePreviewOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "missing_route") {
		t.Fatalf("expected missing_route, got: %s", out.String())
	}
}

func TestRoutePreviewWritesArtifact(t *testing.T) {
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
	if err := RoutePreviewCommand(runID, &bytes.Buffer{}, RoutePreviewOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runDir, "route_preview.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("route_preview.json missing: %v", err)
	}
	var preview RoutePreview
	readJSON(t, path, &preview)
	if preview.SchemaVersion != "route_preview.v1" {
		t.Fatalf("schema_version = %s", preview.SchemaVersion)
	}
	if len(preview.Tasks) == 0 {
		t.Fatal("expected tasks in preview")
	}
}

func TestRoutePreviewManifestRecord(t *testing.T) {
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
	if err := RoutePreviewCommand(runID, &bytes.Buffer{}, RoutePreviewOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "route_preview.json") {
		t.Fatalf("manifest missing route_preview.json: %#v", m.Artifacts)
	}
}

func TestRoutePreviewJSONOutput(t *testing.T) {
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
	if err := RoutePreviewCommand(runID, &out, RoutePreviewOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var preview RoutePreview
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &preview); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out.String())
	}
	if preview.SchemaVersion != "route_preview.v1" {
		t.Fatalf("schema_version = %s", preview.SchemaVersion)
	}
	_ = runDir
}

func TestRoutePreviewRequiresExpansionTasks(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	// No expansion_tasks.json
	err := RoutePreviewCommand(runID, &bytes.Buffer{}, RoutePreviewOptions{})
	if err == nil || !strings.Contains(err.Error(), "expansion_tasks") {
		t.Fatalf("expected expansion_tasks error, got: %v", err)
	}
	_ = runDir
}

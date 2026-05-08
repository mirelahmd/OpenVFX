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

// helper — creates a minimal creative plan on disk and returns the plan_id
func makeTestCreativePlan(t *testing.T) string {
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
	if err := CreativePlanCommand(inputPath, ioDiscard{}, CreativePlanOptions{
		Goal:          "make a short with captions",
		WriteArtifact: true,
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil || len(entries) == 0 {
		t.Fatal("no creative plan created")
	}
	return entries[0].Name()
}

func TestApproveCreativePlan_MarksApproved(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	var out bytes.Buffer
	if err := ApproveCreativePlan(planID, &out, ApproveCreativePlanOptions{}); err != nil {
		t.Fatalf("ApproveCreativePlan error: %v", err)
	}
	if !strings.Contains(out.String(), "approved") {
		t.Fatalf("output missing 'approved': %s", out.String())
	}

	raw, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["approval_status"] != "approved" {
		t.Fatalf("approval_status = %v, want approved", m["approval_status"])
	}
	if m["approval_mode"] != "manual" {
		t.Fatalf("approval_mode = %v, want manual", m["approval_mode"])
	}
}

func TestApproveCreativePlan_NotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := ApproveCreativePlan("nonexistent-plan-id", ioDiscard{}, ApproveCreativePlanOptions{}); err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestApproveCreativePlan_Idempotent(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := ApproveCreativePlan(planID, ioDiscard{}, ApproveCreativePlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ApproveCreativePlan(planID, &out, ApproveCreativePlanOptions{}); err != nil {
		t.Fatalf("second approve error: %v", err)
	}
	if !strings.Contains(out.String(), "already approved") {
		t.Fatalf("expected 'already approved' message; got: %s", out.String())
	}
}

func TestCreativePlanEvents_EmptyWhenNoEvents(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	// remove events file if it exists so we test the empty path
	_ = os.Remove(filepath.Join(creativePlansRoot, planID, "events.jsonl"))

	var out bytes.Buffer
	if err := CreativePlanEvents(planID, &out, CreativePlanEventsOptions{}); err != nil {
		t.Fatalf("CreativePlanEvents error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "No events") {
		t.Fatalf("expected 'No events'; got: %s", text)
	}
}

func TestCreativePlanEvents_AfterApprove(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := ApproveCreativePlan(planID, ioDiscard{}, ApproveCreativePlanOptions{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := CreativePlanEvents(planID, &out, CreativePlanEventsOptions{}); err != nil {
		t.Fatalf("CreativePlanEvents error: %v", err)
	}
	if !strings.Contains(out.String(), "CREATIVE_PLAN_APPROVED") {
		t.Fatalf("expected CREATIVE_PLAN_APPROVED event; got: %s", out.String())
	}
}

func TestCreativePreview_WritesArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	var out bytes.Buffer
	if err := CreativePreview(planID, &out, CreativePreviewOptions{}); err != nil {
		t.Fatalf("CreativePreview error: %v", err)
	}

	previewPath := filepath.Join(creativePlansRoot, planID, "creative_requests.dryrun.json")
	if _, err := os.Stat(previewPath); err != nil {
		t.Fatalf("missing creative_requests.dryrun.json: %v", err)
	}

	raw, _ := os.ReadFile(previewPath)
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("preview JSON invalid: %v", err)
	}
	if p["schema_version"] != "creative_requests.dryrun.v1" {
		t.Fatalf("schema_version = %v", p["schema_version"])
	}
	if _, ok := p["requests"]; !ok {
		t.Fatal("missing 'requests' field")
	}
}

func TestCreativePreview_OverwriteRequired(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := CreativePreview(planID, ioDiscard{}, CreativePreviewOptions{}); err != nil {
		t.Fatal(err)
	}
	// second call without --overwrite should fail
	if err := CreativePreview(planID, ioDiscard{}, CreativePreviewOptions{}); err == nil {
		t.Fatal("expected error on second preview without --overwrite")
	}
	// with --overwrite should succeed
	if err := CreativePreview(planID, ioDiscard{}, CreativePreviewOptions{Overwrite: true}); err != nil {
		t.Fatalf("overwrite preview error: %v", err)
	}
}

func TestCreativePreview_StrictFailsMissingBackend(t *testing.T) {
	t.Chdir(t.TempDir())
	// no tools config — all backends missing
	if err := os.WriteFile(config.DefaultPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.mov")
	if err := os.WriteFile(inputPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreativePlanCommand(inputPath, ioDiscard{}, CreativePlanOptions{
		Goal: "make a cinematic short with narration",
	}); err != nil {
		// plan creation may warn or fail in strict mode; either way find the plan
		_ = err
	}
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil || len(entries) == 0 {
		t.Skip("no plan created — skip strict backend test")
	}
	planID := entries[0].Name()
	err = CreativePreview(planID, ioDiscard{}, CreativePreviewOptions{Strict: true})
	if err == nil {
		t.Fatal("expected strict error when backends are missing")
	}
}

func TestExecuteCreativePlan_RequiresApprovalOrYes(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	err := ExecuteCreativePlan(planID, ioDiscard{}, ExecuteCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected error when plan not approved and --yes not set")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteCreativePlan_YesBypassesApproval(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := ExecuteCreativePlan(planID, ioDiscard{}, ExecuteCreativePlanOptions{Yes: true}); err != nil {
		t.Fatalf("ExecuteCreativePlan --yes error: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m["execution_status"] != "dry_run_completed" {
		t.Fatalf("execution_status = %v, want dry_run_completed", m["execution_status"])
	}
}

func TestExecuteCreativePlan_WritesDryRunArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := ApproveCreativePlan(planID, ioDiscard{}, ApproveCreativePlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteCreativePlan(planID, ioDiscard{}, ExecuteCreativePlanOptions{}); err != nil {
		t.Fatalf("ExecuteCreativePlan error: %v", err)
	}

	previewPath := filepath.Join(creativePlansRoot, planID, "creative_requests.dryrun.json")
	if _, err := os.Stat(previewPath); err != nil {
		t.Fatalf("missing dryrun artifact: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if m["execution_status"] != "dry_run_completed" {
		t.Fatalf("execution_status = %v", m["execution_status"])
	}
	if m["request_preview_artifact"] != "creative_requests.dryrun.json" {
		t.Fatalf("request_preview_artifact = %v", m["request_preview_artifact"])
	}
}

func TestCreativeResult_ShowsStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	var out bytes.Buffer
	if err := CreativeResult(planID, &out, CreativeResultOptions{}); err != nil {
		t.Fatalf("CreativeResult error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "pending") {
		t.Fatalf("expected 'pending' approval_status; got: %s", text)
	}
	if !strings.Contains(text, "not_started") {
		t.Fatalf("expected 'not_started' execution_status; got: %s", text)
	}
}

func TestCreativeResult_WriteArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	if err := CreativeResult(planID, ioDiscard{}, CreativeResultOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("CreativeResult --write-artifact error: %v", err)
	}
	artifactPath := filepath.Join(creativePlansRoot, planID, "creative_result.md")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("missing creative_result.md: %v", err)
	}
	if !strings.Contains(string(data), "# Creative Result") {
		t.Fatalf("artifact missing header: %s", string(data))
	}
}

func TestValidateCreativePlan_ValidPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeTestCreativePlan(t)

	var out bytes.Buffer
	if err := ValidateCreativePlan(planID, &out, ValidateCreativePlanOptions{}); err != nil {
		t.Fatalf("ValidateCreativePlan error: %v", err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("expected 'ok' in output; got: %s", out.String())
	}
}

func TestValidateCreativePlan_MalformedPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(creativePlansRoot, "bad-plan"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(creativePlansRoot, "bad-plan", "creative_plan.json"),
		[]byte(`{"schema_version":"wrong","plan_id":"x"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	err := ValidateCreativePlan("bad-plan", ioDiscard{}, ValidateCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected validation error for bad schema_version")
	}
}

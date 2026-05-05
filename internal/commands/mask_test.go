package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"byom-video/internal/highlights"
	"byom-video/internal/manifest"
	"byom-video/internal/roughcut"
)

func TestMaskPlanFromRoughcutCreatesDecisionsAndManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	var out bytes.Buffer
	if err := MaskPlan(runID, &out, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if len(mask.Decisions) != 1 || mask.Decisions[0].Decision != "keep" || mask.Decisions[0].ClipID != "clip_0001" {
		t.Fatalf("mask decisions = %#v", mask.Decisions)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "inference_mask.json") {
		t.Fatalf("manifest artifacts = %#v", m.Artifacts)
	}
}

func TestMaskPlanFromHighlightsCreatesDecisions(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	var out bytes.Buffer
	if err := MaskPlan(runID, &out, MaskPlanOptions{TopK: 1}); err != nil {
		t.Fatal(err)
	}
	mask := readMaskFile(t, filepath.Join(runDir, "inference_mask.json"))
	if len(mask.Decisions) != 1 || mask.Decisions[0].Decision != "candidate_keep" || mask.Decisions[0].HighlightID != "hl_0001" {
		t.Fatalf("mask decisions = %#v", mask.Decisions)
	}
}

func TestMaskPlanRefusesOverwriteAndValidatesMaxCaptionWords(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v", err)
	}
	err = MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{MaxCaptionWords: -1, Overwrite: true})
	if err == nil || !strings.Contains(err.Error(), "max-caption-words") {
		t.Fatalf("err = %v", err)
	}
}

func TestMaskTemplateWritesTemplatesWhenArtifactsExist(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "20260501T000000Z-masktest"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "media/input.mov", time.Now().UTC())
	m.AddArtifact("chunks", "chunks.json")
	m.AddArtifact("highlights", "highlights.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"chunks.json", "highlights.json"} {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := MaskTemplate(runID, &out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"inference_mask.template.json", "expansion_tasks.template.json", "verification.template.json"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(runDir, "inference_mask.template.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"chunks_artifact": "chunks.json"`) || !strings.Contains(string(data), `"highlights_artifact": "highlights.json"`) {
		t.Fatalf("template source = %s", string(data))
	}
}

func TestInspectMaskDetectsPresentAndMissingTemplates(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "20260501T000000Z-inspectmask"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "inference_mask.template.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := InspectMask(runID, &out, InspectMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "inference_mask.template.json: present") || !strings.Contains(text, "verification.template.json: missing") {
		t.Fatalf("inspect output = %s", text)
	}
	out.Reset()
	if err := InspectMask(runID, &out, InspectMaskOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"exists": true`) || !strings.Contains(out.String(), `"exists": false`) {
		t.Fatalf("inspect json = %s", out.String())
	}
}

func TestMaskValidateAcceptsGeneratedTemplates(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "20260501T000000Z-maskvalid"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaskTemplate(runID, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := MaskValidate(runID, &out, MaskValidateOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status: ok") {
		t.Fatalf("validate output = %s", out.String())
	}
}

func TestMaskValidateRejectsBadSchemaVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "20260501T000000Z-maskbad"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaskTemplate(runID, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runDir, "inference_mask.template.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"bad","source":{},"intent":"x","constraints":{},"decisions":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := MaskValidate(runID, &out, MaskValidateOptions{JSON: true})
	if err == nil {
		t.Fatal("MaskValidate returned nil error")
	}
	if !strings.Contains(out.String(), "schema_version must be inference_mask.v1") {
		t.Fatalf("validate json = %s", out.String())
	}
}

func TestMaskValidateRejectsBadDecisionTiming(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskHighlights(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(runDir, "inference_mask.json")
	mask := readMaskFile(t, path)
	mask.Decisions[0].Start = 10
	mask.Decisions[0].End = 2
	if err := writeJSONFile(path, mask); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := MaskValidate(runID, &out, MaskValidateOptions{})
	if err == nil || !strings.Contains(out.String(), "end must be greater than or equal to start") {
		t.Fatalf("err=%v out=%s", err, out.String())
	}
}

func TestReviewExpansionVerificationAndInspectMask(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := makeMaskRun(t)
	writeMaskRoughcut(t, runDir)
	if err := MaskPlan(runID, &bytes.Buffer{}, MaskPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewMask(runID, &out, ReviewMaskOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "mask_review.md")); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var tasks ExpansionTasks
	readJSON(t, filepath.Join(runDir, "expansion_tasks.json"), &tasks)
	types := []string{}
	for _, task := range tasks.Tasks {
		types = append(types, task.Type)
	}
	joined := strings.Join(types, ",")
	for _, want := range []string{"caption_variants", "timeline_labels", "short_descriptions"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("task types = %s", joined)
		}
	}
	if err := VerificationPlanCommand(runID, &bytes.Buffer{}, VerificationPlanOptions{}); err != nil {
		t.Fatal(err)
	}
	var verification VerificationPlan
	readJSON(t, filepath.Join(runDir, "verification.json"), &verification)
	if verification.Status != "pending" || len(verification.Checks) != 4 {
		t.Fatalf("verification = %#v", verification)
	}
	out.Reset()
	if err := InspectMask(runID, &out, InspectMaskOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "inference_mask.json: present, valid") || !strings.Contains(out.String(), "mask_review.md: present") {
		t.Fatalf("inspect output = %s", out.String())
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"mask_review.md", "expansion_tasks.json", "verification.json"} {
		if !manifestHasArtifact(m, path) {
			t.Fatalf("manifest missing %s: %#v", path, m.Artifacts)
		}
	}
}

func TestMaskValidateHandlesMissingTemplatesCleanly(t *testing.T) {
	t.Chdir(t.TempDir())
	runID := "20260501T000000Z-maskmissing"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := MaskValidate(runID, &out, MaskValidateOptions{})
	if err == nil {
		t.Fatal("MaskValidate returned nil error")
	}
	if !strings.Contains(out.String(), "artifact or template is missing") {
		t.Fatalf("validate output = %s", out.String())
	}
}

func makeMaskRun(t *testing.T) (string, string) {
	t.Helper()
	runID := "20260501T000000Z-maskplan"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "media/input.mov", time.Now().UTC())
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	return runID, runDir
}

func writeMaskRoughcut(t *testing.T, runDir string) {
	t.Helper()
	doc := roughcut.Document{
		SchemaVersion: "roughcut.v1",
		Clips: []roughcut.Clip{
			{ID: "clip_0001", HighlightID: "hl_0001", SourceChunkID: "chunk_0001", Start: 1, End: 5, DurationSeconds: 4, EditIntent: "Strong opening clip.", Text: "This is a useful highlight."},
		},
	}
	if err := writeJSONFile(filepath.Join(runDir, "roughcut.json"), doc); err != nil {
		t.Fatal(err)
	}
}

func writeMaskHighlights(t *testing.T, runDir string) {
	t.Helper()
	doc := highlights.Document{
		SchemaVersion: "highlights.v1",
		Highlights: []highlights.Highlight{
			{ID: "hl_0001", ChunkID: "chunk_0001", Start: 1, End: 5, DurationSeconds: 4, Reason: "contains a hook", Text: "This is a useful highlight."},
			{ID: "hl_0002", ChunkID: "chunk_0002", Start: 8, End: 12, DurationSeconds: 4, Reason: "strong detail", Text: "This is another highlight."},
		},
	}
	if err := writeJSONFile(filepath.Join(runDir, "highlights.json"), doc); err != nil {
		t.Fatal(err)
	}
}

func readMaskFile(t *testing.T, path string) InferenceMask {
	t.Helper()
	var mask InferenceMask
	readJSON(t, path, &mask)
	return mask
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func manifestHasArtifact(m manifest.Manifest, path string) bool {
	for _, artifact := range m.Artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

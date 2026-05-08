package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/modelrouter"
)

const testConfigWithEnabledStubRoutes = `models:
  enabled: true

  entries:
    local_stub:
      provider: stub
      model: deterministic-stub
      role: expander

  routes:
    caption_expansion: local_stub
    timeline_labeling: local_stub
    description_expansion: local_stub
`

const testConfigWithEnabledOllamaRoutes = `models:
  enabled: true

  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
      base_url: http://localhost:11434

  routes:
    caption_expansion: local_qwen
    timeline_labeling: local_qwen
    description_expansion: local_qwen
`

const testConfigEnabledNoRoutes = `models:
  enabled: true

  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: expander
`

type httpDoerFunc func(req *http.Request) (*http.Response, error)

func (fn httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestExpandDryRunCreatesRequestPreviews(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)

	var out bytes.Buffer
	if err := ExpandDryRun(runID, &out, ExpandDryRunOptions{}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(runDir, "model_requests.dryrun.json")
	var doc ModelRequestsDryRun
	readJSON(t, path, &doc)
	if doc.SchemaVersion != "model_requests.dryrun.v1" {
		t.Fatalf("schema_version = %q", doc.SchemaVersion)
	}
	if len(doc.Requests) == 0 {
		t.Fatal("expected requests")
	}
	if doc.Requests[0].RequestPreview.User == "" || doc.Requests[0].RequestPreview.OutputSchema == "" {
		t.Fatalf("missing request preview: %+v", doc.Requests[0].RequestPreview)
	}
}

func TestExpandDryRunStrictFailsOnMissingRoute(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, _ := setupExpandStubRun(t)
	err := ExpandDryRun(runID, &bytes.Buffer{}, ExpandDryRunOptions{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("expected strict error, got: %v", err)
	}
}

func TestExpandDryRunTaskTypeFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandDryRun(runID, &bytes.Buffer{}, ExpandDryRunOptions{TaskType: "caption_variants"}); err != nil {
		t.Fatal(err)
	}

	var doc ModelRequestsDryRun
	readJSON(t, filepath.Join(runDir, "model_requests.dryrun.json"), &doc)
	if len(doc.Requests) != 1 || doc.Requests[0].TaskType != "caption_variants" {
		t.Fatalf("requests = %#v", doc.Requests)
	}
}

func TestValidateModelRequestsDryRunShapeAcceptsValidOutput(t *testing.T) {
	payload := map[string]any{
		"schema_version": "model_requests.dryrun.v1",
		"run_id":         "run-1",
		"requests": []any{
			map[string]any{
				"task_id":    "task_0001",
				"task_type":  "caption_variants",
				"route_name": "caption_expansion",
				"status":     "dry_run",
				"request_preview": map[string]any{
					"system":        "x",
					"user":          "y",
					"output_schema": "expansion_output.v1",
				},
			},
		},
	}
	if errs := validateModelRequestsDryRunShape(payload); len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestValidateModelRequestsDryRunShapeRejectsMalformedOutput(t *testing.T) {
	payload := map[string]any{
		"schema_version": "bad",
		"run_id":         "",
		"requests": []any{
			map[string]any{
				"task_id": "task_0001",
			},
		},
	}
	if errs := validateModelRequestsDryRunShape(payload); len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
}

func TestExpandLocalStubProducesExpansionArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandLocalStub(runID, &bytes.Buffer{}, ExpandLocalStubOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"caption_variants.json",
		"timeline_labels.json",
		"short_descriptions.json",
	} {
		if _, err := os.Stat(filepath.Join(runDir, "expansions", name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestExpandLocalStubSkipsRejectedDecisions(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)

	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask := readMaskFile(t, maskPath)
	mask.Decisions[0].Decision = "reject"
	if err := writeJSONFile(maskPath, mask); err != nil {
		t.Fatal(err)
	}

	if err := ExpandLocalStub(runID, &bytes.Buffer{}, ExpandLocalStubOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}

	output := readExpansionOutput(t, filepath.Join(runDir, "expansions", "caption_variants.json"))
	if len(output.Items) != 0 {
		t.Fatalf("expected no items, got %d", len(output.Items))
	}
}

func TestExpandDryRunRecordsManifestArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)

	if err := ExpandDryRun(runID, &bytes.Buffer{}, ExpandDryRunOptions{}); err != nil {
		t.Fatal(err)
	}

	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifestHasArtifact(m, "model_requests.dryrun.json") {
		t.Fatalf("manifest missing model_requests.dryrun.json: %#v", m.Artifacts)
	}
}

func TestExpandDryRunJSONOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, _ := setupExpandStubRun(t)
	var out bytes.Buffer
	if err := ExpandDryRun(runID, &out, ExpandDryRunOptions{JSON: true}); err != nil {
		t.Fatal(err)
	}
	var summary ExpandDryRunSummary
	if err := json.Unmarshal(out.Bytes(), &summary); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if summary.RunID != runID {
		t.Fatalf("run_id = %q", summary.RunID)
	}
}

func TestExpandFailsWhenModelsDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, _ := setupExpandStubRun(t)
	err := Expand(runID, &bytes.Buffer{}, ExpandOptions{})
	if err == nil || !strings.Contains(err.Error(), "models.enabled is false") {
		t.Fatalf("expected models.enabled error, got: %v", err)
	}
}

func TestExpandDryRunDoesNotCallProvider(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	runID, _ := setupExpandStubRun(t)
	if err := Expand(runID, &bytes.Buffer{}, ExpandOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
}

func TestExpandMissingRouteFailsInStrictMode(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigEnabledNoRoutes)
	runID, _ := setupExpandStubRun(t)
	err := Expand(runID, &bytes.Buffer{}, ExpandOptions{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("expected strict error, got: %v", err)
	}
}

func TestExpandTaskTypeFilterWorks(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledStubRoutes)
	runID, runDir := setupExpandStubRun(t)
	if err := Expand(runID, &bytes.Buffer{}, ExpandOptions{TaskType: "caption_variants"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "expansions", "caption_variants.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "expansions", "timeline_labels.json")); err == nil {
		t.Fatal("timeline_labels.json should not exist when filtered")
	}
}

func TestExpandRefusesOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledStubRoutes)
	runID, _ := setupExpandStubRun(t)
	if err := Expand(runID, &bytes.Buffer{}, ExpandOptions{}); err != nil {
		t.Fatal(err)
	}
	err := Expand(runID, &bytes.Buffer{}, ExpandOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected overwrite error, got: %v", err)
	}
}

func TestExpandPlainTextProviderResponseBecomesExpansionItemText(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	runID, runDir := setupExpandStubRun(t)
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"response":"plain text caption from ollama"}`)),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	if err := Expand(runID, &bytes.Buffer{}, ExpandOptions{TaskType: "caption_variants"}); err != nil {
		t.Fatal(err)
	}
	output := readExpansionOutput(t, filepath.Join(runDir, "expansions", "caption_variants.json"))
	if len(output.Items) == 0 {
		t.Fatal("expected items")
	}
	if !strings.Contains(output.Items[0].Text, "plain text caption from ollama") {
		t.Fatalf("text = %q", output.Items[0].Text)
	}
	if output.Items[0].Metadata["response_mode"] != "plain_text" {
		t.Fatalf("metadata = %#v", output.Items[0].Metadata)
	}
}

func TestExpandParsesJSONItemsAndTruncates(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	runID, runDir := setupExpandStubRun(t)
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"response":"{\"items\":[{\"text\":\"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen nineteen\"},{\"text\":\"second variant text\"}]}"}`)),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	if err := Expand(runID, &bytes.Buffer{}, ExpandOptions{TaskType: "caption_variants"}); err != nil {
		t.Fatal(err)
	}
	output := readExpansionOutput(t, filepath.Join(runDir, "expansions", "caption_variants.json"))
	if len(output.Items) != 2 {
		t.Fatalf("items = %#v", output.Items)
	}
	if output.Items[0].Metadata["response_mode"] != "json" {
		t.Fatalf("metadata = %#v", output.Items[0].Metadata)
	}
	if output.Items[0].Metadata["truncated"] != true {
		t.Fatalf("expected truncated metadata, got %#v", output.Items[0].Metadata)
	}
}

func TestExpandPartialFailureContinuesWhenFailFastFalse(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	runID, runDir := setupExpandStubRun(t)
	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask := readMaskFile(t, maskPath)
	mask.Decisions = append(mask.Decisions, MaskDecision{
		ID:          "decision_0002",
		Start:       5,
		End:         7,
		Decision:    "keep",
		Reason:      "extra",
		TextPreview: "second clip",
	})
	if err := writeJSONFile(maskPath, mask); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	call := 0
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			call++
			if call == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"response":"ok second"}`)),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	err := Expand(runID, &bytes.Buffer{}, ExpandOptions{TaskType: "timeline_labels"})
	if err == nil || !strings.Contains(err.Error(), "failed request") {
		t.Fatalf("expected partial failure, got %v", err)
	}
	output := readExpansionOutput(t, filepath.Join(runDir, "expansions", "timeline_labels.json"))
	if len(output.Items) != 1 {
		t.Fatalf("expected one successful item, got %d", len(output.Items))
	}
	var executed ExecutedModelRequests
	readJSON(t, filepath.Join(runDir, "model_requests.executed.json"), &executed)
	if len(executed.Requests) != 2 {
		t.Fatalf("requests = %#v", executed.Requests)
	}
}

func TestExpandFailFastStopsOnFirstFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	runID, runDir := setupExpandStubRun(t)
	maskPath := filepath.Join(runDir, "inference_mask.json")
	mask := readMaskFile(t, maskPath)
	mask.Decisions = append(mask.Decisions, MaskDecision{
		ID:          "decision_0002",
		Start:       5,
		End:         7,
		Decision:    "keep",
		Reason:      "extra",
		TextPreview: "second clip",
	})
	if err := writeJSONFile(maskPath, mask); err != nil {
		t.Fatal(err)
	}
	if err := ExpansionPlanCommand(runID, &bytes.Buffer{}, ExpansionPlanOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	call := 0
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			call++
			return nil, io.ErrUnexpectedEOF
		})
	})
	defer restore()

	err := Expand(runID, &bytes.Buffer{}, ExpandOptions{TaskType: "timeline_labels", FailFast: true})
	if err == nil {
		t.Fatal("expected error")
	}
	var executed ExecutedModelRequests
	readJSON(t, filepath.Join(runDir, "model_requests.executed.json"), &executed)
	if len(executed.Requests) != 1 {
		t.Fatalf("expected one request due to fail-fast, got %#v", executed.Requests)
	}
}

func TestValidateExecutedModelRequestsShapeAcceptsValidOutput(t *testing.T) {
	payload := map[string]any{
		"schema_version": "model_requests.executed.v1",
		"run_id":         "run-1",
		"requests": []any{
			map[string]any{
				"task_id":         "task_0001",
				"decision_id":     "decision_0001",
				"task_type":       "caption_variants",
				"model_route":     "caption_expansion",
				"model_entry":     "local_qwen",
				"provider":        "ollama",
				"model":           "qwen2.5:7b",
				"status":          "completed",
				"request_preview": map[string]any{"system": "x", "user": "y", "output_schema": "expansion_output.v1"},
			},
		},
	}
	if errs := validateExecutedModelRequestsShape(payload); len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestReviewModelRequestsSummaryAndArtifact(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithRoutes)
	runID, runDir := setupExpandStubRun(t)
	if err := ExpandDryRun(runID, &bytes.Buffer{}, ExpandDryRunOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "model_requests.executed.json"), ExecutedModelRequests{
		SchemaVersion: "model_requests.executed.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Requests: []ExecutedModelRequestEntry{{
			TaskID:         "task_0001",
			DecisionID:     "decision_0001",
			TaskType:       "caption_variants",
			ModelRoute:     "caption_expansion",
			ModelEntry:     "local_qwen",
			Provider:       "ollama",
			Model:          "qwen2.5:7b",
			Status:         "completed",
			RequestPreview: modelrouter.RequestPreview{System: "x", User: "y", OutputSchema: "expansion_output.v1"},
			ResponseMode:   "json",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewModelRequests(runID, &out, ReviewModelRequestsOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "executed count: 1") {
		t.Fatalf("output = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(runDir, "model_requests_review.md")); err != nil {
		t.Fatal(err)
	}
}

func TestModelsDoctorHandlesMissingOllamaCleanly(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithEnabledOllamaRoutes)
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})
	})
	defer restore()

	var out bytes.Buffer
	err := ModelsDoctor(&out, ModelsDoctorOptions{})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected unavailable error, got: %v", err)
	}
	if !strings.Contains(out.String(), "Ollama request failed") {
		t.Fatalf("output = %s", out.String())
	}
}

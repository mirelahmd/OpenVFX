package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteVisualRequestsRequiresExplicitApproval(t *testing.T) {
	t.Chdir(t.TempDir())
	err := ExecuteVisualRequests("agentplan-missing", ioDiscard{}, ExecuteVisualRequestsOptions{})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got %v", err)
	}
}

func TestExecuteVisualRequestsCustomHTTPVisualBase64(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("VISUAL_API_KEY", "top-secret-token")
	var sawPrompt bool
	client := fakeHTTPDoer{fn: func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer top-secret-token" {
			t.Fatalf("authorization header = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body["prompt"].(string), "AI b-roll") || strings.Contains(body["prompt"].(string), "b-roll") {
			sawPrompt = true
		}
		respBody, _ := json.Marshal(map[string]any{
			"status":    "completed",
			"image_b64": base64.StdEncoding.EncodeToString([]byte("fake-png")),
		})
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(respBody)),
		}, nil
	}}
	writeVisualExecutionConfig(t, "https://example.test/v1/visual")
	input := writeAgentPlanInputFile(t, "clip.mov")
	if err := AgentPlanCommand(ioDiscard{}, AgentPlanCommandOptions{
		Goal:      "make a reel and generate AI b-roll",
		InputPath: input,
	}); err != nil {
		t.Fatal(err)
	}
	planID := latestAgentPlanID(t)
	var out bytes.Buffer
	err := executeVisualRequestsWithDeps(planID, &out, ExecuteVisualRequestsOptions{
		Yes:                  true,
		AllowProviderCalls:   true,
		AllowExternalNetwork: true,
		JSON:                 true,
	}, visualHTTPDeps{client: client})
	if err != nil {
		t.Fatal(err)
	}
	if !sawPrompt {
		t.Fatal("server did not receive rendered prompt")
	}
	var assets GeneratedAssetsArtifact
	if err := json.Unmarshal(out.Bytes(), &assets); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if assets.Status != "completed" || len(assets.Assets) != 1 || assets.Assets[0].OutputFile == "" {
		t.Fatalf("assets = %#v", assets)
	}
	if data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, assets.Assets[0].OutputFile)); err != nil || string(data) != "fake-png" {
		t.Fatalf("output data = %q err=%v", string(data), err)
	}
	audit, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, assets.Assets[0].RequestAuditArtifact))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(audit), "top-secret-token") || !strings.Contains(string(audit), "redacted") {
		t.Fatalf("request audit should redact secret: %s", string(audit))
	}
}

type fakeHTTPDoer struct {
	fn func(*http.Request) (*http.Response, error)
}

func (f fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return f.fn(req)
}

func TestReviewVisualGenerationWritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := "agentplan-test"
	dir := filepath.Join(agentPlansV1Root, planID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := GeneratedAssetsArtifact{
		SchemaVersion: generatedAssetsSchema,
		AgentPlanID:   planID,
		Status:        "completed",
		Assets: []GeneratedVisualAsset{{
			VisualRequestID: "visual_req_0001",
			Kind:            "generated_broll",
			Status:          "completed",
			Backend:         "my_backend",
			OutputFile:      "outputs/visual_assets/visual_req_0001.png",
			Bytes:           8,
		}},
	}
	if err := writeJSONFile(filepath.Join(dir, generatedAssetsArtifact), artifact); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := ReviewVisualGeneration(planID, &out, ReviewVisualGenerationOptions{WriteArtifact: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Visual Generation Review") {
		t.Fatalf("review = %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, visualGenerationReviewArtifact)); err != nil {
		t.Fatal(err)
	}
}

func writeVisualExecutionConfig(t *testing.T, endpoint string) {
	t.Helper()
	content := `tools:
  enabled: true
  routes:
    creative.broll_generate: my_video_backend
  backends:
    my_video_backend:
      kind: video_generation
      provider: custom-http-visual
      model: user-video-model
      endpoint: ` + endpoint + `
      auth:
        type: bearer_env
        env: VISUAL_API_KEY
      request:
        method: POST
        headers:
          Content-Type: application/json
        body_template:
          prompt: "{{prompt}}"
          aspect_ratio: "{{aspect_ratio}}"
          duration_seconds: "{{duration_seconds}}"
          model: "{{model}}"
      response:
        mode: sync
        output_base64_json_path: "$.image_b64"
        status_json_path: "$.status"
`
	if err := os.WriteFile("byom-video.yaml", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

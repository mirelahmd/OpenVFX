package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

// ---- test helpers ----

func makeVoiceTestPlan(t *testing.T, goal string) (planDir string, planID string) {
	t.Helper()
	dir := t.TempDir()
	// Create a short unique plan ID
	suffix := goal
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	planID = "tv-" + strings.ReplaceAll(suffix, " ", "-")
	planDir = filepath.Join(dir, ".byom-video", "creative_plans", planID)
	mustMkdir(t, filepath.Join(planDir, "outputs"))
	plan := map[string]any{
		"schema_version": "creative_plan.v1",
		"plan_id":        planID,
		"goal":           goal,
		"steps":          []any{},
	}
	writeTestJSON(t, filepath.Join(planDir, "creative_plan.json"), plan)
	mustChdir(t, dir)
	return planDir, planID
}

func writeTestVoiceoverText(t *testing.T, planDir, text string) {
	t.Helper()
	out := map[string]any{
		"schema_version":   "voiceover_text.v1",
		"creative_plan_id": filepath.Base(filepath.Dir(planDir)),
		"mode":             "local_text_extract",
		"source":           map[string]any{"source_type": "goal", "source_artifact": ""},
		"text":             text,
		"word_count":       len(strings.Fields(text)),
		"warnings":         []any{},
	}
	writeTestJSON(t, filepath.Join(planDir, "outputs", "voiceover_text.json"), out)
	mustWriteFile(t, filepath.Join(planDir, "outputs", "voiceover_text.txt"), []byte(text))
}

func fakeTTSServer(t *testing.T, statusCode int, body []byte, contentType string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	mustWriteFile(t, path, data)
}

func writeVoiceYAML(t *testing.T, dir, endpoint string) {
	t.Helper()
	yaml := `tools:
  enabled: true
  backends:
    voice_backend:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: ` + endpoint + `
      auth:
        type: header_env
        header: xi-api-key
        env: TEST_ELEVEN_KEY
      options:
        voice_id: voice-test-123
  routes:
    creative.voiceover: voice_backend
`
	mustWriteFile(t, filepath.Join(dir, "byom-video.yaml"), []byte(yaml))
}

// ---- resolveVoiceBackend tests ----

func TestResolveVoiceBackend_MissingRoute(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: true\n  backends: {}\n  routes: {}\n"))

	_, _, err := resolveVoiceBackend("creative.voiceover", "")
	if err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("expected 'no route' error, got: %v", err)
	}
}

func TestResolveVoiceBackend_WrongKind(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	yaml := `tools:
  enabled: true
  backends:
    bad_backend:
      kind: text_generation
      provider: ollama
      model: qwen
      endpoint: http://localhost:11434
      auth:
        type: none
  routes:
    creative.voiceover: bad_backend
`
	mustWriteFile(t, "byom-video.yaml", []byte(yaml))

	_, _, err := resolveVoiceBackend("creative.voiceover", "")
	if err == nil || !strings.Contains(err.Error(), "voice_generation") {
		t.Fatalf("expected kind error, got: %v", err)
	}
}

func TestResolveVoiceBackend_ExplicitBackendOverride(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	yaml := `tools:
  enabled: true
  backends:
    my_voice:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: http://localhost:11434
      auth:
        type: header_env
        header: xi-api-key
        env: ELEVEN_KEY
  routes: {}
`
	mustWriteFile(t, "byom-video.yaml", []byte(yaml))

	name, cfg, err := resolveVoiceBackend("creative.voiceover", "my_voice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my_voice" {
		t.Errorf("expected backend name 'my_voice', got %q", name)
	}
	if cfg.Provider != "elevenlabs-compatible" {
		t.Errorf("expected provider 'elevenlabs-compatible', got %q", cfg.Provider)
	}
}

func TestResolveVoiceBackend_ToolsDisabled(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	_, _, err := resolveVoiceBackend("creative.voiceover", "")
	if err == nil || !strings.Contains(err.Error(), "tools.enabled is false") {
		t.Fatalf("expected tools.enabled error, got: %v", err)
	}
}

// ---- buildElevenLabsURL tests ----

func TestBuildElevenLabsURL_BaseEndpoint(t *testing.T) {
	url := buildElevenLabsURL("https://api.elevenlabs.io/v1", "voice-abc")
	want := "https://api.elevenlabs.io/v1/text-to-speech/voice-abc"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestBuildElevenLabsURL_AlreadyHasTTSSuffix(t *testing.T) {
	url := buildElevenLabsURL("https://api.elevenlabs.io/v1/text-to-speech", "voice-xyz")
	want := "https://api.elevenlabs.io/v1/text-to-speech/voice-xyz"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestBuildElevenLabsURL_TrailingSlash(t *testing.T) {
	url := buildElevenLabsURL("https://api.elevenlabs.io/v1/", "voice-123")
	want := "https://api.elevenlabs.io/v1/text-to-speech/voice-123"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

// ---- extensionFromContentType tests ----

func TestExtensionFromContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want string
	}{
		{"audio/mpeg", ".mp3"},
		{"audio/wav", ".wav"},
		{"audio/x-wav", ".wav"},
		{"audio/aac", ".aac"},
		{"audio/mp4", ".m4a"},
		{"audio/mpeg; charset=utf-8", ".mp3"},
		{"unknown/type", ".mp3"},
		{"", ".mp3"},
	}
	for _, c := range cases {
		got := extensionFromContentType(c.ct)
		if got != c.want {
			t.Errorf("extensionFromContentType(%q) = %q, want %q", c.ct, got, c.want)
		}
	}
}

// ---- callElevenLabsCompatible tests ----

func makeVoiceConfigTest(endpoint string) config.ToolBackendConfig {
	return config.ToolBackendConfig{
		Kind:     "voice_generation",
		Provider: "elevenlabs-compatible",
		Model:    "eleven_multilingual_v2",
		Endpoint: endpoint,
		Auth: config.ToolAuthConfig{
			Type:   "header_env",
			Header: "xi-api-key",
			Env:    "TEST_VOICE_API_KEY",
		},
	}
}

func TestCallElevenLabsCompatible_SuccessfulResponse(t *testing.T) {
	fakeAudio := []byte("FAKE_AUDIO_BYTES_MP3")
	srv := fakeTTSServer(t, 200, fakeAudio, "audio/mpeg")
	defer srv.Close()

	t.Setenv("TEST_VOICE_API_KEY", "test-key-value")

	cfg := makeVoiceConfigTest(srv.URL)
	audioBytes, ct, err := callElevenLabsCompatible(
		"Hello world", "eleven_multilingual_v2", "voice-abc", "", srv.URL, cfg.Auth, 0.5, 0.75, 10, srv.Client(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(audioBytes) != string(fakeAudio) {
		t.Errorf("audio bytes mismatch: got %q, want %q", audioBytes, fakeAudio)
	}
	if !strings.Contains(ct, "audio/mpeg") {
		t.Errorf("expected audio/mpeg content type, got %q", ct)
	}
}

func TestCallElevenLabsCompatible_Non2xxFails(t *testing.T) {
	srv := fakeTTSServer(t, 401, []byte(`{"detail":"Unauthorized"}`), "application/json")
	defer srv.Close()

	t.Setenv("TEST_VOICE_API_KEY", "bad-key")

	cfg := makeVoiceConfigTest(srv.URL)
	_, _, err := callElevenLabsCompatible(
		"Hello", "eleven_multilingual_v2", "voice-abc", "", srv.URL, cfg.Auth, 0.5, 0.75, 10, srv.Client(),
	)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got: %v", err)
	}
}

func TestCallElevenLabsCompatible_MissingEnvFails(t *testing.T) {
	t.Setenv("TEST_VOICE_API_KEY", "")

	cfg := config.ToolAuthConfig{
		Type:   "header_env",
		Header: "xi-api-key",
		Env:    "TEST_VOICE_API_KEY",
	}
	_, _, err := callElevenLabsCompatible(
		"Hello", "eleven_multilingual_v2", "voice-abc", "", "http://localhost:9999", cfg, 0.5, 0.75, 10, http.DefaultClient,
	)
	if err == nil || !strings.Contains(err.Error(), "TEST_VOICE_API_KEY") {
		t.Fatalf("expected env var error, got: %v", err)
	}
}

func TestCallElevenLabsCompatible_MissingVoiceIDFails(t *testing.T) {
	cfg := config.ToolAuthConfig{Type: "none"}
	_, _, err := callElevenLabsCompatible(
		"Hello", "eleven_multilingual_v2", "", "", "http://localhost:9999", cfg, 0.5, 0.75, 10, http.DefaultClient,
	)
	if err == nil || !strings.Contains(err.Error(), "voice_id") {
		t.Fatalf("expected voice_id error, got: %v", err)
	}
}

func TestCallElevenLabsCompatible_TruncatesLongErrorBody(t *testing.T) {
	longBody := strings.Repeat("X", 300)
	srv := fakeTTSServer(t, 500, []byte(longBody), "text/plain")
	defer srv.Close()

	t.Setenv("TEST_VOICE_API_KEY", "key")
	cfg := config.ToolAuthConfig{Type: "header_env", Header: "xi-api-key", Env: "TEST_VOICE_API_KEY"}
	_, _, err := callElevenLabsCompatible(
		"Hello", "eleven_multilingual_v2", "voice-abc", "", srv.URL, cfg, 0.5, 0.75, 10, srv.Client(),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errStr := err.Error()
	if len(errStr) > 400 {
		t.Errorf("error message too long (%d chars); should be truncated", len(errStr))
	}
	if !strings.Contains(errStr, "500") {
		t.Errorf("error should contain status 500, got: %s", errStr)
	}
}

// ---- GenerateVoiceover command tests ----

func setupVoiceGenEnv(t *testing.T, endpoint string) (planDir, planID string) {
	t.Helper()
	planDir, planID = makeVoiceTestPlan(t, "test voice gen goal")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	writeTestVoiceoverText(t, planDir, "This is the voiceover text.")
	writeVoiceYAML(t, cwd, endpoint)
	return planDir, planID
}

func TestGenerateVoiceover_DryRunWritesArtifact(t *testing.T) {
	planDir, planID := setupVoiceGenEnv(t, "http://localhost:9999")

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genPath := filepath.Join(planDir, "outputs", "voiceover_generation.json")
	if _, statErr := os.Stat(genPath); statErr != nil {
		t.Fatalf("voiceover_generation.json not created: %v", statErr)
	}
	data, _ := os.ReadFile(genPath)
	var art VoiceGenerationOutput
	if err2 := json.Unmarshal(data, &art); err2 != nil {
		t.Fatalf("malformed voiceover_generation.json: %v", err2)
	}
	if art.SchemaVersion != "voiceover_generation.v1" {
		t.Errorf("wrong schema_version: %q", art.SchemaVersion)
	}
	if art.Status != "dry_run" {
		t.Errorf("expected status=dry_run, got %q", art.Status)
	}
	if art.Mode != "dry_run" {
		t.Errorf("expected mode=dry_run, got %q", art.Mode)
	}
}

func TestGenerateVoiceover_DryRunDoesNotReadEnvVar(t *testing.T) {
	t.Setenv("TEST_ELEVEN_KEY", "")
	_, planID := setupVoiceGenEnv(t, "http://localhost:9999")

	var buf strings.Builder
	// dry-run without --check-env must NOT require env var
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run should not require env var, got: %v", err)
	}
}

func TestGenerateVoiceover_DryRunCheckEnvFailsWhenMissing(t *testing.T) {
	t.Setenv("TEST_ELEVEN_KEY", "")
	_, planID := setupVoiceGenEnv(t, "http://localhost:9999")

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{DryRun: true, CheckEnv: true})
	if err == nil || !strings.Contains(err.Error(), "TEST_ELEVEN_KEY") {
		t.Fatalf("expected env var error, got: %v", err)
	}
}

func TestGenerateVoiceover_DryRunJSONOutput(t *testing.T) {
	_, planID := setupVoiceGenEnv(t, "http://localhost:9999")

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{DryRun: true, JSON: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no JSON in output: %q", out)
	}
	var art VoiceGenerationOutput
	if err2 := json.Unmarshal([]byte(out[idx:]), &art); err2 != nil {
		t.Fatalf("JSON output invalid: %v", err2)
	}
	if art.Status != "dry_run" {
		t.Errorf("expected dry_run status, got %q", art.Status)
	}
}

func TestGenerateVoiceover_LiveSuccessWritesAudioAndArtifact(t *testing.T) {
	fakeAudio := []byte("ID3FAKEAUDIODATA")
	srv := fakeTTSServer(t, 200, fakeAudio, "audio/mpeg")
	defer srv.Close()

	t.Setenv("TEST_ELEVEN_KEY", "test-key")
	planDir, planID := setupVoiceGenEnv(t, srv.URL)

	var buf strings.Builder
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{}, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	audioPath := filepath.Join(planDir, "outputs", "voiceover.mp3")
	if _, statErr := os.Stat(audioPath); statErr != nil {
		t.Fatalf("voiceover.mp3 not created: %v", statErr)
	}
	written, _ := os.ReadFile(audioPath)
	if string(written) != string(fakeAudio) {
		t.Errorf("audio file content mismatch")
	}

	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "voiceover_generation.json"))
	var art VoiceGenerationOutput
	if err2 := json.Unmarshal(data, &art); err2 != nil {
		t.Fatalf("artifact malformed: %v", err2)
	}
	if art.Status != "completed" {
		t.Errorf("expected status=completed, got %q", art.Status)
	}
	if art.Output == nil {
		t.Fatal("output field is nil")
	}
	if art.Output.Bytes != int64(len(fakeAudio)) {
		t.Errorf("expected bytes=%d, got %d", len(fakeAudio), art.Output.Bytes)
	}
}

func TestGenerateVoiceover_LiveFailWritesFailedArtifact(t *testing.T) {
	srv := fakeTTSServer(t, 500, []byte("internal error"), "text/plain")
	defer srv.Close()

	t.Setenv("TEST_ELEVEN_KEY", "test-key")
	planDir, planID := setupVoiceGenEnv(t, srv.URL)

	var buf strings.Builder
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{}, srv.Client())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "voiceover_generation.json"))
	var art VoiceGenerationOutput
	if err2 := json.Unmarshal(data, &art); err2 != nil {
		t.Fatalf("artifact malformed: %v", err2)
	}
	if art.Status != "failed" {
		t.Errorf("expected status=failed, got %q", art.Status)
	}
	if art.Error == "" {
		t.Error("expected error field to be set")
	}
}

func TestGenerateVoiceover_RejectsExistingWithoutOverwrite(t *testing.T) {
	planDir, planID := setupVoiceGenEnv(t, "http://localhost:9999")
	stub := VoiceGenerationOutput{SchemaVersion: "voiceover_generation.v1", Status: "dry_run"}
	data, _ := json.Marshal(stub)
	mustWriteFile(t, filepath.Join(planDir, "outputs", "voiceover_generation.json"), data)

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got: %v", err)
	}
}

func TestGenerateVoiceover_PrepareTextRunsVoiceoverText(t *testing.T) {
	planDir, planID := makeVoiceTestPlan(t, "Goal without voiceover text")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	writeVoiceYAML(t, cwd, "http://localhost:9999")
	t.Setenv("TEST_ELEVEN_KEY", "")

	// No voiceover_text.json exists yet
	if _, err := os.Stat(filepath.Join(planDir, "outputs", "voiceover_text.json")); err == nil {
		t.Fatal("voiceover_text.json should not exist before test")
	}

	var buf strings.Builder
	genErr := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{
		DryRun:      true,
		PrepareText: true,
	})
	if genErr != nil {
		t.Fatalf("unexpected error: %v", genErr)
	}

	if _, statErr := os.Stat(filepath.Join(planDir, "outputs", "voiceover_text.json")); statErr != nil {
		t.Fatal("voiceover_text.json not created by --prepare-text")
	}
}

func TestGenerateVoiceover_MissingPlanFails(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: true\n  backends: {}\n  routes: {}\n"))

	var buf strings.Builder
	err := GenerateVoiceover("nonexistent-plan-xyz", &buf, GenerateVoiceoverOptions{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

func TestGenerateVoiceover_UnsupportedProviderLiveFails(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Unsupported provider test")
	writeTestVoiceoverText(t, planDir, "Hello")
	yaml := `tools:
  enabled: true
  backends:
    custom_voice:
      kind: voice_generation
      provider: custom-http-voice
      model: some-model
      endpoint: http://localhost:9999
      auth:
        type: none
  routes:
    creative.voiceover: custom_voice
`
	mustWriteFile(t, "byom-video.yaml", []byte(yaml))

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{})
	if err == nil || !strings.Contains(err.Error(), "elevenlabs-compatible") {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}
}

func TestGenerateVoiceover_UnsupportedProviderDryRunAllowed(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Dry-run custom provider")
	writeTestVoiceoverText(t, planDir, "Hello world")
	yaml := `tools:
  enabled: true
  backends:
    custom_voice:
      kind: voice_generation
      provider: custom-http-voice
      model: some-model
      endpoint: http://localhost:9999
      auth:
        type: none
  routes:
    creative.voiceover: custom_voice
`
	mustWriteFile(t, "byom-video.yaml", []byte(yaml))

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run with custom-http-voice should succeed, got: %v", err)
	}
}

func TestGenerateVoiceover_DirectTextOverride(t *testing.T) {
	_, planID := setupVoiceGenEnv(t, "http://localhost:9999")

	var buf strings.Builder
	err := GenerateVoiceover(planID, &buf, GenerateVoiceoverOptions{
		DryRun: true,
		Text:   "Override text directly",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "direct_text") {
		t.Errorf("output should mention direct_text source, got: %q", out)
	}
}

func TestGenerateVoiceover_SecretNotInOutput(t *testing.T) {
	fakeAudio := []byte("FAKEAUDIO")
	srv := fakeTTSServer(t, 200, fakeAudio, "audio/mpeg")
	defer srv.Close()

	secretValue := "super-secret-api-key-12345"
	t.Setenv("TEST_ELEVEN_KEY", secretValue)

	planDir, planID := setupVoiceGenEnv(t, srv.URL)

	var buf strings.Builder
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{}, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Secret must not appear in stdout
	if strings.Contains(buf.String(), secretValue) {
		t.Errorf("secret value appeared in stdout output")
	}

	// Secret must not appear in voiceover_generation.json
	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "voiceover_generation.json"))
	if strings.Contains(string(data), secretValue) {
		t.Errorf("secret value appeared in voiceover_generation.json")
	}
}

// ---- ReviewGeneratedVoiceover tests ----

func TestReviewGeneratedVoiceover_WritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Review test goal")
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	artifact := VoiceGenerationOutput{
		SchemaVersion:  "voiceover_generation.v1",
		CreativePlanID: planID,
		Mode:           "live",
		Status:         "completed",
		Provider:       "elevenlabs-compatible",
		Backend:        "voice_backend",
		Model:          "eleven_multilingual_v2",
		VoiceID:        "voice-abc",
		Source:         VoiceGenerationSource{SourceType: "voiceover_text", TextWordCount: 15},
	}
	writeTestJSON(t, filepath.Join(planDir, "outputs", "voiceover_generation.json"), artifact)

	var buf strings.Builder
	err := ReviewGeneratedVoiceover(planID, &buf, ReviewGeneratedVoiceoverOptions{WriteArtifact: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mdPath := filepath.Join(planDir, "outputs", "voiceover_generation_review.md")
	if _, statErr := os.Stat(mdPath); statErr != nil {
		t.Fatal("voiceover_generation_review.md not created")
	}
	data, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(data), "elevenlabs-compatible") {
		t.Errorf("review markdown missing provider info")
	}
}

func TestReviewGeneratedVoiceover_MissingFileFails(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	makeVoiceTestPlan(t, "Missing review test")
	planID := "tv-Missing rev"

	var buf strings.Builder
	err := ReviewGeneratedVoiceover(planID, &buf, ReviewGeneratedVoiceoverOptions{})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
}

// ---- ValidateVoiceover with generation artifact ----

func TestValidateVoiceover_PassesWhenGeneratedAudioExists(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Validate with generated audio")
	writeTestVoiceoverText(t, planDir, "Voiceover text here")
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	audioPath := filepath.Join(planDir, "outputs", "voiceover.mp3")
	mustWriteFile(t, audioPath, []byte("FAKEAUDIO"))

	artifact := VoiceGenerationOutput{
		SchemaVersion:  "voiceover_generation.v1",
		Status:         "completed",
		CreativePlanID: planID,
		Output: &VoiceGenerationOutputMeta{
			AudioFile:   "outputs/voiceover.mp3",
			ContentType: "audio/mpeg",
			Bytes:       9,
		},
	}
	writeTestJSON(t, filepath.Join(planDir, "outputs", "voiceover_generation.json"), artifact)

	var buf strings.Builder
	err := ValidateVoiceover(planID, &buf, ValidateVoiceoverOptions{RequireAudio: true})
	if err != nil {
		t.Fatalf("validate should pass, got: %v", err)
	}
}

func TestValidateVoiceover_FailsWhenGenerationCompletedButAudioMissing(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Validate missing audio after gen")
	writeTestVoiceoverText(t, planDir, "Some text")
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	// Generation says completed but audio file doesn't exist
	artifact := VoiceGenerationOutput{
		SchemaVersion:  "voiceover_generation.v1",
		Status:         "completed",
		CreativePlanID: planID,
		Output: &VoiceGenerationOutputMeta{
			AudioFile:   "outputs/voiceover.mp3",
			ContentType: "audio/mpeg",
			Bytes:       100,
		},
	}
	writeTestJSON(t, filepath.Join(planDir, "outputs", "voiceover_generation.json"), artifact)

	var buf strings.Builder
	err := ValidateVoiceover(planID, &buf, ValidateVoiceoverOptions{})
	if err == nil {
		t.Fatal("expected error for missing audio after completed generation")
	}
}

// ---- VoiceoverStatus with generation info ----

func TestVoiceoverStatus_ShowsGenerationStatus(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	planDir, planID := makeVoiceTestPlan(t, "Status with generation")
	writeTestVoiceoverText(t, planDir, "Hello world")
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	artifact := VoiceGenerationOutput{
		SchemaVersion: "voiceover_generation.v1",
		Status:        "completed",
		Provider:      "elevenlabs-compatible",
		Model:         "eleven_multilingual_v2",
	}
	writeTestJSON(t, filepath.Join(planDir, "outputs", "voiceover_generation.json"), artifact)

	var buf strings.Builder
	err := VoiceoverStatus(planID, &buf, VoiceoverStatusOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "completed") {
		t.Errorf("status output should mention generation status: %q", out)
	}
	if !strings.Contains(out, "elevenlabs-compatible") {
		t.Errorf("status output should mention provider: %q", out)
	}
}

// ---- make dry-run with --generate-voiceover ----

func TestMake_GenerateVoiceover_DryRunShows4e(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	var buf strings.Builder
	_ = Make("/dev/null", &buf, MakeOptions{
		Goal:              "test goal",
		DryRun:            true,
		Yes:               true,
		GenerateVoiceover: true,
	})
	out := buf.String()
	if !strings.Contains(out, "4e") {
		t.Errorf("dry-run should show step 4e when --generate-voiceover; got: %q", out)
	}
}

func TestMake_WithoutGenerateVoiceover_DryRunOmits4e(t *testing.T) {
	dir := t.TempDir()
	mustChdir(t, dir)
	mustWriteFile(t, "byom-video.yaml", []byte("tools:\n  enabled: false\n"))

	var buf strings.Builder
	_ = Make("/dev/null", &buf, MakeOptions{
		Goal:   "test goal",
		DryRun: true,
		Yes:    true,
	})
	out := buf.String()
	if strings.Contains(out, "4e") {
		t.Errorf("dry-run should NOT show step 4e without --generate-voiceover; got: %q", out)
	}
}

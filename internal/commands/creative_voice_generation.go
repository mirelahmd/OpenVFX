package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
)

// HTTPDoer is a minimal HTTP client interface, allowing httptest injection in tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// defaultHTTPClient is the production HTTP client (zero-value uses http.DefaultClient).
var defaultHTTPClient HTTPDoer = http.DefaultClient

// ---- schema ----

type VoiceGenerationSource struct {
	SourceType     string `json:"source_type"`           // voiceover_text|direct_text
	SourceArtifact string `json:"source_artifact"`       // relative path or ""
	TextWordCount  int    `json:"text_word_count"`
}

type VoiceGenerationRequestMeta struct {
	Endpoint        string  `json:"endpoint"`
	OutputFormat    string  `json:"output_format,omitempty"`
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
}

type VoiceGenerationOutputMeta struct {
	AudioFile   string `json:"audio_file"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
}

type VoiceGenerationOutput struct {
	SchemaVersion  string                     `json:"schema_version"`
	CreatedAt      time.Time                  `json:"created_at"`
	CreativePlanID string                     `json:"creative_plan_id"`
	Mode           string                     `json:"mode"`   // live|dry_run
	Status         string                     `json:"status"` // completed|failed|dry_run
	Provider       string                     `json:"provider"`
	Backend        string                     `json:"backend"`
	Route          string                     `json:"route"`
	Model          string                     `json:"model"`
	VoiceID        string                     `json:"voice_id"`
	Source         VoiceGenerationSource      `json:"source"`
	Request        VoiceGenerationRequestMeta `json:"request"`
	Output         *VoiceGenerationOutputMeta `json:"output,omitempty"`
	Warnings       []string                   `json:"warnings,omitempty"`
	Error          string                     `json:"error,omitempty"`
}

// ---- options ----

type GenerateVoiceoverOptions struct {
	Overwrite       bool
	JSON            bool
	PrepareText     bool
	Route           string
	Backend         string
	DryRun          bool
	CheckEnv        bool
	TimeoutSeconds  int
	OutputPath      string
	VoiceID         string
	Model           string
	Text            string  // direct text override
	Stability       float64 // 0.0–1.0; -1 = use backend default (0.5)
	SimilarityBoost float64 // 0.0–1.0; -1 = use backend default (0.75)
	OutputFormat    string  // e.g. "mp3_44100_128"; "" = use backend default
}

type ReviewGeneratedVoiceoverOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- backend resolution ----

func resolveVoiceBackend(routeKey, backendName string) (string, config.ToolBackendConfig, error) {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return "", config.ToolBackendConfig{}, fmt.Errorf("load config: %w", err)
	}
	if !cfg.Tools.Enabled {
		return "", config.ToolBackendConfig{}, fmt.Errorf(
			"tools.enabled is false in byom-video.yaml; add:\n\ntools:\n  enabled: true\n  backends:\n    voice_backend:\n      kind: voice_generation\n      provider: elevenlabs-compatible\n      model: eleven_multilingual_v2\n      endpoint: https://api.elevenlabs.io/v1\n      auth:\n        type: header_env\n        header: xi-api-key\n        env: ELEVENLABS_API_KEY\n      options:\n        voice_id: <your-voice-id>\n  routes:\n    %s: voice_backend", routeKey)
	}

	if backendName != "" {
		backend, ok := cfg.Tools.Backends[backendName]
		if !ok {
			return "", config.ToolBackendConfig{}, fmt.Errorf("tools backend %q not found in config", backendName)
		}
		if backend.Kind != "voice_generation" {
			return "", config.ToolBackendConfig{}, fmt.Errorf("backend %q has kind %q; expected voice_generation", backendName, backend.Kind)
		}
		return backendName, backend, nil
	}

	resolvedName, ok := cfg.Tools.Routes[routeKey]
	if !ok || resolvedName == "" {
		return "", config.ToolBackendConfig{}, fmt.Errorf(
			"no route %q configured; add to byom-video.yaml:\n\ntools:\n  routes:\n    %s: voice_backend", routeKey, routeKey)
	}

	backend, ok := cfg.Tools.Backends[resolvedName]
	if !ok {
		return "", config.ToolBackendConfig{}, fmt.Errorf("route %q points to backend %q but that backend is not configured", routeKey, resolvedName)
	}
	if backend.Kind != "voice_generation" {
		return "", config.ToolBackendConfig{}, fmt.Errorf("route %q → backend %q has kind %q; expected voice_generation", routeKey, resolvedName, backend.Kind)
	}
	return resolvedName, backend, nil
}

// ---- URL + HTTP helpers ----

// buildElevenLabsURL constructs the TTS endpoint URL from a base endpoint and voice ID.
// Handles endpoints with or without a trailing /text-to-speech segment.
func buildElevenLabsURL(endpoint, voiceID string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/text-to-speech") {
		return endpoint + "/" + voiceID
	}
	return endpoint + "/text-to-speech/" + voiceID
}

// extensionFromContentType infers an audio file extension from the HTTP Content-Type.
func extensionFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	// strip parameters like "; charset=utf-8"
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch ct {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/aac":
		return ".aac"
	case "audio/mp4", "audio/x-m4a":
		return ".m4a"
	default:
		return ".mp3"
	}
}

// voiceIDFromBackend extracts voice_id from backend options map.
func voiceIDFromBackend(backend config.ToolBackendConfig) string {
	if backend.Options == nil {
		return ""
	}
	v, _ := backend.Options["voice_id"].(string)
	return v
}

// outputFormatFromBackend extracts output_format from backend options map.
func outputFormatFromBackend(backend config.ToolBackendConfig) string {
	if backend.Options == nil {
		return ""
	}
	v, _ := backend.Options["output_format"].(string)
	return v
}

// stabilityFromBackend extracts stability from backend options map.
func stabilityFromBackend(backend config.ToolBackendConfig) float64 {
	if backend.Options == nil {
		return 0.5
	}
	switch v := backend.Options["stability"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0.5
}

// similarityBoostFromBackend extracts similarity_boost from backend options map.
func similarityBoostFromBackend(backend config.ToolBackendConfig) float64 {
	if backend.Options == nil {
		return 0.75
	}
	switch v := backend.Options["similarity_boost"].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0.75
}

// callElevenLabsCompatible sends a TTS request and returns raw audio bytes + content-type.
// The caller must provide a non-nil client.
func callElevenLabsCompatible(
	text, model, voiceID, outputFormat, endpoint string,
	auth config.ToolAuthConfig,
	stability, similarityBoost float64,
	timeoutSec int,
	client HTTPDoer,
) ([]byte, string, error) {
	if voiceID == "" {
		return nil, "", fmt.Errorf("voice_id is required; set options.voice_id in the backend config or pass --voice-id")
	}

	apiKey := ""
	if auth.Type == "header_env" || auth.Type == "bearer_env" {
		if auth.Env == "" {
			return nil, "", fmt.Errorf("backend auth env var name is not set in config")
		}
		apiKey = os.Getenv(auth.Env)
		if apiKey == "" {
			return nil, "", fmt.Errorf("missing required env var %s for voice generation backend", auth.Env)
		}
	}

	url := buildElevenLabsURL(endpoint, voiceID)

	bodyMap := map[string]any{
		"text":     text,
		"model_id": model,
		"voice_settings": map[string]any{
			"stability":        stability,
			"similarity_boost": similarityBoost,
		},
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("build HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "audio/mpeg")
	if outputFormat != "" {
		httpReq.Header.Set("Accept", "audio/*")
	}

	switch auth.Type {
	case "header_env":
		header := auth.Header
		if header == "" {
			header = "xi-api-key"
		}
		httpReq.Header.Set(header, apiKey)
	case "bearer_env":
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, "", fmt.Errorf("provider returned HTTP %d: %s", resp.StatusCode, preview)
	}

	ct := resp.Header.Get("Content-Type")
	return body, ct, nil
}

// ---- resolveVoiceoverText for generation ----

func resolveVoiceoverTextForGeneration(planDir, planID string) (text, sourceType, sourceArtifact string, wordCount int, err error) {
	outputsDir := filepath.Join(planDir, "outputs")

	// Priority 1: voiceover_text.json
	vtPath := filepath.Join(outputsDir, "voiceover_text.json")
	if data, readErr := os.ReadFile(vtPath); readErr == nil {
		var vt VoiceoverTextOutput
		if json.Unmarshal(data, &vt) == nil && strings.TrimSpace(vt.Text) != "" {
			return vt.Text, "voiceover_text", "outputs/voiceover_text.json", vt.WordCount, nil
		}
	}

	// Priority 2: voiceover_text.txt
	txtPath := filepath.Join(outputsDir, "voiceover_text.txt")
	if data, readErr := os.ReadFile(txtPath); readErr == nil {
		t := strings.TrimSpace(string(data))
		if t != "" {
			return t, "voiceover_text", "outputs/voiceover_text.txt", len(strings.Fields(t)), nil
		}
	}

	return "", "", "", 0, fmt.Errorf(
		"voiceover_text.json not found for plan %q; run creative-voiceover-text %s first, or pass --prepare-text", planID, planID)
}

// ---- Part C: creative-generate-voiceover ----

func GenerateVoiceover(planID string, stdout io.Writer, opts GenerateVoiceoverOptions) error {
	return generateVoiceoverWithClient(planID, stdout, opts, defaultHTTPClient)
}

func generateVoiceoverWithClient(planID string, stdout io.Writer, opts GenerateVoiceoverOptions, client HTTPDoer) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")
	outputsDir := filepath.Join(planDir, "outputs")

	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}

	// Guard existing output
	genPath := filepath.Join(outputsDir, "voiceover_generation.json")
	if !opts.DryRun {
		if _, err := os.Stat(genPath); err == nil && !opts.Overwrite {
			return fmt.Errorf("voiceover_generation.json already exists; use --overwrite to replace")
		}
	}
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return fmt.Errorf("create outputs dir: %w", err)
	}

	// Events
	eventLog, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	defer func() {
		if eventLog != nil {
			eventLog.Close()
		}
	}()
	writeEvent := func(kind string, data map[string]any) {
		if eventLog != nil {
			_ = eventLog.Write(kind, data)
		}
	}

	// Resolve route/backend
	routeKey := opts.Route
	if routeKey == "" {
		routeKey = "creative.voiceover"
	}
	backendName, backendCfg, err := resolveVoiceBackend(routeKey, opts.Backend)
	if err != nil {
		writeEvent("VOICEOVER_GENERATION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		return err
	}

	// Only elevenlabs-compatible is supported for live calls
	if !opts.DryRun && backendCfg.Provider != "elevenlabs-compatible" {
		msg := fmt.Sprintf("creative-generate-voiceover supports provider 'elevenlabs-compatible' for live calls; route %q points to provider %q (use --dry-run for a request preview)", routeKey, backendCfg.Provider)
		writeEvent("VOICEOVER_GENERATION_FAILED", map[string]any{"plan_id": planID, "reason": msg})
		return fmt.Errorf("%s", msg)
	}

	// Resolve voice ID
	voiceID := opts.VoiceID
	if voiceID == "" {
		voiceID = voiceIDFromBackend(backendCfg)
	}
	if voiceID == "" && !opts.DryRun {
		return fmt.Errorf("voice_id is required; set options.voice_id in backend %q or pass --voice-id", backendName)
	}

	// Resolve model
	model := opts.Model
	if model == "" {
		model = backendCfg.Model
	}
	if model == "" {
		model = "eleven_multilingual_v2"
	}

	// Resolve timeout
	timeoutSec := opts.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	// Resolve text
	var text, sourceType, sourceArtifact string
	var wordCount int
	var textWarnings []string

	if opts.Text != "" {
		text = opts.Text
		sourceType = "direct_text"
		sourceArtifact = ""
		wordCount = len(strings.Fields(text))
	} else {
		// --prepare-text: run voiceover-text if missing
		if opts.PrepareText {
			vtPath := filepath.Join(outputsDir, "voiceover_text.json")
			if _, statErr := os.Stat(vtPath); statErr != nil {
				fmt.Fprintf(stdout, "  preparing voiceover text (--prepare-text)...\n")
				vtOpts := VoiceoverTextOptions{MaxWords: 120}
				if prepErr := VoiceoverTextCommand(planID, io.Discard, vtOpts); prepErr != nil {
					return fmt.Errorf("--prepare-text: %w", prepErr)
				}
			}
		}
		var textErr error
		text, sourceType, sourceArtifact, wordCount, textErr = resolveVoiceoverTextForGeneration(planDir, planID)
		if textErr != nil {
			writeEvent("VOICEOVER_GENERATION_FAILED", map[string]any{"plan_id": planID, "reason": textErr.Error()})
			return textErr
		}
	}

	// Resolve output format: flag > backend config
	outputFmt := opts.OutputFormat
	if outputFmt == "" {
		outputFmt = outputFormatFromBackend(backendCfg)
	}

	// Resolve stability/similarityBoost: flag (-1 = unset) > backend config > hardcoded defaults
	stability := opts.Stability
	if stability < 0 {
		stability = stabilityFromBackend(backendCfg)
	}
	if stability < 0 || stability > 1 {
		return fmt.Errorf("--stability must be between 0.0 and 1.0 (got %.2f)", stability)
	}
	similarityBoost := opts.SimilarityBoost
	if similarityBoost < 0 {
		similarityBoost = similarityBoostFromBackend(backendCfg)
	}
	if similarityBoost < 0 || similarityBoost > 1 {
		return fmt.Errorf("--similarity-boost must be between 0.0 and 1.0 (got %.2f)", similarityBoost)
	}

	endpoint := backendCfg.Endpoint

	// Print preview header
	fmt.Fprintf(stdout, "creative-generate-voiceover: %s\n", planID)
	fmt.Fprintf(stdout, "  route:    %s → %s (%s/%s)\n", routeKey, backendName, backendCfg.Provider, model)
	fmt.Fprintf(stdout, "  voice:    %s\n", func() string {
		if voiceID != "" {
			return voiceID
		}
		return "(not set)"
	}())
	fmt.Fprintf(stdout, "  source:   %s (%s)\n", sourceType, func() string {
		if sourceArtifact != "" {
			return sourceArtifact
		}
		return "direct"
	}())
	fmt.Fprintf(stdout, "  words:    %d\n", wordCount)
	fmt.Fprintf(stdout, "  endpoint: %s\n", endpoint)
	fmt.Fprintf(stdout, "  stability:%.2f  similarity:%.2f\n", stability, similarityBoost)
	if outputFmt != "" {
		fmt.Fprintf(stdout, "  format:   %s\n", outputFmt)
	}

	// Determine output path
	audioExt := ".mp3"
	outputAudioPath := opts.OutputPath
	if outputAudioPath == "" {
		outputAudioPath = filepath.Join(outputsDir, "voiceover"+audioExt)
	}
	relAudioPath := outputAudioPath
	if rel, relErr := filepath.Rel(planDir, outputAudioPath); relErr == nil {
		relAudioPath = rel
	}

	// Check env var presence (dry-run + --check-env, or always for live)
	envVarName := backendCfg.Auth.Env
	if opts.CheckEnv || !opts.DryRun {
		if backendCfg.Auth.Type == "header_env" || backendCfg.Auth.Type == "bearer_env" {
			if envVarName == "" {
				return fmt.Errorf("backend %q auth.env is not set in config", backendName)
			}
			if os.Getenv(envVarName) == "" {
				return fmt.Errorf("missing required env var %s for backend %q", envVarName, backendName)
			}
			if opts.DryRun {
				fmt.Fprintf(stdout, "  env:      %s ✓ (present)\n", envVarName)
			}
		}
	} else if opts.DryRun && envVarName != "" {
		fmt.Fprintf(stdout, "  env:      %s (not checked; pass --check-env to verify)\n", envVarName)
	}

	// Build generation output artifact
	artifact := VoiceGenerationOutput{
		SchemaVersion:  "voiceover_generation.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Provider:       backendCfg.Provider,
		Backend:        backendName,
		Route:          routeKey,
		Model:          model,
		VoiceID:        voiceID,
		Source: VoiceGenerationSource{
			SourceType:     sourceType,
			SourceArtifact: sourceArtifact,
			TextWordCount:  wordCount,
		},
		Request: VoiceGenerationRequestMeta{
			Endpoint:        buildElevenLabsURL(endpoint, voiceID),
			OutputFormat:    outputFmt,
			Stability:       stability,
			SimilarityBoost: similarityBoost,
			TimeoutSeconds:  timeoutSec,
		},
		Warnings: textWarnings,
	}

	if opts.DryRun {
		artifact.Mode = "dry_run"
		artifact.Status = "dry_run"

		fmt.Fprintf(stdout, "  mode:     dry-run (no provider call)\n")
		fmt.Fprintf(stdout, "  url:      POST %s\n", artifact.Request.Endpoint)
		fmt.Fprintf(stdout, "  output:   %s (would write)\n", relAudioPath)

		if err := writeJSONFile(genPath, artifact); err != nil {
			return fmt.Errorf("write voiceover_generation.json: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "voiceover_generation", "outputs/voiceover_generation.json", "")

		writeEvent("VOICEOVER_GENERATION_DRY_RUN_COMPLETED", map[string]any{
			"plan_id":  planID,
			"backend":  backendName,
			"provider": backendCfg.Provider,
		})

		if opts.JSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(artifact)
		}
		return nil
	}

	// --- Live execution ---
	fmt.Fprintf(stdout, "  calling %s...\n", backendCfg.Provider)
	writeEvent("VOICEOVER_GENERATION_STARTED", map[string]any{
		"plan_id":  planID,
		"backend":  backendName,
		"provider": backendCfg.Provider,
		"model":    model,
	})

	audioBytes, contentType, callErr := callElevenLabsCompatible(
		text, model, voiceID, outputFmt, endpoint, backendCfg.Auth, stability, similarityBoost, timeoutSec, client,
	)
	if callErr != nil {
		artifact.Mode = "live"
		artifact.Status = "failed"
		artifact.Error = callErr.Error()
		_ = writeJSONFile(genPath, artifact)
		writeEvent("VOICEOVER_GENERATION_FAILED", map[string]any{"plan_id": planID, "reason": callErr.Error()})
		return fmt.Errorf("voice generation failed: %w", callErr)
	}

	// Infer extension from content type
	audioExt = extensionFromContentType(contentType)
	if outputAudioPath == filepath.Join(outputsDir, "voiceover.mp3") && audioExt != ".mp3" {
		outputAudioPath = filepath.Join(outputsDir, "voiceover"+audioExt)
		relAudioPath = "outputs/voiceover" + audioExt
	}

	if err := os.WriteFile(outputAudioPath, audioBytes, 0o644); err != nil {
		artifact.Mode = "live"
		artifact.Status = "failed"
		artifact.Error = err.Error()
		_ = writeJSONFile(genPath, artifact)
		writeEvent("VOICEOVER_GENERATION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		return fmt.Errorf("write audio file: %w", err)
	}

	artifact.Mode = "live"
	artifact.Status = "completed"
	artifact.Output = &VoiceGenerationOutputMeta{
		AudioFile:   relAudioPath,
		ContentType: contentType,
		Bytes:       int64(len(audioBytes)),
	}

	if err := writeJSONFile(genPath, artifact); err != nil {
		return fmt.Errorf("write voiceover_generation.json: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "voiceover_generation", "outputs/voiceover_generation.json", "")
	_ = updateCreativeOutputsIndex(planID, "voiceover_audio", relAudioPath, "")

	writeEvent("VOICEOVER_GENERATION_COMPLETED", map[string]any{
		"plan_id":      planID,
		"audio_file":   relAudioPath,
		"content_type": contentType,
		"bytes":        len(audioBytes),
	})

	fmt.Fprintf(stdout, "  done:     %s (%d bytes)\n", relAudioPath, len(audioBytes))
	for _, w := range artifact.Warnings {
		fmt.Fprintf(stdout, "  warning:  %s\n", w)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}
	return nil
}

// ---- Part E: review-generated-voiceover ----

func ReviewGeneratedVoiceover(planID string, stdout io.Writer, opts ReviewGeneratedVoiceoverOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	genPath := filepath.Join(planDir, "outputs", "voiceover_generation.json")

	data, err := os.ReadFile(genPath)
	if err != nil {
		return fmt.Errorf("voiceover_generation.json not found for plan %q; run creative-generate-voiceover first: %w", planID, err)
	}
	var artifact VoiceGenerationOutput
	if err := json.Unmarshal(data, &artifact); err != nil {
		return fmt.Errorf("voiceover_generation.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}

	fmt.Fprintf(stdout, "review-generated-voiceover: %s\n", planID)
	fmt.Fprintf(stdout, "  status:   %s\n", artifact.Status)
	fmt.Fprintf(stdout, "  mode:     %s\n", artifact.Mode)
	fmt.Fprintf(stdout, "  provider: %s\n", artifact.Provider)
	fmt.Fprintf(stdout, "  backend:  %s\n", artifact.Backend)
	fmt.Fprintf(stdout, "  model:    %s\n", artifact.Model)
	fmt.Fprintf(stdout, "  voice:    %s\n", artifact.VoiceID)
	fmt.Fprintf(stdout, "  source:   %s\n", artifact.Source.SourceType)
	fmt.Fprintf(stdout, "  words:    %d\n", artifact.Source.TextWordCount)
	if artifact.Output != nil {
		fmt.Fprintf(stdout, "  audio:    %s (%d bytes)\n", artifact.Output.AudioFile, artifact.Output.Bytes)
	} else {
		fmt.Fprintf(stdout, "  audio:    (not generated)\n")
	}
	if artifact.Error != "" {
		fmt.Fprintf(stdout, "  error:    %s\n", artifact.Error)
	}
	for _, w := range artifact.Warnings {
		fmt.Fprintf(stdout, "  warning:  %s\n", w)
	}

	if opts.WriteArtifact {
		reviewPath := filepath.Join(planDir, "outputs", "voiceover_generation_review.md")
		content := buildVoiceGenerationReviewMarkdown(artifact)
		if err := os.WriteFile(reviewPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write review artifact: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "voiceover_generation_review", "outputs/voiceover_generation_review.md", "")
		fmt.Fprintf(stdout, "  artifact: outputs/voiceover_generation_review.md\n")
	}
	return nil
}

func buildVoiceGenerationReviewMarkdown(a VoiceGenerationOutput) string {
	var b strings.Builder
	b.WriteString("# Voiceover Generation Review\n\n")
	b.WriteString(fmt.Sprintf("- **plan_id**: %s\n", a.CreativePlanID))
	b.WriteString(fmt.Sprintf("- **status**: %s\n", a.Status))
	b.WriteString(fmt.Sprintf("- **mode**: %s\n", a.Mode))
	b.WriteString(fmt.Sprintf("- **provider**: %s\n", a.Provider))
	b.WriteString(fmt.Sprintf("- **backend**: %s\n", a.Backend))
	b.WriteString(fmt.Sprintf("- **model**: %s\n", a.Model))
	b.WriteString(fmt.Sprintf("- **voice_id**: %s\n", a.VoiceID))
	b.WriteString(fmt.Sprintf("- **source_type**: %s\n", a.Source.SourceType))
	b.WriteString(fmt.Sprintf("- **word_count**: %d\n", a.Source.TextWordCount))
	if a.Output != nil {
		b.WriteString(fmt.Sprintf("- **audio_file**: %s\n", a.Output.AudioFile))
		b.WriteString(fmt.Sprintf("- **bytes**: %d\n", a.Output.Bytes))
	}
	if a.Error != "" {
		b.WriteString(fmt.Sprintf("\n**Error**: %s\n", a.Error))
	}
	if len(a.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range a.Warnings {
			b.WriteString(fmt.Sprintf("- %s\n", w))
		}
	}
	return b.String()
}

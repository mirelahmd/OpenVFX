package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

const (
	generatedAssetsSchema          = "openvfx_generated_assets.v1"
	generatedAssetsArtifact        = "generated_assets.json"
	visualGenerationReviewArtifact = "visual_generation_review.md"
)

type ExecuteVisualRequestsOptions struct {
	Yes                  bool
	JSON                 bool
	Overwrite            bool
	AllowProviderCalls   bool
	AllowExternalNetwork bool
	RequestID            string
}

type ReviewVisualGenerationOptions struct {
	JSON          bool
	WriteArtifact bool
}

type GeneratedAssetsArtifact struct {
	SchemaVersion string                 `json:"schema_version"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	AgentPlanID   string                 `json:"agent_plan_id"`
	Status        string                 `json:"status"`
	Assets        []GeneratedVisualAsset `json:"assets"`
	Warnings      []string               `json:"warnings,omitempty"`
	Errors        []string               `json:"errors,omitempty"`
}

type GeneratedVisualAsset struct {
	ID                    string         `json:"id"`
	VisualRequestID       string         `json:"visual_request_id"`
	RequirementID         string         `json:"requirement_id"`
	Kind                  string         `json:"kind"`
	Capability            string         `json:"capability"`
	Route                 string         `json:"route"`
	Backend               string         `json:"backend"`
	Provider              string         `json:"provider"`
	Model                 string         `json:"model,omitempty"`
	Status                string         `json:"status"`
	OutputFile            string         `json:"output_file,omitempty"`
	ContentType           string         `json:"content_type,omitempty"`
	Bytes                 int64          `json:"bytes,omitempty"`
	RequestAuditArtifact  string         `json:"request_audit_artifact,omitempty"`
	ResponseAuditArtifact string         `json:"response_audit_artifact,omitempty"`
	OutputURL             string         `json:"output_url,omitempty"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	Warnings              []string       `json:"warnings,omitempty"`
	Error                 string         `json:"error,omitempty"`
}

type visualHTTPDeps struct {
	client HTTPDoer
	now    func() time.Time
}

func ExecuteVisualRequests(planID string, stdout io.Writer, opts ExecuteVisualRequestsOptions) error {
	return executeVisualRequestsWithDeps(planID, stdout, opts, visualHTTPDeps{client: defaultHTTPClient, now: func() time.Time { return time.Now().UTC() }})
}

func executeVisualRequestsWithDeps(planID string, stdout io.Writer, opts ExecuteVisualRequestsOptions, deps visualHTTPDeps) error {
	if strings.TrimSpace(planID) == "" {
		return fmt.Errorf("execute-visual-requests requires an agent plan id")
	}
	if !opts.Yes {
		return fmt.Errorf("execute-visual-requests requires --yes")
	}
	if !opts.AllowProviderCalls {
		return fmt.Errorf("execute-visual-requests requires --allow-provider-calls")
	}
	if !opts.AllowExternalNetwork {
		return fmt.Errorf("execute-visual-requests requires --allow-external-network")
	}
	if deps.client == nil {
		deps.client = defaultHTTPClient
	}
	if deps.now == nil {
		deps.now = func() time.Time { return time.Now().UTC() }
	}
	planDir := filepath.Join(agentPlansV1Root, planID)
	if _, err := os.Stat(filepath.Join(planDir, "agent_plan.json")); err != nil {
		return err
	}
	visualRequests, err := readVisualRequestsDryRun(planID)
	if err != nil {
		return err
	}
	if len(visualRequests.Requests) == 0 {
		return fmt.Errorf("no visual requests found for plan %s", planID)
	}
	outPath := filepath.Join(planDir, generatedAssetsArtifact)
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("%s already exists; use --overwrite", generatedAssetsArtifact)
		}
	}
	now := deps.now()
	result := GeneratedAssetsArtifact{
		SchemaVersion: generatedAssetsSchema,
		CreatedAt:     now,
		UpdatedAt:     now,
		AgentPlanID:   planID,
		Status:        "completed",
	}
	outputsDir := filepath.Join(planDir, "outputs", "visual_assets")
	auditDir := filepath.Join(planDir, "outputs", "visual_audits")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		return err
	}
	for _, visualReq := range visualRequests.Requests {
		if opts.RequestID != "" && visualReq.ID != opts.RequestID {
			continue
		}
		asset := GeneratedVisualAsset{
			ID:              fmt.Sprintf("generated_asset_%04d", len(result.Assets)+1),
			VisualRequestID: visualReq.ID,
			RequirementID:   visualReq.RequirementID,
			Kind:            visualReq.Kind,
			Capability:      visualReq.Capability,
			Route:           visualReq.Route,
			Backend:         visualReq.Backend,
			Provider:        visualReq.Provider,
			Model:           visualReq.Model,
			Status:          "completed",
			Metadata:        map[string]any{},
		}
		if visualReq.Status != "previewed" {
			asset.Status = "skipped"
			asset.Error = fmt.Sprintf("visual request status is %s", visualReq.Status)
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s skipped: %s", visualReq.ID, asset.Error))
			result.Assets = append(result.Assets, asset)
			continue
		}
		if visualReq.Provider != "custom-http-visual" {
			asset.Status = "failed"
			asset.Error = fmt.Sprintf("backend provider %q is not supported; expected custom-http-visual", visualReq.Provider)
			result.Errors = append(result.Errors, fmt.Sprintf("%s failed: %s", visualReq.ID, asset.Error))
			result.Assets = append(result.Assets, asset)
			continue
		}
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			asset.Status = "failed"
			asset.Error = fmt.Sprintf("load config: %v", err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s failed: %s", visualReq.ID, asset.Error))
			result.Assets = append(result.Assets, asset)
			continue
		}
		backend, ok := cfg.Tools.Backends[visualReq.Backend]
		if !ok {
			asset.Status = "failed"
			asset.Error = fmt.Sprintf("backend %q not found", visualReq.Backend)
			result.Errors = append(result.Errors, fmt.Sprintf("%s failed: %s", visualReq.ID, asset.Error))
			result.Assets = append(result.Assets, asset)
			continue
		}
		out, err := executeCustomHTTPVisual(planDir, visualReq, backend, deps.client)
		if err != nil {
			asset.Status = "failed"
			asset.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("%s failed: %s", visualReq.ID, err.Error()))
		}
		asset.OutputFile = out.OutputFile
		asset.ContentType = out.ContentType
		asset.Bytes = out.Bytes
		asset.RequestAuditArtifact = out.RequestAuditArtifact
		asset.ResponseAuditArtifact = out.ResponseAuditArtifact
		asset.OutputURL = out.OutputURL
		asset.Metadata = out.Metadata
		result.Assets = append(result.Assets, asset)
	}
	if opts.RequestID != "" && len(result.Assets) == 0 {
		return fmt.Errorf("visual request %q not found", opts.RequestID)
	}
	if len(result.Errors) > 0 {
		result.Status = "failed"
	}
	if err := writeJSONFile(outPath, result); err != nil {
		return err
	}
	_ = updateCreativeOutputsIndex(planID, "generated_assets", generatedAssetsArtifact, "")
	appendAgentPlanEvent(planID, "AGENT_VISUAL_REQUESTS_EXECUTED", map[string]any{"plan_id": planID, "assets": len(result.Assets), "status": result.Status})
	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Visual request execution")
	fmt.Fprintf(stdout, "  plan id: %s\n", planID)
	fmt.Fprintf(stdout, "  status:  %s\n", result.Status)
	fmt.Fprintf(stdout, "  assets:  %d\n", len(result.Assets))
	fmt.Fprintf(stdout, "  path:    %s\n", outPath)
	for _, asset := range result.Assets {
		fmt.Fprintf(stdout, "  - %s %s %s\n", asset.VisualRequestID, asset.Status, emptyDash(asset.OutputFile))
	}
	return nil
}

type visualExecutionOutput struct {
	OutputFile            string
	ContentType           string
	Bytes                 int64
	RequestAuditArtifact  string
	ResponseAuditArtifact string
	OutputURL             string
	Metadata              map[string]any
}

func executeCustomHTTPVisual(planDir string, visualReq VisualGenerationRequest, backend config.ToolBackendConfig, client HTTPDoer) (visualExecutionOutput, error) {
	if backend.Provider != "custom-http-visual" {
		return visualExecutionOutput{}, fmt.Errorf("provider %q is not custom-http-visual", backend.Provider)
	}
	if backend.Endpoint == "" {
		return visualExecutionOutput{}, fmt.Errorf("backend endpoint is required")
	}
	bodyMap := renderVisualBodyTemplate(backend.Request.BodyTemplate, visualReq, backend)
	if len(bodyMap) == 0 {
		bodyMap = map[string]any{
			"prompt":           fmt.Sprint(visualReq.RequestPreview["prompt"]),
			"aspect_ratio":     fmt.Sprint(visualReq.RequestPreview["aspect_ratio"]),
			"duration_seconds": visualReq.RequestPreview["duration_seconds"],
			"model":            backend.Model,
		}
	}
	bodyBytes, _ := json.Marshal(bodyMap)
	method := strings.ToUpper(strings.TrimSpace(backend.Request.Method))
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, backend.Endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return visualExecutionOutput{}, err
	}
	if len(backend.Request.Headers) == 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range backend.Request.Headers {
		req.Header.Set(key, value)
	}
	if err := applyToolAuth(req, backend.Auth); err != nil {
		return visualExecutionOutput{}, err
	}
	requestAudit := map[string]any{
		"method":   method,
		"endpoint": backend.Endpoint,
		"headers":  scrubHeaders(req.Header),
		"body":     bodyMap,
	}
	requestAuditRel := filepath.ToSlash(filepath.Join("outputs", "visual_audits", visualReq.ID+"_request.json"))
	if err := writeJSONFile(filepath.Join(planDir, requestAuditRel), requestAudit); err != nil {
		return visualExecutionOutput{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	responseAudit := map[string]any{
		"status_code":  resp.StatusCode,
		"headers":      scrubHeaders(resp.Header),
		"body_preview": truncate(string(respBody), 4000),
	}
	responseAuditRel := filepath.ToSlash(filepath.Join("outputs", "visual_audits", visualReq.ID+"_response.json"))
	if err := writeJSONFile(filepath.Join(planDir, responseAuditRel), responseAudit); err != nil {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel}, fmt.Errorf("backend returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	var payload map[string]any
	_ = json.Unmarshal(respBody, &payload)
	if errText := jsonPathString(payload, backend.Response.ErrorJSONPath); errText != "" {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel}, fmt.Errorf("backend error: %s", errText)
	}
	outputBytes := []byte{}
	contentType := ""
	outputURL := jsonPathString(payload, backend.Response.OutputURLJSONPath)
	if base64Text := jsonPathString(payload, backend.Response.OutputBase64JSONPath); base64Text != "" {
		decoded, err := base64.StdEncoding.DecodeString(stripDataURLPrefix(base64Text))
		if err != nil {
			return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, fmt.Errorf("decode base64 output: %w", err)
		}
		outputBytes = decoded
		contentType = contentTypeFromDataURL(base64Text)
	}
	if len(outputBytes) == 0 && outputURL != "" {
		downloadReq, err := http.NewRequest(http.MethodGet, outputURL, nil)
		if err != nil {
			return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, err
		}
		downloadResp, err := client.Do(downloadReq)
		if err != nil {
			return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, err
		}
		defer downloadResp.Body.Close()
		outputBytes, _ = io.ReadAll(downloadResp.Body)
		contentType = downloadResp.Header.Get("Content-Type")
		if downloadResp.StatusCode < 200 || downloadResp.StatusCode >= 300 {
			return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, fmt.Errorf("download returned HTTP %d", downloadResp.StatusCode)
		}
	}
	if len(outputBytes) == 0 {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, fmt.Errorf("backend response did not include output_url or output_base64")
	}
	ext := extensionFromVisualContentType(contentType, visualReq.Kind)
	outputRel := filepath.ToSlash(filepath.Join("outputs", "visual_assets", visualReq.ID+ext))
	if err := os.WriteFile(filepath.Join(planDir, outputRel), outputBytes, 0o644); err != nil {
		return visualExecutionOutput{RequestAuditArtifact: requestAuditRel, ResponseAuditArtifact: responseAuditRel, OutputURL: outputURL}, err
	}
	return visualExecutionOutput{
		OutputFile:            outputRel,
		ContentType:           contentType,
		Bytes:                 int64(len(outputBytes)),
		RequestAuditArtifact:  requestAuditRel,
		ResponseAuditArtifact: responseAuditRel,
		OutputURL:             outputURL,
		Metadata:              map[string]any{"status": jsonPathString(payload, backend.Response.StatusJSONPath)},
	}, nil
}

func renderVisualBodyTemplate(template map[string]any, visualReq VisualGenerationRequest, backend config.ToolBackendConfig) map[string]any {
	out := map[string]any{}
	for key, value := range template {
		out[key] = renderVisualTemplateValue(value, visualReq, backend)
	}
	return out
}

func renderVisualTemplateValue(value any, visualReq VisualGenerationRequest, backend config.ToolBackendConfig) any {
	switch v := value.(type) {
	case string:
		replacements := map[string]string{
			"{{prompt}}":           fmt.Sprint(visualReq.RequestPreview["prompt"]),
			"{{aspect_ratio}}":     fmt.Sprint(visualReq.RequestPreview["aspect_ratio"]),
			"{{duration_seconds}}": fmt.Sprint(visualReq.RequestPreview["duration_seconds"]),
			"{{model}}":            backend.Model,
			"{{capability}}":       visualReq.Capability,
			"{{kind}}":             visualReq.Kind,
		}
		for old, replacement := range replacements {
			v = strings.ReplaceAll(v, old, replacement)
		}
		return v
	default:
		return v
	}
}

func applyToolAuth(req *http.Request, auth config.ToolAuthConfig) error {
	switch auth.Type {
	case "", "none":
		return nil
	case "bearer_env":
		if auth.Env == "" {
			return fmt.Errorf("auth env var name is required for bearer_env")
		}
		value := os.Getenv(auth.Env)
		if value == "" {
			return fmt.Errorf("missing required env var %s", auth.Env)
		}
		req.Header.Set("Authorization", "Bearer "+value)
	case "header_env":
		if auth.Env == "" || auth.Header == "" {
			return fmt.Errorf("auth env and header are required for header_env")
		}
		value := os.Getenv(auth.Env)
		if value == "" {
			return fmt.Errorf("missing required env var %s", auth.Env)
		}
		req.Header.Set(auth.Header, value)
	case "query_env":
		if auth.Env == "" {
			return fmt.Errorf("auth env var name is required for query_env")
		}
		value := os.Getenv(auth.Env)
		if value == "" {
			return fmt.Errorf("missing required env var %s", auth.Env)
		}
		q := req.URL.Query()
		key := auth.Header
		if key == "" {
			key = "api_key"
		}
		q.Set(key, value)
		req.URL.RawQuery = q.Encode()
	default:
		return fmt.Errorf("unsupported auth type %q", auth.Type)
	}
	return nil
}

func scrubHeaders(headers http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "authorization") || strings.Contains(lower, "api-key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			out[key] = "<redacted>"
			continue
		}
		out[key] = strings.Join(values, ", ")
	}
	return out
}

func jsonPathString(payload map[string]any, path string) string {
	if path == "" || payload == nil {
		return ""
	}
	path = strings.TrimPrefix(strings.TrimSpace(path), "$.")
	if path == "" {
		return ""
	}
	var current any = payload
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[part]
	}
	switch v := current.(type) {
	case string:
		return v
	case float64, bool:
		return fmt.Sprint(v)
	default:
		return ""
	}
}

func stripDataURLPrefix(value string) string {
	if idx := strings.Index(value, ","); strings.HasPrefix(value, "data:") && idx >= 0 {
		return value[idx+1:]
	}
	return value
}

func contentTypeFromDataURL(value string) string {
	if !strings.HasPrefix(value, "data:") {
		return ""
	}
	rest := strings.TrimPrefix(value, "data:")
	if idx := strings.Index(rest, ";"); idx >= 0 {
		return rest[:idx]
	}
	return ""
}

func extensionFromVisualContentType(contentType string, kind string) string {
	if ext, err := mime.ExtensionsByType(strings.TrimSpace(strings.Split(contentType, ";")[0])); err == nil && len(ext) > 0 {
		return ext[0]
	}
	switch kind {
	case "generated_broll":
		return ".mp4"
	default:
		return ".png"
	}
}

func readGeneratedAssets(planID string) (GeneratedAssetsArtifact, error) {
	var artifact GeneratedAssetsArtifact
	err := readJSONFile(filepath.Join(agentPlansV1Root, planID, generatedAssetsArtifact), &artifact)
	return artifact, err
}

func ReviewVisualGeneration(planID string, stdout io.Writer, opts ReviewVisualGenerationOptions) error {
	artifact, err := readGeneratedAssets(planID)
	if err != nil {
		return err
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(artifact, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	review := renderVisualGenerationReview(artifact)
	if opts.WriteArtifact {
		path := filepath.Join(agentPlansV1Root, planID, visualGenerationReviewArtifact)
		if err := os.WriteFile(path, []byte(review), 0o644); err != nil {
			return err
		}
		_ = updateCreativeOutputsIndex(planID, "visual_generation_review", visualGenerationReviewArtifact, "")
	}
	fmt.Fprint(stdout, review)
	return nil
}

func renderVisualGenerationReview(artifact GeneratedAssetsArtifact) string {
	var b strings.Builder
	b.WriteString("# Visual Generation Review\n\n")
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", artifact.AgentPlanID)
	fmt.Fprintf(&b, "- Status: `%s`\n", artifact.Status)
	b.WriteString("\n## Generated Assets\n\n")
	b.WriteString("| Request | Kind | Status | Backend | Output | Bytes |\n|---|---|---|---|---|---:|\n")
	for _, asset := range artifact.Assets {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d |\n", asset.VisualRequestID, asset.Kind, asset.Status, asset.Backend, emptyDash(asset.OutputFile), asset.Bytes)
	}
	if len(artifact.Errors) > 0 {
		b.WriteString("\n## Errors\n\n")
		for _, errText := range artifact.Errors {
			fmt.Fprintf(&b, "- %s\n", errText)
		}
	}
	if len(artifact.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range artifact.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func sortedGeneratedAssetOutputs(artifact GeneratedAssetsArtifact) []string {
	outputs := []string{}
	for _, asset := range artifact.Assets {
		if asset.OutputFile != "" {
			outputs = append(outputs, asset.OutputFile)
		}
	}
	sort.Strings(outputs)
	return outputs
}

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/modelrouter"
)

// ---- schema ----

type CaptionVariantSource struct {
	SourceType     string `json:"source_type"`     // script_draft|script_text|goal
	SourceArtifact string `json:"source_artifact"` // relative path or ""
}

type CaptionVariantRequest struct {
	Count    int    `json:"count,omitempty"`
	MaxWords int    `json:"max_words,omitempty"`
	Tone     string `json:"tone,omitempty"`
}

type CaptionVariant struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Style  string `json:"style,omitempty"`  // hook|caption|cta|short
	Reason string `json:"reason,omitempty"`
}

type CaptionVariantsOutput struct {
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	CreativePlanID string               `json:"creative_plan_id"`
	Mode          string                `json:"mode"`
	Provider      string                `json:"provider,omitempty"`
	Model         string                `json:"model,omitempty"`
	Route         string                `json:"route,omitempty"`
	Backend       string                `json:"backend,omitempty"`
	Source        CaptionVariantSource  `json:"source"`
	StyleContext  *StyleContext          `json:"style_context,omitempty"`
	Request       CaptionVariantRequest `json:"request,omitempty"`
	Variants      []CaptionVariant      `json:"variants"`
	Warnings      []string              `json:"warnings,omitempty"`
}

// ---- options ----

type CaptionVariantsOptions struct {
	Overwrite    bool
	JSON         bool
	StyleDir     string
	NoStyle      bool
	ModelEntry   string
	Route        string
	FallbackStub bool
	Count        int
	MaxWords     int
	Tone         string
}

type ReviewCaptionVariantsOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- main command ----

func CaptionVariants(planID string, stdout io.Writer, opts CaptionVariantsOptions) error {
	adapter := modelrouter.NewOllamaAdapter()
	return captionVariantsWithAdapter(planID, stdout, opts, adapter)
}

func captionVariantsWithAdapter(planID string, stdout io.Writer, opts CaptionVariantsOptions, adapter modelrouter.Adapter) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")
	outputsDir := filepath.Join(planDir, "outputs")

	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var plan CreativePlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	outPath := filepath.Join(outputsDir, "caption_variants.json")
	if _, err := os.Stat(outPath); err == nil && !opts.Overwrite {
		return fmt.Errorf("caption_variants.json already exists; use --overwrite to replace")
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

	writeEvent("CAPTION_VARIANTS_STARTED", map[string]any{"plan_id": planID})

	// Resolve backend
	routeKey := opts.Route
	if routeKey == "" {
		routeKey = "creative.captions"
	}
	backendName, backendCfg, err := resolveCaptionBackend(routeKey, opts.ModelEntry)
	if err != nil {
		writeEvent("CAPTION_VARIANTS_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		if opts.FallbackStub {
			return writeCaptionVariantsStub(planID, planDir, plan, opts, "no backend — "+err.Error(), outputsDir, stdout)
		}
		return err
	}

	if !adapter.Supports(backendCfg.Provider) {
		msg := fmt.Sprintf("creative-caption-variants currently supports local Ollama only; route %q points to provider %q", routeKey, backendCfg.Provider)
		writeEvent("CAPTION_VARIANTS_FAILED", map[string]any{"plan_id": planID, "reason": msg})
		if opts.FallbackStub {
			return writeCaptionVariantsStub(planID, planDir, plan, opts, msg, outputsDir, stdout)
		}
		return fmt.Errorf("%s", msg)
	}

	// Load style pack
	count := opts.Count
	if count <= 0 {
		count = 5
	}
	maxWords := opts.MaxWords
	if maxWords <= 0 {
		maxWords = 12
	}
	var stylectx *StyleContext
	if !opts.NoStyle {
		stylectx = LoadStylePack(opts.StyleDir, styleMaxChars)
		if stylectx.Enabled {
			writeEvent("STYLE_PACK_LOADED", map[string]any{
				"plan_id":    planID,
				"style_dir":  stylectx.StyleDir,
				"files_used": stylectx.FilesUsed,
			})
		}
	}

	// Resolve source text
	sourceText, sourceType, sourceArtifact := resolveCaptionSource(planDir, plan.Goal)

	// Build prompt
	prompt := buildCaptionVariantsPrompt(sourceText, sourceType, plan.Goal, stylectx, count, maxWords, opts.Tone)

	modelReq := modelrouter.Request{
		TaskID:    "caption_" + planID,
		TaskType:  "caption_variants",
		RouteName: routeKey,
		Provider:  backendCfg.Provider,
		Model:     backendCfg.Model,
		BaseURL:   backendCfg.Endpoint,
		RequestPreview: modelrouter.RequestPreview{
			System: "You are a creative caption writer. Output valid JSON only.",
			User:   prompt,
		},
	}

	fmt.Fprintf(stdout, "creative-caption-variants: %s\n", planID)
	fmt.Fprintf(stdout, "  goal:     %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  source:   %s (%s)\n", sourceType, sourceArtifact)
	fmt.Fprintf(stdout, "  route:    %s → %s (%s/%s)\n", routeKey, backendName, backendCfg.Provider, backendCfg.Model)
	if stylectx != nil && stylectx.Enabled {
		fmt.Fprintf(stdout, "  style:    %s (%d files)\n", stylectx.StyleDir, len(stylectx.FilesUsed))
	} else {
		fmt.Fprintf(stdout, "  style:    (none)\n")
	}
	fmt.Fprintf(stdout, "  count:    %d, max_words: %d\n", count, maxWords)
	fmt.Fprintf(stdout, "  calling Ollama at %s...\n", backendCfg.Endpoint)

	resp, err := adapter.Execute(modelReq)
	if err != nil {
		writeEvent("CAPTION_VARIANTS_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		fmt.Fprintf(stdout, "  error:    %v\n", err)
		if opts.FallbackStub {
			return writeCaptionVariantsStub(planID, planDir, plan, opts, "Ollama call failed: "+err.Error(), outputsDir, stdout)
		}
		return fmt.Errorf("Ollama call failed: %w", err)
	}

	rawText := ""
	if len(resp.Texts) > 0 {
		rawText = strings.TrimSpace(resp.Texts[0])
	}

	variants, parseWarnings := parseCaptionVariantsResponse(rawText, count, plan.Goal)

	var allWarnings []string
	allWarnings = append(allWarnings, resp.Warnings...)
	allWarnings = append(allWarnings, parseWarnings...)
	if stylectx != nil {
		allWarnings = append(allWarnings, stylectx.Warnings...)
	}

	output := CaptionVariantsOutput{
		SchemaVersion:  "caption_variants.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Mode:           "ollama",
		Provider:       backendCfg.Provider,
		Model:          backendCfg.Model,
		Route:          routeKey,
		Backend:        backendName,
		Source: CaptionVariantSource{
			SourceType:     sourceType,
			SourceArtifact: sourceArtifact,
		},
		StyleContext: stylectx,
		Request:      CaptionVariantRequest{Count: count, MaxWords: maxWords, Tone: opts.Tone},
		Variants:     variants,
		Warnings:     allWarnings,
	}

	if err := writeJSONFile(outPath, output); err != nil {
		return fmt.Errorf("write caption_variants.json: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "caption_variants", "outputs/caption_variants.json", "")

	writeEvent("CAPTION_VARIANTS_COMPLETED", map[string]any{
		"plan_id": planID, "mode": "ollama", "model": backendCfg.Model,
		"count": len(variants),
	})

	fmt.Fprintf(stdout, "  done:     caption_variants.json written (%d variants)\n", len(variants))
	for _, w := range allWarnings {
		fmt.Fprintf(stdout, "  warning:  %s\n", w)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}
	return nil
}

// ---- review-caption-variants ----

func ReviewCaptionVariants(planID string, stdout io.Writer, opts ReviewCaptionVariantsOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	variantsPath := filepath.Join(planDir, "outputs", "caption_variants.json")

	data, err := os.ReadFile(variantsPath)
	if err != nil {
		return fmt.Errorf("caption_variants.json not found for plan %q; run creative-caption-variants first: %w", planID, err)
	}
	var output CaptionVariantsOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return fmt.Errorf("caption_variants.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	text := renderCaptionVariantsReviewMD(output)
	fmt.Fprint(stdout, text)

	if opts.WriteArtifact {
		outputsDir := filepath.Join(planDir, "outputs")
		if err := os.MkdirAll(outputsDir, 0o755); err != nil {
			return fmt.Errorf("create outputs dir: %w", err)
		}
		reviewPath := filepath.Join(outputsDir, "caption_variants_review.md")
		if err := os.WriteFile(reviewPath, []byte(text), 0o644); err != nil {
			return fmt.Errorf("write caption_variants_review.md: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "caption_variants_review", "outputs/caption_variants_review.md", "")
		fmt.Fprintf(stdout, "\nWritten: %s\n", reviewPath)
	}
	return nil
}

func renderCaptionVariantsReviewMD(o CaptionVariantsOutput) string {
	var b strings.Builder
	b.WriteString("# Caption Variants Review\n\n")
	fmt.Fprintf(&b, "- **Plan ID:** %s\n", o.CreativePlanID)
	fmt.Fprintf(&b, "- **Mode:** %s\n", o.Mode)
	if o.Provider != "" {
		fmt.Fprintf(&b, "- **Provider:** %s\n", o.Provider)
	}
	if o.Model != "" {
		fmt.Fprintf(&b, "- **Model:** %s\n", o.Model)
	}
	if o.Route != "" {
		fmt.Fprintf(&b, "- **Route:** %s\n", o.Route)
	}
	fmt.Fprintf(&b, "- **Source:** %s", o.Source.SourceType)
	if o.Source.SourceArtifact != "" {
		fmt.Fprintf(&b, " (%s)", o.Source.SourceArtifact)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- **Variant Count:** %d\n", len(o.Variants))
	if o.StyleContext != nil && o.StyleContext.Enabled {
		fmt.Fprintf(&b, "- **Style Used:** %s (%d files)\n", o.StyleContext.StyleDir, len(o.StyleContext.FilesUsed))
		for _, f := range o.StyleContext.FilesUsed {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	} else {
		fmt.Fprintf(&b, "- **Style Used:** no\n")
	}

	if len(o.Variants) > 0 {
		b.WriteString("\n## Variants\n\n")
		for _, v := range o.Variants {
			fmt.Fprintf(&b, "### %s\n", v.ID)
			fmt.Fprintf(&b, "**Text:** %s\n\n", v.Text)
			if v.Style != "" {
				fmt.Fprintf(&b, "**Style:** %s\n\n", v.Style)
			}
			if v.Reason != "" {
				fmt.Fprintf(&b, "**Reason:** %s\n\n", v.Reason)
			}
		}
	}

	if len(o.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range o.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	return b.String()
}

// ---- backend resolution ----

func resolveCaptionBackend(routeKey, modelEntry string) (string, config.ToolBackendConfig, error) {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return "", config.ToolBackendConfig{}, fmt.Errorf("load config: %w", err)
	}
	if !cfg.Tools.Enabled {
		return "", config.ToolBackendConfig{}, fmt.Errorf(
			"tools.enabled is false in byom-video.yaml; add:\n\ntools:\n  enabled: true\n  backends:\n    local_writer:\n      kind: text_generation\n      provider: ollama\n      model: qwen2.5:7b\n      endpoint: http://localhost:11434\n      auth:\n        type: none\n  routes:\n    %s: local_writer", routeKey)
	}

	if modelEntry != "" {
		backend, ok := cfg.Tools.Backends[modelEntry]
		if !ok {
			return "", config.ToolBackendConfig{}, fmt.Errorf("tools backend %q not found in config", modelEntry)
		}
		return modelEntry, backend, nil
	}

	backendName, ok := cfg.Tools.Routes[routeKey]
	if !ok {
		// Try fallback routes in order
		for _, alt := range []string{"creative.script", "caption_generation"} {
			if name, found := cfg.Tools.Routes[alt]; found {
				backendName = name
				ok = true
				break
			}
		}
	}
	if !ok || backendName == "" {
		return "", config.ToolBackendConfig{}, fmt.Errorf(
			"no route %q configured; add to byom-video.yaml:\n\ntools:\n  routes:\n    %s: local_writer", routeKey, routeKey)
	}

	backend, ok := cfg.Tools.Backends[backendName]
	if !ok {
		return "", config.ToolBackendConfig{}, fmt.Errorf("route %q points to backend %q but that backend is not configured", routeKey, backendName)
	}
	return backendName, backend, nil
}

// ---- source resolution ----

func resolveCaptionSource(planDir, goal string) (text, sourceType, sourceArtifact string) {
	// Priority 1: script_draft.json
	scriptJSONPath := filepath.Join(planDir, "outputs", "script_draft.json")
	if data, err := os.ReadFile(scriptJSONPath); err == nil {
		var s CreativeScriptOutput
		if json.Unmarshal(data, &s) == nil && strings.TrimSpace(s.Text) != "" {
			return s.Text, "script_draft", "outputs/script_draft.json"
		}
	}

	// Priority 2: script_draft.txt
	scriptTXTPath := filepath.Join(planDir, "outputs", "script_draft.txt")
	if data, err := os.ReadFile(scriptTXTPath); err == nil {
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, "script_text", "outputs/script_draft.txt"
		}
	}

	// Priority 3: goal text
	return goal, "goal", ""
}

// ---- prompt ----

func buildCaptionVariantsPrompt(sourceText, sourceType, goal string, stylectx *StyleContext, count, maxWords int, tone string) string {
	var b strings.Builder
	b.WriteString("Generate caption variants for a short video.\n\n")
	fmt.Fprintf(&b, "GOAL: %s\n", goal)
	fmt.Fprintf(&b, "COUNT: %d variants\n", count)
	fmt.Fprintf(&b, "MAX WORDS PER CAPTION: %d\n", maxWords)
	if tone != "" {
		fmt.Fprintf(&b, "TONE: %s\n", tone)
	}

	switch sourceType {
	case "script_draft", "script_text":
		b.WriteString("\nSOURCE SCRIPT:\n")
		preview := sourceText
		if len(preview) > 600 {
			preview = preview[:600] + "..."
		}
		b.WriteString(preview)
		b.WriteString("\n")
	case "goal":
		b.WriteString("\nNOTE: No script available. Generate captions from the goal only.\n")
	}

	if stylectx != nil && stylectx.Enabled && stylectx.content != "" {
		b.WriteString("\nSTYLE CONTEXT:\n\n")
		b.WriteString(stylectx.content)
		b.WriteString("\n")
	}

	b.WriteString("\nIMPORTANT:\n")
	b.WriteString("- Do not invent facts about the video content.\n")
	b.WriteString("- Each caption should be punchy, short, and suitable for on-screen text.\n")
	fmt.Fprintf(&b, "- Keep each caption under %d words.\n", maxWords)
	b.WriteString("- Use styles: hook, caption, cta, or short.\n\n")

	b.WriteString("Respond with valid JSON in exactly this format:\n")
	b.WriteString(`{
  "variants": [
    {
      "text": "Caption text here",
      "style": "hook",
      "reason": "Why this works"
    }
  ]
}`)
	b.WriteString("\n\nRespond with JSON only. No markdown, no explanation.")
	return b.String()
}

// ---- response parsing ----

func parseCaptionVariantsResponse(raw string, count int, fallbackGoal string) ([]CaptionVariant, []string) {
	var warnings []string
	jsonStr := extractJSON(raw)

	var parsed struct {
		Variants []struct {
			Text   string `json:"text"`
			Style  string `json:"style"`
			Reason string `json:"reason"`
		} `json:"variants"`
	}

	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil && len(parsed.Variants) > 0 {
			variants := make([]CaptionVariant, 0, len(parsed.Variants))
			for i, v := range parsed.Variants {
				if strings.TrimSpace(v.Text) == "" {
					continue
				}
				variants = append(variants, CaptionVariant{
					ID:     fmt.Sprintf("caption_variant_%04d", i+1),
					Text:   strings.TrimSpace(v.Text),
					Style:  v.Style,
					Reason: v.Reason,
				})
			}
			if len(variants) > 0 {
				return variants, warnings
			}
		}
	}

	// Fallback: split lines into variants
	warnings = append(warnings, "Ollama response was not valid JSON; split into line-based variants.")
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	variants := make([]CaptionVariant, 0, count)
	idx := 1
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		if line == "" || len(line) < 3 {
			continue
		}
		variants = append(variants, CaptionVariant{
			ID:   fmt.Sprintf("caption_variant_%04d", idx),
			Text: line,
		})
		idx++
		if idx > count {
			break
		}
	}
	if len(variants) == 0 {
		// Last resort: single variant from goal
		variants = append(variants, CaptionVariant{
			ID:   "caption_variant_0001",
			Text: fallbackGoal,
		})
	}
	return variants, warnings
}

// ---- fallback stub ----

func writeCaptionVariantsStub(planID, planDir string, plan CreativePlan, opts CaptionVariantsOptions, warnMsg string, outputsDir string, stdout io.Writer) error {
	count := opts.Count
	if count <= 0 {
		count = 5
	}
	maxWords := opts.MaxWords
	if maxWords <= 0 {
		maxWords = 12
	}

	_, sourceType, sourceArtifact := resolveCaptionSource(planDir, plan.Goal)

	// Deterministic stub variants based on goal
	goalWords := strings.Fields(plan.Goal)
	stub := []CaptionVariant{
		{ID: "caption_variant_0001", Text: plan.Goal, Style: "hook", Reason: "direct goal statement"},
	}
	if len(goalWords) > 3 {
		stub = append(stub, CaptionVariant{
			ID: "caption_variant_0002", Text: strings.Join(goalWords[:3], " ") + "...", Style: "short",
		})
	}
	for i := len(stub) + 1; i <= count; i++ {
		stub = append(stub, CaptionVariant{
			ID:   fmt.Sprintf("caption_variant_%04d", i),
			Text: fmt.Sprintf("[stub caption %d for: %s]", i, plan.Goal),
		})
	}
	if len(stub) > count {
		stub = stub[:count]
	}

	output := CaptionVariantsOutput{
		SchemaVersion:  "caption_variants.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Mode:           "stub",
		Source:         CaptionVariantSource{SourceType: sourceType, SourceArtifact: sourceArtifact},
		Request:        CaptionVariantRequest{Count: count, MaxWords: maxWords, Tone: opts.Tone},
		Variants:       stub,
		Warnings:       []string{warnMsg},
	}

	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return fmt.Errorf("create outputs dir: %w", err)
	}
	outPath := filepath.Join(outputsDir, "caption_variants.json")
	if err := writeJSONFile(outPath, output); err != nil {
		return fmt.Errorf("write caption_variants.json: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "caption_variants", "outputs/caption_variants.json", "")
	fmt.Fprintf(stdout, "  fallback-stub: caption_variants.json written (%d variants, mode=stub)\n", len(stub))
	fmt.Fprintf(stdout, "  warning:  %s\n", warnMsg)
	return nil
}

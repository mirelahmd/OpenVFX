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

// ---- options ----

type CreativeGenerateScriptOptions struct {
	Overwrite    bool
	JSON         bool
	StyleDir     string
	NoStyle      bool
	ModelEntry   string
	Route        string
	FallbackStub bool
	MaxWords     int
	Tone         string
}

type ReviewScriptOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- creative-generate-script ----

// scriptGeneratorFunc is used for dependency injection in tests.
type scriptGeneratorFunc func(req modelrouter.Request) (modelrouter.Response, error)

func CreativeGenerateScript(planID string, stdout io.Writer, opts CreativeGenerateScriptOptions) error {
	adapter := modelrouter.NewOllamaAdapter()
	return creativeGenerateScriptWithAdapter(planID, stdout, opts, adapter)
}

func creativeGenerateScriptWithAdapter(planID string, stdout io.Writer, opts CreativeGenerateScriptOptions, adapter modelrouter.Adapter) error {
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

	// Find generate_script step
	var scriptStep *CreativeStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == "generate_script" {
			scriptStep = &plan.Steps[i]
			break
		}
	}
	if scriptStep == nil {
		return fmt.Errorf("creative plan %s has no generate_script step; does the goal require script generation?", planID)
	}

	scriptPath := filepath.Join(outputsDir, "script_draft.json")
	if _, err := os.Stat(scriptPath); err == nil && !opts.Overwrite {
		return fmt.Errorf("script_draft.json already exists; use --overwrite to replace")
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
	writeEvent := func(eventLog *events.Log, kind string, data map[string]any) {
		if eventLog != nil {
			_ = eventLog.Write(kind, data)
		}
	}

	writeEvent(eventLog, "CREATIVE_SCRIPT_GENERATION_STARTED", map[string]any{
		"plan_id": planID, "step_id": scriptStep.ID,
	})

	// Resolve backend from config
	routeKey := opts.Route
	if routeKey == "" {
		routeKey = "creative.script"
	}
	backendName, backendCfg, err := resolveScriptBackend(routeKey, opts.ModelEntry)
	if err != nil {
		writeEvent(eventLog, "CREATIVE_SCRIPT_GENERATION_FAILED", map[string]any{
			"plan_id": planID, "reason": err.Error(),
		})
		if opts.FallbackStub {
			return writeScriptStub(planID, scriptStep, plan.Goal, opts, "stub", "no backend — "+err.Error(), outputsDir, stdout)
		}
		return err
	}

	// Validate provider is Ollama-compatible
	if !adapter.Supports(backendCfg.Provider) {
		msg := fmt.Sprintf("creative-generate-script currently supports local Ollama only; route %q points to provider %q", routeKey, backendCfg.Provider)
		writeEvent(eventLog, "CREATIVE_SCRIPT_GENERATION_FAILED", map[string]any{
			"plan_id": planID, "reason": msg,
		})
		if opts.FallbackStub {
			return writeScriptStub(planID, scriptStep, plan.Goal, opts, "stub", msg, outputsDir, stdout)
		}
		return fmt.Errorf("%s", msg)
	}

	// Load style pack
	maxWords := opts.MaxWords
	if maxWords <= 0 {
		maxWords = 120
	}
	var stylectx *StyleContext
	if !opts.NoStyle {
		stylectx = LoadStylePack(opts.StyleDir, styleMaxChars)
		if stylectx.Enabled {
			writeEvent(eventLog, "STYLE_PACK_LOADED", map[string]any{
				"plan_id":    planID,
				"style_dir":  stylectx.StyleDir,
				"files_used": stylectx.FilesUsed,
				"truncated":  stylectx.Truncated,
			})
			for _, w := range stylectx.Warnings {
				writeEvent(eventLog, "STYLE_PACK_WARNING", map[string]any{
					"plan_id": planID, "warning": w,
				})
			}
		}
	}

	// Build prompt
	prompt := buildScriptPrompt(plan.Goal, plan.InputPath, stylectx, maxWords, opts.Tone)

	// Build model request
	modelReq := modelrouter.Request{
		TaskID:    "script_" + planID,
		TaskType:  "script_generation",
		RouteName: routeKey,
		Provider:  backendCfg.Provider,
		Model:     backendCfg.Model,
		BaseURL:   backendCfg.Endpoint,
		RequestPreview: modelrouter.RequestPreview{
			System: "You are a creative video script writer. Output valid JSON only.",
			User:   prompt,
		},
	}

	fmt.Fprintf(stdout, "creative-generate-script: %s\n", planID)
	fmt.Fprintf(stdout, "  step:     %s\n", scriptStep.ID)
	fmt.Fprintf(stdout, "  goal:     %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  route:    %s → %s (%s/%s)\n", routeKey, backendName, backendCfg.Provider, backendCfg.Model)
	if stylectx != nil && stylectx.Enabled {
		fmt.Fprintf(stdout, "  style:    %s (%d files)\n", stylectx.StyleDir, len(stylectx.FilesUsed))
	} else {
		fmt.Fprintf(stdout, "  style:    (none)\n")
	}
	fmt.Fprintf(stdout, "  calling Ollama at %s...\n", backendCfg.Endpoint)

	resp, err := adapter.Execute(modelReq)
	if err != nil {
		writeEvent(eventLog, "CREATIVE_SCRIPT_GENERATION_FAILED", map[string]any{
			"plan_id": planID, "reason": err.Error(),
		})
		fmt.Fprintf(stdout, "  error:    %v\n", err)
		if opts.FallbackStub {
			return writeScriptStub(planID, scriptStep, plan.Goal, opts, "stub", "Ollama call failed: "+err.Error(), outputsDir, stdout)
		}
		return fmt.Errorf("Ollama call failed: %w", err)
	}

	rawText := ""
	if len(resp.Texts) > 0 {
		rawText = strings.TrimSpace(resp.Texts[0])
	}

	// Parse response
	title, hook, scriptText, onScreenText, notes, parseWarnings := parseScriptResponse(rawText)

	var allWarnings []string
	allWarnings = append(allWarnings, resp.Warnings...)
	allWarnings = append(allWarnings, parseWarnings...)

	if stylectx != nil {
		allWarnings = append(allWarnings, stylectx.Warnings...)
	}

	output := CreativeScriptOutput{
		SchemaVersion:           "creative_script.v1",
		CreatedAt:               time.Now().UTC(),
		CreativePlanID:          planID,
		StepID:                  scriptStep.ID,
		Goal:                    plan.Goal,
		Mode:                    "ollama",
		Provider:                backendCfg.Provider,
		Model:                   backendCfg.Model,
		Route:                   routeKey,
		Backend:                 backendName,
		Title:                   title,
		Hook:                    hook,
		Text:                    scriptText,
		OnScreenTextSuggestions: onScreenText,
		PlatformHint:            inferPlatformHint(plan.Goal),
		Notes:                   notes,
		StyleContext:            stylectx,
		Request:                 ScriptRequest{MaxWords: maxWords, Tone: opts.Tone},
		Warnings:                allWarnings,
	}

	if err := writeJSONFile(scriptPath, output); err != nil {
		return fmt.Errorf("write script_draft.json: %w", err)
	}
	txtPath := filepath.Join(outputsDir, "script_draft.txt")
	if err := os.WriteFile(txtPath, []byte(scriptText), 0o644); err != nil {
		return fmt.Errorf("write script_draft.txt: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "script_draft", "outputs/script_draft.json", scriptStep.ID)

	writeEvent(eventLog, "CREATIVE_SCRIPT_GENERATION_COMPLETED", map[string]any{
		"plan_id": planID, "step_id": scriptStep.ID,
		"mode": "ollama", "model": backendCfg.Model,
		"word_count": wordCount(scriptText),
	})

	fmt.Fprintf(stdout, "  done:     script_draft.json written (%d words, platform=%s)\n", wordCount(scriptText), output.PlatformHint)
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

// ---- review-script ----

func ReviewScript(planID string, stdout io.Writer, opts ReviewScriptOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	scriptPath := filepath.Join(planDir, "outputs", "script_draft.json")

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("script_draft.json not found for plan %q; run creative-generate-script or creative-execute-stub first: %w", planID, err)
	}
	var script CreativeScriptOutput
	if err := json.Unmarshal(data, &script); err != nil {
		return fmt.Errorf("script_draft.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(script)
	}

	text := renderScriptReviewMD(script)
	fmt.Fprint(stdout, text)

	if opts.WriteArtifact {
		outputsDir := filepath.Join(planDir, "outputs")
		if err := os.MkdirAll(outputsDir, 0o755); err != nil {
			return fmt.Errorf("create outputs dir: %w", err)
		}
		reviewPath := filepath.Join(outputsDir, "script_review.md")
		if err := os.WriteFile(reviewPath, []byte(text), 0o644); err != nil {
			return fmt.Errorf("write script_review.md: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "script_review", "outputs/script_review.md", script.StepID)
		fmt.Fprintf(stdout, "\nWritten: %s\n", reviewPath)
	}
	return nil
}

func renderScriptReviewMD(s CreativeScriptOutput) string {
	var b strings.Builder
	b.WriteString("# Script Review\n\n")
	fmt.Fprintf(&b, "- **Plan ID:** %s\n", s.CreativePlanID)
	fmt.Fprintf(&b, "- **Mode:** %s\n", s.Mode)
	if s.Provider != "" {
		fmt.Fprintf(&b, "- **Provider:** %s\n", s.Provider)
	}
	if s.Model != "" {
		fmt.Fprintf(&b, "- **Model:** %s\n", s.Model)
	}
	if s.Route != "" {
		fmt.Fprintf(&b, "- **Route:** %s\n", s.Route)
	}
	if s.Backend != "" {
		fmt.Fprintf(&b, "- **Backend:** %s\n", s.Backend)
	}
	if s.Title != "" {
		fmt.Fprintf(&b, "- **Title:** %s\n", s.Title)
	}
	if s.Hook != "" {
		fmt.Fprintf(&b, "- **Hook:** %s\n", s.Hook)
	}
	fmt.Fprintf(&b, "- **Word Count:** %d\n", wordCount(s.Text))
	if s.PlatformHint != "" {
		fmt.Fprintf(&b, "- **Platform:** %s\n", s.PlatformHint)
	}
	if s.StyleContext != nil && s.StyleContext.Enabled {
		fmt.Fprintf(&b, "- **Style Used:** %s (%d files)\n", s.StyleContext.StyleDir, len(s.StyleContext.FilesUsed))
		for _, f := range s.StyleContext.FilesUsed {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	} else {
		fmt.Fprintf(&b, "- **Style Used:** no\n")
	}
	if s.Text != "" {
		b.WriteString("\n## Script\n\n")
		b.WriteString(s.Text)
		b.WriteString("\n")
	}
	if len(s.OnScreenTextSuggestions) > 0 {
		b.WriteString("\n## On-Screen Text Suggestions\n\n")
		for _, t := range s.OnScreenTextSuggestions {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	if len(s.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range s.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	return b.String()
}

// ---- internal helpers ----

func resolveScriptBackend(routeKey, modelEntry string) (string, config.ToolBackendConfig, error) {
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
		// Try fallback routes
		for _, alt := range []string{"creative_script", "script_generation"} {
			if name, found := cfg.Tools.Routes[alt]; found {
				backendName = name
				ok = true
				break
			}
		}
	}
	if !ok || backendName == "" {
		return "", config.ToolBackendConfig{}, fmt.Errorf(
			"no route %q configured; add to byom-video.yaml:\n\ntools:\n  routes:\n    %s: local_writer\n\n(where local_writer is a configured backend with provider: ollama)", routeKey, routeKey)
	}

	backend, ok := cfg.Tools.Backends[backendName]
	if !ok {
		return "", config.ToolBackendConfig{}, fmt.Errorf("route %q points to backend %q but that backend is not configured", routeKey, backendName)
	}
	return backendName, backend, nil
}

func buildScriptPrompt(goal, inputPath string, stylectx *StyleContext, maxWords int, tone string) string {
	var b strings.Builder
	b.WriteString("Write a short video script based on the following information.\n\n")
	fmt.Fprintf(&b, "GOAL: %s\n", goal)
	fmt.Fprintf(&b, "MAX WORDS: %d\n", maxWords)
	if tone != "" {
		fmt.Fprintf(&b, "TONE: %s\n", tone)
	}
	if inputPath != "" {
		fmt.Fprintf(&b, "SOURCE FILE: %s\n", filepath.Base(inputPath))
	}

	if stylectx != nil && stylectx.Enabled && stylectx.content != "" {
		b.WriteString("\nSTYLE CONTEXT:\n\n")
		b.WriteString(stylectx.content)
		b.WriteString("\n")
	}

	b.WriteString("\nIMPORTANT:\n")
	b.WriteString("- Base the script only on the goal and style context provided.\n")
	b.WriteString("- Do not invent facts, statistics, or claims about video content.\n")
	b.WriteString("- This script is a concept draft based on the goal only — no transcript was provided.\n")
	fmt.Fprintf(&b, "- Keep the script under %d words.\n\n", maxWords)

	b.WriteString("Respond with valid JSON in exactly this format:\n")
	b.WriteString(`{
  "title": "Short video title",
  "hook": "Opening hook line (1-2 sentences)",
  "script": "Full script text",
  "on_screen_text": ["Text overlay suggestion 1", "Text overlay suggestion 2"],
  "notes": ["Optional note 1"]
}`)
	b.WriteString("\n\non_screen_text should contain 2-4 short text overlays to display on screen (under 8 words each).")
	b.WriteString("\nRespond with JSON only. No markdown, no explanation.")
	return b.String()
}

func parseScriptResponse(raw string) (title, hook, scriptText string, onScreenText []string, notes []string, warnings []string) {
	// Try to extract JSON from response (model may wrap it)
	jsonStr := extractJSON(raw)

	var parsed struct {
		Title        string   `json:"title"`
		Hook         string   `json:"hook"`
		Script       string   `json:"script"`
		OnScreenText []string `json:"on_screen_text"`
		Notes        []string `json:"notes"`
	}
	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			title = strings.TrimSpace(parsed.Title)
			hook = strings.TrimSpace(parsed.Hook)
			scriptText = strings.TrimSpace(parsed.Script)
			if len(scriptText) == 0 && len(raw) > 0 {
				scriptText = raw
				warnings = append(warnings, "Parsed JSON had empty script field; used raw response text.")
			}
			onScreenText = parsed.OnScreenText
			notes = parsed.Notes
			return
		}
	}

	// Fallback: treat entire response as plain text
	warnings = append(warnings, "Ollama response was not valid JSON; stored plain text fallback.")
	scriptText = raw
	// Try to extract hook from first line
	lines := strings.SplitN(strings.TrimSpace(raw), "\n", 2)
	if len(lines) > 0 && len(lines[0]) > 0 && len(lines[0]) < 200 {
		hook = strings.TrimSpace(lines[0])
	}
	return
}

// inferPlatformHint guesses the target platform from the goal text.
func inferPlatformHint(goal string) string {
	lower := strings.ToLower(goal)
	switch {
	case strings.Contains(lower, "tiktok"):
		return "tiktok"
	case strings.Contains(lower, "instagram") || strings.Contains(lower, "reels"):
		return "instagram"
	case strings.Contains(lower, "youtube shorts") || strings.Contains(lower, "yt shorts"):
		return "youtube_shorts"
	case strings.Contains(lower, "youtube"):
		return "youtube"
	case strings.Contains(lower, "linkedin"):
		return "linkedin"
	case strings.Contains(lower, "short") || strings.Contains(lower, "cinematic"):
		return "youtube_shorts"
	default:
		return "general"
	}
}

// extractJSON tries to find a JSON object in the response string.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func wordCount(s string) int {
	fields := strings.Fields(s)
	return len(fields)
}

func writeScriptStub(planID string, step *CreativeStep, goal string, opts CreativeGenerateScriptOptions, mode, warnMsg string, outputsDir string, stdout io.Writer) error {
	stubText := fmt.Sprintf("Script concept for: %s\n\n[This is a stub. %s]", goal, warnMsg)
	if len(stubText) == 0 {
		stubText = goal
	}
	maxWords := opts.MaxWords
	if maxWords <= 0 {
		maxWords = 120
	}
	output := CreativeScriptOutput{
		SchemaVersion:  "creative_script.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		StepID:         step.ID,
		Goal:           goal,
		Mode:           mode,
		Text:           stubText,
		Notes:          []string{"stub generated due to fallback"},
		Request:        ScriptRequest{MaxWords: maxWords, Tone: opts.Tone},
		Warnings:       []string{warnMsg},
	}
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return fmt.Errorf("create outputs dir: %w", err)
	}
	scriptPath := filepath.Join(outputsDir, "script_draft.json")
	if err := writeJSONFile(scriptPath, output); err != nil {
		return fmt.Errorf("write script_draft.json: %w", err)
	}
	txtPath := filepath.Join(outputsDir, "script_draft.txt")
	if err := os.WriteFile(txtPath, []byte(stubText), 0o644); err != nil {
		return fmt.Errorf("write script_draft.txt: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "script_draft", "outputs/script_draft.json", step.ID)
	fmt.Fprintf(stdout, "  fallback-stub: script written (mode=stub)\n")
	fmt.Fprintf(stdout, "  warning:  %s\n", warnMsg)
	return nil
}

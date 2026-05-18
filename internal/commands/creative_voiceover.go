package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
)

// ---- schema ----

type VoiceoverTextSource struct {
	SourceType     string `json:"source_type"`     // script_draft|script_text|goal
	SourceArtifact string `json:"source_artifact"` // relative path or ""
}

type VoiceoverTextRequest struct {
	MaxWords int    `json:"max_words,omitempty"`
	Tone     string `json:"tone,omitempty"`
	Source   string `json:"source,omitempty"` // auto|script|goal
}

type VoiceoverTextOutput struct {
	SchemaVersion  string               `json:"schema_version"`
	CreatedAt      time.Time            `json:"created_at"`
	CreativePlanID string               `json:"creative_plan_id"`
	Mode           string               `json:"mode"` // local_text_extract
	Source         VoiceoverTextSource  `json:"source"`
	Text           string               `json:"text"`
	WordCount      int                  `json:"word_count"`
	Request        VoiceoverTextRequest `json:"request,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// ---- options ----

type VoiceoverTextOptions struct {
	Overwrite bool
	JSON      bool
	MaxWords  int
	Tone      string
	Source    string // auto|script|goal
}

type VoiceoverStatusOptions struct {
	JSON bool
}

type ReviewVoiceoverOptions struct {
	JSON          bool
	WriteArtifact bool
}

type ValidateVoiceoverOptions struct {
	JSON         bool
	RequireAudio bool
}

// ---- audio file discovery (shared with creative_assemble.go) ----

var voiceoverAudioNames = []string{"voiceover.wav", "voiceover.mp3", "voiceover.m4a", "voiceover.aac"}

func discoverVoiceoverAudio(outputsDir string) string {
	for _, name := range voiceoverAudioNames {
		p := filepath.Join(outputsDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ---- Part A: creative-voiceover-text ----

func VoiceoverTextCommand(planID string, stdout io.Writer, opts VoiceoverTextOptions) error {
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

	outJSON := filepath.Join(outputsDir, "voiceover_text.json")
	outTXT := filepath.Join(outputsDir, "voiceover_text.txt")
	if _, err := os.Stat(outJSON); err == nil && !opts.Overwrite {
		return fmt.Errorf("voiceover_text.json already exists; use --overwrite to replace")
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
	writeEvent("VOICEOVER_TEXT_STARTED", map[string]any{"plan_id": planID})

	maxWords := opts.MaxWords
	if maxWords <= 0 {
		maxWords = 120
	}

	// Resolve source
	sourceMode := opts.Source
	if sourceMode == "" {
		sourceMode = "auto"
	}

	text, sourceType, sourceArtifact, warnings := resolveVoiceoverText(planDir, plan.Goal, sourceMode, maxWords)

	wordCount := len(strings.Fields(text))

	output := VoiceoverTextOutput{
		SchemaVersion:  "voiceover_text.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Mode:           "local_text_extract",
		Source: VoiceoverTextSource{
			SourceType:     sourceType,
			SourceArtifact: sourceArtifact,
		},
		Text:      text,
		WordCount: wordCount,
		Request:   VoiceoverTextRequest{MaxWords: maxWords, Tone: opts.Tone, Source: opts.Source},
		Warnings:  warnings,
	}

	if err := writeJSONFile(outJSON, output); err != nil {
		writeEvent("VOICEOVER_TEXT_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		return fmt.Errorf("write voiceover_text.json: %w", err)
	}
	if err := os.WriteFile(outTXT, []byte(text), 0o644); err != nil {
		writeEvent("VOICEOVER_TEXT_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
		return fmt.Errorf("write voiceover_text.txt: %w", err)
	}
	_ = updateCreativeOutputsIndex(planID, "voiceover_text", "outputs/voiceover_text.json", "")
	_ = updateCreativeOutputsIndex(planID, "voiceover_text_file", "outputs/voiceover_text.txt", "")

	writeEvent("VOICEOVER_TEXT_COMPLETED", map[string]any{
		"plan_id":    planID,
		"source":     sourceType,
		"word_count": wordCount,
	})

	fmt.Fprintf(stdout, "creative-voiceover-text: %s\n", planID)
	fmt.Fprintf(stdout, "  source:     %s", sourceType)
	if sourceArtifact != "" {
		fmt.Fprintf(stdout, " (%s)", sourceArtifact)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  words:      %d / %d max\n", wordCount, maxWords)
	fmt.Fprintf(stdout, "  written:    outputs/voiceover_text.json + outputs/voiceover_text.txt\n")
	for _, w := range warnings {
		fmt.Fprintf(stdout, "  warning:    %s\n", w)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}
	return nil
}

// resolveVoiceoverText picks the best text source and applies max-word truncation.
func resolveVoiceoverText(planDir, goal, sourceMode string, maxWords int) (text, sourceType, sourceArtifact string, warnings []string) {
	outputsDir := filepath.Join(planDir, "outputs")

	tryScript := sourceMode == "auto" || sourceMode == "script"

	if tryScript {
		// 1. Try script_draft.json
		sdJSON := filepath.Join(outputsDir, "script_draft.json")
		if data, err := os.ReadFile(sdJSON); err == nil {
			var sd CreativeScriptOutput
			if json.Unmarshal(data, &sd) == nil && strings.TrimSpace(sd.Text) != "" {
				text = strings.TrimSpace(sd.Text)
				sourceType = "script_draft"
				sourceArtifact = "outputs/script_draft.json"
				text, warnings = truncateWords(text, maxWords)
				return
			}
		}
		// 2. Try script_draft.txt
		sdTXT := filepath.Join(outputsDir, "script_draft.txt")
		if data, err := os.ReadFile(sdTXT); err == nil {
			t := strings.TrimSpace(string(data))
			if t != "" {
				text = t
				sourceType = "script_text"
				sourceArtifact = "outputs/script_draft.txt"
				text, warnings = truncateWords(text, maxWords)
				return
			}
		}
	}

	// 3. Goal fallback
	text = fmt.Sprintf("Voiceover for: %s", goal)
	sourceType = "goal"
	sourceArtifact = ""
	warnings = []string{"No script draft found; voiceover text generated from goal only."}
	text, extraWarns := truncateWords(text, maxWords)
	warnings = append(warnings, extraWarns...)
	return
}

// truncateWords truncates text to at most maxWords words, returning any truncation warning.
func truncateWords(text string, maxWords int) (string, []string) {
	if maxWords <= 0 {
		return text, nil
	}
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text, nil
	}
	truncated := strings.Join(words[:maxWords], " ")
	warn := fmt.Sprintf("Voiceover text truncated from %d to %d words.", len(words), maxWords)
	return truncated, []string{warn}
}

// ---- Part B: voiceover-status ----

type VoiceoverReadiness string

const (
	VoiceoverReady              VoiceoverReadiness = "ready"
	VoiceoverMissingText        VoiceoverReadiness = "missing_text"
	VoiceoverMissingAudio       VoiceoverReadiness = "missing_audio"
	VoiceoverMissingTextAndAudio VoiceoverReadiness = "missing_text_and_audio"
)

type VoiceoverStatusResult struct {
	PlanID              string             `json:"plan_id"`
	TextExists          bool               `json:"text_exists"`
	WordCount           int                `json:"word_count,omitempty"`
	AudioPath           string             `json:"audio_path,omitempty"`
	AudioExists         bool               `json:"audio_exists"`
	Readiness           VoiceoverReadiness `json:"readiness"`
	GenerationStatus    string             `json:"generation_status,omitempty"`    // dry_run|completed|failed
	GenerationProvider  string             `json:"generation_provider,omitempty"`
	GenerationModel     string             `json:"generation_model,omitempty"`
	NextSteps           []string           `json:"next_steps,omitempty"`
	Warnings            []string           `json:"warnings,omitempty"`
}

func VoiceoverStatus(planID string, stdout io.Writer, opts VoiceoverStatusOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	if _, err := os.Stat(planDir); err != nil {
		return fmt.Errorf("creative plan %q not found", planID)
	}
	outputsDir := filepath.Join(planDir, "outputs")

	result := VoiceoverStatusResult{PlanID: planID}

	// Check voiceover_text.json
	textData, err := os.ReadFile(filepath.Join(outputsDir, "voiceover_text.json"))
	if err == nil {
		var vt VoiceoverTextOutput
		if json.Unmarshal(textData, &vt) == nil {
			result.TextExists = true
			result.WordCount = vt.WordCount
		}
	}

	// Discover audio
	audioPath := discoverVoiceoverAudio(outputsDir)
	if audioPath != "" {
		result.AudioExists = true
		rel, _ := filepath.Rel(planDir, audioPath)
		result.AudioPath = rel
	}

	// Check voiceover_generation.json
	if genData, genErr := os.ReadFile(filepath.Join(outputsDir, "voiceover_generation.json")); genErr == nil {
		var gen VoiceGenerationOutput
		if json.Unmarshal(genData, &gen) == nil {
			result.GenerationStatus = gen.Status
			result.GenerationProvider = gen.Provider
			result.GenerationModel = gen.Model
		}
	}

	// Determine readiness
	switch {
	case result.TextExists && result.AudioExists:
		result.Readiness = VoiceoverReady
	case result.TextExists && !result.AudioExists:
		result.Readiness = VoiceoverMissingAudio
		result.NextSteps = []string{
			fmt.Sprintf("Record or generate audio and save to .byom-video/creative_plans/%s/outputs/voiceover.wav", planID),
			fmt.Sprintf("Then: byom-video creative-assemble %s --mix-voiceover", planID),
		}
	case !result.TextExists && result.AudioExists:
		result.Readiness = VoiceoverMissingText
		result.NextSteps = []string{
			fmt.Sprintf("byom-video creative-voiceover-text %s", planID),
		}
	default:
		result.Readiness = VoiceoverMissingTextAndAudio
		result.NextSteps = []string{
			fmt.Sprintf("byom-video creative-generate-script %s --fallback-stub", planID),
			fmt.Sprintf("byom-video creative-voiceover-text %s", planID),
			fmt.Sprintf("Record or generate audio and save to .byom-video/creative_plans/%s/outputs/voiceover.wav", planID),
		}
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(stdout, "voiceover-status: %s\n", planID)
	fmt.Fprintf(stdout, "  readiness:  %s\n", result.Readiness)
	fmt.Fprintf(stdout, "  text:       %v", result.TextExists)
	if result.TextExists {
		fmt.Fprintf(stdout, " (%d words)", result.WordCount)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  audio:      %v", result.AudioExists)
	if result.AudioExists {
		fmt.Fprintf(stdout, " (%s)", result.AudioPath)
	}
	fmt.Fprintln(stdout)
	if result.GenerationStatus != "" {
		fmt.Fprintf(stdout, "  generated:  %s (%s/%s)\n", result.GenerationStatus, result.GenerationProvider, result.GenerationModel)
	}
	if len(result.NextSteps) > 0 {
		fmt.Fprintf(stdout, "next steps:\n")
		for _, s := range result.NextSteps {
			fmt.Fprintf(stdout, "  %s\n", s)
		}
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	return nil
}

// ---- Part B: review-voiceover ----

func ReviewVoiceover(planID string, stdout io.Writer, opts ReviewVoiceoverOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")

	if _, err := os.Stat(planDir); err != nil {
		return fmt.Errorf("creative plan %q not found", planID)
	}

	// Read voiceover text if present
	var vt *VoiceoverTextOutput
	if data, err := os.ReadFile(filepath.Join(outputsDir, "voiceover_text.json")); err == nil {
		var out VoiceoverTextOutput
		if json.Unmarshal(data, &out) == nil {
			vt = &out
		}
	}

	if opts.JSON {
		type jsonOut struct {
			PlanID     string               `json:"plan_id"`
			TextOutput *VoiceoverTextOutput `json:"voiceover_text,omitempty"`
			AudioPath  string               `json:"audio_path,omitempty"`
			AudioFound bool                 `json:"audio_found"`
		}
		audioPath := discoverVoiceoverAudio(outputsDir)
		rel := ""
		found := false
		if audioPath != "" {
			found = true
			rel, _ = filepath.Rel(planDir, audioPath)
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonOut{PlanID: planID, TextOutput: vt, AudioPath: rel, AudioFound: found})
	}

	text := renderVoiceoverReviewMD(planID, planDir, outputsDir, vt)
	fmt.Fprint(stdout, text)

	if opts.WriteArtifact {
		if err := os.MkdirAll(outputsDir, 0o755); err != nil {
			return fmt.Errorf("create outputs dir: %w", err)
		}
		reviewPath := filepath.Join(outputsDir, "voiceover_review.md")
		if err := os.WriteFile(reviewPath, []byte(text), 0o644); err != nil {
			return fmt.Errorf("write voiceover_review.md: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "voiceover_review", "outputs/voiceover_review.md", "")
		fmt.Fprintf(stdout, "\nWritten: %s\n", reviewPath)
	}
	return nil
}

func renderVoiceoverReviewMD(planID, planDir, outputsDir string, vt *VoiceoverTextOutput) string {
	var b strings.Builder
	b.WriteString("# Voiceover Review\n\n")
	fmt.Fprintf(&b, "- **Plan ID:** %s\n", planID)

	if vt != nil {
		fmt.Fprintf(&b, "- **Source:** %s", vt.Source.SourceType)
		if vt.Source.SourceArtifact != "" {
			fmt.Fprintf(&b, " (`%s`)", vt.Source.SourceArtifact)
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "- **Word Count:** %d\n", vt.WordCount)
		if vt.Request.MaxWords > 0 {
			fmt.Fprintf(&b, "- **Max Words:** %d\n", vt.Request.MaxWords)
		}
		if vt.Request.Tone != "" {
			fmt.Fprintf(&b, "- **Tone:** %s\n", vt.Request.Tone)
		}
	} else {
		fmt.Fprintf(&b, "- **Text:** not generated yet\n")
	}

	// Audio status
	audioPath := discoverVoiceoverAudio(outputsDir)
	if audioPath != "" {
		rel, _ := filepath.Rel(planDir, audioPath)
		fmt.Fprintf(&b, "- **Audio:** found (`%s`)\n", rel)
	} else {
		fmt.Fprintf(&b, "- **Audio:** not found\n")
	}

	// Text preview
	if vt != nil && vt.Text != "" {
		b.WriteString("\n## Voiceover Text\n\n")
		b.WriteString("```\n")
		preview := vt.Text
		if len(preview) > 800 {
			preview = preview[:800] + "\n... (truncated)"
		}
		b.WriteString(preview)
		b.WriteString("\n```\n")
	}

	// Warnings
	if vt != nil && len(vt.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range vt.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}

	// Next steps
	b.WriteString("\n## Next Steps\n\n")
	if vt == nil {
		fmt.Fprintf(&b, "```\nbyom-video creative-voiceover-text %s\n```\n", planID)
	} else if audioPath == "" {
		outputsDirRel := fmt.Sprintf(".byom-video/creative_plans/%s/outputs", planID)
		fmt.Fprintf(&b, "Record or generate audio and save it here:\n\n")
		fmt.Fprintf(&b, "```\n%s/voiceover.wav\n```\n\n", outputsDirRel)
		fmt.Fprintf(&b, "Then assemble with voiceover:\n\n")
		fmt.Fprintf(&b, "```\nbyom-video creative-assemble %s --mix-voiceover\n```\n", planID)
	} else {
		fmt.Fprintf(&b, "```\nbyom-video creative-assemble %s --mix-voiceover\n```\n", planID)
	}

	return b.String()
}

// ---- Part C: validate-voiceover ----

func ValidateVoiceover(planID string, stdout io.Writer, opts ValidateVoiceoverOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")

	if _, err := os.Stat(planDir); err != nil {
		return fmt.Errorf("creative plan %q not found", planID)
	}

	type result struct {
		PlanID   string   `json:"plan_id"`
		Status   string   `json:"status"` // ok|failed
		Errors   []string `json:"errors,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
	}

	res := result{PlanID: planID, Status: "ok"}

	// Validate voiceover_text.json
	textPath := filepath.Join(outputsDir, "voiceover_text.json")
	textData, err := os.ReadFile(textPath)
	if err != nil {
		res.Warnings = append(res.Warnings, "voiceover_text.json not found; run creative-voiceover-text first")
	} else {
		var vt VoiceoverTextOutput
		if json.Unmarshal(textData, &vt) != nil {
			res.Errors = append(res.Errors, "voiceover_text.json is malformed JSON")
		} else {
			if vt.SchemaVersion != "voiceover_text.v1" {
				res.Errors = append(res.Errors, fmt.Sprintf("voiceover_text.json: schema_version=%q, want voiceover_text.v1", vt.SchemaVersion))
			}
			if strings.TrimSpace(vt.Text) == "" {
				res.Errors = append(res.Errors, "voiceover_text.json: text field is empty")
			}
			if vt.WordCount < 0 {
				res.Errors = append(res.Errors, fmt.Sprintf("voiceover_text.json: word_count=%d is negative", vt.WordCount))
			}
			if vt.Source.SourceType == "" {
				res.Errors = append(res.Errors, "voiceover_text.json: source.source_type is missing")
			}
		}

		// Also check voiceover_text.txt
		if _, err := os.Stat(filepath.Join(outputsDir, "voiceover_text.txt")); err != nil {
			res.Warnings = append(res.Warnings, "voiceover_text.txt not found (optional companion file)")
		}
	}

	// Validate voiceover_generation.json if present
	genPath := filepath.Join(outputsDir, "voiceover_generation.json")
	if genData, genErr := os.ReadFile(genPath); genErr == nil {
		var gen VoiceGenerationOutput
		if json.Unmarshal(genData, &gen) != nil {
			res.Errors = append(res.Errors, "voiceover_generation.json is malformed JSON")
		} else {
			if gen.SchemaVersion != "voiceover_generation.v1" {
				res.Errors = append(res.Errors, fmt.Sprintf("voiceover_generation.json: schema_version=%q, want voiceover_generation.v1", gen.SchemaVersion))
			}
			if gen.Status == "completed" {
				// If generation completed, audio file must exist and be non-empty
				if gen.Output == nil || gen.Output.AudioFile == "" {
					res.Errors = append(res.Errors, "voiceover_generation.json: status=completed but output.audio_file is missing")
				} else {
					audioAbsPath := filepath.Join(planDir, gen.Output.AudioFile)
					if info, statErr := os.Stat(audioAbsPath); statErr != nil {
						res.Errors = append(res.Errors, fmt.Sprintf("voiceover_generation.json: status=completed but audio file not found: %s", gen.Output.AudioFile))
					} else if info.Size() == 0 {
						res.Errors = append(res.Errors, fmt.Sprintf("voiceover_generation.json: audio file is empty: %s", gen.Output.AudioFile))
					}
				}
			}
		}
	}

	// Audio file
	audioPath := discoverVoiceoverAudio(outputsDir)
	if audioPath != "" {
		ext := strings.ToLower(filepath.Ext(audioPath))
		supported := map[string]bool{".wav": true, ".mp3": true, ".m4a": true, ".aac": true}
		if !supported[ext] {
			res.Errors = append(res.Errors, fmt.Sprintf("audio file %q has unsupported extension %q", audioPath, ext))
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("audio file found: %s", filepath.Base(audioPath)))
		}
	} else if opts.RequireAudio {
		outputsDirShort := fmt.Sprintf(".byom-video/creative_plans/%s/outputs/", planID)
		res.Errors = append(res.Errors, fmt.Sprintf(
			"--require-audio: no voiceover audio found; place audio at %s{voiceover.wav|mp3|m4a|aac}", outputsDirShort))
	} else {
		res.Warnings = append(res.Warnings, "no voiceover audio file found (not required unless --require-audio)")
	}

	if len(res.Errors) > 0 {
		res.Status = "failed"
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		if res.Status == "failed" {
			return fmt.Errorf("validate-voiceover: %d error(s)", len(res.Errors))
		}
		return nil
	}

	fmt.Fprintf(stdout, "validate-voiceover: %s\n", planID)
	fmt.Fprintf(stdout, "  status:  %s\n", res.Status)
	for _, e := range res.Errors {
		fmt.Fprintf(stdout, "  error:   %s\n", e)
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	if res.Status == "failed" {
		return fmt.Errorf("validate-voiceover: %d error(s)", len(res.Errors))
	}
	return nil
}

// ---- shared helper: readVoiceoverTextOutput ----

func readVoiceoverTextOutput(planID string) (*VoiceoverTextOutput, error) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_text.json"))
	if err != nil {
		return nil, err
	}
	var out VoiceoverTextOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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

// ---- output schema types ----

type CreativeOutputsIndex struct {
	SchemaVersion  string                   `json:"schema_version"`
	CreatedAt      time.Time                `json:"created_at"`
	CreativePlanID string                   `json:"creative_plan_id"`
	Mode           string                   `json:"mode"`
	Artifacts      []CreativeOutputArtifact `json:"artifacts"`
	Warnings       []string                 `json:"warnings,omitempty"`
}

type CreativeOutputArtifact struct {
	Type   string `json:"type"`
	Path   string `json:"path"`
	StepID string `json:"step_id"`
	Status string `json:"status"`
}

type CreativeScriptOutput struct {
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	CreativePlanID string    `json:"creative_plan_id"`
	StepID         string    `json:"step_id"`
	Goal           string    `json:"goal"`
	Mode           string    `json:"mode"`
	Text           string    `json:"text"`
	Notes          []string  `json:"notes"`
}

type VoiceoverPlanOutput struct {
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	CreativePlanID string    `json:"creative_plan_id"`
	StepID         string    `json:"step_id"`
	Mode           string    `json:"mode"`
	ScriptSource   string    `json:"script_source"`
	VoiceBackend   string    `json:"voice_backend"`
	ExpectedOutput string    `json:"expected_output"`
	Notes          []string  `json:"notes"`
}

type VisualAssetPromptsOutput struct {
	SchemaVersion  string             `json:"schema_version"`
	CreatedAt      time.Time          `json:"created_at"`
	CreativePlanID string             `json:"creative_plan_id"`
	Mode           string             `json:"mode"`
	Prompts        []VisualPromptItem `json:"prompts"`
}

type VisualPromptItem struct {
	ID          string `json:"id"`
	StepID      string `json:"step_id"`
	Kind        string `json:"kind"`
	Prompt      string `json:"prompt"`
	IntendedUse string `json:"intended_use"`
}

type CaptionPlanOutput struct {
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	CreativePlanID string    `json:"creative_plan_id"`
	Mode           string    `json:"mode"`
	Style          string    `json:"style"`
	Notes          []string  `json:"notes"`
}

type AudioAssetPlanOutput struct {
	SchemaVersion   string    `json:"schema_version"`
	CreatedAt       time.Time `json:"created_at"`
	CreativePlanID  string    `json:"creative_plan_id"`
	Mode            string    `json:"mode"`
	AssetType       string    `json:"asset_type"`
	ExpectedOutputs []string  `json:"expected_outputs"`
	Notes           []string  `json:"notes"`
}

type VisualTransformPlanOutput struct {
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	CreativePlanID string    `json:"creative_plan_id"`
	Mode           string    `json:"mode"`
	Operations     []string  `json:"operations"`
	Notes          []string  `json:"notes"`
}

type TranslationPlanOutput struct {
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	CreativePlanID string    `json:"creative_plan_id"`
	Mode           string    `json:"mode"`
	TargetLanguage string    `json:"target_language"`
	Notes          []string  `json:"notes"`
}

type CompositionPlanOutput struct {
	SchemaVersion  string                `json:"schema_version"`
	CreatedAt      time.Time             `json:"created_at"`
	CreativePlanID string                `json:"creative_plan_id"`
	Mode           string                `json:"mode"`
	Inputs         CompositionPlanInputs `json:"inputs"`
	PlannedOutput  string                `json:"planned_output"`
	Notes          []string              `json:"notes"`
}

type CompositionPlanInputs struct {
	Video     string `json:"video"`
	Script    string `json:"script"`
	Voiceover string `json:"voiceover"`
}

// ---- options ----

type CreativeExecuteStubOptions struct {
	Yes       bool
	Overwrite bool
	JSON      bool
	StepType  string
	DryRun    bool
}

type ReviewCreativeOutputsOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- creative-execute-stub ----

func CreativeExecuteStub(planID string, stdout io.Writer, opts CreativeExecuteStubOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")

	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}
	if err := validateCreativePlanMap(m); err != nil {
		return fmt.Errorf("creative plan validation failed: %w", err)
	}

	approvalStatus, _ := m["approval_status"].(string)
	if approvalStatus != "approved" && !opts.Yes {
		return fmt.Errorf("creative plan %s is not approved; run approve-creative-plan first or use --yes", planID)
	}

	goal, _ := m["goal"].(string)
	inputPath, _ := m["input_path"].(string)
	stepsRaw, _ := m["steps"].([]any)
	outputsDir := filepath.Join(planDir, "outputs")

	if opts.DryRun {
		fmt.Fprintf(stdout, "Dry run: creative-execute-stub %s\n", planID)
		fmt.Fprintf(stdout, "  outputs dir:  %s\n", outputsDir)
		for _, s := range stepsRaw {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			stepType, _ := sm["type"].(string)
			stepID, _ := sm["id"].(string)
			if opts.StepType != "" && stepType != opts.StepType {
				continue
			}
			name := stubArtifactName(stepType)
			if name == "" {
				name = "(skipped — unknown type)"
			}
			fmt.Fprintf(stdout, "  would write:  %s -> %s\n", stepID, name)
		}
		fmt.Fprintln(stdout, "  no files written (--dry-run)")
		return nil
	}

	if _, err := os.Stat(outputsDir); err == nil && !opts.Overwrite {
		return fmt.Errorf("outputs directory already exists; use --overwrite")
	}
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return err
	}

	if opts.Yes && approvalStatus != "approved" {
		now := time.Now().UTC()
		m["approval_status"] = "approved"
		m["approved_at"] = now.Format(time.RFC3339)
		m["approval_mode"] = "yes_flag"
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_STUB_EXECUTION_STARTED", map[string]any{"plan_id": planID})
	}

	var artifacts []CreativeOutputArtifact
	var warnings []string
	updatedSteps := make([]any, 0, len(stepsRaw))

	for _, s := range stepsRaw {
		sm, ok := s.(map[string]any)
		if !ok {
			updatedSteps = append(updatedSteps, s)
			continue
		}
		stepType, _ := sm["type"].(string)
		stepID, _ := sm["id"].(string)

		if opts.StepType != "" && stepType != opts.StepType {
			sm["status"] = "skipped"
			updatedSteps = append(updatedSteps, sm)
			if log != nil {
				_ = log.Write("CREATIVE_STUB_STEP_SKIPPED", map[string]any{"step_id": stepID, "reason": "step_type_filter"})
			}
			continue
		}

		written, stepWarnings, writeErr := writeStubArtifact(stepType, stepID, goal, inputPath, planID, outputsDir)
		warnings = append(warnings, stepWarnings...)

		switch {
		case writeErr != nil:
			sm["status"] = "failed"
			warnings = append(warnings, fmt.Sprintf("step %s failed: %v", stepID, writeErr))
			if log != nil {
				_ = log.Write("CREATIVE_STUB_STEP_SKIPPED", map[string]any{"step_id": stepID, "reason": writeErr.Error()})
			}
		case written == "":
			sm["status"] = "skipped"
			warnings = append(warnings, fmt.Sprintf("step %s (%s): unknown step type, skipped", stepID, stepType))
			if log != nil {
				_ = log.Write("CREATIVE_STUB_STEP_SKIPPED", map[string]any{"step_id": stepID, "reason": "unknown_step_type"})
			}
		default:
			sm["status"] = "stub_completed"
			relPath := filepath.Join("outputs", filepath.Base(written))
			artifacts = append(artifacts, CreativeOutputArtifact{
				Type:   stubArtifactType(stepType),
				Path:   relPath,
				StepID: stepID,
				Status: "created",
			})
			if log != nil {
				_ = log.Write("CREATIVE_STUB_STEP_COMPLETED", map[string]any{"step_id": stepID, "artifact": relPath})
			}
		}
		updatedSteps = append(updatedSteps, sm)
	}

	if artifacts == nil {
		artifacts = []CreativeOutputArtifact{}
	}

	index := CreativeOutputsIndex{
		SchemaVersion:  "creative_outputs.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Mode:           "stub",
		Artifacts:      artifacts,
		Warnings:       dedupeStrings(warnings),
	}
	indexPath := filepath.Join(outputsDir, "creative_outputs.json")
	if err := writeJSONFile(indexPath, index); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_STUB_EXECUTION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	artifactPaths := make([]string, len(artifacts))
	for i, a := range artifacts {
		artifactPaths[i] = a.Path
	}
	m["steps"] = updatedSteps
	m["execution_status"] = "stub_completed"
	m["output_artifacts"] = artifactPaths
	if err := writeJSONFile(planPath, m); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_STUB_EXECUTION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	if log != nil {
		_ = log.Write("CREATIVE_STUB_EXECUTION_COMPLETED", map[string]any{
			"plan_id":   planID,
			"artifacts": len(artifacts),
			"warnings":  len(warnings),
		})
		_ = log.Close()
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(map[string]any{
			"plan_id":          planID,
			"execution_status": "stub_completed",
			"artifacts":        len(artifacts),
			"outputs_dir":      outputsDir,
			"warnings":         dedupeStrings(warnings),
		}, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Creative stub execution completed: %s\n", planID)
	fmt.Fprintf(stdout, "  execution_status: stub_completed\n")
	fmt.Fprintf(stdout, "  artifacts:        %d\n", len(artifacts))
	fmt.Fprintf(stdout, "  outputs dir:      %s\n", outputsDir)
	for _, a := range artifacts {
		fmt.Fprintf(stdout, "  %s: %s (%s)\n", a.Type, a.Path, a.Status)
	}
	for _, w := range dedupeStrings(warnings) {
		fmt.Fprintf(stdout, "  warning:          %s\n", w)
	}
	return nil
}

// ---- review-creative-outputs ----

func ReviewCreativeOutputs(planID string, stdout io.Writer, opts ReviewCreativeOutputsOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	indexPath := filepath.Join(planDir, "outputs", "creative_outputs.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("creative outputs not found; run creative-execute-stub %s first", planID)
		}
		return err
	}
	var index CreativeOutputsIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("creative_outputs.json is malformed: %w", err)
	}

	artifactPath := ""
	if opts.WriteArtifact {
		artifactPath = filepath.Join(planDir, "creative_outputs_review.md")
		var b strings.Builder
		b.WriteString("# Creative Outputs Review\n\n")
		fmt.Fprintf(&b, "- Plan ID: `%s`\n", index.CreativePlanID)
		fmt.Fprintf(&b, "- Mode: `%s`\n", index.Mode)
		fmt.Fprintf(&b, "- Artifacts: %d\n\n", len(index.Artifacts))
		if len(index.Artifacts) > 0 {
			b.WriteString("## Artifacts\n\n")
			for _, a := range index.Artifacts {
				fmt.Fprintf(&b, "- `%s` `%s` (`%s`) — %s\n", a.Type, a.Path, a.StepID, a.Status)
			}
		}
		if len(index.Warnings) > 0 {
			b.WriteString("\n## Warnings\n\n")
			for _, w := range index.Warnings {
				fmt.Fprintf(&b, "- %s\n", w)
			}
		}
		if err := os.WriteFile(artifactPath, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	// check for timeline / render plan / assemble artifacts
	timelinePath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	renderPlanPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	assembleResultPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")
	_, hasTimeline := os.Stat(timelinePath)
	_, hasRenderPlan := os.Stat(renderPlanPath)
	_, hasAssembleResult := os.Stat(assembleResultPath)

	var assembleStatus, assembleMode, draftFile, captionsStatus, voiceoverStatus string
	if hasAssembleResult == nil {
		arData, _ := os.ReadFile(assembleResultPath)
		var ar CreativeAssembleResult
		if json.Unmarshal(arData, &ar) == nil {
			assembleStatus = ar.Status
			assembleMode = ar.Mode
			if ar.FinalOutputFile != "" {
				draftFile = ar.FinalOutputFile
			} else {
				draftFile = ar.OutputFile
			}
			if ar.Captions != nil && ar.Captions.Requested {
				captionsStatus = ar.Captions.Status
			}
			if ar.Voiceover != nil && ar.Voiceover.Requested {
				voiceoverStatus = ar.Voiceover.Status
			}
		}
	}

	if opts.JSON {
		out := map[string]any{
			"creative_plan_id":  index.CreativePlanID,
			"mode":              index.Mode,
			"artifacts":         index.Artifacts,
			"warnings":          index.Warnings,
			"has_timeline":      hasTimeline == nil,
			"has_render_plan":   hasRenderPlan == nil,
			"has_assemble":      hasAssembleResult == nil,
			"assemble_status":   assembleStatus,
			"captions_status":   captionsStatus,
			"voiceover_status":  voiceoverStatus,
			"draft_file":        draftFile,
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(enc))
		return nil
	}

	fmt.Fprintln(stdout, "Creative outputs review")
	fmt.Fprintf(stdout, "  plan id:      %s\n", index.CreativePlanID)
	fmt.Fprintf(stdout, "  mode:         %s\n", index.Mode)
	fmt.Fprintf(stdout, "  artifacts:    %d\n", len(index.Artifacts))
	for _, a := range index.Artifacts {
		fmt.Fprintf(stdout, "  %s: %s (%s)\n", a.Type, a.Path, a.Status)
	}
	if hasTimeline == nil {
		fmt.Fprintf(stdout, "  timeline:     %s\n", timelinePath)
	}
	if hasRenderPlan == nil {
		fmt.Fprintf(stdout, "  render plan:  %s\n", renderPlanPath)
	}
	if hasAssembleResult == nil {
		fmt.Fprintf(stdout, "  assemble:     %s (mode=%s)\n", assembleStatus, assembleMode)
		fmt.Fprintf(stdout, "  draft:        %s\n", draftFile)
		if captionsStatus != "" {
			fmt.Fprintf(stdout, "  captions:     %s\n", captionsStatus)
		}
		if voiceoverStatus != "" {
			fmt.Fprintf(stdout, "  voiceover:    %s\n", voiceoverStatus)
		}
	}
	for _, w := range index.Warnings {
		fmt.Fprintf(stdout, "  warning:      %s\n", w)
	}
	if artifactPath != "" {
		fmt.Fprintf(stdout, "  artifact:     %s\n", artifactPath)
	}
	return nil
}

// ---- writeStubArtifact ----

func writeStubArtifact(stepType, stepID, goal, inputPath, planID, outputsDir string) (string, []string, error) {
	now := time.Now().UTC()

	switch stepType {
	case "generate_script":
		out := CreativeScriptOutput{
			SchemaVersion:  "creative_script.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			StepID:         stepID,
			Goal:           goal,
			Mode:           "stub",
			Text:           fmt.Sprintf("Stub script draft for goal: %s", goal),
			Notes:          []string{},
		}
		jsonPath := filepath.Join(outputsDir, "script_draft.json")
		if err := writeJSONFile(jsonPath, out); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(filepath.Join(outputsDir, "script_draft.txt"), []byte(out.Text+"\n"), 0o644); err != nil {
			return "", nil, err
		}
		return jsonPath, nil, nil

	case "generate_voiceover":
		out := VoiceoverPlanOutput{
			SchemaVersion:  "voiceover_plan.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			StepID:         stepID,
			Mode:           "stub",
			ScriptSource:   "outputs/script_draft.txt",
			VoiceBackend:   "",
			ExpectedOutput: "voiceover.wav",
			Notes:          []string{"Stub only; no audio generated."},
		}
		p := filepath.Join(outputsDir, "voiceover_plan.json")
		return p, nil, writeJSONFile(p, out)

	case "generate_visual_asset":
		out := VisualAssetPromptsOutput{
			SchemaVersion:  "visual_asset_prompts.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			Mode:           "stub",
			Prompts: []VisualPromptItem{{
				ID:          "visual_prompt_0001",
				StepID:      stepID,
				Kind:        visualKindForGoal(goal),
				Prompt:      fmt.Sprintf("Stub visual prompt for goal: %s", goal),
				IntendedUse: "b-roll",
			}},
		}
		p := filepath.Join(outputsDir, "visual_asset_prompts.json")
		return p, nil, writeJSONFile(p, out)

	case "generate_captions_or_caption_variants":
		out := CaptionPlanOutput{
			SchemaVersion:  "caption_plan.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			Mode:           "stub",
			Style:          "goal-aware",
			Notes:          []string{},
		}
		p := filepath.Join(outputsDir, "caption_plan.json")
		return p, nil, writeJSONFile(p, out)

	case "generate_audio_asset":
		out := AudioAssetPlanOutput{
			SchemaVersion:   "audio_asset_plan.v1",
			CreatedAt:       now,
			CreativePlanID:  planID,
			Mode:            "stub",
			AssetType:       "music_or_sound_effect",
			ExpectedOutputs: []string{},
			Notes:           []string{"Stub only; no audio generated."},
		}
		p := filepath.Join(outputsDir, "audio_asset_plan.json")
		return p, nil, writeJSONFile(p, out)

	case "visual_transform":
		out := VisualTransformPlanOutput{
			SchemaVersion:  "visual_transform_plan.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			Mode:           "stub",
			Operations:     []string{},
			Notes:          []string{"Stub only; no visual transform performed."},
		}
		p := filepath.Join(outputsDir, "visual_transform_plan.json")
		return p, nil, writeJSONFile(p, out)

	case "translate_text":
		out := TranslationPlanOutput{
			SchemaVersion:  "translation_plan.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			Mode:           "stub",
			TargetLanguage: "unknown",
			Notes:          []string{},
		}
		p := filepath.Join(outputsDir, "translation_plan.json")
		return p, nil, writeJSONFile(p, out)

	case "render_draft":
		out := CompositionPlanOutput{
			SchemaVersion:  "composition_plan.v1",
			CreatedAt:      now,
			CreativePlanID: planID,
			Mode:           "stub",
			Inputs: CompositionPlanInputs{
				Video:     inputPath,
				Script:    "outputs/script_draft.txt",
				Voiceover: "outputs/voiceover.wav",
			},
			PlannedOutput: "outputs/draft.mp4",
			Notes:         []string{"Stub only; no media rendered."},
		}
		p := filepath.Join(outputsDir, "composition_plan.json")
		return p, nil, writeJSONFile(p, out)

	default:
		return "", nil, nil // unknown → caller marks skipped
	}
}

// ---- helpers ----

func stubArtifactName(stepType string) string {
	switch stepType {
	case "generate_script":
		return "script_draft.json"
	case "generate_voiceover":
		return "voiceover_plan.json"
	case "generate_visual_asset":
		return "visual_asset_prompts.json"
	case "generate_captions_or_caption_variants":
		return "caption_plan.json"
	case "generate_audio_asset":
		return "audio_asset_plan.json"
	case "visual_transform":
		return "visual_transform_plan.json"
	case "translate_text":
		return "translation_plan.json"
	case "render_draft":
		return "composition_plan.json"
	default:
		return ""
	}
}

func stubArtifactType(stepType string) string {
	switch stepType {
	case "generate_script":
		return "script"
	case "generate_voiceover":
		return "voiceover_plan"
	case "generate_visual_asset":
		return "visual_asset_prompts"
	case "generate_captions_or_caption_variants":
		return "caption_plan"
	case "generate_audio_asset":
		return "audio_asset_plan"
	case "visual_transform":
		return "visual_transform_plan"
	case "translate_text":
		return "translation_plan"
	case "render_draft":
		return "composition_plan"
	default:
		return "unknown"
	}
}

func expectedArtifactSchemaVersion(artifactType string) string {
	switch artifactType {
	case "script":
		return "creative_script.v1"
	case "voiceover_plan":
		return "voiceover_plan.v1"
	case "visual_asset_prompts":
		return "visual_asset_prompts.v1"
	case "caption_plan":
		return "caption_plan.v1"
	case "audio_asset_plan":
		return "audio_asset_plan.v1"
	case "visual_transform_plan":
		return "visual_transform_plan.v1"
	case "translation_plan":
		return "translation_plan.v1"
	case "composition_plan":
		return "composition_plan.v1"
	case "creative_timeline":
		return "creative_timeline.v1"
	case "creative_render_plan":
		return "creative_render_plan.v1"
	default:
		return ""
	}
}

func visualKindForGoal(goal string) string {
	g := strings.ToLower(goal)
	if strings.Contains(g, "video") || strings.Contains(g, "b-roll") || strings.Contains(g, "footage") {
		return "video_generation"
	}
	if strings.Contains(g, "image") || strings.Contains(g, "photo") || strings.Contains(g, "still") {
		return "image_generation"
	}
	return "unknown"
}

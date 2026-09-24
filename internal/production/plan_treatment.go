package production

import (
	"fmt"
	"strings"
	"time"
)

// PlanFromTreatment builds the deterministic production plan from a validated
// creative treatment.
//
// The treatment decides WHAT should be made; this function turns that into the
// same typed stage DAG the runtime already executes. It does not replace
// deterministic planning — it supplies its inputs. Capability binding, policy,
// execution, revision and validation all remain exactly where they were.
//
// Every stage it emits carries a treatment_decision_id where one applies, so
// the provenance chain runs from an ffmpeg argv back to a creative reason.
func PlanFromTreatment(
	productionID string,
	brief string,
	obs Observation,
	t CreativeTreatment,
	now time.Time,
) (ProductionPlan, error) {

	intent := IntentFromTreatment(t, obs)

	plan := ProductionPlan{
		SchemaVersion: PlanSchemaVersion,
		ProductionID:  productionID,
		PlanVersion:   1,
		CreatedAt:     now,
		Brief:         strings.TrimSpace(brief),
		Intent:        intent,
		Planner:       "creative_director." + t.Reasoning.EffectiveMode,
	}

	// Carry the director's honest self-assessment onto the plan, so a reader of
	// the plan alone can tell whether a model actually reasoned.
	if t.Reasoning.EffectiveMode == ReasoningDeterministic && t.Reasoning.RequestedMode == ReasoningLLM {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"creative reasoning fell back to rules: %s", t.Reasoning.FallbackReason))
	}

	usable, pending := t.UsableSegments()
	if len(usable) == 0 {
		return ProductionPlan{}, fmt.Errorf(
			"creative treatment produced no segments bound to available assets")
	}
	for _, s := range pending {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"segment %s requires generated material that is not available; it was dropped from the edit", s.ID))
	}
	// Unmet generation needs are creative gaps, not silent omissions.
	for _, need := range t.GeneratedAssetNeeds {
		severity := "optional"
		if need.Required {
			severity = "required"
		}
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"unmet %s need %s (%s): %s", severity, need.ID, need.Kind, need.Description))
	}

	byID := map[string]Asset{}
	for _, a := range obs.Assets {
		byID[a.ID] = a
	}

	n := 0
	nextID := func() string {
		n++
		return fmt.Sprintf("stage_%04d", n)
	}

	// ---- probe assets ----
	probe := Stage{
		ID:          nextID(),
		Type:        StageTypeProbeAssets,
		Description: fmt.Sprintf("Probe %d source asset(s) with ffprobe.", len(obs.Assets)),
		Outputs:     []string{"probe/"},
		Requires:    CapFFprobe,
		Requirements: Requirements{
			CPUCores: 0.5, MemoryMB: 256,
			Runtime:            "ffprobe>=6",
			ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6",
		},
		Status: StageStatusPlanned,
	}
	for _, a := range obs.Assets {
		probe.Inputs = append(probe.Inputs, a.Path)
	}
	plan.Stages = append(plan.Stages, probe)

	// ---- transcribe ----
	transcribeID := ""
	if intent.WantsCaptions {
		// Prefer the asset the director designated as narration; this is a real
		// creative decision changing which file gets transcribed.
		src, decisionRef, ok := narrationSource(t, obs)
		if ok {
			transcribeID = nextID()
			stage := Stage{
				ID:          transcribeID,
				Type:        StageTypeTranscribe,
				Description: fmt.Sprintf("Transcribe %s for captions.", src.ID),
				DependsOn:   []string{probe.ID},
				Inputs:      []string{src.Path},
				Outputs:     []string{"transcript.json"},
				Requires:    CapASRWhisper,
				Requirements: Requirements{
					CPUCores: 2, MemoryMB: 2048,
					Runtime:            "python>=3.10+faster-whisper",
					ContainerImageHint: "ghcr.io/openvfx/faster-whisper:tiny",
				},
				Params:       Params{SourcePath: src.Path, ModelSize: "tiny"},
				Status:       StageStatusPlanned,
				TreatmentRef: decisionRef,
			}
			plan.Stages = append(plan.Stages, stage)
		} else {
			plan.Warnings = append(plan.Warnings,
				"captions requested but no asset carries an audio stream; transcription omitted")
		}
	}

	// ---- select clips (driven by the treatment's segments) ----
	selectID := nextID()
	deps := []string{probe.ID}
	if transcribeID != "" {
		deps = append(deps, transcribeID)
	}

	var cuts []SegmentCut
	var decisionIDs []string
	for _, s := range usable {
		asset, ok := byID[s.AssetID]
		if !ok {
			continue
		}
		cuts = append(cuts, SegmentCut{
			ID:         s.ID,
			AssetID:    s.AssetID,
			SourcePath: asset.Path,
			SourceIn:   s.SourceIn,
			SourceOut:  s.SourceOut,
			Purpose:    s.Purpose,
			DecisionID: s.DecisionID,
		})
		if s.DecisionID != "" {
			decisionIDs = append(decisionIDs, s.DecisionID)
		}
	}

	selectStage := Stage{
		ID:   selectID,
		Type: StageTypeSelectClips,
		Description: fmt.Sprintf("Cut %d segment(s) chosen by the creative treatment.",
			len(cuts)),
		DependsOn: deps,
		Outputs:   []string{"edit_decisions.json"},
		Requires:  CapFFprobe,
		Requirements: Requirements{
			CPUCores: 0.25, MemoryMB: 128,
			Runtime: "openvfx-native",
		},
		Params: Params{
			TargetSeconds: intent.TargetDurationSeconds,
			MaxClips:      len(cuts),
			Segments:      cuts,
			// As in intent-only planning, extension is never authorised on a
			// first pass; a shortfall must surface as a failed assertion first.
			ExtendToTarget: false,
		},
		Status: StageStatusPlanned,
	}
	if len(decisionIDs) > 0 {
		selectStage.TreatmentDecisionID = decisionIDs[0]
		if d, ok := t.Decision(decisionIDs[0]); ok {
			selectStage.TreatmentRef = d.Summary
		}
	}
	if selectStage.TreatmentRef == "" && t.OpeningStrategy != "" {
		selectStage.TreatmentRef = t.OpeningStrategy
	}
	plan.Stages = append(plan.Stages, selectStage)

	// ---- assemble ----
	assembleID := nextID()
	plan.Stages = append(plan.Stages, Stage{
		ID:          assembleID,
		Type:        StageTypeAssembleVideo,
		Description: "Cut and concatenate the treatment's segments into one stream.",
		DependsOn:   []string{selectID},
		Outputs:     []string{"assembled.mp4"},
		Requires:    CapFFmpeg,
		Requirements: Requirements{
			CPUCores: 4, MemoryMB: 2048,
			Runtime:            "ffmpeg>=6",
			ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6",
		},
		Status: StageStatusPlanned,
	})
	last := assembleID

	// ---- format ----
	if intent.Width > 0 && intent.Height > 0 {
		formatID := nextID()
		ref := ""
		if t.Platform != "" && t.Platform != "unspecified" {
			ref = fmt.Sprintf("platform: %s", t.Platform)
		}
		plan.Stages = append(plan.Stages, Stage{
			ID:           formatID,
			Type:         StageTypeFormatOutput,
			Description:  fmt.Sprintf("Conform to %dx%d (%s).", intent.Width, intent.Height, intent.AspectRatio),
			DependsOn:    []string{last},
			Inputs:       []string{"assembled.mp4"},
			Outputs:      []string{"formatted.mp4"},
			Requires:     CapFilterScale,
			TreatmentRef: ref,
			Requirements: Requirements{
				CPUCores: 4, MemoryMB: 2048,
				Runtime:            "ffmpeg>=6",
				ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6",
			},
			Params: Params{
				Width: intent.Width, Height: intent.Height,
				Fit: "pad", Background: "black",
			},
			Status: StageStatusPlanned,
		})
		last = formatID
	}

	// ---- text overlay ----
	if intent.WantsCaptions && transcribeID != "" {
		overlayID := nextID()
		requires := CapFilterSubtitles
		if !intent.WantsBurnedCaptions {
			requires = CapSidecarSRT
		}
		ref := t.CaptionStrategy.Notes
		if ref == "" {
			ref = fmt.Sprintf("captions %s", captionShape(t.CaptionStrategy))
		}
		plan.Stages = append(plan.Stages, Stage{
			ID:           overlayID,
			Type:         StageTypeTextOverlay,
			Description:  "Deliver captions on the output.",
			DependsOn:    []string{last},
			Inputs:       []string{"transcript.json"},
			Outputs:      []string{"captioned.mp4"},
			Requires:     requires,
			TreatmentRef: ref,
			Requirements: Requirements{
				CPUCores: 4, MemoryMB: 2048,
				Runtime:            "ffmpeg>=6+libass",
				ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6-libass",
			},
			Status: StageStatusPlanned,
		})
	}

	if err := plan.Validate(); err != nil {
		return ProductionPlan{}, fmt.Errorf("treatment produced an invalid plan: %w", err)
	}
	return plan, nil
}

func captionShape(c CaptionStrategy) string {
	if c.BurnIn {
		return "burned in"
	}
	return "as a sidecar"
}

// narrationSource picks which asset to transcribe, preferring the one the
// director designated as primary narration over the first with audio.
func narrationSource(t CreativeTreatment, obs Observation) (Asset, string, bool) {
	byID := map[string]Asset{}
	for _, a := range obs.Assets {
		byID[a.ID] = a
	}
	for _, r := range t.AssetRoles {
		if r.Role != RolePrimaryNarration {
			continue
		}
		if a, ok := byID[r.AssetID]; ok && a.HasAudio {
			return a, fmt.Sprintf("role: primary_narration (%s confidence)", r.Confidence), true
		}
	}
	if a, ok := obs.FirstWithAudio(); ok {
		return a, "first asset carrying audio", true
	}
	return Asset{}, "", false
}

// IntentFromTreatment derives the assertable intent the validator checks from
// the treatment's own stated goals. Keeping one Intent means "what was asked
// for" has a single definition rather than being re-interpreted per phase.
func IntentFromTreatment(t CreativeTreatment, obs Observation) Intent {
	intent := Intent{
		DurationTolerance:     0.10,
		TargetDurationSeconds: t.TargetDurationSeconds,
		WantsCaptions:         t.CaptionStrategy.Required,
		WantsBurnedCaptions:   t.CaptionStrategy.Required && t.CaptionStrategy.BurnIn,
		WantsAudio:            t.AudioStrategy.UseSourceAudio && obs.Totals.WithAudio > 0,
	}

	switch strings.TrimSpace(t.AspectRatio) {
	case "9:16":
		intent.AspectRatio, intent.Width, intent.Height = "9:16", 1080, 1920
	case "1:1":
		intent.AspectRatio, intent.Width, intent.Height = "1:1", 1080, 1080
	case "16:9":
		intent.AspectRatio, intent.Width, intent.Height = "16:9", 1920, 1080
	case "4:5":
		intent.AspectRatio, intent.Width, intent.Height = "4:5", 1080, 1350
	}

	// A success criterion may restate the duration target; the treatment's own
	// criteria win, since that is what the director committed to.
	for _, c := range t.SuccessCriteria {
		if c.Assertion == "duration_within_tolerance" && c.Seconds > 0 {
			intent.TargetDurationSeconds = c.Seconds
		}
	}
	return intent
}

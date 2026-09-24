package production

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Plan turns a brief plus an observation into a typed stage DAG.
//
// This is a pure function: no I/O, no clock beyond the caller-supplied one, no
// capability lookups. Binding a stage to a backend is deliberately NOT done
// here — late binding at execution time is what lets the same plan run through
// a different backend, which is the whole point of the architecture.
func Plan(productionID string, brief string, obs Observation, now time.Time) (ProductionPlan, error) {
	intent := ParseIntent(brief, obs)

	plan := ProductionPlan{
		SchemaVersion: PlanSchemaVersion,
		ProductionID:  productionID,
		PlanVersion:   1,
		CreatedAt:     now,
		Brief:         strings.TrimSpace(brief),
		Intent:        intent,
		Planner:       "deterministic.v1",
	}

	videoAssets := obs.VideoAssets()
	if len(videoAssets) == 0 {
		return ProductionPlan{}, fmt.Errorf("no video assets to plan against")
	}

	n := 0
	nextID := func() string {
		n++
		return fmt.Sprintf("stage_%04d", n)
	}

	// ---- Stage: probe assets ----
	// Observation already ran, but the plan records it as a stage so the
	// execution record carries the actual ffprobe argv for every asset.
	probeStage := Stage{
		ID:          nextID(),
		Type:        StageTypeProbeAssets,
		Description: fmt.Sprintf("Probe %d source asset(s) with ffprobe.", len(obs.Assets)),
		Outputs:     []string{"probe/"},
		Requires:    CapFFprobe,
		Requirements: Requirements{
			CPUCores: 0.5, MemoryMB: 256, GPU: false,
			Runtime:            "ffprobe>=6",
			ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6",
		},
		Status: StageStatusPlanned,
	}
	for _, a := range obs.Assets {
		probeStage.Inputs = append(probeStage.Inputs, a.Path)
	}
	plan.Stages = append(plan.Stages, probeStage)

	// ---- Stage: transcribe (only if captions were asked for) ----
	transcribeID := ""
	if intent.WantsCaptions {
		src, ok := obs.FirstWithAudio()
		if ok {
			transcribeID = nextID()
			plan.Stages = append(plan.Stages, Stage{
				ID:          transcribeID,
				Type:        StageTypeTranscribe,
				Description: "Transcribe source audio to timed segments.",
				DependsOn:   []string{probeStage.ID},
				Inputs:      []string{src.Path},
				Outputs:     []string{"transcript.json"},
				Requires:    CapASRWhisper,
				Requirements: Requirements{
					CPUCores: 2, MemoryMB: 2048, GPU: false,
					Runtime:            "python>=3.10+faster-whisper",
					ContainerImageHint: "ghcr.io/openvfx/faster-whisper:tiny",
				},
				Params: Params{SourcePath: src.Path, ModelSize: "tiny"},
				Status: StageStatusPlanned,
			})
		} else {
			plan.Warnings = append(plan.Warnings,
				"captions requested but no asset carries an audio stream; transcription stage omitted")
		}
	}

	// ---- Stage: select clips ----
	selectID := nextID()
	selectDeps := []string{probeStage.ID}
	if transcribeID != "" {
		selectDeps = append(selectDeps, transcribeID)
	}
	plan.Stages = append(plan.Stages, Stage{
		ID:          selectID,
		Type:        StageTypeSelectClips,
		Description: fmt.Sprintf("Select source segments to reach the %s target.", durationLabel(intent)),
		DependsOn:   selectDeps,
		Outputs:     []string{"edit_decisions.json"},
		// Clip selection is deterministic arithmetic in Go. It needs ffprobe
		// facts, which the observation already carries.
		Requires: CapFFprobe,
		Requirements: Requirements{
			CPUCores: 0.25, MemoryMB: 128, GPU: false,
			Runtime: "openvfx-native",
		},
		Params: Params{
			TargetSeconds: intent.TargetDurationSeconds,
			MaxClips:      8,
			// Deliberately false on a first pass. If the source cannot cover the
			// target, that shortfall must surface as a failed assertion rather
			// than being papered over by looping footage nobody asked to loop.
			ExtendToTarget: false,
		},
		Status: StageStatusPlanned,
	})

	// ---- Stage: assemble video ----
	assembleID := nextID()
	plan.Stages = append(plan.Stages, Stage{
		ID:          assembleID,
		Type:        StageTypeAssembleVideo,
		Description: "Cut and concatenate the selected segments into one stream.",
		DependsOn:   []string{selectID},
		Outputs:     []string{"assembled.mp4"},
		Requires:    CapFFmpeg,
		Requirements: Requirements{
			CPUCores: 4, MemoryMB: 2048, GPU: false,
			Runtime:            "ffmpeg>=6",
			ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6",
		},
		Status: StageStatusPlanned,
	})
	last := assembleID

	// ---- Stage: format output ----
	if intent.Width > 0 && intent.Height > 0 {
		formatID := nextID()
		plan.Stages = append(plan.Stages, Stage{
			ID:          formatID,
			Type:        StageTypeFormatOutput,
			Description: fmt.Sprintf("Conform to %dx%d (%s).", intent.Width, intent.Height, intent.AspectRatio),
			DependsOn:   []string{last},
			Inputs:      []string{"assembled.mp4"},
			Outputs:     []string{"formatted.mp4"},
			Requires:    CapFilterScale,
			Requirements: Requirements{
				CPUCores: 4, MemoryMB: 2048, GPU: false,
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

	// ---- Stage: text overlay ----
	//
	// This stage asks for burn-in. If this ffmpeg build cannot burn text, the
	// reviser substitutes a different binding for the SAME stage rather than
	// deleting it — the intent to deliver captions survives the substitution.
	if intent.WantsCaptions {
		overlayID := nextID()
		requires := CapFilterSubtitles
		if !intent.WantsBurnedCaptions {
			requires = CapSidecarSRT
		}
		plan.Stages = append(plan.Stages, Stage{
			ID:          overlayID,
			Type:        StageTypeTextOverlay,
			Description: "Deliver captions on the output.",
			DependsOn:   []string{last},
			Inputs:      []string{"transcript.json"},
			Outputs:     []string{"captioned.mp4"},
			Requires:    requires,
			Requirements: Requirements{
				CPUCores: 4, MemoryMB: 2048, GPU: false,
				Runtime:            "ffmpeg>=6+libass",
				ContainerImageHint: "ghcr.io/openvfx/ffmpeg:6-libass",
			},
			Status: StageStatusPlanned,
		})
		last = overlayID
	}

	_ = last
	if err := plan.Validate(); err != nil {
		return ProductionPlan{}, fmt.Errorf("planner produced an invalid plan: %w", err)
	}
	return plan, nil
}

func durationLabel(i Intent) string {
	if i.TargetDurationSeconds <= 0 {
		return "full-length"
	}
	return fmt.Sprintf("%.0fs", i.TargetDurationSeconds)
}

// ---- intent parsing ----

var (
	reDurationSeconds = regexp.MustCompile(`(\d+)[\s-]*(?:second|sec|s)\b`)
	reDurationMinutes = regexp.MustCompile(`(\d+)[\s-]*(?:minute|min)\b`)
)

// ParseIntent reads the brief into structured, assertable intent. Everything
// the validator later checks must originate here, so that "what was asked for"
// is a persisted fact rather than a second interpretation of English.
func ParseIntent(brief string, obs Observation) Intent {
	lower := strings.ToLower(brief)
	intent := Intent{DurationTolerance: 0.10}

	if m := reDurationMinutes.FindStringSubmatch(lower); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			intent.TargetDurationSeconds = float64(v) * 60
		}
	}
	if m := reDurationSeconds.FindStringSubmatch(lower); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			intent.TargetDurationSeconds = float64(v)
		}
	}

	switch {
	case strings.Contains(lower, "vertical") || strings.Contains(lower, "9:16") ||
		strings.Contains(lower, "reel") || strings.Contains(lower, "tiktok") ||
		strings.Contains(lower, "short"):
		intent.AspectRatio = "9:16"
		intent.Width, intent.Height = 1080, 1920
	case strings.Contains(lower, "square") || strings.Contains(lower, "1:1"):
		intent.AspectRatio = "1:1"
		intent.Width, intent.Height = 1080, 1080
	case strings.Contains(lower, "widescreen") || strings.Contains(lower, "16:9") ||
		strings.Contains(lower, "landscape"):
		intent.AspectRatio = "16:9"
		intent.Width, intent.Height = 1920, 1080
	}

	intent.WantsCaptions = strings.Contains(lower, "caption") ||
		strings.Contains(lower, "subtitle")
	// "burned-in", "burn in", "burnt in", "hardcoded" all mean the same request.
	intent.WantsBurnedCaptions = intent.WantsCaptions && (strings.Contains(lower, "burn") ||
		strings.Contains(lower, "hardcode") ||
		strings.Contains(lower, "hard-code") ||
		strings.Contains(lower, "open caption"))
	// Default to burn-in when captions are requested without qualification:
	// it is the stronger reading, and a sidecar fallback is always reachable.
	if intent.WantsCaptions && !intent.WantsBurnedCaptions &&
		!strings.Contains(lower, "sidecar") && !strings.Contains(lower, "srt file") {
		intent.WantsBurnedCaptions = true
	}

	intent.WantsAudio = obs.Totals.WithAudio > 0 && !strings.Contains(lower, "silent") &&
		!strings.Contains(lower, "no audio") && !strings.Contains(lower, "mute")

	return intent
}

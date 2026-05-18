package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/media"
	"github.com/mirelahmd/OpenVFX/internal/runstore"
)

// ---- schema types ----

type CreativeAssembleResult struct {
	SchemaVersion   string                   `json:"schema_version"`
	CreatedAt       time.Time                `json:"created_at"`
	CreativePlanID  string                   `json:"creative_plan_id"`
	Mode            string                   `json:"mode"`
	Status          string                   `json:"status"`
	OutputFile      string                   `json:"output_file"`
	FinalOutputFile string                   `json:"final_output_file"`
	WorkDir         string                   `json:"work_dir"`
	Clips           []AssembledClip          `json:"clips"`
	Captions        *AssembleCaptionsResult  `json:"captions,omitempty"`
	Voiceover       *AssembleVoiceoverResult `json:"voiceover,omitempty"`
	Platform        *AssemblePlatformResult  `json:"platform,omitempty"`
	FinalProbe      *AssembleFinalProbe      `json:"final_probe,omitempty"`
	Stages          []AssembleStage          `json:"stages,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type AssemblePlatformResult struct {
	Requested  bool   `json:"requested"`
	Normalized string `json:"normalized"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Fit        string `json:"fit"`
	Background string `json:"background"`
	Status     string `json:"status"` // applied|skipped|failed
	Error      string `json:"error,omitempty"`
}

type AssembleFinalProbe struct {
	DurationSeconds  float64 `json:"duration_seconds"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	VideoStreamCount int     `json:"video_stream_count"`
	AudioStreamCount int     `json:"audio_stream_count"`
}

type AssembledClip struct {
	ID              string  `json:"id"`
	SourcePath      string  `json:"source_path"`
	Start           float64 `json:"start"`
	End             float64 `json:"end"`
	DurationSeconds float64 `json:"duration_seconds"`
	WorkFile        string  `json:"work_file"`
	Status          string  `json:"status"`
	Error           string  `json:"error,omitempty"`
}

type AssembleCaptionsResult struct {
	Requested   bool   `json:"requested"`
	SourcePath  string `json:"source_path,omitempty"`
	Status      string `json:"status"` // applied|missing|skipped|failed
	Position    string `json:"position,omitempty"`
	Margin      int    `json:"margin,omitempty"`
	Style       string `json:"style,omitempty"`
	FilterStyle string `json:"filter_style,omitempty"`
	Error       string `json:"error,omitempty"`
}

type AssembleVoiceoverResult struct {
	Requested  bool   `json:"requested"`
	SourcePath string `json:"source_path,omitempty"`
	Status     string `json:"status"` // applied|missing|skipped|failed
	Error      string `json:"error,omitempty"`
}

type AssembleStage struct {
	Name   string `json:"name"`   // assembled_video|voiceover_mix|caption_burn
	File   string `json:"file"`
	Status string `json:"status"` // completed|skipped|failed
}

// ---- options ----

type CreativeAssembleOptions struct {
	Overwrite              bool
	JSON                   bool
	Mode                   string // "reencode" (default) | "stream-copy"
	KeepWork               bool
	DryRun                 bool
	MaxClips               int
	CaptionsPath           string
	BurnCaptions           bool
	AllowMissingCaptions   bool
	VoiceoverPath          string
	MixVoiceover           bool
	AllowMissingVoiceover  bool
	RunID                  string // for caption/voiceover discovery
	Platform               string // platform preset (default: "original")
	Fit                    string // "crop" | "pad" (default: per-preset)
	Background             string // color for pad mode (default: "black")
	CaptionPosition        string // bottom|center|top|auto (default: auto)
	CaptionMargin          int    // vertical margin in pixels (0 = use platform default)
	CaptionStyle           string // default|bold|boxed
}

type ValidateCreativeAssembleOptions struct{ JSON bool }

type ReviewCreativeAssembleOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- ffmpeg runner abstraction ----

type ffmpegRunner interface {
	Run(args []string) ([]byte, error)
	// CheckFilter probes ffmpeg for a named filter. Returns true if available.
	CheckFilter(filter string) bool
}

type realFFmpegRunner struct{ path string }

func (r realFFmpegRunner) Run(args []string) ([]byte, error) {
	cmd := exec.Command(r.path, args...)
	return cmd.CombinedOutput()
}

func (r realFFmpegRunner) CheckFilter(filter string) bool {
	out, err := exec.Command(r.path, "-hide_banner", "-filters").CombinedOutput()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		// filter list lines look like " .. subtitles ..." or " TS subtitles ..."
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == filter {
			return true
		}
	}
	return false
}

// truncateStderr returns the last N lines of FFmpeg output, capped to maxBytes.
// This keeps error messages readable without flooding the log.
func truncateStderr(raw []byte, maxLines int, maxBytes int) string {
	s := strings.TrimSpace(string(raw))
	if len(s) == 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	out := strings.Join(lines, "\n")
	if len(out) > maxBytes {
		out = "..." + out[len(out)-maxBytes:]
	}
	return out
}

// ---- creative-assemble ----

func CreativeAssemble(planID string, stdout io.Writer, opts CreativeAssembleOptions) error {
	ffmpegPath := ""
	if !opts.DryRun {
		p, err := media.FindExecutable("ffmpeg")
		if err != nil {
			return fmt.Errorf("ffmpeg not found on PATH; run byom-video doctor to check dependencies")
		}
		ffmpegPath = p
	}
	return creativeAssembleWithRunner(planID, stdout, opts, realFFmpegRunner{path: ffmpegPath})
}

func creativeAssembleWithRunner(planID string, stdout io.Writer, opts CreativeAssembleOptions, runner ffmpegRunner) error {
	if opts.Mode == "" {
		opts.Mode = "reencode"
	}

	planDir := filepath.Join(creativePlansRoot, planID)

	// load timeline
	timelinePath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	tlData, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("creative_timeline.json not found — run creative-timeline first: %w", err)
	}
	var timeline CreativeTimelineArtifact
	if err := json.Unmarshal(tlData, &timeline); err != nil {
		return fmt.Errorf("creative_timeline.json is malformed: %w", err)
	}

	// load render plan
	renderPlanPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	if _, err := os.Stat(renderPlanPath); err != nil {
		return fmt.Errorf("creative_render_plan.json not found — run creative-render-plan first")
	}

	// find source clips from track_video_main
	var sourceClips []CreativeTimelineItem
	for _, track := range timeline.Tracks {
		if track.ID == "track_video_main" {
			for _, item := range track.Items {
				if item.Kind == "source_clip" && item.SourceEnd > item.SourceStart {
					sourceClips = append(sourceClips, item)
				}
			}
			break
		}
	}

	if len(sourceClips) == 0 {
		return fmt.Errorf("no source clips found in creative_timeline.json; use creative-timeline --run-id <run_id> to add clips from a pipeline run")
	}

	// cap clips
	if opts.MaxClips > 0 && len(sourceClips) > opts.MaxClips {
		sourceClips = sourceClips[:opts.MaxClips]
	}

	// resolve source path
	sourcePath := timeline.InputPath
	if sourcePath == "" {
		planRaw, _ := os.ReadFile(filepath.Join(planDir, "creative_plan.json"))
		var pm map[string]any
		if json.Unmarshal(planRaw, &pm) == nil {
			sourcePath, _ = pm["input_path"].(string)
		}
	}
	if sourcePath == "" {
		return fmt.Errorf("source media path not found in creative_timeline.json or creative_plan.json")
	}

	outputsDir := filepath.Join(planDir, "outputs")
	workDir := filepath.Join(outputsDir, "render_work")
	draftPath := filepath.Join(outputsDir, "draft.mp4")
	resultPath := filepath.Join(outputsDir, "creative_assemble_result.json")

	// Validate caption position/style early.
	if _, err := NormalizeCaptionPosition(opts.CaptionPosition); err != nil {
		return err
	}
	if _, err := NormalizeCaptionStyle(opts.CaptionStyle); err != nil {
		return err
	}

	// Resolve platform preset
	normPlatform, platErr := NormalizePlatform(opts.Platform)
	if platErr != nil {
		return platErr
	}
	platPreset := LookupPlatform(normPlatform)
	hasPlatform := normPlatform != "" && normPlatform != "original"
	platFit := opts.Fit
	if platFit == "" {
		platFit = DefaultFitForPlatform(normPlatform)
	}
	platBackground := opts.Background
	if platBackground == "" {
		platBackground = "black"
	}

	needsPostProcess := opts.BurnCaptions || opts.MixVoiceover || hasPlatform

	// determine assembled base file
	var assembledBase string
	if needsPostProcess {
		assembledBase = filepath.Join(outputsDir, "draft_assembled.mp4")
	} else {
		assembledBase = draftPath
	}

	// discover caption / voiceover paths early for dry-run
	captPath := opts.CaptionsPath
	if opts.BurnCaptions && captPath == "" {
		captPath = discoverCaptionsPath(planDir, timeline.RunID, opts.RunID)
	}
	voPath := opts.VoiceoverPath
	if opts.MixVoiceover && voPath == "" {
		voPath = discoverVoiceoverPath(outputsDir)
	}

	if !opts.Overwrite {
		if _, err := os.Stat(draftPath); err == nil {
			return fmt.Errorf("draft.mp4 already exists; use --overwrite to replace")
		}
		if _, err := os.Stat(workDir); err == nil {
			return fmt.Errorf("render_work/ already exists; use --overwrite to replace")
		}
	}

	// pre-validate captions/voiceover before doing any work
	if opts.BurnCaptions && captPath == "" && !opts.AllowMissingCaptions {
		return fmt.Errorf("--burn-captions requested but no caption file found; pass --captions <path> or --allow-missing-captions to skip")
	}
	if opts.MixVoiceover && voPath == "" && !opts.AllowMissingVoiceover {
		placement := fmt.Sprintf("Create or place audio at %s, or pass --voiceover <path>, or rerun with --allow-missing-voiceover.", outputsDir)
		// Check if voiceover_text exists to provide a more actionable message
		if _, vtErr := os.Stat(filepath.Join(outputsDir, "voiceover_text.json")); vtErr == nil {
			return fmt.Errorf("--mix-voiceover requested but no voiceover audio found; voiceover text is ready. %s", placement)
		}
		return fmt.Errorf("--mix-voiceover requested but no voiceover file found; %s", placement)
	}

	// subtitles filter preflight — check before any ffmpeg work is done (skip for dry-run)
	hasSubtitlesFilter := true
	if opts.BurnCaptions && captPath != "" && !opts.DryRun {
		hasSubtitlesFilter = runner.CheckFilter("subtitles")
		if !hasSubtitlesFilter {
			const hint = "ffmpeg does not support the subtitles filter required for caption burn. " +
				"Install ffmpeg with libass support (e.g. brew install ffmpeg --HEAD or compile with --enable-libass) " +
				"or rerun with --allow-missing-captions to skip caption burn."
			if !opts.AllowMissingCaptions {
				return fmt.Errorf(hint)
			}
			// allow-missing-captions: will skip the caption stage below
		}
	}

	// Resolve caption position/style for dry-run output.
	dryNormCaptPos, _ := NormalizeCaptionPosition(opts.CaptionPosition)
	dryResolvedCaptPos := ResolveCaptionPosition(dryNormCaptPos, normPlatform)
	dryCaptMargin := opts.CaptionMargin
	if dryCaptMargin <= 0 {
		dryCaptMargin = DefaultCaptionMargin(normPlatform)
	}
	dryNormCaptStyle, _ := NormalizeCaptionStyle(opts.CaptionStyle)
	dryCaptForceStyle := buildForceStyleArg(dryResolvedCaptPos, dryCaptMargin, dryNormCaptStyle)

	// dry-run
	if opts.DryRun {
		printDryRun(stdout, planID, opts, normPlatform, platPreset, platFit, platBackground,
			dryResolvedCaptPos, dryCaptMargin, dryNormCaptStyle, dryCaptForceStyle,
			sourceClips, sourcePath, workDir, assembledBase, draftPath, captPath, voPath)
		return nil
	}

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("creating render_work dir: %w", err)
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_ASSEMBLE_STARTED", map[string]any{
			"plan_id":    planID,
			"mode":       opts.Mode,
			"clip_count": len(sourceClips),
		})
	}

	var warnings []string
	var stages []AssembleStage

	// ---- Stage 1: clip assembly ----
	assembledClips, clipWarnings, assembleErr := runClipAssembly(planID, sourceClips, sourcePath, workDir, planDir, outputsDir, assembledBase, opts.Mode, runner, log)
	warnings = append(warnings, clipWarnings...)

	if assembleErr != nil {
		writeFailedResult(resultPath, planID, opts.Mode, assembledClips, warnings, nil, nil, nil, nil)
		if log != nil {
			_ = log.Write("CREATIVE_ASSEMBLE_FAILED", map[string]any{"plan_id": planID, "reason": assembleErr.Error()})
			_ = log.Close()
		}
		return assembleErr
	}

	stageAssembled := "outputs/draft_assembled.mp4"
	if !needsPostProcess {
		stageAssembled = "outputs/draft.mp4"
	}
	stages = append(stages, AssembleStage{Name: "assembled_video", File: stageAssembled, Status: "completed"})

	// ---- Stage 2: voiceover mix ----
	var voResult *AssembleVoiceoverResult
	var afterVoiceover string

	if opts.MixVoiceover {
		voResult = &AssembleVoiceoverResult{Requested: true}

		if voPath == "" {
			voResult.Status = "skipped"
			voWarn := "voiceover requested but not found; continuing without voiceover"
			placement := fmt.Sprintf("To add voiceover: place audio at %s or pass --voiceover <path>", outputsDir)
			warnings = append(warnings, voWarn, placement)
			stages = append(stages, AssembleStage{Name: "voiceover_mix", File: "", Status: "skipped"})
			afterVoiceover = assembledBase
		} else {
			voResult.SourcePath = voPath
			if opts.BurnCaptions || hasPlatform {
				afterVoiceover = filepath.Join(outputsDir, "draft_audio.mp4")
			} else {
				afterVoiceover = draftPath
			}

			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_VOICEOVER_STARTED", map[string]any{"plan_id": planID, "voiceover": voPath})
			}
			voArgs := buildVoiceoverArgs(assembledBase, voPath, afterVoiceover)
			voOut, voErr := runner.Run(voArgs)
			if voErr != nil {
				snippet := truncateStderr(voOut, 5, 400)
				voResult.Status = "failed"
				voResult.Error = snippet
				warnings = append(warnings, fmt.Sprintf("voiceover mix failed: %v: %s", voErr, snippet))
				stages = append(stages, AssembleStage{Name: "voiceover_mix", File: relPath(planDir, afterVoiceover), Status: "failed"})
				afterVoiceover = assembledBase // fall back to assembled
				if log != nil {
					_ = log.Write("CREATIVE_ASSEMBLE_VOICEOVER_FAILED", map[string]any{"plan_id": planID, "error": voErr.Error()})
				}
			} else {
				voResult.Status = "applied"
				stages = append(stages, AssembleStage{Name: "voiceover_mix", File: relPath(planDir, afterVoiceover), Status: "completed"})
				if log != nil {
					_ = log.Write("CREATIVE_ASSEMBLE_VOICEOVER_COMPLETED", map[string]any{"plan_id": planID, "output": afterVoiceover})
				}
			}
		}
	} else {
		afterVoiceover = assembledBase
	}

	// ---- Stage 3: platform format ----
	var platResult *AssemblePlatformResult
	afterPlatform := afterVoiceover

	if hasPlatform {
		platResult = &AssemblePlatformResult{
			Requested:  true,
			Normalized: normPlatform,
			Width:      platPreset.Width,
			Height:     platPreset.Height,
			Fit:        platFit,
			Background: platBackground,
		}

		var platOut string
		if opts.BurnCaptions {
			platOut = filepath.Join(outputsDir, "draft_platform.mp4")
		} else {
			platOut = draftPath
		}

		if log != nil {
			_ = log.Write("CREATIVE_ASSEMBLE_PLATFORM_STARTED", map[string]any{
				"plan_id":  planID,
				"platform": normPlatform,
				"width":    platPreset.Width,
				"height":   platPreset.Height,
				"fit":      platFit,
			})
		}

		platArgs := buildPlatformArgs(afterVoiceover, platOut, platPreset, platFit, platBackground)
		platOut2, platErr2 := runner.Run(platArgs)
		if platErr2 != nil {
			snippet := truncateStderr(platOut2, 5, 400)
			platResult.Status = "failed"
			platResult.Error = snippet
			warnings = append(warnings, fmt.Sprintf("platform format failed: %v: %s", platErr2, snippet))
			stages = append(stages, AssembleStage{Name: "platform_format", File: relPath(planDir, platOut), Status: "failed"})
			afterPlatform = afterVoiceover // fall back to pre-platform
			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_PLATFORM_FAILED", map[string]any{"plan_id": planID, "error": platErr2.Error()})
			}
		} else {
			platResult.Status = "applied"
			stages = append(stages, AssembleStage{Name: "platform_format", File: relPath(planDir, platOut), Status: "completed"})
			afterPlatform = platOut
			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_PLATFORM_COMPLETED", map[string]any{
					"plan_id":  planID,
					"platform": normPlatform,
					"output":   platOut,
				})
			}
		}
	}

	// ---- Caption position/style resolution ----
	normCaptPos, _ := NormalizeCaptionPosition(opts.CaptionPosition) // already validated above
	resolvedCaptPos := ResolveCaptionPosition(normCaptPos, normPlatform)
	captMargin := opts.CaptionMargin
	if captMargin <= 0 {
		captMargin = DefaultCaptionMargin(normPlatform)
	}
	normCaptStyle, _ := NormalizeCaptionStyle(opts.CaptionStyle) // already validated above
	captForceStyle := buildForceStyleArg(resolvedCaptPos, captMargin, normCaptStyle)

	// ---- Stage 4: caption burn ----
	var captResult *AssembleCaptionsResult
	var finalFile string

	if opts.BurnCaptions {
		captResult = &AssembleCaptionsResult{
			Requested:   true,
			Position:    resolvedCaptPos,
			Margin:      captMargin,
			Style:       normCaptStyle,
			FilterStyle: captForceStyle,
		}

		switch {
		case captPath == "":
			captResult.Status = "skipped"
			warnings = append(warnings, "caption burn requested but no SRT file found; continuing without captions")
			stages = append(stages, AssembleStage{Name: "caption_burn", File: "", Status: "skipped"})
			finalFile = afterPlatform

		case !hasSubtitlesFilter:
			captResult.Status = "skipped"
			const filterWarn = "caption burn skipped: ffmpeg does not support the subtitles filter on this system. " +
				"Install ffmpeg with libass support to enable caption burn."
			warnings = append(warnings, filterWarn)
			stages = append(stages, AssembleStage{Name: "caption_burn", File: "", Status: "skipped"})
			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_CAPTIONS_FAILED", map[string]any{
					"plan_id": planID, "error": "subtitles filter unavailable",
				})
			}
			finalFile = afterPlatform

		default:
			captResult.SourcePath = captPath
			finalFile = draftPath

			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_CAPTIONS_STARTED", map[string]any{"plan_id": planID, "captions": captPath})
			}
			captArgs := buildCaptionArgs(afterPlatform, captPath, draftPath, captForceStyle)
			captOut, captErr := runner.Run(captArgs)
			if captErr != nil {
				snippet := truncateStderr(captOut, 5, 400)
				captResult.Status = "failed"
				captResult.Error = snippet
				warnings = append(warnings, fmt.Sprintf("caption burn failed: %v: %s", captErr, snippet))
				stages = append(stages, AssembleStage{Name: "caption_burn", File: "outputs/draft.mp4", Status: "failed"})
				finalFile = afterPlatform // fall back
				if log != nil {
					_ = log.Write("CREATIVE_ASSEMBLE_CAPTIONS_FAILED", map[string]any{"plan_id": planID, "error": captErr.Error()})
				}
			} else {
				captResult.Status = "applied"
				stages = append(stages, AssembleStage{Name: "caption_burn", File: "outputs/draft.mp4", Status: "completed"})
				if log != nil {
					_ = log.Write("CREATIVE_ASSEMBLE_CAPTIONS_COMPLETED", map[string]any{"plan_id": planID, "output": draftPath})
				}
			}
		}
	} else {
		finalFile = afterPlatform
	}

	// if finalFile isn't draft.mp4, rename it
	if finalFile != draftPath {
		if err := os.Rename(finalFile, draftPath); err != nil {
			// try ffmpeg remux as fallback
			remuxArgs := []string{"-y", "-i", finalFile, "-c", "copy", draftPath}
			if _, remuxErr := runner.Run(remuxArgs); remuxErr != nil {
				warnings = append(warnings, fmt.Sprintf("could not finalize draft.mp4: %v", remuxErr))
			}
		}
		// The assembled_video intermediate was renamed into draft.mp4. Update the stage
		// record so validate-creative-assemble does not warn about a missing intermediate.
		for i := range stages {
			if stages[i].Name == "assembled_video" && stages[i].File == "outputs/draft_assembled.mp4" {
				stages[i].File = "outputs/draft.mp4"
			}
		}
	}

	overallStatus := "completed"
	if _, err := os.Stat(draftPath); err != nil {
		overallStatus = "failed"
	} else if len(warnings) > 0 {
		overallStatus = "completed_with_warnings"
	}

	if log != nil {
		_ = log.Write("CREATIVE_ASSEMBLE_COMPLETED", map[string]any{
			"plan_id":       planID,
			"output_file":   draftPath,
			"clip_count":    len(assembledClips),
			"has_captions":  captResult != nil && captResult.Status == "applied",
			"has_voiceover": voResult != nil && voResult.Status == "applied",
		})
		_ = log.Close()
	}

	// probe final draft dimensions
	var finalProbe *AssembleFinalProbe
	if overallStatus != "failed" {
		if fp, fpErr := probeFullResult(draftPath); fpErr == nil {
			finalProbe = fp
		}
	}

	result := CreativeAssembleResult{
		SchemaVersion:   "creative_assemble_result.v1",
		CreatedAt:       time.Now().UTC(),
		CreativePlanID:  planID,
		Mode:            opts.Mode,
		Status:          overallStatus,
		OutputFile:      "outputs/draft.mp4",
		FinalOutputFile: "outputs/draft.mp4",
		WorkDir:         "outputs/render_work",
		Clips:           assembledClips,
		Captions:        captResult,
		Voiceover:       voResult,
		Platform:        platResult,
		FinalProbe:      finalProbe,
		Stages:          stages,
		Warnings:        dedupeStrings(warnings),
	}
	if err := writeJSONFile(resultPath, result); err != nil {
		return fmt.Errorf("write creative_assemble_result.json: %w", err)
	}

	_ = updateCreativeOutputsIndex(planID, "creative_assemble_result", "outputs/creative_assemble_result.json", "")
	if overallStatus != "failed" {
		_ = updateCreativeOutputsIndex(planID, "draft_video", "outputs/draft.mp4", "")
	}

	// patch creative_plan.json
	planJSONPath := filepath.Join(planDir, "creative_plan.json")
	if planRaw, err2 := os.ReadFile(planJSONPath); err2 == nil {
		var pm map[string]any
		if json.Unmarshal(planRaw, &pm) == nil {
			if overallStatus != "failed" {
				pm["execution_status"] = "assembled"
			}
			existingArtifacts, _ := pm["output_artifacts"].([]any)
			newArts := []any{"outputs/draft.mp4", "outputs/creative_assemble_result.json"}
			for _, a := range existingArtifacts {
				s, _ := a.(string)
				if s != "outputs/draft.mp4" && s != "outputs/creative_assemble_result.json" {
					newArts = append(newArts, a)
				}
			}
			pm["output_artifacts"] = newArts
			_ = writeJSONFile(planJSONPath, pm)
		}
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(stdout, "creative-assemble: %s\n", planID)
	fmt.Fprintf(stdout, "  mode:      %s\n", opts.Mode)
	fmt.Fprintf(stdout, "  status:    %s\n", overallStatus)
	fmt.Fprintf(stdout, "  clips:     %d rendered\n", len(assembledClips))
	fmt.Fprintf(stdout, "  output:    %s\n", draftPath)
	if platResult != nil {
		if platResult.Status == "applied" {
			fmt.Fprintf(stdout, "  platform:  %s (%dx%d, fit=%s)\n", normPlatform, platPreset.Width, platPreset.Height, platFit)
		} else {
			fmt.Fprintf(stdout, "  platform:  %s (%s)\n", normPlatform, platResult.Status)
		}
	}
	if captResult != nil {
		fmt.Fprintf(stdout, "  captions:  %s\n", captResult.Status)
	}
	if voResult != nil {
		fmt.Fprintf(stdout, "  voiceover: %s\n", voResult.Status)
	}
	if finalProbe != nil && finalProbe.Width > 0 {
		fmt.Fprintf(stdout, "  final:     %dx%d\n", finalProbe.Width, finalProbe.Height)
	}
	for _, w := range dedupeStrings(warnings) {
		fmt.Fprintf(stdout, "  warning:   %s\n", w)
	}
	fmt.Fprintf(stdout, "\nnext: byom-video validate-creative-assemble %s\n", planID)
	return nil
}

// ---- helpers ----

func runClipAssembly(
	planID string,
	sourceClips []CreativeTimelineItem,
	sourcePath, workDir, planDir, outputsDir, targetFile, mode string,
	runner ffmpegRunner,
	log *events.Log,
) ([]AssembledClip, []string, error) {
	var clips []AssembledClip
	var warnings []string
	allOK := true

	for i, item := range sourceClips {
		workFile := filepath.Join(workDir, fmt.Sprintf("clip_%04d.mp4", i+1))
		relWorkFile := filepath.Join("outputs", "render_work", fmt.Sprintf("clip_%04d.mp4", i+1))
		dur := item.SourceEnd - item.SourceStart
		clip := AssembledClip{
			ID:              fmt.Sprintf("clip_%04d", i+1),
			SourcePath:      sourcePath,
			Start:           item.SourceStart,
			End:             item.SourceEnd,
			DurationSeconds: dur,
			WorkFile:        relWorkFile,
		}
		args := buildClipArgs(mode, item.SourceStart, item.SourceEnd, sourcePath, workFile)
		out, runErr := runner.Run(args)
		if runErr != nil {
			snippet := truncateStderr(out, 5, 400)
			clip.Status = "failed"
			clip.Error = snippet
			allOK = false
			warnings = append(warnings, fmt.Sprintf("clip %s failed: %v: %s", clip.ID, runErr, snippet))
			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_CLIP_RENDERED", map[string]any{"clip_id": clip.ID, "status": "failed"})
			}
		} else {
			clip.Status = "completed"
			if log != nil {
				_ = log.Write("CREATIVE_ASSEMBLE_CLIP_RENDERED", map[string]any{"clip_id": clip.ID, "status": "completed"})
			}
		}
		clips = append(clips, clip)
	}

	var completedClips []AssembledClip
	for _, c := range clips {
		if c.Status == "completed" {
			completedClips = append(completedClips, c)
		}
	}

	if len(completedClips) == 0 {
		return clips, warnings, fmt.Errorf("creative-assemble: all clips failed")
	}

	// write concat list — use basename only; ffmpeg resolves paths relative to
	// the concat list file location (render_work/), so using the full path would
	// cause doubling when planDir is relative to CWD.
	concatListPath := filepath.Join(workDir, "concat_list.txt")
	var concatLines strings.Builder
	for _, c := range completedClips {
		fmt.Fprintf(&concatLines, "file '%s'\n", filepath.Base(c.WorkFile))
	}
	if err := os.WriteFile(concatListPath, []byte(concatLines.String()), 0o644); err != nil {
		return clips, warnings, fmt.Errorf("write concat_list.txt: %w", err)
	}

	// assemble to targetFile
	var assembleErr error
	if len(completedClips) == 1 {
		absWork := filepath.Join(planDir, completedClips[0].WorkFile)
		remuxArgs := []string{"-y", "-i", absWork, "-c", "copy", targetFile}
		if out, runErr := runner.Run(remuxArgs); runErr != nil {
			assembleErr = fmt.Errorf("remux single clip: %w: %s", runErr, strings.TrimSpace(string(out)))
		}
	} else {
		concatArgs := []string{"-y", "-f", "concat", "-safe", "0", "-i", concatListPath, "-c", "copy", targetFile}
		if out, runErr := runner.Run(concatArgs); runErr != nil {
			assembleErr = fmt.Errorf("ffmpeg concat: %w: %s", runErr, strings.TrimSpace(string(out)))
		}
	}
	if !allOK {
		warnings = append(warnings, "some clips failed; assembled from successful clips only")
	}
	return clips, warnings, assembleErr
}

func writeFailedResult(resultPath, planID, mode string, clips []AssembledClip, warnings []string, capt *AssembleCaptionsResult, vo *AssembleVoiceoverResult, plat *AssemblePlatformResult, stages []AssembleStage) {
	_ = writeJSONFile(resultPath, CreativeAssembleResult{
		SchemaVersion:   "creative_assemble_result.v1",
		CreatedAt:       time.Now().UTC(),
		CreativePlanID:  planID,
		Mode:            mode,
		Status:          "failed",
		OutputFile:      "outputs/draft.mp4",
		FinalOutputFile: "outputs/draft.mp4",
		WorkDir:         "outputs/render_work",
		Clips:           clips,
		Captions:        capt,
		Voiceover:       vo,
		Platform:        plat,
		Stages:          stages,
		Warnings:        dedupeStrings(warnings),
	})
}

func buildClipArgs(mode string, start, end float64, inputPath, outputPath string) []string {
	base := []string{
		"-y",
		"-ss", fmt.Sprintf("%.6f", start),
		"-to", fmt.Sprintf("%.6f", end),
		"-i", inputPath,
	}
	if mode == "stream-copy" {
		return append(base, "-c", "copy", outputPath)
	}
	return append(base, "-c:v", "libx264", "-c:a", "aac", outputPath)
}

// buildVoiceoverArgs mixes voiceover audio into a video file.
// Uses amix to blend original audio + voiceover; if input has no audio, amix still works with one input.
func buildVoiceoverArgs(videoIn, voiceoverIn, out string) []string {
	return []string{
		"-y",
		"-i", videoIn,
		"-i", voiceoverIn,
		"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=first:dropout_transition=2[outa]",
		"-map", "0:v",
		"-map", "[outa]",
		"-c:v", "copy",
		"-c:a", "aac",
		out,
	}
}

// buildCaptionArgs burns SRT captions into video using FFmpeg subtitles filter.
// forceStyle is the ASS force_style string (from buildForceStyleArg); empty means use defaults.
func buildCaptionArgs(videoIn, srtPath, out, forceStyle string) []string {
	escaped := escapeFilterPath(srtPath)
	filterStr := buildCaptionFilterString(escaped, forceStyle)
	return []string{
		"-y",
		"-i", videoIn,
		"-vf", filterStr,
		"-c:a", "copy",
		out,
	}
}

// escapeFilterPath escapes a file path for use in an FFmpeg filter graph option value.
// FFmpeg filter values use : as separator and ' as quote; \ as escape.
func escapeFilterPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "\\\\")
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "\\'")
	return path
}

// discoverCaptionsPath returns the first existing SRT file found from known locations.
func discoverCaptionsPath(planDir, timelineRunID, optsRunID string) string {
	// prefer explicit run-id from opts, then from timeline
	for _, runID := range []string{optsRunID, timelineRunID} {
		if runID == "" {
			continue
		}
		runDir, err := runstore.ResolveRunDir(runID)
		if err != nil {
			continue
		}
		candidate := filepath.Join(runDir, "captions.srt")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// check if only caption_plan.json exists (not a real SRT)
	captPlanPath := filepath.Join(planDir, "outputs", "caption_plan.json")
	if _, err := os.Stat(captPlanPath); err == nil {
		// It's only a plan — not a usable SRT file. Return "" and let caller warn.
		return ""
	}
	return ""
}

// discoverVoiceoverPath returns the first existing voiceover audio file found in outputsDir.
func discoverVoiceoverPath(outputsDir string) string {
	for _, name := range []string{"voiceover.wav", "voiceover.mp3", "voiceover.m4a", "voiceover.aac"} {
		p := filepath.Join(outputsDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func relPath(planDir, abs string) string {
	rel, err := filepath.Rel(planDir, abs)
	if err != nil {
		return abs
	}
	return rel
}

func printDryRun(
	stdout io.Writer,
	planID string,
	opts CreativeAssembleOptions,
	normPlatform string,
	platPreset PlatformPreset,
	platFit, platBackground string,
	captPos string, captMargin int, captStyle, captForceStyle string,
	sourceClips []CreativeTimelineItem,
	sourcePath, workDir, assembledBase, draftPath, captPath, voPath string,
) {
	hasPlatform := normPlatform != "" && normPlatform != "original"
	fmt.Fprintf(stdout, "creative-assemble dry-run: %s\n", planID)
	fmt.Fprintf(stdout, "  mode:          %s\n", opts.Mode)
	fmt.Fprintf(stdout, "  source:        %s\n", sourcePath)
	fmt.Fprintf(stdout, "  clips:         %d\n", len(sourceClips))
	fmt.Fprintf(stdout, "  work dir:      %s\n", workDir)
	if hasPlatform {
		fmt.Fprintf(stdout, "  platform:      %s (%dx%d, fit=%s)\n", normPlatform, platPreset.Width, platPreset.Height, platFit)
	}
	fmt.Fprintf(stdout, "  burn-captions: %v\n", opts.BurnCaptions)
	fmt.Fprintf(stdout, "  mix-voiceover: %v\n", opts.MixVoiceover)
	if opts.BurnCaptions {
		fmt.Fprintf(stdout, "  caption-pos:   %s (margin=%d, style=%s)\n", captPos, captMargin, captStyle)
		if captPath != "" {
			fmt.Fprintf(stdout, "  captions:      %s\n", captPath)
		} else {
			fmt.Fprintf(stdout, "  captions:      (not found — will skip with warning)\n")
		}
	}
	if voPath != "" {
		fmt.Fprintf(stdout, "  voiceover:     %s\n", voPath)
	} else if opts.MixVoiceover {
		fmt.Fprintf(stdout, "  voiceover:     (not found — will skip with warning)\n")
	}
	fmt.Fprintln(stdout, "\nplanned commands:")

	// clip cuts
	for i, item := range sourceClips {
		workFile := filepath.Join(workDir, fmt.Sprintf("clip_%04d.mp4", i+1))
		args := buildClipArgs(opts.Mode, item.SourceStart, item.SourceEnd, sourcePath, workFile)
		fmt.Fprintf(stdout, "  ffmpeg %s\n", strings.Join(args, " "))
	}
	concatList := filepath.Join(workDir, "concat_list.txt")
	fmt.Fprintf(stdout, "  (write %s)\n", concatList)
	fmt.Fprintf(stdout, "  ffmpeg -y -f concat -safe 0 -i %s -c copy %s\n", concatList, assembledBase)

	// voiceover
	afterVoiceover := assembledBase
	if opts.MixVoiceover && voPath != "" {
		var voOut string
		if opts.BurnCaptions || hasPlatform {
			voOut = filepath.Join(filepath.Dir(assembledBase), "draft_audio.mp4")
		} else {
			voOut = draftPath
		}
		voArgs := buildVoiceoverArgs(assembledBase, voPath, voOut)
		fmt.Fprintf(stdout, "  ffmpeg %s  # voiceover mix\n", strings.Join(voArgs, " "))
		afterVoiceover = voOut
	}

	// platform format
	afterPlatform := afterVoiceover
	if hasPlatform {
		var platOut string
		if opts.BurnCaptions {
			platOut = filepath.Join(filepath.Dir(assembledBase), "draft_platform.mp4")
		} else {
			platOut = draftPath
		}
		platArgs := buildPlatformArgs(afterVoiceover, platOut, platPreset, platFit, platBackground)
		fmt.Fprintf(stdout, "  ffmpeg %s  # platform format (%s)\n", strings.Join(platArgs, " "), normPlatform)
		afterPlatform = platOut
	}

	// captions
	if opts.BurnCaptions && captPath != "" {
		captArgs := buildCaptionArgs(afterPlatform, captPath, draftPath, captForceStyle)
		fmt.Fprintf(stdout, "  ffmpeg %s  # caption burn (pos=%s, style=%s)\n", strings.Join(captArgs, " "), captPos, captStyle)
	}

	fmt.Fprintln(stdout, "\nno files written (dry-run)")
}

// ---- validate-creative-assemble ----

func ValidateCreativeAssemble(planID string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	resultPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("creative_assemble_result.json not found — run creative-assemble first: %w", err)
	}
	var result CreativeAssembleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("creative_assemble_result.json is malformed: %w", err)
	}

	var errs []string
	var warns []string

	if result.SchemaVersion != "creative_assemble_result.v1" {
		errs = append(errs, fmt.Sprintf("schema_version %q != expected creative_assemble_result.v1", result.SchemaVersion))
	}
	if result.OutputFile == "" {
		errs = append(errs, "output_file is empty")
	}

	// check draft.mp4 exists
	draftPath := filepath.Join(planDir, result.OutputFile)
	if _, err := os.Stat(draftPath); err != nil {
		errs = append(errs, fmt.Sprintf("output_file not found: %s", result.OutputFile))
	}

	// check work clips
	for _, clip := range result.Clips {
		if clip.Status != "completed" {
			continue
		}
		workPath := filepath.Join(planDir, clip.WorkFile)
		if _, err := os.Stat(workPath); err != nil {
			warns = append(warns, fmt.Sprintf("work clip not found: %s", clip.WorkFile))
		}
	}

	// validate captions fields
	if result.Captions != nil {
		if result.Captions.Status == "applied" && result.Captions.SourcePath != "" {
			if _, err := os.Stat(result.Captions.SourcePath); err != nil {
				errs = append(errs, fmt.Sprintf("captions.source_path not found: %s", result.Captions.SourcePath))
			}
		}
		if result.Captions.Status == "applied" && result.Captions.SourcePath == "" {
			errs = append(errs, "captions.status=applied but source_path is empty")
		}
	}

	// validate voiceover fields
	if result.Voiceover != nil {
		if result.Voiceover.Status == "applied" && result.Voiceover.SourcePath != "" {
			if _, err := os.Stat(result.Voiceover.SourcePath); err != nil {
				warns = append(warns, fmt.Sprintf("voiceover.source_path not found: %s", result.Voiceover.SourcePath))
			}
		}
		if result.Voiceover.Status == "applied" && result.Voiceover.SourcePath == "" {
			errs = append(errs, "voiceover.status=applied but source_path is empty")
		}
	}

	// validate stage files
	for _, stage := range result.Stages {
		if stage.Status != "completed" || stage.File == "" {
			continue
		}
		stageAbs := filepath.Join(planDir, stage.File)
		if _, err := os.Stat(stageAbs); err != nil {
			warns = append(warns, fmt.Sprintf("stage %s file not found: %s", stage.Name, stage.File))
		}
	}

	// validate platform dimensions if platform was applied
	if result.Platform != nil && result.Platform.Status == "applied" {
		if _, err := os.Stat(draftPath); err == nil {
			w, h, probeErr := probeVideoDimensions(draftPath)
			if probeErr != nil {
				warns = append(warns, fmt.Sprintf("ffprobe for platform dimension check failed: %v", probeErr))
			} else if w == 0 && h == 0 {
				warns = append(warns, "ffprobe not available; cannot verify platform dimensions")
			} else {
				if w != result.Platform.Width || h != result.Platform.Height {
					errs = append(errs, fmt.Sprintf(
						"platform dimension mismatch: expected %dx%d (%s), got %dx%d",
						result.Platform.Width, result.Platform.Height, result.Platform.Normalized, w, h,
					))
				}
			}
		}
	}

	// probe final draft for duration if ffprobe available
	if _, ferr := media.FindExecutable("ffprobe"); ferr == nil {
		if _, err := os.Stat(draftPath); err == nil {
			probeData, probeErr := media.Probe(draftPath)
			if probeErr != nil {
				warns = append(warns, fmt.Sprintf("ffprobe on draft.mp4 failed: %v", probeErr))
			} else {
				var pf map[string]any
				if json.Unmarshal(probeData, &pf) == nil {
					if fmtMap, ok := pf["format"].(map[string]any); ok {
						durStr, _ := fmtMap["duration"].(string)
						if durStr == "" {
							warns = append(warns, "ffprobe: could not read duration from draft.mp4")
						}
					}
				}
			}
		}
	}

	valid := len(errs) == 0
	out := map[string]any{
		"valid":    valid,
		"status":   result.Status,
		"mode":     result.Mode,
		"clips":    len(result.Clips),
		"errors":   errs,
		"warnings": warns,
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Fprintln(stdout, "Validate creative assemble")
	fmt.Fprintf(stdout, "  plan id:  %s\n", planID)
	fmt.Fprintf(stdout, "  status:   %s\n", result.Status)
	fmt.Fprintf(stdout, "  mode:     %s\n", result.Mode)
	if result.Platform != nil {
		fmt.Fprintf(stdout, "  platform: %s (%s)\n", result.Platform.Normalized, result.Platform.Status)
		if result.Platform.Status == "applied" {
			fmt.Fprintf(stdout, "  expected: %dx%d\n", result.Platform.Width, result.Platform.Height)
		}
	}
	if result.FinalProbe != nil && result.FinalProbe.Width > 0 {
		fmt.Fprintf(stdout, "  actual:   %dx%d\n", result.FinalProbe.Width, result.FinalProbe.Height)
	}
	if result.Captions != nil {
		fmt.Fprintf(stdout, "  captions: %s\n", result.Captions.Status)
		if result.Captions.Status == "applied" && result.Captions.Position != "" {
			fmt.Fprintf(stdout, "  caption-pos: %s (margin=%d, style=%s)\n",
				result.Captions.Position, result.Captions.Margin, result.Captions.Style)
		}
	}
	if result.Voiceover != nil {
		fmt.Fprintf(stdout, "  voiceover:%s\n", result.Voiceover.Status)
	}
	if valid {
		fmt.Fprintln(stdout, "  valid:    ok")
	} else {
		fmt.Fprintln(stdout, "  valid:    failed")
	}
	for _, e := range errs {
		fmt.Fprintf(stdout, "  error:    %s\n", e)
	}
	for _, w := range warns {
		fmt.Fprintf(stdout, "  warning:  %s\n", w)
	}
	if !valid {
		return fmt.Errorf("creative-assemble validation failed")
	}
	return nil
}

// ---- review-creative-assemble ----

func ReviewCreativeAssemble(planID string, stdout io.Writer, opts ReviewCreativeAssembleOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	resultPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("creative_assemble_result.json not found — run creative-assemble first: %w", err)
	}
	var result CreativeAssembleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("creative_assemble_result.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	var b strings.Builder
	b.WriteString("# Creative Assemble Review\n\n")
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", result.CreativePlanID)
	fmt.Fprintf(&b, "- Mode: `%s`\n", result.Mode)
	fmt.Fprintf(&b, "- Status: `%s`\n", result.Status)
	fmt.Fprintf(&b, "- Output: `%s`\n", result.FinalOutputFile)
	fmt.Fprintf(&b, "- Work dir: `%s`\n", result.WorkDir)
	fmt.Fprintf(&b, "- Clips: %d\n", len(result.Clips))

	if result.Platform != nil {
		fmt.Fprintf(&b, "- Platform: `%s` (%dx%d, fit=%s, status=%s)\n",
			result.Platform.Normalized, result.Platform.Width, result.Platform.Height,
			result.Platform.Fit, result.Platform.Status)
		if result.Platform.Error != "" {
			fmt.Fprintf(&b, "  - error: %s\n", result.Platform.Error)
		}
	}
	if result.FinalProbe != nil && result.FinalProbe.Width > 0 {
		fmt.Fprintf(&b, "- Final dimensions: `%dx%d` (%.2fs)\n",
			result.FinalProbe.Width, result.FinalProbe.Height, result.FinalProbe.DurationSeconds)
	}
	if result.Captions != nil {
		fmt.Fprintf(&b, "- Captions: `%s`", result.Captions.Status)
		if result.Captions.SourcePath != "" {
			fmt.Fprintf(&b, " (`%s`)", result.Captions.SourcePath)
		}
		b.WriteString("\n")
		if result.Captions.Status == "applied" && result.Captions.Position != "" {
			fmt.Fprintf(&b, "  - position: `%s`, margin: `%d`, style: `%s`\n",
				result.Captions.Position, result.Captions.Margin, result.Captions.Style)
		}
		if result.Captions.FilterStyle != "" {
			fmt.Fprintf(&b, "  - force_style: `%s`\n", result.Captions.FilterStyle)
		}
	}
	if result.Voiceover != nil {
		fmt.Fprintf(&b, "- Voiceover: `%s`", result.Voiceover.Status)
		if result.Voiceover.SourcePath != "" {
			fmt.Fprintf(&b, " (`%s`)", result.Voiceover.SourcePath)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(result.Stages) > 0 {
		b.WriteString("## Stages\n\n")
		for _, stage := range result.Stages {
			fmt.Fprintf(&b, "- `%s` → `%s` (%s)\n", stage.Name, stage.File, stage.Status)
		}
		b.WriteString("\n")
	}

	if len(result.Clips) > 0 {
		b.WriteString("## Clips\n\n")
		for _, clip := range result.Clips {
			fmt.Fprintf(&b, "- `%s` [%.2f–%.2fs] → `%s` (%s)", clip.ID, clip.Start, clip.End, clip.WorkFile, clip.Status)
			if clip.Error != "" {
				fmt.Fprintf(&b, " — error: %s", timelineTruncate(clip.Error, 80))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		b.WriteString("## Warnings\n\n")
		for _, w := range result.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Next Commands\n\n")
	fmt.Fprintf(&b, "```sh\nbyom-video validate-creative-assemble %s\n```\n", result.CreativePlanID)

	review := b.String()
	fmt.Fprint(stdout, review)

	if opts.WriteArtifact {
		reviewPath := filepath.Join(planDir, "outputs", "creative_assemble_review.md")
		if err := os.WriteFile(reviewPath, []byte(review), 0o644); err != nil {
			return fmt.Errorf("writing creative_assemble_review.md: %w", err)
		}
		_ = updateCreativeOutputsIndex(planID, "creative_assemble_review", "outputs/creative_assemble_review.md", "")
		fmt.Fprintf(stdout, "\nartifact written: %s\n", reviewPath)
	}

	return nil
}

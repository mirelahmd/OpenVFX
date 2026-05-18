package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/exporter"
	"github.com/mirelahmd/byom-video/internal/runinfo"
)

const makesRoot = ".byom-video/makes"

// ---- schema types ----

type DraftProbeInfo struct {
	DurationSeconds  float64 `json:"duration_seconds"`
	VideoStreamCount int     `json:"video_stream_count"`
	AudioStreamCount int     `json:"audio_stream_count"`
}

type MakeSummary struct {
	SchemaVersion    string          `json:"schema_version"`
	MakeID           string          `json:"make_id"`
	CreatedAt        time.Time       `json:"created_at"`
	InputPath        string          `json:"input_path"`
	Goal             string          `json:"goal"`
	Preset           string          `json:"preset"`
	Status           string          `json:"status"` // planned|completed|failed
	SkipPipeline     bool            `json:"skip_pipeline,omitempty"`
	ReusedRunID      string          `json:"reused_run_id,omitempty"`
	InputWarning     string          `json:"input_warning,omitempty"`
	RunID            string          `json:"run_id,omitempty"`
	CreativePlanID   string          `json:"creative_plan_id,omitempty"`
	PipelineStatus   string          `json:"pipeline_status,omitempty"`
	CreativeStatus   string          `json:"creative_status,omitempty"`
	ScriptStatus          string          `json:"script_status,omitempty"`
	ScriptMode            string          `json:"script_mode,omitempty"`
	ScriptModel           string          `json:"script_model,omitempty"`
	StyleUsed             bool            `json:"style_used,omitempty"`
	CaptionVariantsStatus string          `json:"caption_variants_status,omitempty"`
	CaptionVariantsMode   string          `json:"caption_variants_mode,omitempty"`
	CaptionVariantsCount  int             `json:"caption_variants_count,omitempty"`
	CaptionVariantsModel  string          `json:"caption_variants_model,omitempty"`
	VoiceoverTextStatus            string `json:"voiceover_text_status,omitempty"`
	VoiceoverTextWordCount         int    `json:"voiceover_text_word_count,omitempty"`
	VoiceoverAudioStatus           string `json:"voiceover_audio_status,omitempty"`
	VoiceoverAudioPath             string `json:"voiceover_audio_path,omitempty"`
	GeneratedVoiceoverStatus       string `json:"generated_voiceover_status,omitempty"`
	GeneratedVoiceoverProvider     string `json:"generated_voiceover_provider,omitempty"`
	GeneratedVoiceoverModel        string `json:"generated_voiceover_model,omitempty"`
	GeneratedVoiceoverAudioPath    string `json:"generated_voiceover_audio_path,omitempty"`
	GeneratedVoiceoverBytes        int64  `json:"generated_voiceover_bytes,omitempty"`
	PlatformPreset   string          `json:"platform_preset,omitempty"`
	PlatformWidth    int             `json:"platform_width,omitempty"`
	PlatformHeight   int             `json:"platform_height,omitempty"`
	PlatformFit      string          `json:"platform_fit,omitempty"`
	PlatformStatus   string          `json:"platform_status,omitempty"`
	CaptionPosition  string          `json:"caption_position,omitempty"`
	CaptionMargin    int             `json:"caption_margin,omitempty"`
	CaptionStyle     string          `json:"caption_style,omitempty"`
	FinalWidth       int             `json:"final_width,omitempty"`
	FinalHeight      int             `json:"final_height,omitempty"`
	AssembleStatus   string          `json:"assemble_status,omitempty"`
	ValidationStatus string          `json:"validation_status,omitempty"`
	ExportStatus     string          `json:"export_status,omitempty"`
	ExportedFiles    []string        `json:"exported_files,omitempty"`
	CaptionStatus    string          `json:"caption_status,omitempty"`
	VoiceoverStatus  string          `json:"voiceover_status,omitempty"`
	DraftPath        string          `json:"draft_path,omitempty"`
	DraftProbe       *DraftProbeInfo `json:"draft_probe,omitempty"`
	Errors             []string        `json:"errors,omitempty"`
	Warnings           []string        `json:"warnings,omitempty"`
	NextCommands       []string        `json:"next_commands"`
	LatestRevisionID   string          `json:"latest_revision_id,omitempty"`
	RevisionCount      int             `json:"revision_count,omitempty"`
	RevisionStatus     string          `json:"revision_status,omitempty"` // planned|completed|failed
}

// ---- options ----

type MakeOptions struct {
	Goal                  string
	Preset                string // shorts | metadata
	Yes                   bool
	DryRun                bool
	SkipPipeline          string // run_id to reuse
	StrictInput           bool
	Export                bool
	RequireExport         bool
	BurnCaptions          bool
	AllowMissingCaptions  bool
	MixVoiceover          bool
	VoiceoverPath         string
	AllowMissingVoiceover bool
	Mode                  string // reencode | stream-copy
	GoalAware             bool
	UseOllamaGoal         bool
	GenerateScript        bool
	ScriptFallbackStub    bool
	ScriptStyleDir        string
	ScriptNoStyle         bool
	ScriptRoute           string
	ScriptModelEntry      string
	ScriptMaxWords        int
	ScriptTone            string
	GenerateCaptions      bool
	CaptionFallbackStub   bool
	CaptionCount          int
	CaptionMaxWords       int
	CaptionTone           string
	PrepareVoiceover             bool
	VoiceoverMaxWords            int
	VoiceoverTone                string
	RequireVoiceover             bool
	GenerateVoiceover            bool
	VoiceoverRoute               string
	VoiceoverBackend             string
	VoiceoverVoiceID             string
	VoiceoverTimeoutSeconds      int
	VoiceoverDryRun              bool
	VoiceoverCheckEnv            bool
	AllowMissingGeneratedVoiceover bool
	Platform                       string // platform preset for creative-assemble
	Fit                            string // "crop" | "pad"
	Background                     string // color for pad mode
	CaptionPosition                string // bottom|center|top|auto
	CaptionMargin                  int    // vertical margin px; 0 = platform default
	CaptionStyle                   string // default|bold|boxed
	VoiceoverStability             float64 // 0–1; -1 = use backend default
	VoiceoverSimilarityBoost       float64 // 0–1; -1 = use backend default
	VoiceoverOutputFormat          string  // e.g. "mp3_44100_128"; "" = backend default
	JSON                           bool
	KeepWork                       bool
	Overwrite                      bool
	PythonInterpreter              string
}

type MakesOptions struct{ JSON bool }

type InspectMakeOptions struct{ JSON bool }

type MakeResultOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- Make command ----

func Make(inputPath string, stdout io.Writer, opts MakeOptions) error {
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("--goal is required")
	}

	// Validate and normalise preset
	preset := opts.Preset
	if preset == "" {
		preset = "shorts"
	}
	switch preset {
	case "shorts", "metadata":
	default:
		return fmt.Errorf("unknown preset %q; supported presets: shorts, metadata", preset)
	}

	// metadata preset can only run in planning mode or when reusing an existing run
	if preset == "metadata" && opts.Yes && opts.SkipPipeline == "" {
		return fmt.Errorf("make --preset metadata cannot assemble a draft because it does not produce clips; use --preset shorts or --skip-pipeline <run_id>")
	}

	absInput := ""
	if inputPath != "" {
		var err error
		absInput, err = filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("resolve input path: %w", err)
		}
		if _, err := os.Stat(absInput); err != nil {
			return fmt.Errorf("input file not found: %s", absInput)
		}
	}

	if opts.Mode == "" {
		opts.Mode = "reencode"
	}

	// Validate platform preset early so unknown values fail before any work begins.
	if opts.Platform != "" {
		if _, platErr := NormalizePlatform(opts.Platform); platErr != nil {
			return platErr
		}
	}

	makeID := time.Now().UTC().Format("20060102T150405Z") + "-" + shortID(opts.Goal)
	createdAt := time.Now().UTC()

	if opts.DryRun {
		printMakeDryRun(stdout, absInput, preset, opts)
		return nil
	}

	summary := MakeSummary{
		SchemaVersion: "make_summary.v1",
		MakeID:        makeID,
		CreatedAt:     createdAt,
		InputPath:     absInput,
		Goal:          opts.Goal,
		Preset:        preset,
	}

	// ---- Step 1: pipeline or skip-pipeline ----
	var runID string

	if opts.SkipPipeline != "" {
		// Reuse an existing run
		fmt.Fprintf(stdout, "make: %s\n", func() string {
			if absInput != "" {
				return filepath.Base(absInput)
			}
			return "(no input — reusing run)"
		}())
		fmt.Fprintf(stdout, "  goal:        %s\n", opts.Goal)
		fmt.Fprintf(stdout, "  preset:      %s\n", preset)
		fmt.Fprintf(stdout, "  step 1/6:    skip-pipeline, reusing run %s...\n", opts.SkipPipeline)

		reusedID, reusedInputPath, inputWarning, err := validateSkipPipelineRun(opts.SkipPipeline, absInput, opts.StrictInput)
		if err != nil {
			return err
		}
		runID = reusedID
		summary.SkipPipeline = true
		summary.ReusedRunID = reusedID
		summary.PipelineStatus = "skipped"
		if absInput == "" && reusedInputPath != "" {
			absInput = reusedInputPath
			summary.InputPath = absInput
		}
		if inputWarning != "" {
			summary.InputWarning = inputWarning
			summary.Warnings = append(summary.Warnings, inputWarning)
			fmt.Fprintf(stdout, "  warning:     %s\n", inputWarning)
		}
		fmt.Fprintf(stdout, "  run_id:      %s\n", runID)
	} else {
		if absInput == "" {
			return fmt.Errorf("make requires an input file argument (or --skip-pipeline <run_id>)")
		}
		fmt.Fprintf(stdout, "make: %s\n", filepath.Base(absInput))
		fmt.Fprintf(stdout, "  goal:        %s\n", opts.Goal)
		fmt.Fprintf(stdout, "  preset:      %s\n", preset)
		fmt.Fprintf(stdout, "  step 1/6:    pipeline (%s preset)...\n", preset)

		runsBefore, _ := listRunIDs()

		pipelineOpts := RunOptions{
			WithTranscript:    true,
			WithCaptions:      true,
			WithChunks:        true,
			WithHighlights:    true,
			WithRoughcut:      true,
			WithFFmpegScript:  true,
			WithReport:        true,
			PythonInterpreter: opts.PythonInterpreter,
		}
		if err := Run(absInput, io.Discard, pipelineOpts); err != nil {
			return fmt.Errorf("pipeline failed: %w", err)
		}
		var err error
		runID, err = newestRunID(runsBefore)
		if err != nil {
			return fmt.Errorf("could not determine run_id after pipeline: %w", err)
		}
		summary.PipelineStatus = "completed"
		fmt.Fprintf(stdout, "  run_id:      %s\n", runID)
	}
	summary.RunID = runID

	// ---- Step 1b (optional): export pipeline run ----
	if opts.Export {
		exportStatus, exportedFiles := runExport(runID, stdout, opts.RequireExport)
		summary.ExportStatus = exportStatus
		summary.ExportedFiles = exportedFiles
		if exportStatus == "failed" && opts.RequireExport {
			return fmt.Errorf("export failed and --require-export is set")
		}
	}

	// ---- Step 2 (optional): goal-aware reranking ----
	if opts.GoalAware {
		fmt.Fprintf(stdout, "  step 2/6:    goal-rerank + goal-roughcut...\n")
		rerankOpts := GoalRerankOptions{
			Goal:                  opts.Goal,
			UseOllama:             opts.UseOllamaGoal,
			FallbackDeterministic: opts.UseOllamaGoal,
		}
		if err := GoalRerankCommand(runID, io.Discard, rerankOpts); err != nil {
			msg := fmt.Sprintf("goal-rerank failed: %v (continuing)", err)
			summary.Warnings = append(summary.Warnings, msg)
			fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		} else {
			if err := GoalRoughcutCommand(runID, io.Discard, GoalRoughcutOptions{}); err != nil {
				msg := fmt.Sprintf("goal-roughcut failed: %v (continuing)", err)
				summary.Warnings = append(summary.Warnings, msg)
				fmt.Fprintf(stdout, "  warning:     %s\n", msg)
			}
		}
	} else {
		fmt.Fprintf(stdout, "  step 2/6:    goal-rerank skipped (use --goal-aware to enable)\n")
	}

	// ---- Step 3: creative-plan ----
	fmt.Fprintf(stdout, "  step 3/6:    creative-plan...\n")

	plansBefore, _ := listPlanIDs()

	planOpts := CreativePlanOptions{Goal: opts.Goal}
	if err := CreativePlanCommand(absInput, io.Discard, planOpts); err != nil {
		return fmt.Errorf("creative-plan failed: %w", err)
	}
	planID, err := newestPlanID(plansBefore)
	if err != nil {
		return fmt.Errorf("could not determine creative_plan_id after planning: %w", err)
	}
	summary.CreativePlanID = planID
	summary.CreativeStatus = "planned"
	fmt.Fprintf(stdout, "  plan_id:     %s\n", planID)

	// ---- Planning-only mode (no --yes) ----
	if !opts.Yes {
		nextCmds := buildNextCommands(runID, planID, "")
		summary.Status = "planned"
		summary.NextCommands = nextCmds
		if err := writeMakeSummary(makeID, summary); err != nil {
			fmt.Fprintf(stdout, "  warning:     could not write make_summary.json: %v\n", err)
		}

		fmt.Fprintf(stdout, "\nPlan created. Review before executing:\n")
		for _, cmd := range nextCmds {
			fmt.Fprintf(stdout, "  %s\n", cmd)
		}
		fmt.Fprintf(stdout, "\nRerun with --yes to execute end-to-end.\n")
		return nil
	}

	// ---- Execution mode (--yes) ----
	fmt.Fprintf(stdout, "  step 4/6:    approve + stub execution...\n")

	if err := ApproveCreativePlan(planID, io.Discard, ApproveCreativePlanOptions{}); err != nil {
		return fmt.Errorf("approve-creative-plan failed: %w", err)
	}
	stubOpts := CreativeExecuteStubOptions{Yes: true, Overwrite: opts.Overwrite}
	if err := CreativeExecuteStub(planID, io.Discard, stubOpts); err != nil {
		return fmt.Errorf("creative-execute-stub failed: %w", err)
	}
	summary.CreativeStatus = "stub_completed"

	// ---- Step 4b (optional): generate script via Ollama ----
	if opts.GenerateScript {
		fmt.Fprintf(stdout, "  step 4b/6:   generate-script...\n")
		scriptOpts := CreativeGenerateScriptOptions{
			Overwrite:    opts.Overwrite,
			StyleDir:     opts.ScriptStyleDir,
			NoStyle:      opts.ScriptNoStyle,
			ModelEntry:   opts.ScriptModelEntry,
			Route:        opts.ScriptRoute,
			FallbackStub: opts.ScriptFallbackStub,
			MaxWords:     opts.ScriptMaxWords,
			Tone:         opts.ScriptTone,
		}
		if err := CreativeGenerateScript(planID, io.Discard, scriptOpts); err != nil {
			msg := fmt.Sprintf("generate-script failed: %v (continuing)", err)
			summary.ScriptStatus = "failed"
			summary.Warnings = append(summary.Warnings, msg)
			fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		} else {
			summary.ScriptStatus = "completed"
			if sOut, sErr := readScriptOutput(planID); sErr == nil {
				summary.ScriptMode = sOut.Mode
				summary.ScriptModel = sOut.Model
				if sOut.StyleContext != nil && sOut.StyleContext.Enabled {
					summary.StyleUsed = true
				}
			}
		}
	}

	// ---- Step 4c (optional): generate caption variants via Ollama ----
	if opts.GenerateCaptions {
		fmt.Fprintf(stdout, "  step 4c/6:   generate-caption-variants...\n")
		captionOpts := CaptionVariantsOptions{
			Overwrite:    opts.Overwrite,
			StyleDir:     opts.ScriptStyleDir,
			NoStyle:      opts.ScriptNoStyle,
			FallbackStub: opts.CaptionFallbackStub,
			Count:        opts.CaptionCount,
			MaxWords:     opts.CaptionMaxWords,
			Tone:         opts.CaptionTone,
		}
		if err := CaptionVariants(planID, io.Discard, captionOpts); err != nil {
			msg := fmt.Sprintf("generate-caption-variants failed: %v (continuing)", err)
			summary.CaptionVariantsStatus = "failed"
			summary.Warnings = append(summary.Warnings, msg)
			fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		} else {
			summary.CaptionVariantsStatus = "completed"
			if cvOut, cvErr := readCaptionVariantsOutput(planID); cvErr == nil {
				summary.CaptionVariantsMode = cvOut.Mode
				summary.CaptionVariantsModel = cvOut.Model
				summary.CaptionVariantsCount = len(cvOut.Variants)
				if cvOut.StyleContext != nil && cvOut.StyleContext.Enabled {
					summary.StyleUsed = true
				}
			}
		}
	}

	// ---- Step 4d (optional): prepare voiceover text ----
	if opts.PrepareVoiceover {
		fmt.Fprintf(stdout, "  step 4d/6:   prepare-voiceover-text...\n")
		voTextOpts := VoiceoverTextOptions{
			Overwrite: opts.Overwrite,
			MaxWords:  opts.VoiceoverMaxWords,
			Tone:      opts.VoiceoverTone,
		}
		if err := VoiceoverTextCommand(planID, io.Discard, voTextOpts); err != nil {
			msg := fmt.Sprintf("prepare-voiceover-text failed: %v (continuing)", err)
			summary.VoiceoverTextStatus = "failed"
			summary.Warnings = append(summary.Warnings, msg)
			fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		} else {
			summary.VoiceoverTextStatus = "completed"
			if vtOut, vtErr := readVoiceoverTextOutput(planID); vtErr == nil {
				summary.VoiceoverTextWordCount = vtOut.WordCount
			}
		}
		// Check audio status
		planOutputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
		if audioPath := discoverVoiceoverAudio(planOutputsDir); audioPath != "" {
			summary.VoiceoverAudioStatus = "found"
			rel, _ := filepath.Rel(filepath.Join(creativePlansRoot, planID), audioPath)
			summary.VoiceoverAudioPath = rel
		} else {
			summary.VoiceoverAudioStatus = "missing"
			msg := fmt.Sprintf("Voiceover text prepared. Record/generate audio and save to outputs/voiceover.wav, then rerun with --mix-voiceover.")
			summary.Warnings = append(summary.Warnings, msg)
			fmt.Fprintf(stdout, "  next:        %s\n", msg)
		}
	}

	// ---- Step 4e (optional): generate voiceover via provider ----
	if opts.GenerateVoiceover {
		fmt.Fprintf(stdout, "  step 4e/6:   generate-voiceover (%s)...\n", func() string {
			if opts.VoiceoverDryRun {
				return "dry-run"
			}
			return "live"
		}())
		genVoOpts := GenerateVoiceoverOptions{
			PrepareText:     true,
			Overwrite:       opts.Overwrite,
			Route:           opts.VoiceoverRoute,
			Backend:         opts.VoiceoverBackend,
			VoiceID:         opts.VoiceoverVoiceID,
			TimeoutSeconds:  opts.VoiceoverTimeoutSeconds,
			DryRun:          opts.VoiceoverDryRun,
			CheckEnv:        opts.VoiceoverCheckEnv,
			Stability:       opts.VoiceoverStability,
			SimilarityBoost: opts.VoiceoverSimilarityBoost,
			OutputFormat:    opts.VoiceoverOutputFormat,
		}
		if err := GenerateVoiceover(planID, io.Discard, genVoOpts); err != nil {
			msg := fmt.Sprintf("generate-voiceover failed: %v", err)
			summary.GeneratedVoiceoverStatus = "failed"
			summary.Errors = append(summary.Errors, msg)
			if !opts.AllowMissingGeneratedVoiceover {
				return fmt.Errorf("%s", msg)
			}
			summary.Warnings = append(summary.Warnings, msg+" (continuing — --allow-missing-generated-voiceover)")
			fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		} else {
			if opts.VoiceoverDryRun {
				summary.GeneratedVoiceoverStatus = "dry_run"
			} else {
				summary.GeneratedVoiceoverStatus = "completed"
			}
			if genOut, genErr := readVoiceGenerationOutput(planID); genErr == nil {
				summary.GeneratedVoiceoverProvider = genOut.Provider
				summary.GeneratedVoiceoverModel = genOut.Model
				if genOut.Output != nil {
					summary.GeneratedVoiceoverAudioPath = genOut.Output.AudioFile
					summary.GeneratedVoiceoverBytes = genOut.Output.Bytes
				}
			}
		}
	}

	// ---- require-voiceover check (before assemble) ----
	if opts.RequireVoiceover && opts.MixVoiceover {
		planOutputsDir := filepath.Join(creativePlansRoot, planID, "outputs")
		voPath := opts.VoiceoverPath
		if voPath == "" {
			voPath = discoverVoiceoverAudio(planOutputsDir)
		}
		if voPath == "" {
			return fmt.Errorf("--require-voiceover: no voiceover audio found; place audio at outputs/voiceover.wav or pass --voiceover <path>")
		}
	}

	fmt.Fprintf(stdout, "  step 5/6:    timeline + render plan...\n")

	tlOpts := CreativeTimelineOptions{
		RunID:      runID,
		Overwrite:  opts.Overwrite,
		PreferGoal: opts.GoalAware,
	}
	if err := CreativeTimeline(planID, io.Discard, tlOpts); err != nil {
		return fmt.Errorf("creative-timeline failed: %w", err)
	}

	// Verify clips were found
	clipCount, err := countTimelineClips(planID)
	if err != nil || clipCount == 0 {
		return fmt.Errorf("creative-timeline produced 0 clips; check run artifacts with: byom-video inspect %s", runID)
	}

	if err := CreativeRenderPlan(planID, io.Discard, CreativeRenderPlanOptions{Overwrite: opts.Overwrite}); err != nil {
		return fmt.Errorf("creative-render-plan failed: %w", err)
	}

	fmt.Fprintf(stdout, "  step 6/6:    assemble draft video...\n")

	assembleOpts := CreativeAssembleOptions{
		Overwrite:             opts.Overwrite,
		Mode:                  opts.Mode,
		KeepWork:              opts.KeepWork,
		BurnCaptions:          opts.BurnCaptions,
		AllowMissingCaptions:  opts.AllowMissingCaptions,
		MixVoiceover:          opts.MixVoiceover,
		VoiceoverPath:         opts.VoiceoverPath,
		AllowMissingVoiceover: opts.AllowMissingVoiceover,
		RunID:                 runID,
		Platform:              opts.Platform,
		Fit:                   opts.Fit,
		Background:            opts.Background,
		CaptionPosition:       opts.CaptionPosition,
		CaptionMargin:         opts.CaptionMargin,
		CaptionStyle:          opts.CaptionStyle,
	}
	if err := CreativeAssemble(planID, io.Discard, assembleOpts); err != nil {
		return fmt.Errorf("creative-assemble failed: %w", err)
	}

	// read assemble result for summary
	draftPath, captStatus, voStatus, platSummary, assembleWarnings := readAssembleResult(planID)
	summary.AssembleStatus = "completed"
	summary.CaptionStatus = captStatus
	summary.VoiceoverStatus = voStatus
	summary.DraftPath = draftPath
	if platSummary != nil {
		summary.PlatformPreset = platSummary.Normalized
		summary.PlatformWidth = platSummary.Width
		summary.PlatformHeight = platSummary.Height
		summary.PlatformFit = platSummary.Fit
		summary.PlatformStatus = platSummary.Status
	}
	if captSummary := readAssembleCaptionsResult(planID); captSummary != nil && captSummary.Status == "applied" {
		summary.CaptionPosition = captSummary.Position
		summary.CaptionMargin = captSummary.Margin
		summary.CaptionStyle = captSummary.Style
	}
	summary.Warnings = append(summary.Warnings, assembleWarnings...)

	valErr := ValidateCreativeAssemble(planID, io.Discard, ValidateCreativeAssembleOptions{})
	if valErr != nil {
		summary.ValidationStatus = "failed"
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("validate-creative-assemble: %v", valErr))
	} else {
		summary.ValidationStatus = "ok"
	}
	_ = CreativeResult(planID, io.Discard, CreativeResultOptions{WriteArtifact: true})

	// Probe draft.mp4 if ffprobe available
	if draftPath != "" {
		if probe, err := probeDraft(draftPath); err == nil {
			summary.DraftProbe = probe
		}
		if w, h, probeErr := probeVideoDimensions(draftPath); probeErr == nil && w > 0 {
			summary.FinalWidth = w
			summary.FinalHeight = h
		}
	}

	nextCmds := buildNextCommands(runID, planID, draftPath)
	summary.Status = "completed"
	summary.NextCommands = nextCmds
	if err := writeMakeSummary(makeID, summary); err != nil {
		fmt.Fprintf(stdout, "  warning:     could not write make_summary.json: %v\n", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	printMakeResult(stdout, summary)
	return nil
}

// ---- Makes listing ----

func Makes(stdout io.Writer, opts MakesOptions) error {
	entries, err := os.ReadDir(makesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "No makes found. Run `byom-video make <video> --goal <text>` first.")
			return nil
		}
		return fmt.Errorf("list makes: %w", err)
	}

	type makeRow struct {
		MakeID    string
		Status    string
		Preset    string
		Goal      string
		CreatedAt time.Time
		RunID     string
		PlanID    string
		HasDraft  bool
	}
	var rows []makeRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		row := makeRow{MakeID: e.Name()}
		data, err := os.ReadFile(filepath.Join(makesRoot, e.Name(), "make_summary.json"))
		if err == nil {
			var s MakeSummary
			if json.Unmarshal(data, &s) == nil {
				row.Status = s.Status
				row.Preset = s.Preset
				row.Goal = s.Goal
				row.CreatedAt = s.CreatedAt
				row.RunID = s.RunID
				row.PlanID = s.CreativePlanID
				row.HasDraft = s.DraftPath != ""
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No makes found. Run `byom-video make <video> --goal <text>` first.")
		return nil
	}
	fmt.Fprintf(stdout, "%-34s %-10s %-8s %-6s %-35s\n", "MAKE ID", "STATUS", "PRESET", "DRAFT", "GOAL")
	for _, row := range rows {
		goal := row.Goal
		if len(goal) > 35 {
			goal = goal[:32] + "..."
		}
		draft := "no"
		if row.HasDraft {
			draft = "yes"
		}
		preset := row.Preset
		if preset == "" {
			preset = "shorts"
		}
		fmt.Fprintf(stdout, "%-34s %-10s %-8s %-6s %-35s\n", row.MakeID, row.Status, preset, draft, goal)
	}
	return nil
}

// ---- InspectMake ----

func InspectMake(makeID string, stdout io.Writer, opts InspectMakeOptions) error {
	summaryPath := filepath.Join(makesRoot, makeID, "make_summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("make %q not found: %w", makeID, err)
	}
	var summary MakeSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("make_summary.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Fprintf(stdout, "Make summary (technical)\n")
	fmt.Fprintf(stdout, "  make_id:          %s\n", summary.MakeID)
	fmt.Fprintf(stdout, "  schema_version:   %s\n", summary.SchemaVersion)
	fmt.Fprintf(stdout, "  status:           %s\n", summary.Status)
	fmt.Fprintf(stdout, "  preset:           %s\n", summary.Preset)
	fmt.Fprintf(stdout, "  goal:             %s\n", summary.Goal)
	fmt.Fprintf(stdout, "  input:            %s\n", summary.InputPath)
	fmt.Fprintf(stdout, "  created_at:       %s\n", summary.CreatedAt.Format(time.RFC3339))
	if summary.SkipPipeline {
		fmt.Fprintf(stdout, "  skip_pipeline:    true\n")
		fmt.Fprintf(stdout, "  reused_run_id:    %s\n", summary.ReusedRunID)
	}
	if summary.InputWarning != "" {
		fmt.Fprintf(stdout, "  input_warning:    %s\n", summary.InputWarning)
	}
	if summary.RunID != "" {
		fmt.Fprintf(stdout, "  run_id:           %s\n", summary.RunID)
	}
	if summary.CreativePlanID != "" {
		fmt.Fprintf(stdout, "  plan_id:          %s\n", summary.CreativePlanID)
	}
	if summary.PipelineStatus != "" {
		fmt.Fprintf(stdout, "  pipeline_status:  %s\n", summary.PipelineStatus)
	}
	if summary.CreativeStatus != "" {
		fmt.Fprintf(stdout, "  creative_status:  %s\n", summary.CreativeStatus)
	}
	if summary.ScriptStatus != "" {
		fmt.Fprintf(stdout, "  script_status:    %s\n", summary.ScriptStatus)
	}
	if summary.ScriptModel != "" {
		fmt.Fprintf(stdout, "  script_model:     %s/%s\n", summary.ScriptMode, summary.ScriptModel)
	}
	if summary.StyleUsed {
		fmt.Fprintf(stdout, "  style_used:       true\n")
	}
	if summary.CaptionVariantsStatus != "" {
		fmt.Fprintf(stdout, "  captions_gen:     %s\n", summary.CaptionVariantsStatus)
	}
	if summary.CaptionVariantsCount > 0 {
		fmt.Fprintf(stdout, "  captions_count:   %d\n", summary.CaptionVariantsCount)
	}
	if summary.CaptionVariantsModel != "" {
		fmt.Fprintf(stdout, "  captions_model:   %s/%s\n", summary.CaptionVariantsMode, summary.CaptionVariantsModel)
	}
	if summary.VoiceoverTextStatus != "" {
		fmt.Fprintf(stdout, "  vo_text_status:   %s\n", summary.VoiceoverTextStatus)
	}
	if summary.VoiceoverTextWordCount > 0 {
		fmt.Fprintf(stdout, "  vo_text_words:    %d\n", summary.VoiceoverTextWordCount)
	}
	if summary.VoiceoverAudioStatus != "" {
		fmt.Fprintf(stdout, "  vo_audio_status:  %s\n", summary.VoiceoverAudioStatus)
	}
	if summary.VoiceoverAudioPath != "" {
		fmt.Fprintf(stdout, "  vo_audio_path:    %s\n", summary.VoiceoverAudioPath)
	}
	if summary.PlatformPreset != "" {
		fmt.Fprintf(stdout, "  platform:         %s\n", summary.PlatformPreset)
		if summary.PlatformWidth > 0 {
			fmt.Fprintf(stdout, "  platform_dims:    %dx%d (fit=%s, %s)\n",
				summary.PlatformWidth, summary.PlatformHeight, summary.PlatformFit, summary.PlatformStatus)
		}
	}
	if summary.FinalWidth > 0 {
		fmt.Fprintf(stdout, "  final_dims:       %dx%d\n", summary.FinalWidth, summary.FinalHeight)
	}
	if summary.AssembleStatus != "" {
		fmt.Fprintf(stdout, "  assemble_status:  %s\n", summary.AssembleStatus)
	}
	if summary.ValidationStatus != "" {
		fmt.Fprintf(stdout, "  validation:       %s\n", summary.ValidationStatus)
	}
	if summary.ExportStatus != "" {
		fmt.Fprintf(stdout, "  export_status:    %s\n", summary.ExportStatus)
	}
	for _, f := range summary.ExportedFiles {
		fmt.Fprintf(stdout, "  exported:         %s\n", f)
	}
	if summary.CaptionStatus != "" {
		fmt.Fprintf(stdout, "  captions:         %s\n", summary.CaptionStatus)
	}
	if summary.VoiceoverStatus != "" {
		fmt.Fprintf(stdout, "  voiceover:        %s\n", summary.VoiceoverStatus)
	}
	if summary.DraftPath != "" {
		fmt.Fprintf(stdout, "  draft:            %s\n", summary.DraftPath)
	}
	if summary.DraftProbe != nil {
		fmt.Fprintf(stdout, "  draft_duration:   %.2fs\n", summary.DraftProbe.DurationSeconds)
		fmt.Fprintf(stdout, "  video_streams:    %d\n", summary.DraftProbe.VideoStreamCount)
		fmt.Fprintf(stdout, "  audio_streams:    %d\n", summary.DraftProbe.AudioStreamCount)
	}
	for _, w := range summary.Warnings {
		fmt.Fprintf(stdout, "  warning:          %s\n", w)
	}
	if summary.RevisionCount > 0 {
		fmt.Fprintf(stdout, "  revision_count:   %d\n", summary.RevisionCount)
		fmt.Fprintf(stdout, "  latest_revision:  %s (%s)\n", summary.LatestRevisionID, summary.RevisionStatus)
	}
	if len(summary.NextCommands) > 0 {
		fmt.Fprintf(stdout, "next:\n")
		for _, cmd := range summary.NextCommands {
			fmt.Fprintf(stdout, "  %s\n", cmd)
		}
	}
	return nil
}

// ---- MakeResult command (user-facing) ----

func MakeResult(makeID string, stdout io.Writer, opts MakeResultOptions) error {
	summaryPath := filepath.Join(makesRoot, makeID, "make_summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("make %q not found: %w", makeID, err)
	}
	var summary MakeSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("make_summary.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	text := renderMakeResultMD(summary)
	fmt.Fprint(stdout, text)

	if opts.WriteArtifact {
		makeDir := filepath.Join(makesRoot, makeID)
		if err := os.MkdirAll(makeDir, 0o755); err != nil {
			return fmt.Errorf("create make dir: %w", err)
		}
		mdPath := filepath.Join(makeDir, "make_result.md")
		if err := os.WriteFile(mdPath, []byte(text), 0o644); err != nil {
			return fmt.Errorf("write make_result.md: %w", err)
		}
		fmt.Fprintf(stdout, "\nWritten: %s\n", mdPath)
	}
	return nil
}

func renderMakeResultMD(s MakeSummary) string {
	var b strings.Builder
	b.WriteString("# Make Result\n\n")
	fmt.Fprintf(&b, "- **Make ID:** %s\n", s.MakeID)
	fmt.Fprintf(&b, "- **Status:** %s\n", s.Status)
	fmt.Fprintf(&b, "- **Goal:** %s\n", s.Goal)
	fmt.Fprintf(&b, "- **Preset:** %s\n", s.Preset)
	fmt.Fprintf(&b, "- **Created:** %s\n", s.CreatedAt.Format(time.RFC3339))
	if s.RunID != "" {
		fmt.Fprintf(&b, "- **Run ID:** %s\n", s.RunID)
	}
	if s.CreativePlanID != "" {
		fmt.Fprintf(&b, "- **Creative Plan ID:** %s\n", s.CreativePlanID)
	}
	if s.DraftPath != "" {
		fmt.Fprintf(&b, "- **Draft:** %s\n", s.DraftPath)
	}
	if s.DraftProbe != nil {
		fmt.Fprintf(&b, "- **Draft Duration:** %.2fs\n", s.DraftProbe.DurationSeconds)
		fmt.Fprintf(&b, "- **Video Streams:** %d\n", s.DraftProbe.VideoStreamCount)
		fmt.Fprintf(&b, "- **Audio Streams:** %d\n", s.DraftProbe.AudioStreamCount)
	}
	if s.PlatformPreset != "" && s.PlatformPreset != "original" {
		fmt.Fprintf(&b, "- **Platform:** %s (%dx%d, fit=%s, %s)\n",
			s.PlatformPreset, s.PlatformWidth, s.PlatformHeight, s.PlatformFit, s.PlatformStatus)
	}
	if s.FinalWidth > 0 {
		fmt.Fprintf(&b, "- **Final Dimensions:** %dx%d\n", s.FinalWidth, s.FinalHeight)
	}
	if s.ValidationStatus != "" {
		fmt.Fprintf(&b, "- **Validation:** %s\n", s.ValidationStatus)
	}
	if s.ExportStatus != "" {
		fmt.Fprintf(&b, "- **Export Status:** %s\n", s.ExportStatus)
	}
	for _, f := range s.ExportedFiles {
		fmt.Fprintf(&b, "  - %s\n", f)
	}
	if s.CaptionStatus != "" {
		fmt.Fprintf(&b, "- **Captions:** %s", s.CaptionStatus)
		if s.CaptionPosition != "" {
			fmt.Fprintf(&b, " (pos=%s, margin=%d, style=%s)", s.CaptionPosition, s.CaptionMargin, s.CaptionStyle)
		}
		b.WriteString("\n")
	}
	if s.VoiceoverStatus != "" {
		fmt.Fprintf(&b, "- **Voiceover:** %s\n", s.VoiceoverStatus)
	}
	if s.RevisionCount > 0 {
		fmt.Fprintf(&b, "- **Revisions:** %d (latest: %s, %s)\n", s.RevisionCount, s.LatestRevisionID, s.RevisionStatus)
		fmt.Fprintf(&b, "  - `byom-video inspect-make-revision %s %s`\n", s.MakeID, s.LatestRevisionID)
	}
	if len(s.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range s.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	if len(s.NextCommands) > 0 {
		b.WriteString("\n## Next Steps\n\n")
		for _, cmd := range s.NextCommands {
			fmt.Fprintf(&b, "```\n%s\n```\n", cmd)
		}
	}
	return b.String()
}

// ---- internal helpers ----

func validateSkipPipelineRun(runID, inputPath string, strictInput bool) (string, string, string, error) {
	runDir := filepath.Join(".byom-video", "runs", runID)
	if _, err := os.Stat(runDir); err != nil {
		return "", "", "", fmt.Errorf("run %q not found at %s; run pipeline first", runID, runDir)
	}

	// Check for usable clip source
	clipSources := []string{
		"selected_clips.json",
		"goal_roughcut.json",
		"enhanced_roughcut.json",
		"roughcut.json",
	}
	found := false
	for _, name := range clipSources {
		if _, err := os.Stat(filepath.Join(runDir, name)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return "", "", "", fmt.Errorf("run %s has no usable clip source; run `byom-video pipeline --preset shorts` or `byom-video selected-clips %s` first", runID, runID)
	}

	// Read manifest to get input_path
	var manifestInputPath string
	manifestData, err := os.ReadFile(filepath.Join(runDir, "manifest.json"))
	if err == nil {
		var m struct {
			InputPath string `json:"input_path"`
		}
		if json.Unmarshal(manifestData, &m) == nil {
			manifestInputPath = m.InputPath
		}
	}

	// Check input path match
	var inputWarning string
	if inputPath != "" && manifestInputPath != "" && inputPath != manifestInputPath {
		msg := fmt.Sprintf("input path %q differs from run manifest input_path %q", inputPath, manifestInputPath)
		if strictInput {
			return "", "", "", fmt.Errorf("%s (use --skip-pipeline without input or remove --strict-input)", msg)
		}
		inputWarning = msg
	}

	// Use manifest input_path if no input was provided
	resolvedInput := manifestInputPath
	if inputPath != "" {
		resolvedInput = inputPath
	}

	return runID, resolvedInput, inputWarning, nil
}

func runExport(runID string, stdout io.Writer, requireExport bool) (status string, files []string) {
	fmt.Fprintf(stdout, "  export:      running export for %s...\n", runID)
	summary, err := exporter.Run(runID, io.Discard)
	if err != nil {
		msg := fmt.Sprintf("export failed: %v", err)
		fmt.Fprintf(stdout, "  warning:     %s\n", msg)
		return "failed", nil
	}
	fmt.Fprintf(stdout, "  export:      %d file(s) exported to %s\n", len(summary.ExportedFiles), summary.ExportsDir)
	return "completed", summary.ExportedFiles
}

func probeDraft(draftPath string) (*DraftProbeInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		draftPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Format  struct{ Duration string } `json:"format"`
		Streams []struct{ CodecType string } `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	dur, _ := strconv.ParseFloat(result.Format.Duration, 64)
	info := &DraftProbeInfo{DurationSeconds: dur}
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			info.VideoStreamCount++
		case "audio":
			info.AudioStreamCount++
		}
	}
	return info, nil
}

func printMakeDryRun(stdout io.Writer, absInput, preset string, opts MakeOptions) {
	label := "(no input)"
	if absInput != "" {
		label = filepath.Base(absInput)
	}
	fmt.Fprintf(stdout, "make dry-run: %s\n", label)
	fmt.Fprintf(stdout, "  goal:     %s\n", opts.Goal)
	fmt.Fprintf(stdout, "  preset:   %s\n", preset)
	if opts.SkipPipeline != "" {
		fmt.Fprintf(stdout, "  skip-pipeline: %s\n", opts.SkipPipeline)
	}
	fmt.Fprintf(stdout, "\nplanned stages:\n")
	if opts.SkipPipeline != "" {
		fmt.Fprintf(stdout, "  1. reuse run %s (skip pipeline)\n", opts.SkipPipeline)
	} else {
		fmt.Fprintf(stdout, "  1. byom-video pipeline <input> --preset %s\n", preset)
	}
	if opts.Export {
		fmt.Fprintf(stdout, "     byom-video export <run_id>\n")
	}
	if opts.GoalAware {
		fmt.Fprintf(stdout, "  2. byom-video goal-rerank <run_id> --goal %q\n", opts.Goal)
		fmt.Fprintf(stdout, "     byom-video goal-roughcut <run_id>\n")
	} else {
		fmt.Fprintf(stdout, "  2. (goal-rerank skipped; use --goal-aware to enable)\n")
	}
	fmt.Fprintf(stdout, "  3. byom-video creative-plan <input> --goal %q\n", opts.Goal)
	if opts.Platform != "" && opts.Platform != "original" {
		norm, _ := NormalizePlatform(opts.Platform)
		p := LookupPlatform(norm)
		fit := opts.Fit
		if fit == "" {
			fit = DefaultFitForPlatform(norm)
		}
		fmt.Fprintf(stdout, "  platform: %s (%dx%d, fit=%s)\n", norm, p.Width, p.Height, fit)
	}
	if opts.Yes {
		fmt.Fprintf(stdout, "  4. byom-video approve-creative-plan <plan_id>\n")
		fmt.Fprintf(stdout, "     byom-video creative-execute-stub <plan_id>\n")
		if opts.GenerateScript {
			fmt.Fprintf(stdout, "  4b. byom-video creative-generate-script <plan_id>")
			if opts.ScriptFallbackStub {
				fmt.Fprintf(stdout, " --fallback-stub")
			}
			fmt.Fprintln(stdout)
		}
		if opts.GenerateCaptions {
			fmt.Fprintf(stdout, "  4c. byom-video creative-caption-variants <plan_id>")
			if opts.CaptionFallbackStub {
				fmt.Fprintf(stdout, " --fallback-stub")
			}
			if opts.CaptionCount > 0 {
				fmt.Fprintf(stdout, " --count %d", opts.CaptionCount)
			}
			fmt.Fprintln(stdout)
		}
		if opts.PrepareVoiceover {
			fmt.Fprintf(stdout, "  4d. byom-video creative-voiceover-text <plan_id>")
			if opts.VoiceoverMaxWords > 0 {
				fmt.Fprintf(stdout, " --max-words %d", opts.VoiceoverMaxWords)
			}
			fmt.Fprintln(stdout)
			fmt.Fprintf(stdout, "      # Record/generate audio → outputs/voiceover.wav\n")
		}
		if opts.GenerateVoiceover {
			fmt.Fprintf(stdout, "  4e. byom-video creative-generate-voiceover <plan_id>")
			if opts.VoiceoverRoute != "" {
				fmt.Fprintf(stdout, " --route %s", opts.VoiceoverRoute)
			}
			if opts.VoiceoverVoiceID != "" {
				fmt.Fprintf(stdout, " --voice-id %s", opts.VoiceoverVoiceID)
			}
			if opts.VoiceoverDryRun {
				fmt.Fprintf(stdout, " --dry-run")
			}
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "  5. byom-video creative-timeline <plan_id> --run-id <run_id>\n")
		fmt.Fprintf(stdout, "     byom-video creative-render-plan <plan_id>\n")
		assembleFlags := ""
		if opts.BurnCaptions {
			assembleFlags += " --burn-captions"
		}
		if opts.AllowMissingCaptions {
			assembleFlags += " --allow-missing-captions"
		}
		if opts.MixVoiceover {
			assembleFlags += " --mix-voiceover"
		}
		if opts.VoiceoverPath != "" {
			assembleFlags += " --voiceover " + opts.VoiceoverPath
		}
		if opts.Platform != "" && opts.Platform != "original" {
			assembleFlags += " --platform " + opts.Platform
		}
		if opts.Fit != "" {
			assembleFlags += " --fit " + opts.Fit
		}
		if opts.Background != "" {
			assembleFlags += " --background " + opts.Background
		}
		if opts.CaptionPosition != "" {
			assembleFlags += " --caption-position " + opts.CaptionPosition
		}
		if opts.CaptionMargin > 0 {
			assembleFlags += fmt.Sprintf(" --caption-margin %d", opts.CaptionMargin)
		}
		if opts.CaptionStyle != "" {
			assembleFlags += " --caption-style " + opts.CaptionStyle
		}
		fmt.Fprintf(stdout, "  6. byom-video creative-assemble <plan_id>%s\n", assembleFlags)
	} else {
		fmt.Fprintf(stdout, "  (plan only; add --yes to continue to execution)\n")
	}
	fmt.Fprintln(stdout, "\nno files written (dry-run)")
}

func printMakeResult(stdout io.Writer, s MakeSummary) {
	fmt.Fprintf(stdout, "\nmake complete\n")
	fmt.Fprintf(stdout, "  input:       %s\n", s.InputPath)
	fmt.Fprintf(stdout, "  run_id:      %s\n", s.RunID)
	fmt.Fprintf(stdout, "  plan_id:     %s\n", s.CreativePlanID)
	if s.DraftPath != "" {
		fmt.Fprintf(stdout, "  draft:       %s\n", s.DraftPath)
	}
	if s.DraftProbe != nil {
		fmt.Fprintf(stdout, "  duration:    %.2fs\n", s.DraftProbe.DurationSeconds)
	}
	if s.PlatformPreset != "" && s.PlatformPreset != "original" {
		fmt.Fprintf(stdout, "  platform:    %s (%dx%d, fit=%s, %s)\n",
			s.PlatformPreset, s.PlatformWidth, s.PlatformHeight, s.PlatformFit, s.PlatformStatus)
	}
	if s.FinalWidth > 0 {
		fmt.Fprintf(stdout, "  final dims:  %dx%d\n", s.FinalWidth, s.FinalHeight)
	}
	if s.ScriptStatus != "" {
		fmt.Fprintf(stdout, "  script:      %s", s.ScriptStatus)
		if s.ScriptModel != "" {
			fmt.Fprintf(stdout, " (%s/%s)", s.ScriptMode, s.ScriptModel)
		}
		if s.StyleUsed {
			fmt.Fprintf(stdout, " +style")
		}
		fmt.Fprintln(stdout)
	}
	if s.CaptionVariantsStatus != "" {
		fmt.Fprintf(stdout, "  captions_gen: %s", s.CaptionVariantsStatus)
		if s.CaptionVariantsCount > 0 {
			fmt.Fprintf(stdout, " (%d variants", s.CaptionVariantsCount)
			if s.CaptionVariantsModel != "" {
				fmt.Fprintf(stdout, ", %s/%s", s.CaptionVariantsMode, s.CaptionVariantsModel)
			}
			fmt.Fprintf(stdout, ")")
		}
		fmt.Fprintln(stdout)
	}
	if s.VoiceoverTextStatus != "" {
		fmt.Fprintf(stdout, "  voiceover_text: %s", s.VoiceoverTextStatus)
		if s.VoiceoverTextWordCount > 0 {
			fmt.Fprintf(stdout, " (%d words)", s.VoiceoverTextWordCount)
		}
		fmt.Fprintln(stdout)
	}
	if s.VoiceoverAudioStatus != "" {
		fmt.Fprintf(stdout, "  voiceover_audio: %s", s.VoiceoverAudioStatus)
		if s.VoiceoverAudioPath != "" {
			fmt.Fprintf(stdout, " (%s)", s.VoiceoverAudioPath)
		}
		fmt.Fprintln(stdout)
	}
	if s.CaptionStatus != "" {
		fmt.Fprintf(stdout, "  captions:    %s", s.CaptionStatus)
		if s.CaptionPosition != "" {
			fmt.Fprintf(stdout, " (pos=%s, margin=%d, style=%s)", s.CaptionPosition, s.CaptionMargin, s.CaptionStyle)
		}
		fmt.Fprintln(stdout)
	}
	if s.VoiceoverStatus != "" {
		fmt.Fprintf(stdout, "  voiceover:   %s\n", s.VoiceoverStatus)
	}
	if s.ValidationStatus != "" {
		fmt.Fprintf(stdout, "  validation:  %s\n", s.ValidationStatus)
	}
	if s.ExportStatus != "" {
		fmt.Fprintf(stdout, "  export:      %s\n", s.ExportStatus)
	}
	for _, f := range s.ExportedFiles {
		fmt.Fprintf(stdout, "    - %s\n", f)
	}
	for _, w := range s.Warnings {
		fmt.Fprintf(stdout, "  warning:     %s\n", w)
	}
	if s.RevisionCount > 0 {
		fmt.Fprintf(stdout, "  revisions:   %d (latest: %s, %s)\n", s.RevisionCount, s.LatestRevisionID, s.RevisionStatus)
		fmt.Fprintf(stdout, "               byom-video inspect-make-revision %s %s\n", s.MakeID, s.LatestRevisionID)
	}
	fmt.Fprintf(stdout, "\nnext:\n")
	for _, cmd := range s.NextCommands {
		fmt.Fprintf(stdout, "  %s\n", cmd)
	}
}

func writeMakeSummary(makeID string, summary MakeSummary) error {
	makeDir := filepath.Join(makesRoot, makeID)
	if err := os.MkdirAll(makeDir, 0o755); err != nil {
		return fmt.Errorf("create make dir: %w", err)
	}
	return writeJSONFile(filepath.Join(makeDir, "make_summary.json"), summary)
}

func listRunIDs() (map[string]bool, error) {
	rows, err := runinfo.ListRuns(runinfo.RunListOptions{All: true})
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(rows))
	for _, r := range rows {
		m[r.RunID] = true
	}
	return m, nil
}

func newestRunID(before map[string]bool) (string, error) {
	rows, err := runinfo.ListRuns(runinfo.RunListOptions{All: true})
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if !before[r.RunID] {
			return r.RunID, nil
		}
	}
	// fallback: return newest run even if in before set (pipeline updated it)
	if len(rows) > 0 {
		return rows[0].RunID, nil
	}
	return "", fmt.Errorf("no runs found after pipeline")
}

func listPlanIDs() (map[string]bool, error) {
	entries, err := os.ReadDir(creativePlansRoot)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			m[e.Name()] = true
		}
	}
	return m, nil
}

func newestPlanID(before map[string]bool) (string, error) {
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil {
		return "", fmt.Errorf("list creative plans: %w", err)
	}
	// entries are sorted alphabetically; IDs are timestamp-prefixed so newest = last
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.IsDir() && !before[e.Name()] {
			return e.Name(), nil
		}
	}
	if len(entries) > 0 {
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].IsDir() {
				return entries[i].Name(), nil
			}
		}
	}
	return "", fmt.Errorf("no creative plans found after planning")
}

func countTimelineClips(planID string) (int, error) {
	tlPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_timeline.json")
	data, err := os.ReadFile(tlPath)
	if err != nil {
		return 0, err
	}
	var tl CreativeTimelineArtifact
	if err := json.Unmarshal(data, &tl); err != nil {
		return 0, err
	}
	count := 0
	for _, track := range tl.Tracks {
		if track.ID == "track_video_main" {
			for _, item := range track.Items {
				if item.Kind == "source_clip" {
					count++
				}
			}
		}
	}
	return count, nil
}

func readVoiceGenerationOutput(planID string) (*VoiceGenerationOutput, error) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "voiceover_generation.json"))
	if err != nil {
		return nil, err
	}
	var out VoiceGenerationOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func readCaptionVariantsOutput(planID string) (*CaptionVariantsOutput, error) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "caption_variants.json"))
	if err != nil {
		return nil, err
	}
	var out CaptionVariantsOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func readScriptOutput(planID string) (*CreativeScriptOutput, error) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "script_draft.json"))
	if err != nil {
		return nil, err
	}
	var out CreativeScriptOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func readAssembleResult(planID string) (draftPath, captStatus, voStatus string, plat *AssemblePlatformResult, warnings []string) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	if err != nil {
		return "", "", "", nil, nil
	}
	var result CreativeAssembleResult
	if json.Unmarshal(data, &result) != nil {
		return "", "", "", nil, nil
	}
	draftAbs := filepath.Join(creativePlansRoot, planID, result.FinalOutputFile)
	if result.Captions != nil {
		captStatus = result.Captions.Status
	}
	if result.Voiceover != nil {
		voStatus = result.Voiceover.Status
	}
	return draftAbs, captStatus, voStatus, result.Platform, result.Warnings
}

func readAssembleCaptionsResult(planID string) *AssembleCaptionsResult {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	if err != nil {
		return nil
	}
	var result CreativeAssembleResult
	if json.Unmarshal(data, &result) != nil {
		return nil
	}
	return result.Captions
}

func buildNextCommands(runID, planID, draftPath string) []string {
	cmds := []string{
		fmt.Sprintf("byom-video inspect %s", runID),
		fmt.Sprintf("byom-video inspect-creative-plan %s", planID),
		fmt.Sprintf("byom-video open-report %s", runID),
	}
	if draftPath != "" {
		cmds = append(cmds,
			fmt.Sprintf("byom-video review-creative-assemble %s", planID),
			fmt.Sprintf("byom-video validate-creative-assemble %s", planID),
		)
	} else {
		cmds = append(cmds,
			fmt.Sprintf("byom-video approve-creative-plan %s", planID),
			fmt.Sprintf("byom-video make %s --goal <goal> --yes", filepath.Base("<input>")),
		)
	}
	return cmds
}

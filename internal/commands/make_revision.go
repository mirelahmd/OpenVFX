package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const makeRevisionsDir = "revisions"

// ---- schema types ----

type RevisionSummary struct {
	SchemaVersion  string          `json:"schema_version"`
	RevisionID     string          `json:"revision_id"`
	CreatedAt      time.Time       `json:"created_at"`
	MakeID         string          `json:"make_id"`
	Request        string          `json:"request"`
	Status         string          `json:"status"` // planned|completed|failed
	RunID          string          `json:"run_id,omitempty"`
	CreativePlanID string          `json:"creative_plan_id,omitempty"`
	PlannedActions []PlannedAction `json:"planned_actions"`
	Outputs        RevisionOutputs `json:"outputs"`
	Warnings       []string        `json:"warnings"`
	Errors         []string        `json:"errors"`
	NextCommands   []string        `json:"next_commands"`
}

type PlannedAction struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Status           string            `json:"status"` // planned|completed|failed|skipped
	Description      string            `json:"description"`
	RequiresProvider bool              `json:"requires_provider"`
	RequiresApproval bool              `json:"requires_approval"`
	Params           map[string]string `json:"params,omitempty"`
	Error            string            `json:"error,omitempty"`
}

type RevisionOutputs struct {
	DraftPath           string `json:"draft_path,omitempty"`
	ScriptPath          string `json:"script_path,omitempty"`
	CaptionVariantsPath string `json:"caption_variants_path,omitempty"`
	VoiceoverTextPath   string `json:"voiceover_text_path,omitempty"`
	VoiceoverAudioPath  string `json:"voiceover_audio_path,omitempty"`
}

// ---- options ----

type ReviseMakeOptions struct {
	Request            string
	DryRun             bool
	JSON               bool
	Yes                bool
	Overwrite          bool
	NewMake            bool
	AllowProviderCalls bool
	FallbackStub       bool
	Reassemble         bool
	Validate           bool
}

type MakeRevisionsOptions struct{ JSON bool }

type InspectMakeRevisionOptions struct{ JSON bool }

type ReviewMakeRevisionOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- injectable deps (for testability) ----

type revisionDeps struct {
	assemble             func(planID string, stdout io.Writer, opts CreativeAssembleOptions) error
	validate             func(planID string, stdout io.Writer, opts ValidateCreativeAssembleOptions) error
	prepareVoiceoverText func(planID string, stdout io.Writer, opts VoiceoverTextOptions) error
	generateScript       func(planID string, stdout io.Writer, opts CreativeGenerateScriptOptions) error
	generateCaptions     func(planID string, stdout io.Writer, opts CaptionVariantsOptions) error
	generateVoiceover    func(planID string, stdout io.Writer, opts GenerateVoiceoverOptions) error
}

var defaultRevisionDeps = revisionDeps{
	assemble:             CreativeAssemble,
	validate:             ValidateCreativeAssemble,
	prepareVoiceoverText: VoiceoverTextCommand,
	generateScript:       CreativeGenerateScript,
	generateCaptions:     CaptionVariants,
	generateVoiceover:    GenerateVoiceover,
}

// ---- revise-make ----

func ReviseMake(makeID string, stdout io.Writer, opts ReviseMakeOptions) error {
	return reviseMakeWithDeps(makeID, stdout, opts, defaultRevisionDeps)
}

func reviseMakeWithDeps(makeID string, stdout io.Writer, opts ReviseMakeOptions, deps revisionDeps) error {
	if strings.TrimSpace(opts.Request) == "" {
		return fmt.Errorf("--request is required")
	}

	// Load make summary
	summaryPath := filepath.Join(makesRoot, makeID, "make_summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return fmt.Errorf("make %q not found: %w", makeID, err)
	}
	var summary MakeSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("make_summary.json is malformed: %w", err)
	}

	// Parse revision request
	parsed, err := ParseRevisionRequest(opts.Request)
	if err != nil {
		return err
	}

	// Build planned actions
	actions := buildPlannedActions(parsed, opts)

	// Check provider-call requirements. Block early only when neither allow-provider-calls
	// nor fallback-stub is set — voiceover generation never has a fallback stub.
	if !opts.AllowProviderCalls && !opts.FallbackStub {
		for _, a := range actions {
			if a.RequiresProvider {
				return fmt.Errorf("action %q requires a provider call; re-run with --allow-provider-calls to proceed\n  (use --fallback-stub for script/caption generation without a live provider)", a.Type)
			}
		}
	}
	// Voiceover generation always needs allow-provider-calls even with fallback-stub
	if !opts.AllowProviderCalls {
		for _, a := range actions {
			if a.Type == RevisionActionGenerateVoiceover {
				return fmt.Errorf("voiceover generation requires --allow-provider-calls")
			}
		}
	}

	// Dry-run: print planned actions, write nothing
	if opts.DryRun {
		printRevisionPlan(stdout, makeID, opts.Request, summary, actions)
		fmt.Fprintln(stdout, "\nno files written (dry-run)")
		return nil
	}

	// Determine revision ID
	revisionID, err := nextRevisionID(makeID)
	if err != nil {
		return fmt.Errorf("allocate revision id: %w", err)
	}
	revDir := filepath.Join(makesRoot, makeID, makeRevisionsDir, revisionID)
	if err := os.MkdirAll(revDir, 0o755); err != nil {
		return fmt.Errorf("create revision dir: %w", err)
	}

	// Snapshot before mutation
	if err := snapshotBeforeRevision(revDir, makeID, summary); err != nil {
		fmt.Fprintf(stdout, "  warning: snapshot failed: %v\n", err)
	}

	// Write request.txt
	_ = os.WriteFile(filepath.Join(revDir, "request.txt"), []byte(opts.Request+"\n"), 0o644)

	// Build initial revision summary
	revSummary := RevisionSummary{
		SchemaVersion:  "make_revision.v1",
		RevisionID:     revisionID,
		CreatedAt:      time.Now().UTC(),
		MakeID:         makeID,
		Request:        opts.Request,
		Status:         "planned",
		RunID:          summary.RunID,
		CreativePlanID: summary.CreativePlanID,
		PlannedActions: actions,
		Warnings:       []string{},
		Errors:         []string{},
		NextCommands:   []string{},
	}

	// Planned-only mode (no --yes): write artifact, print next command
	if !opts.Yes {
		revSummaryPath := filepath.Join(revDir, "revision_summary.json")
		if err := writeJSONFile(revSummaryPath, revSummary); err != nil {
			return fmt.Errorf("write revision_summary.json: %w", err)
		}
		printRevisionPlan(stdout, makeID, opts.Request, summary, actions)
		fmt.Fprintf(stdout, "\nrevision %s written (status: planned)\n", revisionID)
		nextCmd := fmt.Sprintf("byom-video revise-make %s --request %q --yes --overwrite", makeID, opts.Request)
		if opts.Reassemble {
			nextCmd += " --reassemble"
		}
		if opts.Validate {
			nextCmd += " --validate"
		}
		if opts.AllowProviderCalls {
			nextCmd += " --allow-provider-calls"
		}
		fmt.Fprintf(stdout, "\nnext:\n  %s\n", nextCmd)
		revSummary.NextCommands = []string{nextCmd}
		_ = writeJSONFile(revSummaryPath, revSummary)
		return nil
	}

	// Execution mode (--yes)
	fmt.Fprintf(stdout, "revise-make %s\n", makeID)
	fmt.Fprintf(stdout, "  request:   %s\n", opts.Request)
	fmt.Fprintf(stdout, "  revision:  %s\n", revisionID)
	fmt.Fprintln(stdout)

	// Accumulate state changes across actions
	state := &execRevisionState{
		Platform:        summary.PlatformPreset,
		CaptionPosition: summary.CaptionPosition,
		CaptionStyle:    summary.CaptionStyle,
		CaptionMargin:   summary.CaptionMargin,
		MixVoiceover:    summary.VoiceoverStatus == "applied",
		BurnCaptions:    summary.CaptionStatus == "applied",
	}

	for i := range actions {
		a := &actions[i]
		fmt.Fprintf(stdout, "  [%s] %s\n", a.Type, a.Description)
		err := executeRevisionAction(a, state, summary, stdout, opts, deps)
		if err != nil {
			a.Status = "failed"
			a.Error = err.Error()
			revSummary.Errors = append(revSummary.Errors, fmt.Sprintf("%s: %v", a.Type, err))
			fmt.Fprintf(stdout, "  ! failed: %v\n", err)
		} else {
			a.Status = "completed"
		}
	}

	// Determine overall status
	failed := 0
	for _, a := range actions {
		if a.Status == "failed" {
			failed++
		}
	}
	if failed == len(actions) {
		revSummary.Status = "failed"
	} else if failed > 0 {
		revSummary.Status = "completed"
		revSummary.Warnings = append(revSummary.Warnings, fmt.Sprintf("%d action(s) failed", failed))
	} else {
		revSummary.Status = "completed"
	}
	revSummary.PlannedActions = actions

	// Populate outputs
	planOutDir := ""
	if summary.CreativePlanID != "" {
		planOutDir = filepath.Join(creativePlansRoot, summary.CreativePlanID, "outputs")
	}
	if planOutDir != "" {
		revSummary.Outputs = collectRevisionOutputs(planOutDir)
	}

	// Write revision summary
	revSummaryPath := filepath.Join(revDir, "revision_summary.json")
	if err := writeJSONFile(revSummaryPath, revSummary); err != nil {
		fmt.Fprintf(stdout, "  warning: could not write revision_summary.json: %v\n", err)
	}

	// Update make_summary.json revision fields
	summary.LatestRevisionID = revisionID
	summary.RevisionCount = summary.RevisionCount + 1
	summary.RevisionStatus = revSummary.Status
	if err := writeMakeSummary(makeID, summary); err != nil {
		fmt.Fprintf(stdout, "  warning: could not update make_summary.json: %v\n", err)
	}

	// Print result
	fmt.Fprintf(stdout, "\nrevision %s: %s\n", revisionID, revSummary.Status)
	for _, w := range revSummary.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	for _, e := range revSummary.Errors {
		fmt.Fprintf(stdout, "  error:   %s\n", e)
	}
	fmt.Fprintf(stdout, "\nnext:\n")
	fmt.Fprintf(stdout, "  byom-video inspect-make-revision %s %s\n", makeID, revisionID)
	fmt.Fprintf(stdout, "  byom-video review-make-revision %s %s --write-artifact\n", makeID, revisionID)

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(revSummary)
	}
	return nil
}

// ---- execution state ----

type execRevisionState struct {
	Platform        string
	CaptionPosition string
	CaptionStyle    string
	CaptionMargin   int
	MixVoiceover    bool
	BurnCaptions    bool
}

// ---- execute a single action ----

func executeRevisionAction(
	action *PlannedAction,
	state *execRevisionState,
	summary MakeSummary,
	stdout io.Writer,
	opts ReviseMakeOptions,
	deps revisionDeps,
) error {
	planID := summary.CreativePlanID

	switch action.Type {
	case RevisionActionSetPlatform:
		state.Platform = action.Params["platform"]
		fmt.Fprintf(stdout, "    platform → %s\n", state.Platform)

	case RevisionActionSetCaptionPosition:
		state.CaptionPosition = action.Params["position"]
		fmt.Fprintf(stdout, "    caption-position → %s\n", state.CaptionPosition)

	case RevisionActionSetCaptionStyle:
		state.CaptionStyle = action.Params["style"]
		fmt.Fprintf(stdout, "    caption-style → %s\n", state.CaptionStyle)

	case RevisionActionReassemble, RevisionActionMixVoiceover:
		if action.Type == RevisionActionMixVoiceover {
			state.MixVoiceover = true
		}
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot reassemble")
		}
		assembleOpts := buildReassembleOpts(state, summary, opts.Overwrite)
		return deps.assemble(planID, stdout, assembleOpts)

	case RevisionActionValidate:
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot validate")
		}
		return deps.validate(planID, stdout, ValidateCreativeAssembleOptions{})

	case RevisionActionPrepareVoiceover:
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot prepare voiceover")
		}
		return deps.prepareVoiceoverText(planID, stdout, VoiceoverTextOptions{Overwrite: opts.Overwrite})

	case RevisionActionGenerateScript:
		if !opts.AllowProviderCalls && !opts.FallbackStub {
			return fmt.Errorf("script generation requires --allow-provider-calls or --fallback-stub")
		}
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot generate script")
		}
		tone := action.Params["tone"]
		return deps.generateScript(planID, stdout, CreativeGenerateScriptOptions{
			Overwrite:    opts.Overwrite,
			FallbackStub: opts.FallbackStub,
			Tone:         tone,
		})

	case RevisionActionGenerateCaptions:
		if !opts.AllowProviderCalls && !opts.FallbackStub {
			return fmt.Errorf("caption variant generation requires --allow-provider-calls or --fallback-stub")
		}
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot generate captions")
		}
		return deps.generateCaptions(planID, stdout, CaptionVariantsOptions{
			Overwrite:    opts.Overwrite,
			FallbackStub: opts.FallbackStub,
		})

	case RevisionActionGenerateVoiceover:
		if !opts.AllowProviderCalls {
			return fmt.Errorf("voiceover generation requires --allow-provider-calls")
		}
		if planID == "" {
			return fmt.Errorf("no creative_plan_id in make summary; cannot generate voiceover")
		}
		return deps.generateVoiceover(planID, stdout, GenerateVoiceoverOptions{
			Overwrite:       opts.Overwrite,
			Stability:       -1,
			SimilarityBoost: -1,
		})

	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
	return nil
}

// buildReassembleOpts constructs CreativeAssembleOptions from the current revision state
// merged with the original make's settings.
func buildReassembleOpts(state *execRevisionState, summary MakeSummary, overwrite bool) CreativeAssembleOptions {
	platform := state.Platform
	if platform == "" {
		platform = summary.PlatformPreset
	}

	captionPos := state.CaptionPosition
	if captionPos == "" {
		captionPos = summary.CaptionPosition
	}

	captionStyle := state.CaptionStyle
	if captionStyle == "" {
		captionStyle = summary.CaptionStyle
	}

	captionMargin := state.CaptionMargin
	if captionMargin == 0 {
		captionMargin = summary.CaptionMargin
	}

	return CreativeAssembleOptions{
		Overwrite:             overwrite,
		Platform:              platform,
		CaptionPosition:       captionPos,
		CaptionMargin:         captionMargin,
		CaptionStyle:          captionStyle,
		BurnCaptions:          state.BurnCaptions,
		AllowMissingCaptions:  true,
		MixVoiceover:          state.MixVoiceover,
		AllowMissingVoiceover: true,
		Mode:                  "reencode",
	}
}

// buildPlannedActions converts parsed actions + flags into the final action list.
func buildPlannedActions(parsed []ParsedRevisionAction, opts ReviseMakeOptions) []PlannedAction {
	var actions []PlannedAction
	for i, p := range parsed {
		actions = append(actions, PlannedAction{
			ID:               fmt.Sprintf("action_%04d", i+1),
			Type:             p.Type,
			Status:           "planned",
			Description:      p.Description,
			RequiresProvider: p.RequiresProvider,
			RequiresApproval: !opts.Yes,
			Params:           p.Params,
		})
	}

	// Append implicit reassemble when --reassemble and last action isn't already one
	needsReassemble := opts.Reassemble
	if needsReassemble && len(actions) > 0 {
		last := actions[len(actions)-1]
		if last.Type == RevisionActionReassemble || last.Type == RevisionActionMixVoiceover {
			needsReassemble = false
		}
	}
	if needsReassemble {
		actions = append(actions, PlannedAction{
			ID:          fmt.Sprintf("action_%04d", len(actions)+1),
			Type:        RevisionActionReassemble,
			Status:      "planned",
			Description: "Reassemble draft with updated settings",
		})
	}

	// Append validate when --validate
	if opts.Validate {
		actions = append(actions, PlannedAction{
			ID:          fmt.Sprintf("action_%04d", len(actions)+1),
			Type:        RevisionActionValidate,
			Status:      "planned",
			Description: "Validate assembled draft",
		})
	}

	return actions
}

// ---- snapshot ----

func snapshotBeforeRevision(revDir string, makeID string, summary MakeSummary) error {
	// Snapshot make_summary.json
	srcSummary := filepath.Join(makesRoot, makeID, "make_summary.json")
	if err := copyFileIfExists(srcSummary, filepath.Join(revDir, "before_make_summary.json")); err != nil {
		return err
	}

	if summary.CreativePlanID == "" {
		return nil
	}
	outDir := filepath.Join(creativePlansRoot, summary.CreativePlanID, "outputs")

	// Snapshot creative plan JSON artifacts (no MP4/audio)
	artifacts := []string{
		"creative_plan.json",
		"script_draft.json",
		"script_draft.txt",
		"caption_variants.json",
		"voiceover_text.json",
		"voiceover_text.txt",
		"creative_timeline.json",
		"creative_render_plan.json",
		"creative_assemble_result.json",
	}
	// creative_plan.json is in planDir, not outDir
	planDir := filepath.Join(creativePlansRoot, summary.CreativePlanID)
	for _, name := range artifacts {
		src := filepath.Join(outDir, name)
		if name == "creative_plan.json" {
			src = filepath.Join(planDir, name)
		}
		_ = copyFileIfExists(src, filepath.Join(revDir, "before_"+name))
	}
	return nil
}

func copyFileIfExists(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// ---- revision ID allocation ----

func nextRevisionID(makeID string) (string, error) {
	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, err := os.ReadDir(revBase)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	max := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "revision_") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(name, "revision_"))
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("revision_%04d", max+1), nil
}

// ---- collect outputs from plan dir ----

func collectRevisionOutputs(outDir string) RevisionOutputs {
	var out RevisionOutputs
	check := func(names ...string) string {
		for _, n := range names {
			p := filepath.Join(outDir, n)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return ""
	}
	out.DraftPath = check("draft.mp4")
	out.ScriptPath = check("script_draft.json", "script_draft.txt")
	out.CaptionVariantsPath = check("caption_variants.json")
	out.VoiceoverTextPath = check("voiceover_text.json", "voiceover_text.txt")
	out.VoiceoverAudioPath = check("voiceover.mp3", "voiceover.wav", "voiceover.m4a", "voiceover.aac")
	return out
}

// ---- print helpers ----

func printRevisionPlan(stdout io.Writer, makeID, request string, summary MakeSummary, actions []PlannedAction) {
	fmt.Fprintf(stdout, "revision plan\n")
	fmt.Fprintf(stdout, "  make_id:   %s\n", makeID)
	fmt.Fprintf(stdout, "  request:   %s\n", request)
	if summary.CreativePlanID != "" {
		fmt.Fprintf(stdout, "  plan_id:   %s\n", summary.CreativePlanID)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "planned actions:\n")
	for _, a := range actions {
		providerNote := ""
		if a.RequiresProvider {
			providerNote = " [requires-provider]"
		}
		fmt.Fprintf(stdout, "  [%s] %s%s\n", a.Type, a.Description, providerNote)
		for k, v := range a.Params {
			fmt.Fprintf(stdout, "    %s: %s\n", k, v)
		}
	}
}

// ---- make-revisions ----

func MakeRevisions(makeID string, stdout io.Writer, opts MakeRevisionsOptions) error {
	revBase := filepath.Join(makesRoot, makeID, makeRevisionsDir)
	entries, err := os.ReadDir(revBase)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stdout, "No revisions found for make %q.\n", makeID)
			fmt.Fprintf(stdout, "Run: byom-video revise-make %s --request \"...\"\n", makeID)
			return nil
		}
		return fmt.Errorf("list revisions: %w", err)
	}

	type row struct {
		RevisionID string
		Status     string
		Request    string
		CreatedAt  time.Time
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "revision_") {
			continue
		}
		r := row{RevisionID: e.Name()}
		data, err := os.ReadFile(filepath.Join(revBase, e.Name(), "revision_summary.json"))
		if err == nil {
			var rs RevisionSummary
			if json.Unmarshal(data, &rs) == nil {
				r.Status = rs.Status
				r.Request = rs.Request
				r.CreatedAt = rs.CreatedAt
			}
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.Before(rows[j].CreatedAt)
	})

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintf(stdout, "No revisions found for make %q.\n", makeID)
		return nil
	}
	fmt.Fprintf(stdout, "%-18s %-10s %s\n", "REVISION ID", "STATUS", "REQUEST")
	for _, r := range rows {
		req := r.Request
		if len(req) > 50 {
			req = req[:47] + "..."
		}
		fmt.Fprintf(stdout, "%-18s %-10s %s\n", r.RevisionID, r.Status, req)
	}
	return nil
}

// ---- inspect-make-revision ----

func InspectMakeRevision(makeID, revisionID string, stdout io.Writer, opts InspectMakeRevisionOptions) error {
	revPath := filepath.Join(makesRoot, makeID, makeRevisionsDir, revisionID, "revision_summary.json")
	data, err := os.ReadFile(revPath)
	if err != nil {
		return fmt.Errorf("revision %q not found for make %q: %w", revisionID, makeID, err)
	}
	var rs RevisionSummary
	if err := json.Unmarshal(data, &rs); err != nil {
		return fmt.Errorf("revision_summary.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rs)
	}

	fmt.Fprintf(stdout, "Make Revision\n")
	fmt.Fprintf(stdout, "  revision_id:  %s\n", rs.RevisionID)
	fmt.Fprintf(stdout, "  make_id:      %s\n", rs.MakeID)
	fmt.Fprintf(stdout, "  status:       %s\n", rs.Status)
	fmt.Fprintf(stdout, "  request:      %s\n", rs.Request)
	fmt.Fprintf(stdout, "  created_at:   %s\n", rs.CreatedAt.Format(time.RFC3339))
	if rs.RunID != "" {
		fmt.Fprintf(stdout, "  run_id:       %s\n", rs.RunID)
	}
	if rs.CreativePlanID != "" {
		fmt.Fprintf(stdout, "  plan_id:      %s\n", rs.CreativePlanID)
	}
	fmt.Fprintf(stdout, "\nplanned_actions:\n")
	for _, a := range rs.PlannedActions {
		fmt.Fprintf(stdout, "  [%s] %s — %s\n", a.Status, a.Type, a.Description)
		if a.Error != "" {
			fmt.Fprintf(stdout, "    error: %s\n", a.Error)
		}
	}
	if rs.Outputs.DraftPath != "" {
		fmt.Fprintf(stdout, "\noutputs:\n")
		fmt.Fprintf(stdout, "  draft: %s\n", rs.Outputs.DraftPath)
	}
	for _, w := range rs.Warnings {
		fmt.Fprintf(stdout, "\nwarning: %s\n", w)
	}
	for _, e := range rs.Errors {
		fmt.Fprintf(stdout, "error: %s\n", e)
	}
	return nil
}

// ---- review-make-revision ----

func ReviewMakeRevision(makeID, revisionID string, stdout io.Writer, opts ReviewMakeRevisionOptions) error {
	revPath := filepath.Join(makesRoot, makeID, makeRevisionsDir, revisionID, "revision_summary.json")
	data, err := os.ReadFile(revPath)
	if err != nil {
		return fmt.Errorf("revision %q not found for make %q: %w", revisionID, makeID, err)
	}
	var rs RevisionSummary
	if err := json.Unmarshal(data, &rs); err != nil {
		return fmt.Errorf("revision_summary.json is malformed: %w", err)
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rs)
	}

	text := renderRevisionReviewMD(rs)
	fmt.Fprint(stdout, text)

	if opts.WriteArtifact {
		revDir := filepath.Join(makesRoot, makeID, makeRevisionsDir, revisionID)
		mdPath := filepath.Join(revDir, "revision_review.md")
		if err := os.WriteFile(mdPath, []byte(text), 0o644); err != nil {
			return fmt.Errorf("write revision_review.md: %w", err)
		}
		fmt.Fprintf(stdout, "\nWritten: %s\n", mdPath)
	}
	return nil
}

func renderRevisionReviewMD(rs RevisionSummary) string {
	var b strings.Builder
	b.WriteString("# Make Revision Review\n\n")
	fmt.Fprintf(&b, "- **Revision ID:** %s\n", rs.RevisionID)
	fmt.Fprintf(&b, "- **Make ID:** %s\n", rs.MakeID)
	fmt.Fprintf(&b, "- **Status:** %s\n", rs.Status)
	fmt.Fprintf(&b, "- **Request:** %s\n", rs.Request)
	fmt.Fprintf(&b, "- **Created:** %s\n", rs.CreatedAt.Format(time.RFC3339))
	if rs.RunID != "" {
		fmt.Fprintf(&b, "- **Run ID:** %s\n", rs.RunID)
	}
	if rs.CreativePlanID != "" {
		fmt.Fprintf(&b, "- **Creative Plan ID:** %s\n", rs.CreativePlanID)
	}

	b.WriteString("\n## Planned Actions\n\n")
	for _, a := range rs.PlannedActions {
		icon := "⬜"
		switch a.Status {
		case "completed":
			icon = "✅"
		case "failed":
			icon = "❌"
		case "skipped":
			icon = "⏭"
		}
		fmt.Fprintf(&b, "- %s `%s` — %s\n", icon, a.Type, a.Description)
		if a.RequiresProvider {
			b.WriteString("  - *(requires provider)*\n")
		}
		if a.Error != "" {
			fmt.Fprintf(&b, "  - **error:** %s\n", a.Error)
		}
	}

	if rs.Outputs.DraftPath != "" {
		b.WriteString("\n## Outputs\n\n")
		fmt.Fprintf(&b, "- **Draft:** %s\n", rs.Outputs.DraftPath)
		if rs.Outputs.ScriptPath != "" {
			fmt.Fprintf(&b, "- **Script:** %s\n", rs.Outputs.ScriptPath)
		}
		if rs.Outputs.CaptionVariantsPath != "" {
			fmt.Fprintf(&b, "- **Captions:** %s\n", rs.Outputs.CaptionVariantsPath)
		}
		if rs.Outputs.VoiceoverTextPath != "" {
			fmt.Fprintf(&b, "- **Voiceover Text:** %s\n", rs.Outputs.VoiceoverTextPath)
		}
		if rs.Outputs.VoiceoverAudioPath != "" {
			fmt.Fprintf(&b, "- **Voiceover Audio:** %s\n", rs.Outputs.VoiceoverAudioPath)
		}
	}

	if len(rs.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range rs.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	if len(rs.Errors) > 0 {
		b.WriteString("\n## Errors\n\n")
		for _, e := range rs.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
	}
	if len(rs.NextCommands) > 0 {
		b.WriteString("\n## Next Steps\n\n")
		for _, cmd := range rs.NextCommands {
			fmt.Fprintf(&b, "```\n%s\n```\n", cmd)
		}
	}
	return b.String()
}

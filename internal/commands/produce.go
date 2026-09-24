package commands

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/events"
	"github.com/mirelahmd/OpenVFX/internal/media"
	"github.com/mirelahmd/OpenVFX/internal/production"
)

type ProduceOptions struct {
	Brief             string
	JSON              bool
	DryRun            bool
	TargetDuration    float64
	PythonInterpreter string

	// Creative Director controls.
	NoDirector      bool
	DirectorMode    string // "" (deterministic) | "llm"
	DirectorModel   string
	DirectorBackend string
	DirectorRoute   string
	DirectorTimeout int
}

type CreativeTreatmentOptions struct {
	JSON bool
}

type ReplayOptions struct {
	JSON bool
}

type ProductionsOptions struct {
	JSON  bool
	Limit int
}

// Produce runs the full OpenVFX control loop:
// observe → plan → execute → validate → revise → complete.
//
// Every phase writes durable artifacts before the next begins, so killing the
// process at any point leaves an inspectable partial production on disk.
func Produce(inputPath string, stdout io.Writer, opts ProduceOptions) error {
	if strings.TrimSpace(opts.Brief) == "" {
		return fmt.Errorf("--brief is required")
	}

	productionID, err := newProductionID(opts.Brief)
	if err != nil {
		return err
	}
	layout := production.NewLayout(productionID)
	startedAt := time.Now().UTC()

	cfg, _ := config.Load(config.DefaultPath)

	// ---- OBSERVE ----
	fmt.Fprintf(stdout, "production %s\n", productionID)
	fmt.Fprintf(stdout, "  brief:     %s\n\n", opts.Brief)

	obs, err := production.Observe(inputPath, media.Probe)
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}
	prober := production.NewExecProber(opts.PythonInterpreter)
	caps := production.ProbeCapabilities(prober, cfg)

	fmt.Fprintf(stdout, "  observe    %d asset(s), %.2fs total, %d with audio\n",
		obs.Totals.AssetCount, obs.Totals.DurationSeconds, obs.Totals.WithAudio)
	for _, s := range obs.Skipped {
		fmt.Fprintf(stdout, "             skipped %s (%s)\n", filepath.Base(s.Path), s.Reason)
	}

	fmt.Fprintln(stdout, "  capabilities")
	for _, c := range caps.Capabilities {
		mark := "available"
		if c.Status != production.CapAvailable {
			mark = "UNAVAILABLE"
		}
		detail := ""
		if c.Status != production.CapAvailable && c.Detail != "" {
			detail = "  (" + c.Detail + ")"
		}
		fmt.Fprintf(stdout, "    %-28s %s%s\n", c.ID, mark, detail)
	}
	fmt.Fprintln(stdout)

	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		return err
	}
	if err := production.WriteJSON(layout.ObservationPath(), obs); err != nil {
		return err
	}
	if err := production.WriteJSON(layout.CapabilitiesPath(), caps); err != nil {
		return err
	}

	log, err := events.Open(layout.EventsPath())
	if err != nil {
		return err
	}
	defer log.Close()
	_ = log.Write("PRODUCTION_STARTED", map[string]any{
		"production_id": productionID, "brief": opts.Brief, "input_path": obs.InputPath,
	})
	_ = log.Write("OBSERVED", map[string]any{
		"assets": obs.Totals.AssetCount, "duration_seconds": obs.Totals.DurationSeconds,
	})
	_ = log.Write("CAPABILITIES_PROBED", map[string]any{"unavailable": caps.Unavailable()})

	// ---- CREATIVE DIRECTION ----
	//
	// The director decides WHAT should be made. If it cannot run, or its output
	// does not survive validation, production continues with intent-only
	// planning and says so — it never proceeds on a half-trusted treatment.
	var treatment production.CreativeTreatment
	haveTreatment := false

	if !opts.NoDirector {
		assetObs := production.BuildAssetObservations(productionID, obs, layout.Root, time.Now().UTC())
		if err := production.WriteJSON(layout.AssetObservationsPath(), assetObs); err != nil {
			return err
		}

		dirOpts := buildDirectorOptions(cfg, opts)
		result, unavailable, dErr := production.RunDirector(
			layout, productionID, opts.Brief, dirOpts, production.DefaultDirectorRunner)
		if dErr != nil {
			return fmt.Errorf("creative director: %w", dErr)
		}

		switch {
		case unavailable != "":
			fmt.Fprintf(stdout, "  director   unavailable — %s\n", unavailable)
			fmt.Fprintf(stdout, "             continuing with intent-only planning\n\n")
			_ = log.Write("DIRECTOR_UNAVAILABLE", map[string]any{"reason": unavailable})
		default:
			t, rejection := production.LoadValidatedTreatment(layout, obs)
			if rejection != "" {
				fmt.Fprintf(stdout, "  director   %s\n", rejection)
				fmt.Fprintf(stdout, "             continuing with intent-only planning\n\n")
				_ = log.Write("TREATMENT_REJECTED", map[string]any{"reason": rejection})
			} else {
				treatment = t
				haveTreatment = true
				printTreatment(stdout, t, result)
				_ = log.Write("TREATMENT_CREATED", map[string]any{
					"effective_mode": t.Reasoning.EffectiveMode,
					"requested_mode": t.Reasoning.RequestedMode,
					"llm_calls":      result.LLMCalls,
					"segments":       len(t.Segments),
					"gaps":           len(t.GeneratedAssetNeeds),
					"fallback_used":  t.Reasoning.FallbackUsed,
				})
			}
		}
	}

	// ---- PLAN ----
	var plan production.ProductionPlan
	if haveTreatment {
		plan, err = production.PlanFromTreatment(productionID, opts.Brief, obs, treatment, time.Now().UTC())
		if err != nil {
			fmt.Fprintf(stdout, "  plan       treatment could not be planned (%v); falling back to intent-only\n", err)
			haveTreatment = false
		}
	}
	if !haveTreatment {
		plan, err = production.Plan(productionID, opts.Brief, obs, time.Now().UTC())
	}
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	if opts.TargetDuration > 0 {
		plan.Intent.TargetDurationSeconds = opts.TargetDuration
		for i := range plan.Stages {
			if plan.Stages[i].Type == production.StageTypeSelectClips {
				plan.Stages[i].Params.TargetSeconds = opts.TargetDuration
			}
		}
	}
	if err := production.WritePlan(layout, plan); err != nil {
		return err
	}
	_ = log.Write("PLAN_CREATED", map[string]any{
		"plan_version": plan.PlanVersion, "stages": len(plan.Stages),
	})
	printProductionPlan(stdout, plan)

	if opts.DryRun {
		fmt.Fprintf(stdout, "\n  dry-run: plan written to %s; nothing executed\n", layout.PlanPath(plan.PlanVersion))
		return nil
	}

	// ---- EXECUTE / VALIDATE / REVISE ----
	executor := &production.Executor{
		Layout: layout,
		Caps:   caps,
		Obs:    obs,
		Runner: production.NewExecRunner(),
		Python: opts.PythonInterpreter,
		Log:    log,
	}

	plans := []production.ProductionPlan{}
	var revisions []production.Revision
	var validation *production.ValidationResult
	current := plan

	for attempt := 0; ; attempt++ {
		res := executor.Execute(&current)
		_ = production.WritePlan(layout, current)

		if res.Err != nil && errors.Is(res.Err, production.ErrCapabilityUnavailable) {
			fmt.Fprintf(stdout, "\n  execute    %s BLOCKED: required capability %s unavailable\n",
				res.BlockedStage, res.BlockedCap)

			if len(revisions) >= production.MaxRevisions {
				plans = append(plans, current)
				return finishIncomplete(stdout, layout, productionID, obs, plans, revisions, caps, startedAt,
					fmt.Errorf("revision budget of %d exhausted; %s remains unsatisfiable",
						production.MaxRevisions, res.BlockedCap))
			}

			next, rev, revErr := production.ReviseForCapability(current, caps, res.BlockedStage, time.Now().UTC())
			if revErr != nil {
				plans = append(plans, current)
				return finishIncomplete(stdout, layout, productionID, obs, plans, revisions, caps, startedAt, revErr)
			}
			if err := production.WriteJSON(layout.RevisionPath(len(revisions)+1), rev); err != nil {
				return err
			}
			if err := production.WritePlan(layout, next); err != nil {
				return err
			}
			_ = log.Write("PLAN_REVISED", map[string]any{
				"revision_id": rev.RevisionID, "trigger": rev.Trigger,
				"from_plan": rev.FromPlan, "to_plan": rev.ToPlan, "change": rev.Change,
			})

			fmt.Fprintf(stdout, "  revise     %s: %s\n", rev.RevisionID, rev.Trigger)
			fmt.Fprintf(stdout, "             %s\n", rev.Change)
			fmt.Fprintf(stdout, "             plan v%d written; %d of %d stages reused from v%d\n",
				rev.ToPlan, len(rev.ReusedStages), len(current.Stages), rev.FromPlan)

			plans = append(plans, current)
			revisions = append(revisions, rev)
			current = next
			continue
		}

		if res.Err != nil {
			plans = append(plans, current)
			return finishIncomplete(stdout, layout, productionID, obs, plans, revisions, caps, startedAt, res.Err)
		}

		// ---- VALIDATE ----
		finalOutput, sidecars := promoteOutputs(layout, current, res.FinalOutput)
		v := production.Validate(current, finalOutput, media.Probe)
		validation = &v
		if err := production.WriteJSON(layout.ValidationPath(), v); err != nil {
			return err
		}
		_ = log.Write("VALIDATED", map[string]any{
			"status": v.Status, "passed": v.Passed, "failed": v.Failed,
		})

		fmt.Fprintf(stdout, "\n  validate\n%s", v.Summary())

		if v.OK() || len(revisions) >= production.MaxRevisions {
			plans = append(plans, current)
			return finishComplete(stdout, layout, productionID, obs, plans, revisions, validation,
				caps, finalOutput, sidecars, startedAt, opts.JSON, treatmentOrNil(haveTreatment, treatment))
		}

		// ---- REVISE on validation failure ----
		failure, _ := v.FirstFailure()
		actual := actualDurationFrom(v)
		next, rev, revErr := production.ReviseForValidation(current, failure, actual, time.Now().UTC())
		if revErr != nil {
			// No corrective pass exists for this failure. That is a legitimate
			// outcome: record it and stop rather than looping uselessly.
			fmt.Fprintf(stdout, "  revise     no corrective pass available: %v\n", revErr)
			plans = append(plans, current)
			return finishComplete(stdout, layout, productionID, obs, plans, revisions, validation,
				caps, finalOutput, sidecars, startedAt, opts.JSON, treatmentOrNil(haveTreatment, treatment))
		}
		if err := production.WriteJSON(layout.RevisionPath(len(revisions)+1), rev); err != nil {
			return err
		}
		if err := production.WritePlan(layout, next); err != nil {
			return err
		}
		_ = log.Write("PLAN_REVISED", map[string]any{
			"revision_id": rev.RevisionID, "trigger": rev.Trigger,
			"from_plan": rev.FromPlan, "to_plan": rev.ToPlan, "change": rev.Change,
		})
		fmt.Fprintf(stdout, "\n  revise     %s: %s\n", rev.RevisionID, rev.Trigger)
		fmt.Fprintf(stdout, "             %s\n", rev.Change)

		plans = append(plans, current)
		revisions = append(revisions, rev)
		current = next
	}
}

// promoteOutputs copies the last stage's video (and any sidecar) into the
// production's output/ directory, so the deliverable lives at a stable path
// regardless of which stage produced it.
func promoteOutputs(l production.Layout, plan production.ProductionPlan, finalVideo string) (string, []string) {
	outDir := l.OutputDir()
	_ = os.MkdirAll(outDir, 0o755)

	finalPath := ""
	if finalVideo != "" {
		finalPath = filepath.Join(outDir, "final.mp4")
		if err := copyFile(finalVideo, finalPath); err != nil {
			finalPath = finalVideo
		}
	}

	var sidecars []string
	for _, s := range plan.Stages {
		if s.Type != production.StageTypeTextOverlay || s.Status != production.StageStatusCompleted {
			continue
		}
		var rec production.ExecutionRecord
		if err := production.ReadJSON(l.ExecutionPath(s.ID), &rec); err != nil {
			continue
		}
		for _, o := range rec.Outputs {
			if strings.EqualFold(filepath.Ext(o), ".srt") {
				dst := filepath.Join(outDir, "final.srt")
				if err := copyFile(o, dst); err == nil {
					sidecars = append(sidecars, dst)
				}
			}
		}
	}
	return finalPath, sidecars
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func actualDurationFrom(v production.ValidationResult) float64 {
	for _, a := range v.Assertions {
		if a.ID != "duration_within_tolerance" {
			continue
		}
		var secs float64
		if _, err := fmt.Sscanf(a.Actual, "%fs", &secs); err == nil {
			return secs
		}
	}
	return 0
}

func finishComplete(stdout io.Writer, l production.Layout, productionID string,
	obs production.Observation, plans []production.ProductionPlan,
	revisions []production.Revision, validation *production.ValidationResult,
	caps production.CapabilitySet, finalOutput string, sidecars []string,
	startedAt time.Time, asJSON bool, treatment *production.CreativeTreatment) error {

	rec := production.BuildRunRecord(l, productionID, obs, plans, revisions, validation, caps,
		finalOutput, sidecars, startedAt)
	if treatment != nil {
		rec = rec.WithTreatment(l, *treatment)
	}
	if err := production.WriteJSON(l.RunRecordPath(), rec); err != nil {
		return err
	}
	handoff := production.Handoff(rec, plans)
	if err := os.WriteFile(l.HandoffPath(), []byte(handoff), 0o644); err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rec)
	}

	fmt.Fprintf(stdout, "\n  complete   %s\n", rec.Status)
	if rec.FinalOutput != "" {
		fmt.Fprintf(stdout, "             %s\n", rec.FinalOutput)
	}
	for _, s := range rec.Sidecars {
		fmt.Fprintf(stdout, "             %s (sidecar)\n", s)
	}
	fmt.Fprintf(stdout, "             external network calls: %d\n", rec.NetworkEgress)
	fmt.Fprintf(stdout, "\n  run record: %s\n", l.RunRecordPath())
	fmt.Fprintf(stdout, "  handoff:    %s\n", l.HandoffPath())
	fmt.Fprintf(stdout, "  replay:     byom-video replay %s\n", productionID)
	return nil
}

func finishIncomplete(stdout io.Writer, l production.Layout, productionID string,
	obs production.Observation, plans []production.ProductionPlan,
	revisions []production.Revision, caps production.CapabilitySet,
	startedAt time.Time, cause error) error {

	rec := production.BuildRunRecord(l, productionID, obs, plans, revisions, nil, caps, "", nil, startedAt)
	rec.Status = "failed"
	rec.Errors = append(rec.Errors, cause.Error())
	_ = production.WriteJSON(l.RunRecordPath(), rec)
	_ = os.WriteFile(l.HandoffPath(), []byte(production.Handoff(rec, plans)), 0o644)

	fmt.Fprintf(stdout, "\n  failed     %v\n", cause)
	fmt.Fprintf(stdout, "  run record: %s\n", l.RunRecordPath())
	return cause
}

// treatmentOrNil keeps the run record honest: no treatment pointer is attached
// when creative direction did not produce a usable one.
func treatmentOrNil(have bool, t production.CreativeTreatment) *production.CreativeTreatment {
	if !have {
		return nil
	}
	return &t
}

func printProductionPlan(stdout io.Writer, plan production.ProductionPlan) {
	fmt.Fprintf(stdout, "  plan v%d    %d stages   planner=%s\n",
		plan.PlanVersion, len(plan.Stages), plan.Planner)
	for _, s := range plan.Stages {
		fmt.Fprintf(stdout, "    %-12s %-16s requires %-24s [cpu=%.2g mem=%dMB gpu=%t %s]\n",
			s.ID, s.Type, s.Requires,
			s.Requirements.CPUCores, s.Requirements.MemoryMB, s.Requirements.GPU,
			s.Requirements.Runtime)
		if s.TreatmentRef != "" || s.TreatmentDecisionID != "" {
			ref := s.TreatmentRef
			if s.TreatmentDecisionID != "" {
				ref = fmt.Sprintf("%s (%s)", ref, s.TreatmentDecisionID)
			}
			fmt.Fprintf(stdout, "                 ↳ treatment: %s\n", truncateBrief(ref))
		}
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(stdout, "    warning: %s\n", w)
	}
}

// buildDirectorOptions resolves the creative reasoning route. Explicit flags win
// over configuration; configuration wins over nothing.
func buildDirectorOptions(cfg config.Config, opts ProduceOptions) production.DirectorOptions {
	dir := production.DirectorOptions{
		Mode:              strings.ToLower(strings.TrimSpace(opts.DirectorMode)),
		Model:             opts.DirectorModel,
		Backend:           opts.DirectorBackend,
		Route:             opts.DirectorRoute,
		TimeoutSeconds:    opts.DirectorTimeout,
		PythonInterpreter: opts.PythonInterpreter,
	}
	if dir.Mode == "" {
		dir.Mode = production.ReasoningDeterministic
	}
	if dir.Mode != production.ReasoningLLM {
		return dir
	}

	if dir.Model == "" && cfg.Models.Enabled {
		entries := map[string]production.ModelEntry{}
		for name, e := range cfg.Models.Entries {
			entries[name] = production.ModelEntry{Provider: e.Provider, Model: e.Model, BaseURL: e.BaseURL}
		}
		provider, model, baseURL, route := production.DirectorConfigFromModels(
			cfg.Models.Routes, entries, dir.Route)
		if dir.Provider == "" {
			dir.Provider = provider
		}
		if dir.Model == "" {
			dir.Model = model
		}
		if dir.Backend == "" {
			dir.Backend = baseURL
		}
		dir.Route = route
	}
	if dir.Provider == "" {
		dir.Provider = "ollama"
	}
	return dir
}

func printTreatment(stdout io.Writer, t production.CreativeTreatment, res production.DirectorResult) {
	fmt.Fprintf(stdout, "  director   %s\n", production.DirectorSummaryLine(t))
	fmt.Fprintf(stdout, "    objective   %s\n", truncateBrief(t.Objective))
	if t.Platform != "" && t.Platform != "unspecified" {
		fmt.Fprintf(stdout, "    platform    %s", t.Platform)
		if t.TargetDurationSeconds > 0 {
			fmt.Fprintf(stdout, "  target %.0fs", t.TargetDurationSeconds)
		}
		fmt.Fprintln(stdout)
	}
	if len(t.Tone) > 0 {
		fmt.Fprintf(stdout, "    tone        %s\n", strings.Join(t.Tone, ", "))
	}
	if t.OpeningStrategy != "" {
		fmt.Fprintf(stdout, "    opening     %s\n", truncateBrief(t.OpeningStrategy))
	}
	if len(t.AssetRoles) > 0 {
		fmt.Fprintln(stdout, "    roles")
		for _, r := range t.AssetRoles {
			fmt.Fprintf(stdout, "      %-12s %-22s (%s)\n", r.AssetID, r.Role, r.Confidence)
		}
	}
	if len(t.PacingStrategy) > 0 {
		fmt.Fprintln(stdout, "    pacing")
		for _, p := range t.PacingStrategy {
			fmt.Fprintf(stdout, "      %5.1f-%5.1fs  %-8s %s\n",
				p.FromSeconds, p.ToSeconds, p.CutStyle, truncateBrief(p.Intent))
		}
	}
	if len(t.Segments) > 0 {
		fmt.Fprintf(stdout, "    segments    %d planned\n", len(t.Segments))
	}
	for _, need := range t.GeneratedAssetNeeds {
		severity := "optional"
		if need.Required {
			severity = "REQUIRED"
		}
		fmt.Fprintf(stdout, "    gap         [%s] %s — %s\n", severity, need.Kind, truncateBrief(need.Description))
	}
	for _, alt := range t.DegradedAlternatives {
		fmt.Fprintf(stdout, "    fallback    %s → %s\n", alt.ForNeed, truncateBrief(alt.Approach))
	}
	if t.Critique != nil && t.Critique.Performed {
		fmt.Fprintf(stdout, "    critique    1 pass, %d finding(s), %d revision(s)\n",
			len(t.Critique.Findings), len(t.Critique.Revisions))
		for _, f := range t.Critique.Findings {
			fmt.Fprintf(stdout, "      - %s\n", truncateBrief(f))
		}
	}
	if len(t.Uncertainties) > 0 {
		fmt.Fprintf(stdout, "    uncertain   %s\n", truncateBrief(t.Uncertainties[0]))
	}
	fmt.Fprintln(stdout)
}

// CreativeTreatmentCommand prints the persisted treatment for a production.
func CreativeTreatmentCommand(productionID string, stdout io.Writer, opts CreativeTreatmentOptions) error {
	layout := production.NewLayout(productionID)
	t, err := production.ReadTreatment(layout)
	if err != nil {
		return fmt.Errorf("no creative treatment for production %q: %w", productionID, err)
	}
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(t)
	}

	fmt.Fprintf(stdout, "creative treatment %s\n", t.TreatmentID)
	fmt.Fprintf(stdout, "  brief:     %s\n\n", t.Brief)
	printTreatment(stdout, t, production.DirectorResult{})

	if len(t.NarrativeStructure) > 0 {
		fmt.Fprintln(stdout, "  narrative")
		for _, b := range t.NarrativeStructure {
			fmt.Fprintf(stdout, "    %-12s %-16s ~%.1fs  %s\n", b.ID, b.Name, b.ApproxSeconds, b.Purpose)
		}
	}
	if len(t.Segments) > 0 {
		fmt.Fprintln(stdout, "  segments")
		for _, s := range t.OrderedSegments() {
			fmt.Fprintf(stdout, "    %-10s %-12s %6.2f-%6.2fs  %s\n",
				s.ID, s.AssetID, s.SourceIn, s.SourceOut, truncateBrief(s.Purpose))
		}
	}
	if len(t.Decisions) > 0 {
		fmt.Fprintln(stdout, "  decisions")
		for _, d := range t.Decisions {
			fmt.Fprintf(stdout, "    %-10s %s\n", d.ID, d.Summary)
			if d.Rationale != "" {
				fmt.Fprintf(stdout, "               %s\n", d.Rationale)
			}
		}
	}
	if len(t.SuccessCriteria) > 0 {
		fmt.Fprintln(stdout, "  success criteria")
		for _, c := range t.SuccessCriteria {
			fmt.Fprintf(stdout, "    %-10s %-28s %s\n", c.ID, c.Assertion, c.Target)
		}
	}
	if len(t.Assumptions) > 0 {
		fmt.Fprintln(stdout, "  assumptions")
		for _, a := range t.Assumptions {
			fmt.Fprintf(stdout, "    - %s\n", a)
		}
	}
	return nil
}

// ---- replay ----

// Replay re-issues the exact argv recorded during a production, in stage order,
// into a fresh directory. It never consults the planner — if replay works, the
// execution records are a complete account of what happened.
func Replay(productionID string, stdout io.Writer, opts ReplayOptions) error {
	layout := production.NewLayout(productionID)

	var rec production.RunRecord
	if err := production.ReadJSON(layout.RunRecordPath(), &rec); err != nil {
		return fmt.Errorf("production %q not found (no run_record.json): %w", productionID, err)
	}

	fmt.Fprintf(stdout, "replay %s\n", productionID)
	fmt.Fprintf(stdout, "  brief: %s\n\n", rec.Brief)

	runner := production.NewExecRunner()
	total, failed := 0, 0

	for _, s := range rec.Stages {
		if s.Status != production.StageStatusCompleted {
			fmt.Fprintf(stdout, "  %-12s skipped (status %s)\n", s.ID, s.Status)
			continue
		}
		var er production.ExecutionRecord
		if err := production.ReadJSON(layout.ExecutionPath(s.ID), &er); err != nil {
			fmt.Fprintf(stdout, "  %-12s no execution record\n", s.ID)
			continue
		}
		if len(er.Commands) == 0 {
			fmt.Fprintf(stdout, "  %-12s %s (native, no external commands)\n", s.ID, s.Type)
			continue
		}
		for i, c := range er.Commands {
			total++
			fmt.Fprintf(stdout, "  %-12s [%d/%d] %s\n", s.ID, i+1, len(er.Commands), strings.Join(c.Argv, " "))
			if _, err := runner.Run(c.Argv); err != nil {
				failed++
				fmt.Fprintf(stdout, "               FAILED: %v\n", err)
			}
		}
	}

	fmt.Fprintf(stdout, "\n  %d command(s) replayed, %d failed\n", total, failed)
	if failed > 0 {
		return fmt.Errorf("replay completed with %d failed command(s)", failed)
	}
	return nil
}

// ---- productions listing ----

func Productions(stdout io.Writer, opts ProductionsOptions) error {
	entries, err := os.ReadDir(production.ProductionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "No productions found.")
			return nil
		}
		return err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if opts.Limit > 0 && len(ids) > opts.Limit {
		ids = ids[:opts.Limit]
	}

	type row struct {
		ProductionID string `json:"production_id"`
		Status       string `json:"status"`
		Brief        string `json:"brief"`
		Revisions    int    `json:"revisions"`
		FinalOutput  string `json:"final_output,omitempty"`
	}
	var rows []row
	for _, id := range ids {
		var rec production.RunRecord
		if err := production.ReadJSON(production.NewLayout(id).RunRecordPath(), &rec); err != nil {
			rows = append(rows, row{ProductionID: id, Status: "incomplete"})
			continue
		}
		rows = append(rows, row{
			ProductionID: id, Status: rec.Status, Brief: rec.Brief,
			Revisions: len(rec.Revisions), FinalOutput: rec.FinalOutput,
		})
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No productions found.")
		return nil
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s  %-22s revisions=%d  %s\n", r.ProductionID, r.Status, r.Revisions, truncateBrief(r.Brief))
	}
	return nil
}

func truncateBrief(s string) string {
	if len(s) <= 60 {
		return s
	}
	return s[:57] + "..."
}

func newProductionID(brief string) (string, error) {
	var suffix [3]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("create production id: %w", err)
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}

package production

import (
	"fmt"
	"strings"
	"time"
)

// RunRecord is the single replayable account of a production. One id, one
// file, spanning brief → observation → every plan version → every stage
// execution → validation → revisions → final output.
//
// The existing `make` path scatters this across three directories with three
// unrelated ids. This is the correction.
type RunRecord struct {
	SchemaVersion string    `json:"schema_version"`
	ProductionID  string    `json:"production_id"`
	CreatedAt     time.Time `json:"created_at"`
	CompletedAt   time.Time `json:"completed_at"`
	Brief         string    `json:"brief"`
	InputPath     string    `json:"input_path"`
	Status        string    `json:"status"`

	Intent Intent `json:"intent"`

	AssetCount       int     `json:"asset_count"`
	SourceSeconds    float64 `json:"source_seconds"`
	PlanVersions     []int   `json:"plan_versions"`
	FinalPlanVersion int     `json:"final_plan_version"`

	Stages     []StageOutcome    `json:"stages"`
	Revisions  []Revision        `json:"revisions,omitempty"`
	Validation *ValidationResult `json:"validation,omitempty"`

	FinalOutput string   `json:"final_output,omitempty"`
	Sidecars    []string `json:"sidecars,omitempty"`

	// CapabilityGaps records what this machine could not do. Recording the gap
	// is the difference between an honest system and a hallucinating one.
	CapabilityGaps []string `json:"capability_gaps,omitempty"`
	// NetworkEgress counts external calls made during the production. Zero is
	// a claim a studio security team can check.
	NetworkEgress int `json:"network_egress_calls"`

	// Treatment summarises the creative reasoning that shaped this production,
	// including whether a model actually reasoned.
	Treatment *TreatmentSummary `json:"treatment,omitempty"`

	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// TreatmentSummary is the compact record of creative direction. The full
// treatment stays in creative_treatment.json.
type TreatmentSummary struct {
	TreatmentID      string              `json:"treatment_id"`
	Reasoning        ReasoningProvenance `json:"reasoning"`
	Objective        string              `json:"objective"`
	Platform         string              `json:"platform,omitempty"`
	SegmentCount     int                 `json:"segment_count"`
	RoleCount        int                 `json:"role_count"`
	GapCount         int                 `json:"gap_count"`
	CritiqueFindings int                 `json:"critique_findings"`
	CritiquePasses   int                 `json:"critique_passes"`
	Artifact         string              `json:"artifact"`
}

// WithTreatment attaches creative provenance to a run record.
func (r RunRecord) WithTreatment(l Layout, t CreativeTreatment) RunRecord {
	summary := &TreatmentSummary{
		TreatmentID:  t.TreatmentID,
		Reasoning:    t.Reasoning,
		Objective:    t.Objective,
		Platform:     t.Platform,
		SegmentCount: len(t.Segments),
		RoleCount:    len(t.AssetRoles),
		GapCount:     len(t.GeneratedAssetNeeds),
		Artifact:     l.TreatmentPath(),
	}
	if t.Critique != nil {
		summary.CritiqueFindings = len(t.Critique.Findings)
		summary.CritiquePasses = t.Critique.Passes
	}
	r.Treatment = summary
	return r
}

// StageOutcome is the compact per-stage summary carried in the run record. The
// full provenance lives in each stage's execution.json.
type StageOutcome struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Requires     string `json:"requires"`
	BoundTo      string `json:"bound_to,omitempty"`
	Fallback     bool   `json:"fallback,omitempty"`
	Degraded     bool   `json:"degraded,omitempty"`
	DurationMS   int64  `json:"duration_ms"`
	CommandCount int    `json:"command_count"`
	Record       string `json:"record"`
	// TreatmentRef echoes the creative decision that caused this stage.
	TreatmentRef string `json:"treatment_ref,omitempty"`
}

// BuildRunRecord assembles the final account from the pieces already on disk.
func BuildRunRecord(l Layout, productionID string, obs Observation, plans []ProductionPlan, revisions []Revision, validation *ValidationResult, caps CapabilitySet, finalOutput string, sidecars []string, createdAt time.Time) RunRecord {
	final := plans[len(plans)-1]

	rec := RunRecord{
		SchemaVersion:    RunRecordSchemaVersion,
		ProductionID:     productionID,
		CreatedAt:        createdAt,
		CompletedAt:      time.Now().UTC(),
		Brief:            final.Brief,
		InputPath:        obs.InputPath,
		Intent:           final.Intent,
		AssetCount:       obs.Totals.AssetCount,
		SourceSeconds:    obs.Totals.DurationSeconds,
		FinalPlanVersion: final.PlanVersion,
		Revisions:        revisions,
		Validation:       validation,
		FinalOutput:      finalOutput,
		Sidecars:         sidecars,
		Warnings:         final.Warnings,
	}
	for _, p := range plans {
		rec.PlanVersions = append(rec.PlanVersions, p.PlanVersion)
	}

	for _, s := range final.Stages {
		out := StageOutcome{
			ID: s.ID, Type: s.Type, Status: s.Status,
			Requires: s.Requires, Degraded: s.Degraded,
			Record:       l.ExecutionPath(s.ID),
			TreatmentRef: s.TreatmentRef,
		}
		if s.Binding != nil {
			out.BoundTo = s.Binding.Backend
			out.Fallback = s.Binding.Fallback
		}
		var er ExecutionRecord
		if err := ReadJSON(l.ExecutionPath(s.ID), &er); err == nil {
			out.DurationMS = er.DurationMS
			out.CommandCount = len(er.Commands)
		}
		rec.Stages = append(rec.Stages, out)
	}

	rec.CapabilityGaps = caps.Unavailable()

	// Every stage that actually ran was bound to a local capability; any
	// binding whose capability declares egress is counted here.
	for _, s := range final.Stages {
		if s.Binding == nil {
			continue
		}
		if c, ok := caps.Get(s.Binding.Backend); ok && c.NetworkEgress {
			rec.NetworkEgress++
		}
	}

	switch {
	case validation == nil:
		rec.Status = "incomplete"
	case !validation.OK():
		rec.Status = "completed_with_failures"
	case validation.Status == "passed_degraded":
		rec.Status = "completed_degraded"
	default:
		rec.Status = "completed"
	}
	return rec
}

// Handoff renders the human-readable provenance document. It is deliberately
// plain: the point is that someone can read it and understand exactly what the
// machine did and why, without running anything.
func Handoff(rec RunRecord, plans []ProductionPlan) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# OpenVFX Production Handoff\n\n")
	fmt.Fprintf(&b, "**Production:** `%s`  \n", rec.ProductionID)
	fmt.Fprintf(&b, "**Status:** %s  \n", rec.Status)
	fmt.Fprintf(&b, "**Completed:** %s\n\n", rec.CompletedAt.Format(time.RFC3339))

	fmt.Fprintf(&b, "## Brief\n\n> %s\n\n", rec.Brief)

	if rec.Treatment != nil {
		t := rec.Treatment
		fmt.Fprintf(&b, "## Creative direction\n\n")
		fmt.Fprintf(&b, "- objective: %s\n", t.Objective)
		fmt.Fprintf(&b, "- reasoning: **%s**", t.Reasoning.EffectiveMode)
		if t.Reasoning.EffectiveMode == ReasoningLLM {
			fmt.Fprintf(&b, " (%s %s)", t.Reasoning.Provider, t.Reasoning.Model)
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "- semantic reasoning: %t\n", t.Reasoning.SemanticReasoning)
		if t.Reasoning.FallbackUsed {
			fmt.Fprintf(&b, "- fell back because: %s\n", t.Reasoning.FallbackReason)
		}
		fmt.Fprintf(&b, "- %d segment(s), %d role(s), %d gap(s); critique %d pass with %d finding(s)\n",
			t.SegmentCount, t.RoleCount, t.GapCount, t.CritiquePasses, t.CritiqueFindings)
		fmt.Fprintf(&b, "- full treatment: `%s`\n\n", t.Artifact)
	}

	fmt.Fprintf(&b, "## Interpreted intent\n\n")
	if rec.Intent.TargetDurationSeconds > 0 {
		fmt.Fprintf(&b, "- target duration: %.2fs (±%.0f%%)\n",
			rec.Intent.TargetDurationSeconds, rec.Intent.DurationTolerance*100)
	}
	if rec.Intent.Width > 0 {
		fmt.Fprintf(&b, "- format: %dx%d (%s)\n", rec.Intent.Width, rec.Intent.Height, rec.Intent.AspectRatio)
	}
	fmt.Fprintf(&b, "- captions requested: %t (burned-in: %t)\n",
		rec.Intent.WantsCaptions, rec.Intent.WantsBurnedCaptions)
	fmt.Fprintf(&b, "- audio expected: %t\n\n", rec.Intent.WantsAudio)

	fmt.Fprintf(&b, "## Source\n\n")
	fmt.Fprintf(&b, "- input: `%s`\n", rec.InputPath)
	fmt.Fprintf(&b, "- assets: %d, totalling %.2fs\n\n", rec.AssetCount, rec.SourceSeconds)

	fmt.Fprintf(&b, "## Stages executed\n\n")
	fmt.Fprintf(&b, "| stage | type | requires | bound to | status | ms | cmds |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|---|\n")
	for _, s := range rec.Stages {
		bound := s.BoundTo
		if s.Fallback {
			bound += " *(fallback)*"
		}
		status := s.Status
		if s.Degraded {
			status += " (degraded)"
		}
		fmt.Fprintf(&b, "| `%s` | %s | `%s` | `%s` | %s | %d | %d |\n",
			s.ID, s.Type, s.Requires, bound, status, s.DurationMS, s.CommandCount)
	}
	fmt.Fprintln(&b)

	if len(rec.Revisions) > 0 {
		fmt.Fprintf(&b, "## Revisions\n\n")
		for _, r := range rec.Revisions {
			fmt.Fprintf(&b, "### %s — %s\n\n", r.RevisionID, r.Trigger)
			fmt.Fprintf(&b, "- plan v%d → v%d\n", r.FromPlan, r.ToPlan)
			fmt.Fprintf(&b, "- change: %s\n", r.Change)
			fmt.Fprintf(&b, "- reason: %s\n", r.Reason)
			if len(r.ReusedStages) > 0 {
				fmt.Fprintf(&b, "- reused unchanged: %s\n", strings.Join(r.ReusedStages, ", "))
			}
			fmt.Fprintln(&b)
		}
	}

	if rec.Validation != nil {
		fmt.Fprintf(&b, "## Validation\n\n")
		fmt.Fprintf(&b, "| assertion | result | expected | actual |\n|---|---|---|---|\n")
		for _, a := range rec.Validation.Assertions {
			result := a.Status
			if a.Degraded {
				result += " (degraded)"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", a.ID, result, a.Expected, a.Actual)
		}
		fmt.Fprintln(&b)
	}

	if len(rec.CapabilityGaps) > 0 {
		fmt.Fprintf(&b, "## Capability gaps on this machine\n\n")
		fmt.Fprintf(&b, "These were probed and found unavailable. Nothing was faked in their place.\n\n")
		for _, g := range rec.CapabilityGaps {
			fmt.Fprintf(&b, "- `%s`\n", g)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## Output\n\n")
	if rec.FinalOutput != "" {
		fmt.Fprintf(&b, "- `%s`\n", rec.FinalOutput)
	}
	for _, s := range rec.Sidecars {
		fmt.Fprintf(&b, "- `%s` (sidecar)\n", s)
	}
	fmt.Fprintf(&b, "\n## Provenance\n\n")
	fmt.Fprintf(&b, "- external network calls: **%d**\n", rec.NetworkEgress)
	fmt.Fprintf(&b, "- plan versions retained: %v\n", rec.PlanVersions)
	fmt.Fprintf(&b, "- every stage's exact argv is recorded in `stages/<stage_id>/execution.json`\n")
	fmt.Fprintf(&b, "- replay this production with: `byom-video replay %s`\n", rec.ProductionID)

	return b.String()
}

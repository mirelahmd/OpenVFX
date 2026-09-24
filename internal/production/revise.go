package production

import (
	"fmt"
	"time"
)

// MaxRevisions bounds the corrective loop. An agentic system that can revise
// without limit is a system that can spin forever; two passes is enough to
// demonstrate recovery and cheap enough to reason about.
const MaxRevisions = 2

const (
	TriggerCapabilityUnavailable = "capability_unavailable"
	TriggerValidationFailed      = "validation_failed"
)

// Revision is the persisted reason a plan changed. It is the artifact that
// makes "the system revised itself" a checkable claim rather than a story.
type Revision struct {
	SchemaVersion string    `json:"schema_version"`
	RevisionID    string    `json:"revision_id"`
	ProductionID  string    `json:"production_id"`
	CreatedAt     time.Time `json:"created_at"`
	Trigger       string    `json:"trigger"`
	StageID       string    `json:"stage_id"`
	FromPlan      int       `json:"from_plan"`
	ToPlan        int       `json:"to_plan"`
	Change        string    `json:"change"`
	Reason        string    `json:"reason"`
	// ReusedStages names the stages carried over unchanged, so the cost of the
	// revision is visible.
	ReusedStages []string `json:"reused_stages,omitempty"`
}

// ReviseForCapability produces the next plan version after a stage was blocked
// because its required capability is unavailable.
//
// The stage is NOT deleted. Its Requires is rewritten to the best available
// substitute, which preserves the intent ("deliver captions") while changing
// the means ("burn them in" → "ship a sidecar"). Everything already completed
// is carried over so only the substituted stage re-executes.
func ReviseForCapability(prev ProductionPlan, caps CapabilitySet, blockedStageID string, now time.Time) (ProductionPlan, Revision, error) {
	next := prev.Clone()
	next.PlanVersion = prev.PlanVersion + 1
	next.DerivedFrom = prev.PlanVersion
	next.CreatedAt = now

	stage := next.Stage(blockedStageID)
	if stage == nil {
		return ProductionPlan{}, Revision{}, fmt.Errorf("blocked stage %q not found in plan", blockedStageID)
	}

	original := stage.Requires
	substitute := ""
	for _, cand := range bindingCandidates(original) {
		if cand == original {
			continue
		}
		if caps.IsAvailable(cand) {
			substitute = cand
			break
		}
	}
	if substitute == "" {
		return ProductionPlan{}, Revision{}, fmt.Errorf(
			"no available substitute for capability %q; stage %s cannot be satisfied on this machine",
			original, blockedStageID)
	}

	detail := "not available on this machine"
	if c, ok := caps.Get(original); ok && c.Detail != "" {
		detail = c.Detail
	}

	stage.Requires = substitute
	stage.Status = StageStatusPlanned
	stage.Binding = nil
	stage.Requirements = requirementsFor(substitute, stage.Requirements)

	rev := Revision{
		SchemaVersion: RevisionSchemaVersion,
		RevisionID:    fmt.Sprintf("rev_%04d", next.PlanVersion-1),
		ProductionID:  prev.ProductionID,
		CreatedAt:     now,
		Trigger:       TriggerCapabilityUnavailable,
		StageID:       blockedStageID,
		FromPlan:      prev.PlanVersion,
		ToPlan:        next.PlanVersion,
		Change:        fmt.Sprintf("%s: requires %s -> %s", blockedStageID, original, substitute),
		Reason:        fmt.Sprintf("%s is unavailable (%s); %s is available and satisfies the same stage intent", original, detail, substitute),
		ReusedStages:  completedStageIDs(next, blockedStageID),
	}

	if err := next.Validate(); err != nil {
		return ProductionPlan{}, Revision{}, fmt.Errorf("revised plan is invalid: %w", err)
	}
	return next, rev, nil
}

// ReviseForValidation produces the next plan version after the output failed a
// goal-derived assertion. Today it corrects the one failure mode the
// deterministic executor can actually fix: an output shorter than the target,
// which is repaired by extending the edit rather than by re-cutting blindly.
func ReviseForValidation(prev ProductionPlan, failure Assertion, actualSeconds float64, now time.Time) (ProductionPlan, Revision, error) {
	if failure.ID != "duration_within_tolerance" {
		return ProductionPlan{}, Revision{}, fmt.Errorf(
			"no corrective pass is defined for failed assertion %q", failure.ID)
	}

	next := prev.Clone()
	next.PlanVersion = prev.PlanVersion + 1
	next.DerivedFrom = prev.PlanVersion
	next.CreatedAt = now

	var selectStage *Stage
	for i := range next.Stages {
		if next.Stages[i].Type == StageTypeSelectClips {
			selectStage = &next.Stages[i]
			break
		}
	}
	if selectStage == nil {
		return ProductionPlan{}, Revision{}, fmt.Errorf("plan has no select_clips stage to correct")
	}

	target := prev.Intent.TargetDurationSeconds
	if actualSeconds >= target {
		// Overshoot is corrected by tightening the selection rather than by
		// looping; the selector already trims to target, so re-running with a
		// hard cap is sufficient.
		selectStage.Params.ExtendToTarget = false
	} else {
		// Undershoot: authorise the selector to repeat the tail of the edit
		// until it reaches the target. The selector owns the arithmetic because
		// it is the only part that knows the actual clip durations; the reviser
		// only grants permission, and records why.
		selectStage.Params.ExtendToTarget = true
	}

	// Everything from selection onward must re-run.
	resetFrom(&next, selectStage.ID)

	rev := Revision{
		SchemaVersion: RevisionSchemaVersion,
		RevisionID:    fmt.Sprintf("rev_%04d", next.PlanVersion-1),
		ProductionID:  prev.ProductionID,
		CreatedAt:     now,
		Trigger:       TriggerValidationFailed,
		StageID:       selectStage.ID,
		FromPlan:      prev.PlanVersion,
		ToPlan:        next.PlanVersion,
		Change:        fmt.Sprintf("%s: extend selection toward the %.2fs target", selectStage.ID, target),
		Reason: fmt.Sprintf("assertion %s failed: expected %s, got %s",
			failure.ID, failure.Expected, failure.Actual),
		ReusedStages: completedStageIDs(next, selectStage.ID),
	}

	if err := next.Validate(); err != nil {
		return ProductionPlan{}, Revision{}, fmt.Errorf("revised plan is invalid: %w", err)
	}
	return next, rev, nil
}

// resetFrom marks the named stage and everything after it as planned again.
// Because plan order is a valid topological order, position is sufficient.
func resetFrom(p *ProductionPlan, stageID string) {
	found := false
	for i := range p.Stages {
		if p.Stages[i].ID == stageID {
			found = true
		}
		if found {
			p.Stages[i].Status = StageStatusPlanned
			p.Stages[i].Binding = nil
			p.Stages[i].Degraded = false
		}
	}
}

func completedStageIDs(p ProductionPlan, excluding string) []string {
	var out []string
	for _, s := range p.Stages {
		if s.ID == excluding {
			continue
		}
		if s.Status == StageStatusCompleted {
			out = append(out, s.ID)
		}
	}
	return out
}

// requirementsFor adjusts the declared compute footprint when a stage is
// rebound to a cheaper backend. A sidecar write is not a 4-core ffmpeg job, and
// a scheduler reading the revised plan should see the real cost.
func requirementsFor(capability string, prev Requirements) Requirements {
	switch capability {
	case CapSidecarSRT:
		return Requirements{
			CPUCores: 0.1, MemoryMB: 64, GPU: false,
			Runtime: "openvfx-native",
		}
	case CapFilterDrawtext:
		r := prev
		r.Runtime = "ffmpeg>=6+freetype"
		r.ContainerImageHint = "ghcr.io/openvfx/ffmpeg:6"
		return r
	default:
		return prev
	}
}

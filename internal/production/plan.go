// Package production implements the OpenVFX production control plane:
// observe → plan → execute → validate → revise → complete.
//
// The defining rule of this package is that the plan IS the program. Every
// media operation that runs is described by a Stage in a ProductionPlan, and
// every Stage that runs leaves an ExecutionRecord carrying the exact argv that
// was invoked. A reader of the persisted artifacts can reconstruct what ran
// without reading any Go source.
package production

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ProductionsRoot = ".byom-video/productions"

	PlanSchemaVersion        = "openvfx_production_plan.v1"
	ObservationSchemaVersion = "openvfx_observation.v1"
	CapabilitySchemaVersion  = "openvfx_capabilities.v1"
	ExecutionSchemaVersion   = "openvfx_stage_execution.v1"
	ValidationSchemaVersion  = "openvfx_validation.v1"
	RevisionSchemaVersion    = "openvfx_revision.v1"
	RunRecordSchemaVersion   = "openvfx_run_record.v1"
)

// ---- stage status ----

const (
	StageStatusPlanned   = "planned"
	StageStatusCompleted = "completed"
	StageStatusFailed    = "failed"
	StageStatusBlocked   = "blocked"
	StageStatusSkipped   = "skipped"
)

// ---- stage types ----
//
// A stage type names WHAT should happen. It never names HOW. The how is the
// binding, resolved at execution time against probed capabilities.
const (
	StageTypeProbeAssets   = "probe_assets"
	StageTypeTranscribe    = "transcribe"
	StageTypeSelectClips   = "select_clips"
	StageTypeAssembleVideo = "assemble_video"
	StageTypeFormatOutput  = "format_output"
	StageTypeTextOverlay   = "text_overlay"
	StageTypeMixAudio      = "mix_audio"
)

// ---- plan ----

// ProductionPlan is the machine-readable program. It is emitted by a planner,
// consumed by the executor, and revised by the reviser. Each revision is
// written to its own directory so that every version survives.
type ProductionPlan struct {
	SchemaVersion string    `json:"schema_version"`
	ProductionID  string    `json:"production_id"`
	PlanVersion   int       `json:"plan_version"`
	CreatedAt     time.Time `json:"created_at"`
	Brief         string    `json:"brief"`
	Intent        Intent    `json:"intent"`
	Planner       string    `json:"planner"`
	Stages        []Stage   `json:"stages"`
	Warnings      []string  `json:"warnings,omitempty"`
	// DerivedFrom names the plan version this one was revised from. Zero for v1.
	DerivedFrom int `json:"derived_from,omitempty"`
}

// Intent is the structured reading of the brief. It is what the validator
// asserts against, so it must be explicit rather than inferred twice.
type Intent struct {
	TargetDurationSeconds float64 `json:"target_duration_seconds,omitempty"`
	DurationTolerance     float64 `json:"duration_tolerance,omitempty"`
	AspectRatio           string  `json:"aspect_ratio,omitempty"`
	Width                 int     `json:"width,omitempty"`
	Height                int     `json:"height,omitempty"`
	WantsCaptions         bool    `json:"wants_captions"`
	WantsBurnedCaptions   bool    `json:"wants_burned_captions"`
	WantsAudio            bool    `json:"wants_audio"`
}

// Stage is one typed unit of work.
//
// Requires names the capability the stage needs. Requirements declares the
// compute footprint explicitly so that a future scheduler could place this
// work without reading OpenVFX source. Binding records which backend actually
// satisfied Requires, and is filled in at execution time — late binding is
// what makes backend substitution possible without changing the plan shape.
type Stage struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Description  string       `json:"description"`
	DependsOn    []string     `json:"depends_on,omitempty"`
	Inputs       []string     `json:"inputs,omitempty"`
	Outputs      []string     `json:"outputs,omitempty"`
	Requires     string       `json:"requires"`
	Requirements Requirements `json:"requirements"`
	Params       Params       `json:"params,omitempty"`
	Binding      *Binding     `json:"binding,omitempty"`
	Status       string       `json:"status"`
	// TreatmentDecisionID links this stage back to the creative decision that
	// caused it, so an executed ffmpeg command can be traced to a reason.
	TreatmentDecisionID string `json:"treatment_decision_id,omitempty"`
	// TreatmentRef is a short human-readable echo of that decision.
	TreatmentRef string `json:"treatment_ref,omitempty"`
	// Produced lists what the stage actually wrote, as opposed to Outputs which
	// is what it was planned to write. The gap between the two is meaningful:
	// a completed stage that produced nothing delivered nothing.
	Produced []string `json:"produced,omitempty"`
	// Notes carries stage-level explanations worth keeping in the plan itself,
	// such as "no speech detected".
	Notes []string `json:"notes,omitempty"`
	// Degraded marks a stage that completed through a fallback binding and so
	// delivered less than the brief asked for. It is not a failure, but it must
	// never be reported as full success.
	Degraded bool `json:"degraded,omitempty"`
}

// Requirements is the explicit compute declaration for a stage. OpenVFX does
// not schedule against these today; it declares them so that something else
// could. Declaring is the whole point — hidden requirements cannot be placed.
type Requirements struct {
	CPUCores           float64 `json:"cpu_cores"`
	MemoryMB           int     `json:"memory_mb"`
	GPU                bool    `json:"gpu"`
	Runtime            string  `json:"runtime"`
	ContainerImageHint string  `json:"container_image_hint,omitempty"`
	NetworkEgress      bool    `json:"network_egress"`
}

// Params carries stage-specific settings. Kept as typed fields rather than a
// map so the plan stays readable and diffable.
type Params struct {
	SourcePath     string  `json:"source_path,omitempty"`
	StartSeconds   float64 `json:"start_seconds,omitempty"`
	EndSeconds     float64 `json:"end_seconds,omitempty"`
	TargetSeconds  float64 `json:"target_seconds,omitempty"`
	Width          int     `json:"width,omitempty"`
	Height         int     `json:"height,omitempty"`
	Fit            string  `json:"fit,omitempty"`
	Background     string  `json:"background,omitempty"`
	ModelSize      string  `json:"model_size,omitempty"`
	MaxClips       int     `json:"max_clips,omitempty"`
	CaptionsPath   string  `json:"captions_path,omitempty"`
	AudioPath      string  `json:"audio_path,omitempty"`
	ExtendToTarget bool    `json:"extend_to_target,omitempty"`
	RepeatLastClip int     `json:"repeat_last_clip,omitempty"`
	// Segments carries an explicit cut list supplied by the Creative Director.
	// When present the selector honours it instead of falling back to its own
	// duration arithmetic — this is how creative reasoning reaches real cuts.
	Segments []SegmentCut `json:"segments,omitempty"`
}

// SegmentCut is one creative-director-chosen cut, resolved to a real file.
type SegmentCut struct {
	ID         string  `json:"id"`
	AssetID    string  `json:"asset_id"`
	SourcePath string  `json:"source_path"`
	SourceIn   float64 `json:"source_in"`
	SourceOut  float64 `json:"source_out"`
	Purpose    string  `json:"purpose,omitempty"`
	DecisionID string  `json:"decision_id,omitempty"`
}

// Binding records which concrete backend satisfied a stage's required
// capability, and when that decision was made.
type Binding struct {
	Backend  string    `json:"backend"`
	BoundAt  time.Time `json:"bound_at"`
	Fallback bool      `json:"fallback,omitempty"`
	Reason   string    `json:"reason,omitempty"`
}

// ---- lookup helpers ----

func (p *ProductionPlan) Stage(id string) *Stage {
	for i := range p.Stages {
		if p.Stages[i].ID == id {
			return &p.Stages[i]
		}
	}
	return nil
}

// Clone returns a deep copy, used by the reviser so that plan v1 is never
// mutated when plan v2 is derived from it.
func (p ProductionPlan) Clone() ProductionPlan {
	out := p
	out.Stages = make([]Stage, len(p.Stages))
	copy(out.Stages, p.Stages)
	for i := range out.Stages {
		if p.Stages[i].DependsOn != nil {
			out.Stages[i].DependsOn = append([]string(nil), p.Stages[i].DependsOn...)
		}
		if p.Stages[i].Inputs != nil {
			out.Stages[i].Inputs = append([]string(nil), p.Stages[i].Inputs...)
		}
		if p.Stages[i].Outputs != nil {
			out.Stages[i].Outputs = append([]string(nil), p.Stages[i].Outputs...)
		}
		if p.Stages[i].Binding != nil {
			b := *p.Stages[i].Binding
			out.Stages[i].Binding = &b
		}
	}
	if p.Warnings != nil {
		out.Warnings = append([]string(nil), p.Warnings...)
	}
	return out
}

// Validate checks the structural invariants of a plan. The executor refuses to
// run a plan that does not satisfy these.
func (p ProductionPlan) Validate() error {
	if p.SchemaVersion != PlanSchemaVersion {
		return fmt.Errorf("schema_version %q != %q", p.SchemaVersion, PlanSchemaVersion)
	}
	if p.ProductionID == "" {
		return fmt.Errorf("production_id is empty")
	}
	if p.PlanVersion < 1 {
		return fmt.Errorf("plan_version must be >= 1, got %d", p.PlanVersion)
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("plan has no stages")
	}
	seen := map[string]bool{}
	for i, s := range p.Stages {
		if s.ID == "" {
			return fmt.Errorf("stage[%d]: id is empty", i)
		}
		if seen[s.ID] {
			return fmt.Errorf("stage[%d]: duplicate id %q", i, s.ID)
		}
		seen[s.ID] = true
		if s.Type == "" {
			return fmt.Errorf("stage %s: type is empty", s.ID)
		}
		if s.Requires == "" {
			return fmt.Errorf("stage %s: requires is empty", s.ID)
		}
		if s.Requirements.Runtime == "" {
			return fmt.Errorf("stage %s: requirements.runtime is empty", s.ID)
		}
	}
	// Dependencies must resolve, and must point backwards so a linear walk is
	// a valid topological order.
	for i, s := range p.Stages {
		for _, dep := range s.DependsOn {
			target := -1
			for j := range p.Stages {
				if p.Stages[j].ID == dep {
					target = j
					break
				}
			}
			if target < 0 {
				return fmt.Errorf("stage %s: depends_on unknown stage %q", s.ID, dep)
			}
			if target >= i {
				return fmt.Errorf("stage %s: depends_on %q which is not earlier in the plan", s.ID, dep)
			}
		}
	}
	return nil
}

// ---- paths ----

// Layout resolves every path inside one production directory. Centralising
// this is what keeps the slice from repeating the existing codebase's habit of
// scattering hardcoded relative paths.
type Layout struct {
	Root string
}

func NewLayout(productionID string) Layout {
	return Layout{Root: filepath.Join(ProductionsRoot, productionID)}
}

func (l Layout) ObservationPath() string { return filepath.Join(l.Root, "observation.json") }
func (l Layout) CapabilitiesPath() string {
	return filepath.Join(l.Root, "capabilities.json")
}
func (l Layout) EventsPath() string    { return filepath.Join(l.Root, "events.jsonl") }
func (l Layout) RunRecordPath() string { return filepath.Join(l.Root, "run_record.json") }
func (l Layout) HandoffPath() string   { return filepath.Join(l.Root, "handoff.md") }

func (l Layout) PlanDir(version int) string {
	return filepath.Join(l.Root, "plan", fmt.Sprintf("v%d", version))
}
func (l Layout) PlanPath(version int) string {
	return filepath.Join(l.PlanDir(version), "production_plan.json")
}
func (l Layout) StageDir(stageID string) string {
	return filepath.Join(l.Root, "stages", stageID)
}
func (l Layout) ExecutionPath(stageID string) string {
	return filepath.Join(l.StageDir(stageID), "execution.json")
}
func (l Layout) ValidationPath() string {
	return filepath.Join(l.Root, "validation", "validation.json")
}
func (l Layout) RevisionPath(n int) string {
	return filepath.Join(l.Root, "revisions", fmt.Sprintf("rev_%04d.json", n))
}
func (l Layout) OutputDir() string { return filepath.Join(l.Root, "output") }

// ---- io ----

// WriteJSON writes v as indented JSON, creating parent directories as needed.
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func WritePlan(l Layout, p ProductionPlan) error {
	return WriteJSON(l.PlanPath(p.PlanVersion), p)
}

func ReadPlan(l Layout, version int) (ProductionPlan, error) {
	var p ProductionPlan
	if err := ReadJSON(l.PlanPath(version), &p); err != nil {
		return ProductionPlan{}, err
	}
	return p, nil
}

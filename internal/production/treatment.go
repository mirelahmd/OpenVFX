package production

import (
	"fmt"
	"strings"
	"time"
)

const (
	AssetObservationsSchemaVersion = "openvfx_asset_observations.v1"
	TreatmentSchemaVersion         = "openvfx_creative_treatment.v1"
)

// Reasoning modes. These are recorded so a reader can always tell whether
// semantic reasoning actually happened or whether the system fell back.
const (
	// ReasoningLLM means a language model produced the treatment.
	ReasoningLLM = "llm"
	// ReasoningDeterministic means the creative graph ran but no model was
	// configured, so the treatment came from rules. It is a real treatment, but
	// it is NOT semantic reasoning and must never be presented as such.
	ReasoningDeterministic = "deterministic"
	// ReasoningUnavailable means the creative director could not run at all.
	// No treatment is written; production falls back to intent-only planning.
	ReasoningUnavailable = "unavailable"
)

// ---- asset observations ----

// AssetObservations is the compact, cheap, local description of the supplied
// source material that the creative director reasons over. It deliberately
// carries no transcript bodies and no media payloads — only facts and
// references, so it stays small enough to put in a prompt.
type AssetObservations struct {
	SchemaVersion string          `json:"schema_version"`
	CreatedAt     time.Time       `json:"created_at"`
	ProductionID  string          `json:"production_id,omitempty"`
	InputPath     string          `json:"input_path"`
	Assets        []ObservedAsset `json:"assets"`
	Totals        Totals          `json:"totals"`
	Warnings      []string        `json:"warnings,omitempty"`
}

type ObservedAsset struct {
	ID              string  `json:"id"`
	Path            string  `json:"path"`
	MediaType       string  `json:"media_type"`
	DurationSeconds float64 `json:"duration_seconds"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	AspectRatio     string  `json:"aspect_ratio,omitempty"`
	FrameRate       string  `json:"frame_rate,omitempty"`
	HasVideo        bool    `json:"has_video"`
	HasAudio        bool    `json:"has_audio"`
	SizeBytes       int64   `json:"size_bytes,omitempty"`
	HasTranscript   bool    `json:"has_transcript"`
	TranscriptRef   string  `json:"transcript_ref,omitempty"`
	TranscriptChars int     `json:"transcript_chars,omitempty"`
	Origin          string  `json:"origin"`
}

// Asset origins. "unknown" is a legitimate answer and is preferred over a
// fabricated one.
const (
	OriginSource    = "source"
	OriginGenerated = "generated"
	OriginUnknown   = "unknown"
)

// ---- creative treatment ----

// CreativeTreatment is the Creative Director's structured interpretation of a
// rich creative request. It records conclusions, decisions, evidence references
// and concise rationale. It deliberately holds no chain-of-thought.
//
// It is an input to deterministic production planning, never a replacement for
// it: the treatment says what should be made, and the Go runtime remains the
// sole authority on what can actually execute.
type CreativeTreatment struct {
	SchemaVersion string    `json:"schema_version"`
	TreatmentID   string    `json:"treatment_id"`
	ProductionID  string    `json:"production_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`

	Reasoning ReasoningProvenance `json:"reasoning"`

	Brief     string `json:"brief"`
	Objective string `json:"objective"`
	Audience  string `json:"audience,omitempty"`
	Platform  string `json:"platform,omitempty"`

	TargetDurationSeconds float64  `json:"target_duration_seconds,omitempty"`
	AspectRatio           string   `json:"aspect_ratio,omitempty"`
	Tone                  []string `json:"tone,omitempty"`
	VisualLanguage        []string `json:"visual_language,omitempty"`

	OpeningStrategy    string          `json:"opening_strategy,omitempty"`
	NarrativeStructure []NarrativeBeat `json:"narrative_structure,omitempty"`
	PacingStrategy     []PacingPhase   `json:"pacing_strategy,omitempty"`

	AssetRoles []AssetRole     `json:"asset_roles,omitempty"`
	Segments   []SegmentIntent `json:"segments,omitempty"`

	AudioStrategy   AudioStrategy   `json:"audio_strategy"`
	CaptionStrategy CaptionStrategy `json:"caption_strategy"`

	GeneratedAssetNeeds    []GeneratedNeed       `json:"generated_asset_needs,omitempty"`
	TransformationRequests []TransformationNeed  `json:"transformation_requests,omitempty"`
	DegradedAlternatives   []DegradedAlternative `json:"degraded_alternatives,omitempty"`

	Constraints     []string           `json:"constraints,omitempty"`
	SuccessCriteria []SuccessCriterion `json:"success_criteria,omitempty"`
	Uncertainties   []string           `json:"uncertainties,omitempty"`
	Assumptions     []string           `json:"assumptions,omitempty"`

	Decisions []TreatmentDecision `json:"decisions,omitempty"`
	Critique  *CritiqueRecord     `json:"critique,omitempty"`
	Warnings  []string            `json:"warnings,omitempty"`
}

// ReasoningProvenance makes the honesty requirement structural: a reader can
// always tell what was asked for, what actually reasoned, and why it fell back.
type ReasoningProvenance struct {
	RequestedMode  string `json:"requested_mode"`
	EffectiveMode  string `json:"effective_mode"`
	Graph          string `json:"graph,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	Backend        string `json:"backend,omitempty"`
	Route          string `json:"route,omitempty"`
	FallbackUsed   bool   `json:"fallback_used"`
	FallbackReason string `json:"fallback_reason,omitempty"`
	// SemanticReasoning is true only when a language model actually produced
	// the treatment. Rules-based output must never set this.
	SemanticReasoning bool `json:"semantic_reasoning"`
}

type NarrativeBeat struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Purpose       string  `json:"purpose,omitempty"`
	ApproxSeconds float64 `json:"approx_seconds,omitempty"`
}

type PacingPhase struct {
	ID          string  `json:"id"`
	FromSeconds float64 `json:"from_seconds"`
	ToSeconds   float64 `json:"to_seconds"`
	Intent      string  `json:"intent"`
	CutStyle    string  `json:"cut_style,omitempty"`
}

// AssetRole is a planning decision about how a source asset should be used.
// It never moves or mutates the file. Confidence is explicit because an
// uncertain role is more useful than a fabricated one.
type AssetRole struct {
	AssetID    string `json:"asset_id"`
	Path       string `json:"path,omitempty"`
	Role       string `json:"role"`
	Confidence string `json:"confidence"`
	Rationale  string `json:"rationale,omitempty"`
}

// Asset roles the treatment may assign. "unknown" is valid and expected.
const (
	RolePrimaryNarration = "primary_narration"
	RoleHeroShot         = "hero_shot"
	RoleHook             = "hook"
	RoleBRoll            = "b_roll"
	RoleAtmospheric      = "atmospheric_insert"
	RoleReaction         = "reaction"
	RoleTransition       = "transition_material"
	RoleOutro            = "outro"
	RoleAudioBed         = "audio_bed"
	RoleReference        = "reference_asset"
	RoleGeneratedCand    = "generated_asset_candidate"
	RoleUnknown          = "unknown"
)

const (
	ConfidenceHigh      = "high"
	ConfidenceMedium    = "medium"
	ConfidenceLow       = "low"
	ConfidenceUncertain = "uncertain"
)

// SegmentIntent is one intended piece of the edit. This is what the
// deterministic planner turns into real cuts — it is the load-bearing bridge
// between creative reasoning and execution.
type SegmentIntent struct {
	ID              string  `json:"id"`
	Order           int     `json:"order"`
	AssetID         string  `json:"asset_id,omitempty"`
	SourceIn        float64 `json:"source_in"`
	SourceOut       float64 `json:"source_out"`
	Purpose         string  `json:"purpose,omitempty"`
	BeatRef         string  `json:"beat_ref,omitempty"`
	DecisionID      string  `json:"decision_id,omitempty"`
	NeedsGeneration bool    `json:"needs_generation,omitempty"`
}

func (s SegmentIntent) Duration() float64 {
	if s.SourceOut <= s.SourceIn {
		return 0
	}
	return round3(s.SourceOut - s.SourceIn)
}

type AudioStrategy struct {
	UseSourceAudio bool     `json:"use_source_audio"`
	NarrationPlan  string   `json:"narration_plan,omitempty"`
	MusicPlan      string   `json:"music_plan,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

type CaptionStrategy struct {
	Required bool   `json:"required"`
	BurnIn   bool   `json:"burn_in"`
	Style    string `json:"style,omitempty"`
	Position string `json:"position,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// GeneratedNeed points at the existing asset-requirement vocabulary rather than
// introducing a second generation-request system.
type GeneratedNeed struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	Capability     string `json:"capability,omitempty"`
	SuggestedRoute string `json:"suggested_route,omitempty"`
	Required       bool   `json:"required"`
	Reason         string `json:"reason,omitempty"`
	// RequirementRef links to an existing asset_requirements.json entry when one
	// covers this need.
	RequirementRef string `json:"requirement_ref,omitempty"`
}

type TransformationNeed struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Capability  string `json:"capability,omitempty"`
	TargetAsset string `json:"target_asset,omitempty"`
}

// DegradedAlternative is what the treatment proposes if a need cannot be met.
// Stating the fallback up front is what lets the runtime degrade honestly
// instead of silently dropping the intent.
type DegradedAlternative struct {
	ID       string `json:"id"`
	ForNeed  string `json:"for_need"`
	Approach string `json:"approach"`
	Impact   string `json:"impact,omitempty"`
}

// SuccessCriterion is the treatment's own statement of what "done" means. The
// production validator turns supported criteria into real assertions.
type SuccessCriterion struct {
	ID        string  `json:"id"`
	Assertion string  `json:"assertion"`
	Target    string  `json:"target,omitempty"`
	Seconds   float64 `json:"seconds,omitempty"`
}

// TreatmentDecision is a citable creative decision. Production stages carry a
// treatment_decision_id pointing back here, so an executed ffmpeg command can
// be traced to the creative reason it exists.
type TreatmentDecision struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Rationale string `json:"rationale,omitempty"`
	Evidence  string `json:"evidence,omitempty"`
}

// CritiqueRecord holds the result of the single bounded critique pass. It
// stores findings and what changed — never hidden reasoning.
type CritiqueRecord struct {
	Performed bool     `json:"performed"`
	Findings  []string `json:"findings,omitempty"`
	Revisions []string `json:"revisions,omitempty"`
	Passes    int      `json:"passes"`
}

// ---- lookups ----

func (t CreativeTreatment) Decision(id string) (TreatmentDecision, bool) {
	for _, d := range t.Decisions {
		if d.ID == id {
			return d, true
		}
	}
	return TreatmentDecision{}, false
}

func (t CreativeTreatment) RoleFor(assetID string) (AssetRole, bool) {
	for _, r := range t.AssetRoles {
		if r.AssetID == assetID {
			return r, true
		}
	}
	return AssetRole{}, false
}

// OrderedSegments returns segments sorted by their declared order.
func (t CreativeTreatment) OrderedSegments() []SegmentIntent {
	out := append([]SegmentIntent(nil), t.Segments...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Order < out[j-1].Order; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ---- validation ----

// Validate rejects a structurally unusable treatment. The creative director may
// be a language model, so its output is treated as untrusted input: anything
// that fails here is discarded rather than fed to the planner.
func (t CreativeTreatment) Validate(obs Observation) error {
	if t.SchemaVersion != TreatmentSchemaVersion {
		return fmt.Errorf("schema_version %q != %q", t.SchemaVersion, TreatmentSchemaVersion)
	}
	if strings.TrimSpace(t.Objective) == "" {
		return fmt.Errorf("objective is empty")
	}
	switch t.Reasoning.EffectiveMode {
	case ReasoningLLM, ReasoningDeterministic:
	default:
		return fmt.Errorf("unknown effective reasoning mode %q", t.Reasoning.EffectiveMode)
	}
	// Rules-based output must never claim semantic reasoning.
	if t.Reasoning.SemanticReasoning && t.Reasoning.EffectiveMode != ReasoningLLM {
		return fmt.Errorf("semantic_reasoning is true but effective mode is %q", t.Reasoning.EffectiveMode)
	}

	known := map[string]Asset{}
	for _, a := range obs.Assets {
		known[a.ID] = a
	}

	seenRole := map[string]bool{}
	for i, r := range t.AssetRoles {
		if r.AssetID == "" {
			return fmt.Errorf("asset_roles[%d]: asset_id is empty", i)
		}
		if _, ok := known[r.AssetID]; !ok {
			return fmt.Errorf("asset_roles[%d]: unknown asset_id %q", i, r.AssetID)
		}
		if seenRole[r.AssetID] {
			return fmt.Errorf("asset_roles[%d]: duplicate asset_id %q", i, r.AssetID)
		}
		seenRole[r.AssetID] = true
		if r.Role == "" {
			return fmt.Errorf("asset_roles[%d]: role is empty (use %q rather than omitting)", i, RoleUnknown)
		}
	}

	seenSeg := map[string]bool{}
	for i, s := range t.Segments {
		if s.ID == "" {
			return fmt.Errorf("segments[%d]: id is empty", i)
		}
		if seenSeg[s.ID] {
			return fmt.Errorf("segments[%d]: duplicate id %q", i, s.ID)
		}
		seenSeg[s.ID] = true
		if s.NeedsGeneration {
			// A segment that requires generation need not reference a source asset.
			continue
		}
		asset, ok := known[s.AssetID]
		if !ok {
			return fmt.Errorf("segments[%d] (%s): unknown asset_id %q", i, s.ID, s.AssetID)
		}
		if s.SourceOut <= s.SourceIn {
			return fmt.Errorf("segments[%d] (%s): source_out %.3f must exceed source_in %.3f",
				i, s.ID, s.SourceOut, s.SourceIn)
		}
		// A model will happily invent in/out points past the end of a clip.
		if s.SourceOut > asset.DurationSeconds+0.05 {
			return fmt.Errorf("segments[%d] (%s): source_out %.3f exceeds asset %s duration %.3f",
				i, s.ID, s.SourceOut, s.AssetID, asset.DurationSeconds)
		}
		if s.SourceIn < 0 {
			return fmt.Errorf("segments[%d] (%s): source_in is negative", i, s.ID)
		}
	}

	for i, d := range t.Decisions {
		if d.ID == "" {
			return fmt.Errorf("decisions[%d]: id is empty", i)
		}
	}
	// Every segment decision reference must resolve, or the stage-level
	// provenance link would dangle.
	for _, s := range t.Segments {
		if s.DecisionID == "" {
			continue
		}
		if _, ok := t.Decision(s.DecisionID); !ok {
			return fmt.Errorf("segment %s references unknown decision %q", s.ID, s.DecisionID)
		}
	}
	return nil
}

// UsableSegments returns the segments that can drive real cuts today: those
// bound to an observed asset. Segments awaiting generation are excluded and
// reported separately so the gap stays visible.
func (t CreativeTreatment) UsableSegments() (usable []SegmentIntent, pendingGeneration []SegmentIntent) {
	for _, s := range t.OrderedSegments() {
		if s.NeedsGeneration || s.AssetID == "" {
			pendingGeneration = append(pendingGeneration, s)
			continue
		}
		usable = append(usable, s)
	}
	return usable, pendingGeneration
}

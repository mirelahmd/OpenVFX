package commands

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ---- planner interface ----

// AgentPlanner is the generic planner interface. All planner modes must implement this.
// Planners produce a list of AgentActionV1 given a PlannerContext.
// The plan contract (openvfx_agent_plan.v1) is fixed; the planner brain may change.
type AgentPlanner interface {
	Name() string
	// Plan returns a PlanResult containing actions, warnings, and resolved metadata.
	Plan(planCtx PlannerContext) (PlanResult, error)
}

// PlanResult is returned by AgentPlanner.Plan. It carries the actions plus
// rich metadata so the caller can populate the planner block in agent_plan.json.
type PlanResult struct {
	Actions         []AgentActionV1
	Warnings        []string
	EffectiveMode   string // planner that actually executed (differs from requested on fallback)
	FallbackUsed    bool
	FallbackReason  string
	ResolvedModel   string // model resolved from opts or config route
	ResolvedBackend string // backend URL resolved from opts or config
	ResolvedRoute   string // config route key that was used
	RequestArtifact *PlannerRequestArtifact
}

// PlannerRequestArtifact is written as planner_request.json in the plan directory
// for observability — it records what was sent to the planner brain.
type PlannerRequestArtifact struct {
	SchemaVersion  string         `json:"schema_version"`
	CreatedAt      time.Time      `json:"created_at"`
	PlannerMode    string         `json:"planner_mode"`
	EffectiveMode  string         `json:"effective_mode,omitempty"`
	Model          string         `json:"model,omitempty"`
	Backend        string         `json:"backend,omitempty"`
	Route          string         `json:"route,omitempty"`
	SystemPrompt   string         `json:"system_prompt,omitempty"`
	UserPrompt     string         `json:"user_prompt,omitempty"`
	GoalHints      map[string]any `json:"goal_hints,omitempty"`
	FallbackUsed   bool           `json:"fallback_used,omitempty"`
	FallbackReason string         `json:"fallback_reason,omitempty"`
}

// ---- planner context ----

// PlannerContext carries everything a planner needs to produce a plan.
type PlannerContext struct {
	Intent         string
	InputPath      string
	MakeID         string
	CreativePlanID string
	RunID          string
	Context        AgentPlanContextSnapshot
	Options        PlannerOptions
}

// PlannerOptions controls planner selection and behaviour.
type PlannerOptions struct {
	// PlannerName selects the planner: "deterministic" (default) or "ollama".
	PlannerName string
	// PlannerBackend overrides the Ollama base URL (e.g. "http://localhost:11434").
	PlannerBackend string
	// PlannerRoute is a config.models.routes key to use for model lookup.
	PlannerRoute string
	// PlannerModel overrides the model name (e.g. "llama3").
	PlannerModel string
	// FallbackDeterministic falls back to deterministic if the LLM planner fails.
	FallbackDeterministic bool
	// TimeoutSeconds is the planner HTTP call timeout (0 = 120s default).
	TimeoutSeconds int
	// Temperature controls LLM output randomness (0.0 = model default, -1 = unset).
	Temperature float64
	// MaxOutputChars truncates the raw LLM response before parsing (0 = no limit).
	MaxOutputChars int
	// AllowProviderCalls propagates into planned actions that require provider.
	AllowProviderCalls bool
	// AllowOverwrite propagates into planned action inputs.
	AllowOverwrite bool
	// Platform overrides the platform preset for make actions.
	Platform string
	// StyleDir overrides the style directory path.
	StyleDir string
}

// ---- schema validation ----

var agentActionIDRegexp = regexp.MustCompile(`^action_\d+$`)

var validAgentActionTypes = map[string]bool{
	agentActionTypeMake:       true,
	agentActionTypeReviseMake: true,
	agentActionTypeValidate:   true,
	agentActionTypeQueue:      true,
}

// validateAgentActions checks that a planner's output conforms to the v1 contract.
func validateAgentActions(actions []AgentActionV1) error {
	if len(actions) == 0 {
		return fmt.Errorf("planner returned zero actions")
	}
	if len(actions) > 10 {
		return fmt.Errorf("planner returned %d actions; maximum is 10", len(actions))
	}
	seen := map[string]bool{}
	for i, a := range actions {
		if !agentActionIDRegexp.MatchString(a.ID) {
			return fmt.Errorf("action[%d]: id %q does not match action_NNNN format", i, a.ID)
		}
		if seen[a.ID] {
			return fmt.Errorf("action[%d]: duplicate id %q", i, a.ID)
		}
		seen[a.ID] = true
		if !validAgentActionTypes[a.Type] {
			return fmt.Errorf("action[%d] id=%s: unknown type %q; allowed: %s", i, a.ID, a.Type,
				strings.Join(validAgentActionTypesList(), ", "))
		}
		if strings.TrimSpace(a.Description) == "" {
			return fmt.Errorf("action[%d] id=%s: description is empty", i, a.ID)
		}
	}
	return nil
}

func validAgentActionTypesList() []string {
	return []string{
		agentActionTypeMake,
		agentActionTypeReviseMake,
		agentActionTypeValidate,
		agentActionTypeQueue,
	}
}

// applyDefaultActionFields sets status and policy_status if not set by the planner.
func applyDefaultActionFields(actions []AgentActionV1) []AgentActionV1 {
	for i := range actions {
		if actions[i].Status == "" {
			actions[i].Status = "planned"
		}
		if actions[i].PolicyStatus == "" {
			actions[i].PolicyStatus = "review_pending"
		}
	}
	return actions
}

// ---- planner registry / dispatch ----

// selectPlanner returns the appropriate AgentPlanner for the given options.
// Default is always deterministic.
func selectPlanner(opts AgentPlanCommandOptions) AgentPlanner {
	switch strings.ToLower(opts.PlannerName) {
	case "ollama":
		return newOllamaAgentPlanner(opts)
	default:
		return deterministicAgentPlanner{}
	}
}

// buildPlannerContext converts AgentPlanCommandOptions + snapshot into a PlannerContext.
func buildPlannerContext(opts AgentPlanCommandOptions, ctx AgentPlanContextSnapshot) PlannerContext {
	return PlannerContext{
		Intent:         strings.TrimSpace(opts.Goal),
		InputPath:      opts.InputPath,
		MakeID:         opts.MakeID,
		CreativePlanID: opts.CreativePlanID,
		RunID:          opts.RunID,
		Context:        ctx,
		Options: PlannerOptions{
			PlannerName:           opts.PlannerName,
			PlannerBackend:        opts.PlannerBackend,
			PlannerRoute:          opts.PlannerRoute,
			PlannerModel:          opts.PlannerModel,
			FallbackDeterministic: opts.FallbackDeterministic,
			TimeoutSeconds:        opts.PlannerTimeoutSeconds,
			Temperature:           opts.PlannerTemperature,
			MaxOutputChars:        opts.PlannerMaxOutputChars,
			AllowProviderCalls:    opts.AllowProviderCalls,
			AllowOverwrite:        opts.AllowOverwrite,
			Platform:              opts.Platform,
			StyleDir:              opts.StyleDir,
		},
	}
}

// ---- deterministic planner ----

type deterministicAgentPlanner struct{}

func (deterministicAgentPlanner) Name() string { return "deterministic" }

func (p deterministicAgentPlanner) Plan(planCtx PlannerContext) (PlanResult, error) {
	opts := AgentPlanCommandOptions{
		Goal:               planCtx.Intent,
		InputPath:          planCtx.InputPath,
		MakeID:             planCtx.MakeID,
		CreativePlanID:     planCtx.CreativePlanID,
		RunID:              planCtx.RunID,
		AllowProviderCalls: planCtx.Options.AllowProviderCalls,
		AllowOverwrite:     planCtx.Options.AllowOverwrite,
		Platform:           planCtx.Options.Platform,
		StyleDir:           planCtx.Options.StyleDir,
	}
	hints := parseAgentGoalHints(planCtx.Intent, planCtx.Options.Platform, planCtx.Options.AllowProviderCalls, planCtx.Context)
	actions, warnings, errs := buildAgentActions(opts, hints)
	if len(errs) > 0 {
		return PlanResult{}, fmt.Errorf("deterministic planner: %s", strings.Join(errs, "; "))
	}

	goalHints := map[string]any{
		"has_input_media":   planCtx.InputPath != "",
		"has_make_id":       planCtx.MakeID != "",
		"platform":          planCtx.Options.Platform,
		"allow_provider":    planCtx.Options.AllowProviderCalls,
		"generate_script":   hints.GenerateScript,
		"generate_captions": hints.GenerateCaptions,
		"generate_voiceover": hints.GenerateVoiceover,
	}

	return PlanResult{
		Actions:       applyDefaultActionFields(actions),
		Warnings:      warnings,
		EffectiveMode: "deterministic",
		RequestArtifact: &PlannerRequestArtifact{
			SchemaVersion: "openvfx_planner_request.v1",
			CreatedAt:     time.Now().UTC(),
			PlannerMode:   "deterministic",
			EffectiveMode: "deterministic",
			GoalHints:     goalHints,
		},
	}, nil
}

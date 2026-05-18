package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/events"
)

const (
	agentPlansV1Root          = ".byom-video/agent_plans"
	agentPlanV1Schema         = "openvfx_agent_plan.v1"
	contextSnapshotV1Schema   = "openvfx_context_snapshot.v1"
	policyReviewV1Schema      = "openvfx_policy_review.v1"
	agentPlanStatusDraft      = "draft"
	agentPlanStatusReviewed   = "reviewed"
	agentPlanStatusApproved   = "approved"
	agentPlanStatusRejected   = "rejected"
	agentPlanStatusConverted  = "converted"
	agentPlanStatusFailed     = "failed"
	agentActionTypeMake       = "make"
	agentActionTypeReviseMake = "revise_make"
	agentActionTypeQueue      = "queue_health"
	agentActionTypeValidate   = "validate_creative_assemble"
)

type AgentPlanCommandOptions struct {
	Goal               string
	InputPath          string
	MakeID             string
	CreativePlanID     string
	RunID              string
	JSON               bool
	WriteReview        bool
	AllowProviderCalls bool
	AllowOverwrite     bool
	Platform           string
	StyleDir           string
	DryRun             bool

	// Planner selection
	PlannerName           string  // "deterministic" (default) | "ollama"
	PlannerBackend        string  // Ollama base URL or backend key
	PlannerRoute          string  // config.models.routes key for model lookup
	PlannerModel          string  // explicit model name (e.g. "llama3")
	FallbackDeterministic bool    // fall back to deterministic if LLM planner fails
	PlannerTimeoutSeconds int     // planner HTTP timeout (0 = 120s)
	PlannerTemperature    float64 // LLM temperature (0 = model default)
	PlannerMaxOutputChars int     // truncate raw LLM output before parsing (0 = no limit)
}

type AgentPlansListOptions struct {
	JSON   bool
	Status string
	Limit  int
}

type InspectAgentPlanCommandOptions struct{ JSON bool }
type ReviewAgentPlanCommandOptions struct {
	JSON          bool
	WriteArtifact bool
}
type AgentPolicyCommandOptions struct{ JSON bool }

type AgentPlanV1 struct {
	SchemaVersion   string              `json:"schema_version"`
	PlanID          string              `json:"plan_id"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Status          string              `json:"status"`
	ApprovedAt      *time.Time          `json:"approved_at,omitempty"`
	ApprovalMode    string              `json:"approval_mode,omitempty"`
	RejectedAt      *time.Time          `json:"rejected_at,omitempty"`
	RejectionReason string              `json:"rejection_reason,omitempty"`
	Intent          string              `json:"intent"`
	Input           AgentPlanInput      `json:"input"`
	Planner         AgentPlannerInfo    `json:"planner"`
	Actions         []AgentActionV1     `json:"actions"`
	References      AgentPlanReferences `json:"references"`
	Warnings        []string            `json:"warnings,omitempty"`
	Errors          []string            `json:"errors,omitempty"`
	NextCommands    []string            `json:"next_commands,omitempty"`
}

type AgentPlanInput struct {
	MediaPath      string `json:"media_path,omitempty"`
	Goal           string `json:"goal"`
	MakeID         string `json:"make_id,omitempty"`
	CreativePlanID string `json:"creative_plan_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
}

type AgentPlannerInfo struct {
	// Mode is kept for backward compatibility; same as RequestedMode.
	Mode           string `json:"mode"`
	RequestedMode  string `json:"requested_mode"`
	EffectiveMode  string `json:"effective_mode"`
	Backend        string `json:"backend,omitempty"`
	Route          string `json:"route,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model"`
	Version        string `json:"version"`
	FallbackUsed   bool   `json:"fallback_used,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}

type AgentActionV1 struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	Description       string         `json:"description"`
	Input             map[string]any `json:"input"`
	RequiresApproval  bool           `json:"requires_approval"`
	RequiresProvider  bool           `json:"requires_provider"`
	RequiresNetwork   bool           `json:"requires_network"`
	RequiresOverwrite bool           `json:"requires_overwrite"`
	PolicyStatus      string         `json:"policy_status"`
	Status            string         `json:"status"`
}

type AgentPlanReferences struct {
	ContextSnapshot   string `json:"context_snapshot"`
	PolicyReview      string `json:"policy_review"`
	PlanReview        string `json:"plan_review"`
	PlannerRequest    string `json:"planner_request,omitempty"`
	CreativeBrief     string `json:"creative_brief,omitempty"`
	Deliverables      string `json:"deliverables,omitempty"`
	AssetRequirements string `json:"asset_requirements,omitempty"`
	VisualRequests    string `json:"visual_requests,omitempty"`
}

type AgentPlanContextSnapshot struct {
	SchemaVersion string                   `json:"schema_version"`
	CreatedAt     time.Time                `json:"created_at"`
	SafeToDelete  bool                     `json:"safe_to_delete"`
	TTLDays       int                      `json:"ttl_days"`
	InputMedia    AgentPlanInputMedia      `json:"input_media"`
	StylePack     AgentPlanStylePack       `json:"style_pack"`
	Runtime       AgentPlanRuntimeSnapshot `json:"runtime"`
	Capabilities  AgentPlanCapabilities    `json:"capabilities"`
	Warnings      []string                 `json:"warnings,omitempty"`
}

type AgentPlanInputMedia struct {
	Path      string `json:"path,omitempty"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Extension string `json:"extension,omitempty"`
}

type AgentPlanStylePack struct {
	Exists   bool     `json:"exists"`
	Path     string   `json:"path"`
	Warnings []string `json:"warnings,omitempty"`
}

type AgentPlanRuntimeSnapshot struct {
	QueueStatus    string `json:"queue_status"`
	DaemonStatus   string `json:"daemon_status"`
	WorkerStatus   string `json:"worker_status"`
	PendingJobs    int    `json:"pending_jobs"`
	ApprovalNeeded int    `json:"approval_needed"`
	FailedJobs     int    `json:"failed_jobs"`
}

type AgentPlanCapabilities struct {
	ScriptGeneration  string `json:"script_generation"`
	CaptionGeneration string `json:"caption_generation"`
	VoiceGeneration   string `json:"voice_generation"`
	CaptionBurn       string `json:"caption_burn"`
}

type AgentPlanPolicyReview struct {
	SchemaVersion               string             `json:"schema_version"`
	CreatedAt                   time.Time          `json:"created_at"`
	PlanID                      string             `json:"plan_id"`
	Status                      string             `json:"status"`
	RequiresUserApproval        bool               `json:"requires_user_approval"`
	RequiresProviderPermission  bool               `json:"requires_provider_permission"`
	RequiresExternalNetwork     bool               `json:"requires_external_network"`
	RequiresOverwritePermission bool               `json:"requires_overwrite_permission"`
	Checks                      []AgentPolicyCheck `json:"checks"`
	Warnings                    []string           `json:"warnings,omitempty"`
	Errors                      []string           `json:"errors,omitempty"`
	NextCommands                []string           `json:"next_commands,omitempty"`
}

type AgentPolicyCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type agentPlanDraft struct {
	Plan              AgentPlanV1
	Context           AgentPlanContextSnapshot
	Policy            AgentPlanPolicyReview
	Review            string
	PlannerRequest    *PlannerRequestArtifact
	CreativeBrief     CreativeBriefArtifact
	Deliverables      DeliverablesArtifact
	AssetRequirements AssetRequirementsArtifact
	VisualRequests    VisualRequestsDryRunArtifact
}

type agentGoalHints struct {
	Platform          string
	GenerateCaptions  bool
	BurnCaptions      bool
	GenerateScript    bool
	PrepareVoiceover  bool
	GenerateVoiceover bool
	CaptionPosition   string
	CaptionStyle      string
	Reassemble        bool
	Validate          bool
	Warnings          []string
}

func AgentPlanCommand(stdout io.Writer, opts AgentPlanCommandOptions) error {
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("--goal is required")
	}
	draft, err := buildAgentPlanDraft(opts)
	if err != nil {
		return err
	}
	if opts.DryRun {
		if opts.JSON {
			payload := map[string]any{
				"plan":               draft.Plan,
				"context_snapshot":   draft.Context,
				"policy_review":      draft.Policy,
				"creative_brief":     draft.CreativeBrief,
				"deliverables":       draft.Deliverables,
				"asset_requirements": draft.AssetRequirements,
				"visual_requests":    draft.VisualRequests,
				"plan_review":        draft.Review,
			}
			data, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Agent plan dry-run")
		fmt.Fprintf(stdout, "  intent:    %s\n", draft.Plan.Intent)
		fmt.Fprintf(stdout, "  status:    %s\n", draft.Policy.Status)
		for _, action := range draft.Plan.Actions {
			fmt.Fprintf(stdout, "  action:    %s (%s) policy=%s\n", action.Type, action.ID, action.PolicyStatus)
		}
		for _, warning := range dedupeStrings(append(append([]string{}, draft.Plan.Warnings...), draft.Policy.Warnings...)) {
			fmt.Fprintf(stdout, "  warning:   %s\n", warning)
		}
		return nil
	}

	planDir := filepath.Join(agentPlansV1Root, draft.Plan.PlanID)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("AGENT_PLAN_CREATED", map[string]any{"plan_id": draft.Plan.PlanID, "intent": draft.Plan.Intent})
	}
	if err := writeJSONFile(filepath.Join(planDir, "agent_plan.json"), draft.Plan); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if err := writeJSONFile(filepath.Join(planDir, "context_snapshot.json"), draft.Context); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_CONTEXT_SNAPSHOT_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if err := writeJSONFile(filepath.Join(planDir, creativeBriefArtifact), draft.CreativeBrief); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_CREATIVE_BRIEF_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if err := writeJSONFile(filepath.Join(planDir, deliverablesArtifact), draft.Deliverables); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_DELIVERABLES_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if err := writeJSONFile(filepath.Join(planDir, assetRequirementsArtifact), draft.AssetRequirements); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_ASSET_REQUIREMENTS_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if err := writeJSONFile(filepath.Join(planDir, visualRequestsArtifact), draft.VisualRequests); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_VISUAL_REQUESTS_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if err := writeJSONFile(filepath.Join(planDir, "policy_review.json"), draft.Policy); err != nil {
		if log != nil {
			_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
			_ = log.Close()
		}
		return err
	}
	if log != nil {
		_ = log.Write("AGENT_POLICY_REVIEW_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
	}
	if opts.WriteReview {
		if err := os.WriteFile(filepath.Join(planDir, "plan_review.md"), []byte(draft.Review), 0o644); err != nil {
			if log != nil {
				_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
				_ = log.Close()
			}
			return err
		}
		if log != nil {
			_ = log.Write("AGENT_PLAN_REVIEW_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
		}
	}
	if draft.PlannerRequest != nil {
		if err := writeJSONFile(filepath.Join(planDir, "planner_request.json"), draft.PlannerRequest); err != nil {
			if log != nil {
				_ = log.Write("AGENT_PLAN_FAILED", map[string]any{"plan_id": draft.Plan.PlanID, "error": err.Error()})
				_ = log.Close()
			}
			return err
		}
		if log != nil {
			_ = log.Write("AGENT_PLANNER_REQUEST_WRITTEN", map[string]any{"plan_id": draft.Plan.PlanID})
		}
	}
	if log != nil {
		_ = log.Close()
	}
	if opts.JSON {
		data, err := json.MarshalIndent(draft.Plan, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Agent plan")
	fmt.Fprintf(stdout, "  plan id:    %s\n", draft.Plan.PlanID)
	fmt.Fprintf(stdout, "  status:     %s\n", draft.Plan.Status)
	fmt.Fprintf(stdout, "  policy:     %s\n", draft.Policy.Status)
	fmt.Fprintf(stdout, "  actions:    %d\n", len(draft.Plan.Actions))
	fmt.Fprintf(stdout, "  path:       %s\n", planDir)
	if opts.WriteReview {
		fmt.Fprintf(stdout, "  review:     %s\n", filepath.Join(planDir, "plan_review.md"))
	}
	return nil
}

func AgentPlans(stdout io.Writer, opts AgentPlansListOptions) error {
	plans, err := listAgentPlans()
	if err != nil {
		return err
	}
	if opts.Status != "" {
		filtered := make([]AgentPlanV1, 0, len(plans))
		for _, plan := range plans {
			if plan.Status == opts.Status {
				filtered = append(filtered, plan)
			}
		}
		plans = filtered
	}
	if opts.Limit > 0 && len(plans) > opts.Limit {
		plans = plans[:opts.Limit]
	}
	if opts.JSON {
		data, err := json.MarshalIndent(plans, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	if len(plans) == 0 {
		fmt.Fprintln(stdout, "No agent plans found.")
		return nil
	}
	fmt.Fprintf(stdout, "%-28s %-10s %-28s %-20s %s\n", "PLAN ID", "STATUS", "ACTIONS", "CREATED AT", "INTENT")
	for _, plan := range plans {
		actionTypes := []string{}
		for _, action := range plan.Actions {
			actionTypes = append(actionTypes, action.Type)
		}
		fmt.Fprintf(stdout, "%-28s %-10s %-28s %-20s %s\n",
			plan.PlanID,
			plan.Status,
			truncate(strings.Join(actionTypes, ","), 28),
			plan.CreatedAt.Format(time.RFC3339),
			truncate(plan.Intent, 60),
		)
	}
	return nil
}

func InspectAgentPlan(planID string, stdout io.Writer, opts InspectAgentPlanCommandOptions) error {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	ctx, _ := readAgentPlanContext(planID)
	policy, _ := readAgentPlanPolicy(planID)
	brief, _ := readCreativeBrief(planID)
	deliverables, _ := readDeliverables(planID)
	assets, _ := readAssetRequirements(planID)
	visualRequests, _ := readVisualRequestsDryRun(planID)
	if opts.JSON {
		payload := map[string]any{
			"plan":               plan,
			"context_snapshot":   ctx,
			"policy_review":      policy,
			"creative_brief":     brief,
			"deliverables":       deliverables,
			"asset_requirements": assets,
			"visual_requests":    visualRequests,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Agent plan")
	fmt.Fprintf(stdout, "  plan id:    %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  status:     %s\n", plan.Status)
	if plan.ApprovedAt != nil {
		fmt.Fprintf(stdout, "  approved:   %s (%s)\n", plan.ApprovedAt.Format(time.RFC3339), emptyDash(plan.ApprovalMode))
	}
	if plan.RejectedAt != nil {
		fmt.Fprintf(stdout, "  rejected:   %s (%s)\n", plan.RejectedAt.Format(time.RFC3339), emptyDash(plan.RejectionReason))
	}
	fmt.Fprintf(stdout, "  intent:     %s\n", plan.Intent)
	fmt.Fprintf(stdout, "  planner:    %s/%s\n", plan.Planner.Mode, plan.Planner.Version)
	fmt.Fprintf(stdout, "  policy:     %s\n", policy.Status)
	fmt.Fprintf(stdout, "  references: %s, %s, %s\n", plan.References.ContextSnapshot, plan.References.PolicyReview, plan.References.PlanReview)
	if plan.References.CreativeBrief != "" {
		fmt.Fprintf(stdout, "  brief:      %s\n", filepath.Join(agentPlansV1Root, planID, plan.References.CreativeBrief))
	}
	if plan.References.Deliverables != "" {
		fmt.Fprintf(stdout, "  deliverables: %s\n", filepath.Join(agentPlansV1Root, planID, plan.References.Deliverables))
	}
	if plan.References.AssetRequirements != "" {
		fmt.Fprintf(stdout, "  assets:     %s\n", filepath.Join(agentPlansV1Root, planID, plan.References.AssetRequirements))
	}
	if plan.References.VisualRequests != "" {
		fmt.Fprintf(stdout, "  visual dry-run: %s\n", filepath.Join(agentPlansV1Root, planID, plan.References.VisualRequests))
		if visualRequests.SchemaVersion != "" {
			fmt.Fprintf(stdout, "  visual requests: %d missing=%d\n", len(visualRequests.Requests), len(visualRequests.MissingCapabilities))
		}
	}
	for _, action := range plan.Actions {
		fmt.Fprintf(stdout, "  action:     %s %s status=%s policy=%s\n", action.ID, action.Type, action.Status, action.PolicyStatus)
	}
	if ctx.Runtime.QueueStatus != "" {
		fmt.Fprintf(stdout, "  runtime:    queue=%s daemon=%s worker=%s\n", ctx.Runtime.QueueStatus, ctx.Runtime.DaemonStatus, ctx.Runtime.WorkerStatus)
	}
	if linked, err := readAgentLinkedJobs(planID); err == nil {
		linked = refreshAgentLinkedJobsInMemory(linked)
		fmt.Fprintf(stdout, "  linked jobs: %d (%s)\n", len(linked.Jobs), filepath.Join(agentPlansV1Root, planID, "linked_jobs.json"))
		for _, job := range linked.Jobs {
			fmt.Fprintf(stdout, "    - %s %s approval=%s\n", job.JobID, job.Status, job.ApprovalStatus)
		}
	}
	for _, name := range []string{"agent_result.md", "agent_run_summary.json"} {
		path := filepath.Join(agentPlansV1Root, planID, name)
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(stdout, "  %s: %s\n", strings.TrimSuffix(name, filepath.Ext(name)), path)
		}
	}
	return nil
}

func ReviewAgentPlan(planID string, stdout io.Writer, opts ReviewAgentPlanCommandOptions) error {
	plan, err := readAgentPlan(planID)
	if err != nil {
		return err
	}
	ctx, _ := readAgentPlanContext(planID)
	policy, _ := readAgentPlanPolicy(planID)
	review := renderAgentPlanReview(plan, ctx, policy)
	if opts.WriteArtifact {
		path := filepath.Join(agentPlansV1Root, planID, "plan_review.md")
		if err := os.WriteFile(path, []byte(review), 0o644); err != nil {
			return err
		}
		log, _ := events.Open(filepath.Join(agentPlansV1Root, planID, "events.jsonl"))
		if log != nil {
			_ = log.Write("AGENT_PLAN_REVIEW_WRITTEN", map[string]any{"plan_id": planID, "path": path})
			_ = log.Close()
		}
	}
	if opts.JSON {
		payload := map[string]any{
			"plan_id":         plan.PlanID,
			"status":          plan.Status,
			"policy_status":   policy.Status,
			"review_markdown": review,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprint(stdout, review)
	return nil
}

func AgentPolicy(planID string, stdout io.Writer, opts AgentPolicyCommandOptions) error {
	policy, err := readAgentPlanPolicy(planID)
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := json.MarshalIndent(policy, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Agent policy")
	fmt.Fprintf(stdout, "  plan id:      %s\n", policy.PlanID)
	fmt.Fprintf(stdout, "  status:       %s\n", policy.Status)
	fmt.Fprintf(stdout, "  approval:     %t\n", policy.RequiresUserApproval)
	fmt.Fprintf(stdout, "  provider:     %t\n", policy.RequiresProviderPermission)
	fmt.Fprintf(stdout, "  network:      %t\n", policy.RequiresExternalNetwork)
	fmt.Fprintf(stdout, "  overwrite:    %t\n", policy.RequiresOverwritePermission)
	for _, check := range policy.Checks {
		fmt.Fprintf(stdout, "  - %s: %s (%s)\n", check.ID, check.Status, check.Message)
	}
	return nil
}

func buildAgentPlanDraft(opts AgentPlanCommandOptions) (agentPlanDraft, error) {
	now := time.Now().UTC()
	planID := "agentplan-" + now.Format("20060102T150405.000000000Z")
	ctx := observeAgentPlanContext(opts, now)

	// Dispatch through the planner adapter interface.
	planner := selectPlanner(opts)
	planCtx := buildPlannerContext(opts, ctx)
	planResult, planErr := planner.Plan(planCtx)
	if planErr != nil {
		return agentPlanDraft{}, planErr
	}

	// Collect hint warnings for the deterministic planner (non-deterministic planners
	// return their own warnings directly).
	warnings := planResult.Warnings
	if planner.Name() == "deterministic" {
		hints := parseAgentGoalHints(opts.Goal, opts.Platform, opts.AllowProviderCalls, ctx)
		warnings = dedupeStrings(append(warnings, hints.Warnings...))
	}

	requestedMode := planner.Name()
	effectiveMode := planResult.EffectiveMode
	if effectiveMode == "" {
		effectiveMode = requestedMode
	}

	refs := AgentPlanReferences{
		ContextSnapshot:   "context_snapshot.json",
		PolicyReview:      "policy_review.json",
		PlanReview:        "plan_review.md",
		CreativeBrief:     creativeBriefArtifact,
		Deliverables:      deliverablesArtifact,
		AssetRequirements: assetRequirementsArtifact,
		VisualRequests:    visualRequestsArtifact,
	}
	if planResult.RequestArtifact != nil {
		refs.PlannerRequest = "planner_request.json"
	}

	brief, deliverables, assets := buildCreativePlanningArtifacts(planID, now, opts, ctx)
	visualRequests := buildVisualRequestsDryRun(planID, now, brief, assets)
	planResult.Actions = enrichAgentActionsWithCreativeBrief(planResult.Actions, brief, deliverables, assets)
	plan := AgentPlanV1{
		SchemaVersion: agentPlanV1Schema,
		PlanID:        planID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Status:        agentPlanStatusDraft,
		Intent:        strings.TrimSpace(opts.Goal),
		Input: AgentPlanInput{
			MediaPath:      opts.InputPath,
			Goal:           strings.TrimSpace(opts.Goal),
			MakeID:         opts.MakeID,
			CreativePlanID: opts.CreativePlanID,
			RunID:          opts.RunID,
		},
		Planner: AgentPlannerInfo{
			Mode:           requestedMode,
			RequestedMode:  requestedMode,
			EffectiveMode:  effectiveMode,
			Backend:        planResult.ResolvedBackend,
			Route:          planResult.ResolvedRoute,
			Provider:       plannerProvider(requestedMode),
			Model:          planResult.ResolvedModel,
			Version:        "v1",
			FallbackUsed:   planResult.FallbackUsed,
			FallbackReason: planResult.FallbackReason,
		},
		Actions:    planResult.Actions,
		References: refs,
		Warnings:   dedupeStrings(warnings),
	}
	policy := buildAgentPolicyReview(plan, ctx)
	plan.NextCommands = buildAgentPlanNextCommands(plan, policy)
	policy.NextCommands = append([]string{}, plan.NextCommands...)
	review := renderAgentPlanReview(plan, ctx, policy)
	return agentPlanDraft{Plan: plan, Context: ctx, Policy: policy, Review: review, PlannerRequest: planResult.RequestArtifact, CreativeBrief: brief, Deliverables: deliverables, AssetRequirements: assets, VisualRequests: visualRequests}, nil
}

// plannerProvider maps a planner mode name to a provider label for the artifact.
func plannerProvider(mode string) string {
	switch strings.ToLower(mode) {
	case "ollama":
		return "ollama"
	default:
		return "deterministic"
	}
}

func observeAgentPlanContext(opts AgentPlanCommandOptions, now time.Time) AgentPlanContextSnapshot {
	ctx := AgentPlanContextSnapshot{
		SchemaVersion: contextSnapshotV1Schema,
		CreatedAt:     now,
		SafeToDelete:  true,
		TTLDays:       7,
		InputMedia: AgentPlanInputMedia{
			Path: opts.InputPath,
		},
		StylePack: AgentPlanStylePack{
			Path: defaultStyleDir,
		},
		Runtime: AgentPlanRuntimeSnapshot{
			QueueStatus:  "unknown",
			DaemonStatus: "unknown",
			WorkerStatus: "unknown",
		},
		Capabilities: AgentPlanCapabilities{
			ScriptGeneration:  "unknown",
			CaptionGeneration: "unknown",
			VoiceGeneration:   "unknown",
			CaptionBurn:       "unknown",
		},
	}
	if opts.StyleDir != "" {
		ctx.StylePack.Path = opts.StyleDir
	}
	if opts.InputPath != "" {
		if info, err := os.Stat(opts.InputPath); err == nil {
			ctx.InputMedia.Exists = true
			ctx.InputMedia.SizeBytes = info.Size()
			ctx.InputMedia.Extension = strings.ToLower(filepath.Ext(opts.InputPath))
		} else {
			ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("input media not found: %s", opts.InputPath))
		}
	}
	if _, err := os.Stat(ctx.StylePack.Path); err == nil {
		ctx.StylePack.Exists = true
	} else {
		ctx.StylePack.Warnings = append(ctx.StylePack.Warnings, "style pack missing")
	}
	if summary, err := buildQueueSummary(5, 30*time.Minute, defaultQueueDeps); err == nil {
		ctx.Runtime.QueueStatus = deriveQueueHealthStatus(summary, false)
		ctx.Runtime.DaemonStatus = summary.Daemon.Status
		ctx.Runtime.WorkerStatus = summary.Worker.Status
		ctx.Runtime.PendingJobs = summary.Jobs.ByStatus[JobStatusPending]
		ctx.Runtime.ApprovalNeeded = len(summary.Jobs.ApprovalNeeded)
		ctx.Runtime.FailedJobs = len(summary.Jobs.Failed)
	} else {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("queue observation failed: %v", err))
	}
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("config observation failed: %v", err))
		return ctx
	}
	scriptBackend := cfg.Tools.Routes["creative.script"]
	captionBackend := cfg.Tools.Routes["creative.captions"]
	voiceBackend := cfg.Tools.Routes["creative.voiceover"]
	if scriptBackend != "" {
		ctx.Capabilities.ScriptGeneration = "available"
	} else {
		ctx.Capabilities.ScriptGeneration = "missing"
	}
	if captionBackend != "" || scriptBackend != "" {
		ctx.Capabilities.CaptionGeneration = "available"
	} else {
		ctx.Capabilities.CaptionGeneration = "missing"
	}
	if voiceBackend == "" {
		ctx.Capabilities.VoiceGeneration = "missing"
	} else {
		backend := cfg.Tools.Backends[voiceBackend]
		switch {
		case backend.Auth.Env == "":
			ctx.Capabilities.VoiceGeneration = "available"
		case os.Getenv(backend.Auth.Env) == "":
			ctx.Capabilities.VoiceGeneration = "missing_env"
		default:
			ctx.Capabilities.VoiceGeneration = "available"
		}
	}
	ctx.Capabilities.CaptionBurn = "unknown"
	return ctx
}

func parseAgentGoalHints(goal string, platformOverride string, allowProviderCalls bool, ctx AgentPlanContextSnapshot) agentGoalHints {
	lower := strings.ToLower(goal)
	hints := agentGoalHints{}
	switch {
	case strings.Contains(lower, "tik tok"), strings.Contains(lower, "tiktok"):
		hints.Platform = "tiktok"
	case strings.Contains(lower, "instagram"), strings.Contains(lower, "reel"), strings.Contains(lower, "reels"), strings.Contains(lower, " ig "):
		hints.Platform = "instagram-reel"
	case strings.Contains(lower, "youtube short"), strings.Contains(lower, "shorts"), strings.Contains(lower, "yt-short"):
		hints.Platform = "youtube-short"
	case strings.Contains(lower, "square"):
		hints.Platform = "square"
	case strings.Contains(lower, "youtube"), strings.Contains(lower, " yt "):
		hints.Platform = "youtube"
	case strings.Contains(lower, "vertical"):
		hints.Platform = "instagram-reel"
	default:
		hints.Platform = "original"
	}
	if platformOverride != "" {
		hints.Platform = platformOverride
	}
	if strings.Contains(lower, "captions") || strings.Contains(lower, "subtitles") || strings.Contains(lower, "text on screen") || strings.Contains(lower, "caption variants") || strings.Contains(lower, "caption options") {
		hints.GenerateCaptions = true
		hints.BurnCaptions = true
		if ctx.Capabilities.CaptionBurn == "missing" {
			hints.Warnings = append(hints.Warnings, "caption burn capability is missing; captions can still be generated without burn-in")
			hints.BurnCaptions = false
		}
	}
	if strings.Contains(lower, "script") || strings.Contains(lower, "hook") || strings.Contains(lower, "ad copy") || strings.Contains(lower, "narration") || strings.Contains(lower, "voiceover") {
		hints.GenerateScript = true
	}
	if strings.Contains(lower, "narration") || strings.Contains(lower, "voiceover") || strings.Contains(lower, "spoken") || strings.Contains(lower, "read aloud") {
		hints.PrepareVoiceover = true
		if allowProviderCalls && ctx.Capabilities.VoiceGeneration == "available" {
			hints.GenerateVoiceover = true
		} else {
			hints.Warnings = append(hints.Warnings, "voiceover requested; planner limited to prepare_voiceover without provider permission/config")
		}
	}
	if strings.Contains(lower, "boxed captions") {
		hints.CaptionStyle = "boxed"
	} else if strings.Contains(lower, "bold captions") {
		hints.CaptionStyle = "bold"
	}
	switch {
	case strings.Contains(lower, "captions center"), strings.Contains(lower, "subtitles center"):
		hints.CaptionPosition = "center"
	case strings.Contains(lower, "captions top"), strings.Contains(lower, "subtitles top"):
		hints.CaptionPosition = "top"
	case strings.Contains(lower, "captions bottom"), strings.Contains(lower, "subtitles bottom"):
		hints.CaptionPosition = "bottom"
	}
	if strings.Contains(lower, "switch platform") || strings.Contains(lower, "reassemble") || strings.Contains(lower, "boxed captions") || strings.Contains(lower, "bold captions") || strings.Contains(lower, "captions") {
		hints.Reassemble = true
	}
	if strings.Contains(lower, "validate") || strings.Contains(lower, "check") {
		hints.Validate = true
	}
	return hints
}

func buildAgentActions(opts AgentPlanCommandOptions, hints agentGoalHints) ([]AgentActionV1, []string, []string) {
	actions := []AgentActionV1{}
	warnings := []string{}
	errors := []string{}
	intent := strings.ToLower(opts.Goal)
	actionType := ""
	switch {
	case strings.Contains(intent, "queue") || strings.Contains(intent, "health") || strings.Contains(intent, "status"):
		actionType = agentActionTypeQueue
	case opts.MakeID != "":
		actionType = agentActionTypeReviseMake
	case opts.CreativePlanID != "" && (strings.Contains(intent, "validate") || strings.Contains(intent, "check")):
		actionType = agentActionTypeValidate
	case opts.InputPath != "":
		actionType = agentActionTypeMake
	default:
		actionType = agentActionTypeQueue
		warnings = append(warnings, "planner defaulted to queue_health because no input media, make_id, or creative_plan_id was provided")
	}

	switch actionType {
	case agentActionTypeMake:
		input := map[string]any{
			"input_path":             opts.InputPath,
			"goal":                   opts.Goal,
			"platform":               hints.Platform,
			"generate_script":        hints.GenerateScript,
			"generate_captions":      hints.GenerateCaptions,
			"prepare_voiceover":      hints.PrepareVoiceover,
			"generate_voiceover":     hints.GenerateVoiceover,
			"burn_captions":          hints.BurnCaptions,
			"allow_missing_captions": true,
			"mix_voiceover":          false,
			"overwrite":              opts.AllowOverwrite,
		}
		if hints.CaptionPosition != "" {
			input["caption_position"] = hints.CaptionPosition
		}
		if hints.CaptionStyle != "" {
			input["caption_style"] = hints.CaptionStyle
		}
		actions = append(actions, AgentActionV1{
			ID:                "action_0001",
			Type:              agentActionTypeMake,
			Description:       "Create a make plan from input media and the user goal.",
			Input:             input,
			RequiresApproval:  true,
			RequiresProvider:  hints.GenerateVoiceover,
			RequiresNetwork:   hints.GenerateVoiceover,
			RequiresOverwrite: opts.AllowOverwrite,
			PolicyStatus:      "review_pending",
			Status:            "planned",
		})
	case agentActionTypeReviseMake:
		if opts.MakeID == "" {
			errors = append(errors, "make_id is required for revise_make")
		}
		actions = append(actions, AgentActionV1{
			ID:          "action_0001",
			Type:        agentActionTypeReviseMake,
			Description: "Revise an existing make based on the user request.",
			Input: map[string]any{
				"make_id":    opts.MakeID,
				"request":    opts.Goal,
				"reassemble": hints.Reassemble,
				"validate":   hints.Validate,
				"overwrite":  opts.AllowOverwrite,
			},
			RequiresApproval:  true,
			RequiresProvider:  false,
			RequiresNetwork:   false,
			RequiresOverwrite: opts.AllowOverwrite,
			PolicyStatus:      "review_pending",
			Status:            "planned",
		})
	case agentActionTypeValidate:
		if opts.CreativePlanID == "" {
			errors = append(errors, "creative_plan_id is required for validate_creative_assemble")
		}
		actions = append(actions, AgentActionV1{
			ID:          "action_0001",
			Type:        agentActionTypeValidate,
			Description: "Validate a creative assemble plan.",
			Input: map[string]any{
				"creative_plan_id": opts.CreativePlanID,
			},
			RequiresApproval:  false,
			RequiresProvider:  false,
			RequiresNetwork:   false,
			RequiresOverwrite: false,
			PolicyStatus:      "review_pending",
			Status:            "planned",
		})
	case agentActionTypeQueue:
		actions = append(actions, AgentActionV1{
			ID:          "action_0001",
			Type:        agentActionTypeQueue,
			Description: "Inspect runtime queue health and attention items.",
			Input: map[string]any{
				"strict":      false,
				"stale_after": "30m",
			},
			RequiresApproval:  false,
			RequiresProvider:  false,
			RequiresNetwork:   false,
			RequiresOverwrite: false,
			PolicyStatus:      "review_pending",
			Status:            "planned",
		})
	}
	return actions, warnings, errors
}

func buildAgentPolicyReview(plan AgentPlanV1, ctx AgentPlanContextSnapshot) AgentPlanPolicyReview {
	policy := AgentPlanPolicyReview{
		SchemaVersion: contextless(policyReviewV1Schema),
		CreatedAt:     plan.CreatedAt,
		PlanID:        plan.PlanID,
		Status:        "allowed",
	}
	addCheck := func(id, status, message string) {
		policy.Checks = append(policy.Checks, AgentPolicyCheck{ID: id, Status: status, Message: message})
	}
	for i := range plan.Actions {
		action := &plan.Actions[i]
		switch action.Type {
		case agentActionTypeMake, agentActionTypeReviseMake:
			policy.RequiresUserApproval = true
			action.PolicyStatus = "approval_required"
			addCheck(action.Type+"_approval", "approval_required", fmt.Sprintf("%s requires user approval", action.Type))
		case agentActionTypeQueue, agentActionTypeValidate:
			if action.PolicyStatus == "review_pending" {
				action.PolicyStatus = "allowed"
			}
			addCheck(action.Type+"_approval", "ok", fmt.Sprintf("%s can run without extra approval", action.Type))
		}
		if action.RequiresProvider {
			policy.RequiresProviderPermission = true
			policy.RequiresExternalNetwork = true
			action.PolicyStatus = "approval_required"
			addCheck(action.Type+"_provider", "approval_required", "provider generation requested; explicit permission required")
		}
		if action.RequiresOverwrite {
			policy.RequiresOverwritePermission = true
			action.PolicyStatus = "approval_required"
			addCheck(action.Type+"_overwrite", "approval_required", "overwrite requested; explicit permission required")
		}
		if action.Type == agentActionTypeMake && inputString(action.Input, "input_path") == "" {
			action.PolicyStatus = "blocked"
			action.Status = "blocked"
			policy.Errors = append(policy.Errors, "missing input media path for make plan")
			addCheck("input_media", "blocked", "input media path is required for make plans")
		} else if action.Type == agentActionTypeMake {
			if !ctx.InputMedia.Exists {
				action.PolicyStatus = "blocked"
				action.Status = "blocked"
				policy.Errors = append(policy.Errors, "input media is missing")
				addCheck("input_media", "blocked", "input media does not exist")
			} else {
				addCheck("input_media", "ok", "input media exists")
			}
		}
		if action.Type == agentActionTypeReviseMake && inputString(action.Input, "make_id") == "" {
			action.PolicyStatus = "blocked"
			action.Status = "blocked"
			policy.Errors = append(policy.Errors, "missing make_id for revise_make plan")
			addCheck("make_id", "blocked", "make_id is required for revise_make")
		}
		if action.Type == agentActionTypeValidate && inputString(action.Input, "creative_plan_id") == "" {
			action.PolicyStatus = "blocked"
			action.Status = "blocked"
			policy.Errors = append(policy.Errors, "missing creative_plan_id for validate_creative_assemble plan")
			addCheck("creative_plan_id", "blocked", "creative_plan_id is required for validate_creative_assemble")
		}
	}
	if ctx.Capabilities.VoiceGeneration == "missing_env" {
		policy.Warnings = append(policy.Warnings, "voice backend is configured but env var is missing")
		addCheck("voice_env", "warning", "voice backend env var is missing")
	}
	if ctx.Capabilities.CaptionBurn == "missing" {
		policy.Warnings = append(policy.Warnings, "caption burn capability is missing; burn-in may be skipped")
		addCheck("caption_burn", "warning", "caption burn capability is missing")
	}
	switch {
	case len(policy.Errors) > 0:
		policy.Status = "blocked"
	case policy.RequiresUserApproval || policy.RequiresProviderPermission || policy.RequiresOverwritePermission || policy.RequiresExternalNetwork:
		policy.Status = "approval_required"
	case len(policy.Warnings) > 0:
		policy.Status = "warning"
	default:
		policy.Status = "allowed"
	}
	return policy
}

func buildAgentPlanNextCommands(plan AgentPlanV1, policy AgentPlanPolicyReview) []string {
	next := []string{fmt.Sprintf("byom-video inspect-agent-plan %s", plan.PlanID)}
	next = append(next, fmt.Sprintf("byom-video review-agent-plan %s --write-artifact", plan.PlanID))
	next = append(next, fmt.Sprintf("byom-video agent-policy %s", plan.PlanID))
	if policy.Status == "approval_required" {
		next = append(next, "# approval required before future execution/job conversion")
	}
	return dedupeStrings(next)
}

func renderAgentPlanReview(plan AgentPlanV1, ctx AgentPlanContextSnapshot, policy AgentPlanPolicyReview) string {
	var b strings.Builder
	b.WriteString("# Agent Plan Review\n\n")
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", plan.PlanID)
	fmt.Fprintf(&b, "- Status: `%s`\n", plan.Status)
	if plan.ApprovedAt != nil {
		fmt.Fprintf(&b, "- Approved: `%s` (`%s`)\n", plan.ApprovedAt.Format(time.RFC3339), emptyDash(plan.ApprovalMode))
	}
	if plan.RejectedAt != nil {
		fmt.Fprintf(&b, "- Rejected: `%s` (%s)\n", plan.RejectedAt.Format(time.RFC3339), emptyDash(plan.RejectionReason))
	}
	fmt.Fprintf(&b, "- Intent: %s\n", plan.Intent)
	fmt.Fprintf(&b, "- Policy Status: `%s`\n", policy.Status)
	b.WriteString("\n## Context\n\n")
	fmt.Fprintf(&b, "- Media: `%s` (exists=`%t`)\n", emptyDash(ctx.InputMedia.Path), ctx.InputMedia.Exists)
	fmt.Fprintf(&b, "- Style Pack: `%s` (exists=`%t`)\n", ctx.StylePack.Path, ctx.StylePack.Exists)
	fmt.Fprintf(&b, "- Runtime: queue=`%s`, daemon=`%s`, worker=`%s`\n", ctx.Runtime.QueueStatus, ctx.Runtime.DaemonStatus, ctx.Runtime.WorkerStatus)
	fmt.Fprintf(&b, "- Capabilities: script=`%s`, captions=`%s`, voice=`%s`, caption_burn=`%s`\n", ctx.Capabilities.ScriptGeneration, ctx.Capabilities.CaptionGeneration, ctx.Capabilities.VoiceGeneration, ctx.Capabilities.CaptionBurn)
	if brief, err := readCreativeBrief(plan.PlanID); err == nil && brief.SchemaVersion != "" {
		b.WriteString("\n## Creative Brief\n\n")
		fmt.Fprintf(&b, "- Summary: %s\n", brief.Summary)
		fmt.Fprintf(&b, "- Platform: `%s`, aspect=`%s`, duration=`%d`\n", emptyDash(brief.Platform), emptyDash(brief.AspectRatio), brief.Duration.TargetSeconds)
		if len(brief.Style.Tone)+len(brief.Style.VisualStyle) > 0 {
			fmt.Fprintf(&b, "- Style: `%s`\n", strings.Join(append(append([]string{}, brief.Style.Tone...), brief.Style.VisualStyle...), "`, `"))
		}
		if brief.Captions.Required {
			fmt.Fprintf(&b, "- Captions: style=`%s`, position=`%s`\n", emptyDash(brief.Captions.Style), emptyDash(brief.Captions.Position))
		}
	}
	if deliverables, err := readDeliverables(plan.PlanID); err == nil && len(deliverables.Deliverables) > 0 {
		b.WriteString("\n## Deliverables\n\n")
		for _, item := range deliverables.Deliverables {
			fmt.Fprintf(&b, "- `%s` `%s` — %s\n", item.ID, item.Type, item.Description)
		}
	}
	if assets, err := readAssetRequirements(plan.PlanID); err == nil && len(assets.Requirements) > 0 {
		b.WriteString("\n## Asset Requirements\n\n")
		for _, req := range assets.Requirements {
			fmt.Fprintf(&b, "- `%s` `%s` status=`%s` — %s\n", req.ID, req.Capability, req.Status, req.Description)
		}
	}
	if visualRequests, err := readVisualRequestsDryRun(plan.PlanID); err == nil && len(visualRequests.Requests) > 0 {
		b.WriteString("\n## Visual Dry-Run Requests\n\n")
		for _, req := range visualRequests.Requests {
			fmt.Fprintf(&b, "- `%s` `%s` status=`%s` route=`%s` backend=`%s`\n", req.ID, req.Capability, req.Status, emptyDash(req.Route), emptyDash(req.Backend))
		}
		for _, missing := range visualRequests.MissingCapabilities {
			fmt.Fprintf(&b, "- Missing: %s\n", missing)
		}
	}
	b.WriteString("\n## Actions\n\n")
	for _, action := range plan.Actions {
		fmt.Fprintf(&b, "- `%s` `%s` — %s (`%s`)\n", action.ID, action.Type, action.Description, action.PolicyStatus)
	}
	b.WriteString("\n## Policy Checks\n\n")
	for _, check := range policy.Checks {
		fmt.Fprintf(&b, "- `%s` `%s` — %s\n", check.ID, check.Status, check.Message)
	}
	if len(plan.Warnings)+len(policy.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range dedupeStrings(append(append([]string{}, plan.Warnings...), policy.Warnings...)) {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	if len(plan.Errors)+len(policy.Errors) > 0 {
		b.WriteString("\n## Errors\n\n")
		for _, errText := range dedupeStrings(append(append([]string{}, plan.Errors...), policy.Errors...)) {
			fmt.Fprintf(&b, "- %s\n", errText)
		}
	}
	if len(plan.NextCommands) > 0 {
		b.WriteString("\n## Next Commands\n\n")
		for _, command := range agentPlanReviewNextCommands(plan) {
			fmt.Fprintf(&b, "- `%s`\n", command)
		}
	}
	if linked, err := readAgentLinkedJobs(plan.PlanID); err == nil {
		linked = refreshAgentLinkedJobsInMemory(linked)
		b.WriteString("\n## Linked Jobs\n\n")
		for _, job := range linked.Jobs {
			fmt.Fprintf(&b, "- `%s` from `%s` (`%s`, approval `%s`)\n", job.JobID, job.ActionID, job.Status, job.ApprovalStatus)
		}
		for _, warning := range linked.Warnings {
			fmt.Fprintf(&b, "- Warning: %s\n", warning)
		}
	}
	return b.String()
}

func agentPlanReviewNextCommands(plan AgentPlanV1) []string {
	next := append([]string{}, plan.NextCommands...)
	next = append(next, fmt.Sprintf("byom-video agent-result %s", plan.PlanID))
	switch plan.Status {
	case agentPlanStatusDraft, agentPlanStatusReviewed:
		next = append(next, fmt.Sprintf("byom-video approve-agent-plan %s", plan.PlanID))
	case agentPlanStatusApproved:
		if _, err := readAgentLinkedJobs(plan.PlanID); os.IsNotExist(err) {
			next = append(next, fmt.Sprintf("byom-video agent-plan-to-job %s --dry-run", plan.PlanID))
			next = append(next, fmt.Sprintf("byom-video agent-plan-to-job %s", plan.PlanID))
		}
	case agentPlanStatusConverted:
		next = append(next, fmt.Sprintf("byom-video agent-plan-jobs %s", plan.PlanID))
		next = append(next, "byom-video jobs")
		next = append(next, "byom-video daemon start --interval 10s")
	}
	return dedupeStrings(next)
}

func listAgentPlans() ([]AgentPlanV1, error) {
	entries, err := os.ReadDir(agentPlansV1Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plans := []AgentPlanV1{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		plan, err := readAgentPlan(entry.Name())
		if err != nil {
			continue
		}
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})
	return plans, nil
}

func readAgentPlan(planID string) (AgentPlanV1, error) {
	var plan AgentPlanV1
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "agent_plan.json"))
	if err != nil {
		return plan, err
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func writeAgentPlan(plan AgentPlanV1) error {
	plan.UpdatedAt = time.Now().UTC()
	return writeJSONFile(filepath.Join(agentPlansV1Root, plan.PlanID, "agent_plan.json"), plan)
}

func readAgentPlanContext(planID string) (AgentPlanContextSnapshot, error) {
	var ctx AgentPlanContextSnapshot
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "context_snapshot.json"))
	if err != nil {
		return ctx, err
	}
	if err := json.Unmarshal(data, &ctx); err != nil {
		return ctx, err
	}
	return ctx, nil
}

func readAgentPlanPolicy(planID string) (AgentPlanPolicyReview, error) {
	var policy AgentPlanPolicyReview
	data, err := os.ReadFile(filepath.Join(agentPlansV1Root, planID, "policy_review.json"))
	if err != nil {
		return policy, err
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		return policy, err
	}
	return policy, nil
}

func contextless(s string) string { return s }

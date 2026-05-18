package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/modelrouter"
)

// ---- Ollama planner ----

type ollamaAgentPlanner struct {
	// plannerOpts carries the Ollama-specific options baked in at construction.
	plannerOpts AgentPlanCommandOptions
}

func newOllamaAgentPlanner(opts AgentPlanCommandOptions) AgentPlanner {
	return &ollamaAgentPlanner{plannerOpts: opts}
}

func (p *ollamaAgentPlanner) Name() string { return "ollama" }

func (p *ollamaAgentPlanner) Plan(planCtx PlannerContext) (PlanResult, error) {
	result, err := p.callOllama(planCtx)
	if err != nil {
		if planCtx.Options.FallbackDeterministic {
			det := deterministicAgentPlanner{}
			detResult, detErr := det.Plan(planCtx)
			fallbackMsg := fmt.Sprintf("ollama planner failed; using deterministic fallback: %v", err)
			detResult.EffectiveMode = "deterministic"
			detResult.FallbackUsed = true
			detResult.FallbackReason = fallbackMsg
			detResult.ResolvedModel = result.ResolvedModel
			detResult.ResolvedBackend = result.ResolvedBackend
			detResult.ResolvedRoute = result.ResolvedRoute
			detResult.Warnings = append(detResult.Warnings, fallbackMsg)
			// Annotate the request artifact from the deterministic run with the ollama attempt context.
			if detResult.RequestArtifact != nil {
				detResult.RequestArtifact.FallbackUsed = true
				detResult.RequestArtifact.FallbackReason = fallbackMsg
				detResult.RequestArtifact.EffectiveMode = "deterministic"
				detResult.RequestArtifact.Model = result.ResolvedModel
				detResult.RequestArtifact.Backend = result.ResolvedBackend
				detResult.RequestArtifact.Route = result.ResolvedRoute
				// Carry the ollama prompts from the attempted request if present.
				if result.RequestArtifact != nil {
					detResult.RequestArtifact.SystemPrompt = result.RequestArtifact.SystemPrompt
					detResult.RequestArtifact.UserPrompt = result.RequestArtifact.UserPrompt
				}
			}
			return detResult, detErr
		}
		return result, err
	}
	return result, nil
}

func (p *ollamaAgentPlanner) callOllama(planCtx PlannerContext) (PlanResult, error) {
	opts := planCtx.Options

	// Resolve model + base URL from config (if no explicit model given).
	model := opts.PlannerModel
	baseURL := opts.PlannerBackend
	resolvedRoute := opts.PlannerRoute
	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	if model == "" {
		cfg, err := config.Load(config.DefaultPath)
		if err == nil && cfg.Models.Enabled {
			routeKey := opts.PlannerRoute
			if routeKey == "" {
				routeKey = "agent.planning"
			}
			resolvedRoute = routeKey
			entryName, ok := cfg.Models.Routes[routeKey]
			if ok && entryName != "" {
				if entry, ok := cfg.Models.Entries[entryName]; ok {
					if model == "" {
						model = entry.Model
					}
					if baseURL == "" {
						baseURL = entry.BaseURL
					}
				}
			}
		}
	}

	// Build partial result for metadata (returned even on error so fallback can reuse it).
	partialResult := PlanResult{
		ResolvedModel:   model,
		ResolvedBackend: baseURL,
		ResolvedRoute:   resolvedRoute,
	}

	if strings.TrimSpace(model) == "" {
		return partialResult, fmt.Errorf("ollama planner requires a model; set --planner-model or configure models.routes.agent.planning in byom-video.yaml")
	}

	adapter := modelrouter.OllamaAdapter{Timeout: timeout}

	systemPrompt := buildOllamaPlannerSystemPrompt()
	userPrompt := buildOllamaPlannerUserPrompt(planCtx)

	// Attach the prompts to partialResult so the fallback path can include them.
	partialResult.RequestArtifact = &PlannerRequestArtifact{
		SchemaVersion: "openvfx_planner_request.v1",
		CreatedAt:     time.Now().UTC(),
		PlannerMode:   "ollama",
		EffectiveMode: "ollama",
		Model:         model,
		Backend:       baseURL,
		Route:         resolvedRoute,
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
	}

	adapterOpts := map[string]any{}
	if opts.Temperature > 0 {
		adapterOpts["temperature"] = opts.Temperature
	}

	req := modelrouter.Request{
		TaskID:   "task_agent_plan_0001",
		TaskType: "agent_planning",
		Provider: "ollama",
		Model:    model,
		BaseURL:  baseURL,
		Options:  adapterOpts,
		Input:    modelrouter.RequestInput{},
		RequestPreview: modelrouter.RequestPreview{
			System: systemPrompt,
			User:   userPrompt,
		},
	}

	resp, err := adapter.Execute(req)
	if err != nil {
		return partialResult, fmt.Errorf("ollama planner request failed: %w", err)
	}

	raw := ""
	if r, ok := resp.Details["raw_response"].(string); ok {
		raw = r
	}
	if raw == "" && len(resp.Texts) > 0 {
		raw = strings.Join(resp.Texts, "\n")
	}

	if opts.MaxOutputChars > 0 && len(raw) > opts.MaxOutputChars {
		raw = raw[:opts.MaxOutputChars]
	}

	actions, warnings, err := parseOllamaPlannerResponse(raw)
	if err != nil {
		return partialResult, err
	}

	// Strict schema validation.
	if err := validateAgentActions(actions); err != nil {
		return partialResult, fmt.Errorf("ollama planner produced invalid actions: %w", err)
	}

	return PlanResult{
		Actions:         applyDefaultActionFields(actions),
		Warnings:        append(resp.Warnings, warnings...),
		EffectiveMode:   "ollama",
		ResolvedModel:   model,
		ResolvedBackend: baseURL,
		ResolvedRoute:   resolvedRoute,
		RequestArtifact: partialResult.RequestArtifact,
	}, nil
}

// ---- prompt builders ----

func buildOllamaPlannerSystemPrompt() string {
	return `You are a video workflow planner for byom-video.
Your job is to read a user intent and produce a JSON action plan.

OUTPUT FORMAT — respond with ONLY valid JSON, no other text:
{
  "actions": [
    {
      "id": "action_0001",
      "type": "<action_type>",
      "description": "<one sentence describing what this action does>",
      "input": { <action-specific fields> },
      "requires_approval": <true|false>,
      "requires_provider": <true|false>,
      "requires_network": <true|false>,
      "requires_overwrite": <true|false>,
      "status": "planned",
      "policy_status": "review_pending"
    }
  ],
  "warnings": ["<optional warning strings>"]
}

ALLOWED ACTION TYPES (v1):
1. "make"
   - Use when: user wants to create a new video from source media.
   - requires_approval: true
   - Input fields: goal (string, required), input_path (string), platform (string), generate_script (bool), generate_captions (bool), burn_captions (bool), generate_voiceover (bool), overwrite (bool)

2. "revise_make"
   - Use when: user wants to change an existing make (e.g. switch platform, change captions).
   - requires_approval: true
   - Input fields: make_id (string, required), request (string, required), reassemble (bool), overwrite (bool)

3. "validate_creative_assemble"
   - Use when: user wants to validate or check a creative plan.
   - requires_approval: false
   - Input fields: creative_plan_id (string, required)

4. "queue_health"
   - Use when: user wants to check system queue status, pending jobs, or runtime health.
   - requires_approval: false
   - Input fields: strict (bool), stale_after (string, e.g. "30m")

RULES:
- Produce 1 to 3 actions only.
- Action IDs must be "action_0001", "action_0002", etc. in sequence.
- Do not invent types beyond the four listed above.
- Set requires_provider: true only if the action calls an AI model or TTS.
- Set requires_network: true only if the action needs external network (provider calls).
- Default to requires_overwrite: false unless the user explicitly asks to overwrite.
- Do not add extra fields. Do not add markdown. Respond with JSON only.`
}

func buildOllamaPlannerUserPrompt(planCtx PlannerContext) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "User intent: %s\n\n", planCtx.Intent)
	fmt.Fprintln(&sb, "Context:")

	if planCtx.InputPath != "" {
		fmt.Fprintf(&sb, "- Input media path: %s (exists: %v)\n", planCtx.InputPath, planCtx.Context.InputMedia.Exists)
	} else {
		fmt.Fprintln(&sb, "- Input media path: none")
	}

	if planCtx.MakeID != "" {
		fmt.Fprintf(&sb, "- Existing make_id: %s\n", planCtx.MakeID)
	}
	if planCtx.CreativePlanID != "" {
		fmt.Fprintf(&sb, "- Creative plan_id: %s\n", planCtx.CreativePlanID)
	}

	caps := planCtx.Context.Capabilities
	fmt.Fprintf(&sb, "- Script generation: %s\n", caps.ScriptGeneration)
	fmt.Fprintf(&sb, "- Caption generation: %s\n", caps.CaptionGeneration)
	fmt.Fprintf(&sb, "- Voice generation: %s\n", caps.VoiceGeneration)
	fmt.Fprintf(&sb, "- Allow provider calls: %v\n", planCtx.Options.AllowProviderCalls)
	fmt.Fprintf(&sb, "- Allow overwrite: %v\n", planCtx.Options.AllowOverwrite)

	rt := planCtx.Context.Runtime
	fmt.Fprintf(&sb, "- Queue status: %s (pending: %d, failed: %d)\n", rt.QueueStatus, rt.PendingJobs, rt.FailedJobs)

	if planCtx.Options.Platform != "" {
		fmt.Fprintf(&sb, "- Platform override: %s\n", planCtx.Options.Platform)
	}

	fmt.Fprintln(&sb, "\nProduce the JSON action plan for this request.")
	return sb.String()
}

// ---- response parser ----

type ollamaPlannerResponse struct {
	Actions  []ollamaPlannerAction `json:"actions"`
	Warnings []string              `json:"warnings"`
}

// ollamaPlannerAction is a lenient parse target — we accept any JSON fields then
// convert to AgentActionV1 after validation.
type ollamaPlannerAction struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	Description       string         `json:"description"`
	Input             map[string]any `json:"input"`
	RequiresApproval  bool           `json:"requires_approval"`
	RequiresProvider  bool           `json:"requires_provider"`
	RequiresNetwork   bool           `json:"requires_network"`
	RequiresOverwrite bool           `json:"requires_overwrite"`
	Status            string         `json:"status"`
	PolicyStatus      string         `json:"policy_status"`
}

func parseOllamaPlannerResponse(raw string) ([]AgentActionV1, []string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil, fmt.Errorf("ollama planner returned empty response")
	}

	// Strip markdown code fences if present.
	raw = stripJSONCodeFences(raw)

	var parsed ollamaPlannerResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Try wrapping in {"actions": ...} in case model returned an array directly.
		wrappedRaw := `{"actions":` + raw + `}`
		var wrapped ollamaPlannerResponse
		if err2 := json.Unmarshal([]byte(wrappedRaw), &wrapped); err2 == nil {
			parsed = wrapped
		} else {
			return nil, nil, fmt.Errorf("ollama planner response is not valid JSON: %w", err)
		}
	}

	if len(parsed.Actions) == 0 {
		return nil, parsed.Warnings, fmt.Errorf("ollama planner returned JSON with no actions")
	}

	actions := make([]AgentActionV1, 0, len(parsed.Actions))
	for _, a := range parsed.Actions {
		if a.Input == nil {
			a.Input = map[string]any{}
		}
		actions = append(actions, AgentActionV1{
			ID:                a.ID,
			Type:              a.Type,
			Description:       a.Description,
			Input:             a.Input,
			RequiresApproval:  a.RequiresApproval,
			RequiresProvider:  a.RequiresProvider,
			RequiresNetwork:   a.RequiresNetwork,
			RequiresOverwrite: a.RequiresOverwrite,
			Status:            a.Status,
			PolicyStatus:      a.PolicyStatus,
		})
	}

	return actions, parsed.Warnings, nil
}

// stripJSONCodeFences removes ```json ... ``` or ``` ... ``` wrapping from LLM output.
func stripJSONCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line
		idx := strings.Index(s, "\n")
		if idx >= 0 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

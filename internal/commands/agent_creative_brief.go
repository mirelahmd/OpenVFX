package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
)

const (
	creativeBriefSchema        = "openvfx_creative_brief.v1"
	deliverablesSchema         = "openvfx_deliverables.v1"
	assetRequirementsSchema    = "openvfx_asset_requirements.v1"
	visualRequestsDryRunSchema = "openvfx_visual_requests.dryrun.v1"
	creativeBriefArtifact      = "creative_brief.json"
	deliverablesArtifact       = "deliverables.json"
	assetRequirementsArtifact  = "asset_requirements.json"
	visualRequestsArtifact     = "visual_requests.dryrun.json"
)

type CreativeBriefArtifact struct {
	SchemaVersion string                  `json:"schema_version"`
	CreatedAt     time.Time               `json:"created_at"`
	PlanID        string                  `json:"plan_id,omitempty"`
	RawPrompt     string                  `json:"raw_prompt"`
	Summary       string                  `json:"summary"`
	Duration      CreativeBriefDuration   `json:"duration"`
	Platform      string                  `json:"platform"`
	AspectRatio   string                  `json:"aspect_ratio"`
	Style         CreativeBriefStyle      `json:"style"`
	Pacing        CreativeBriefPacing     `json:"pacing"`
	Captions      CreativeBriefCaptions   `json:"captions"`
	SourceMedia   []CreativeBriefMediaUse `json:"source_media,omitempty"`
	Requests      CreativeBriefRequests   `json:"requests"`
	Constraints   []string                `json:"constraints,omitempty"`
	Warnings      []string                `json:"warnings,omitempty"`
}

type CreativeBriefDuration struct {
	TargetSeconds int `json:"target_seconds,omitempty"`
	MaxSeconds    int `json:"max_seconds,omitempty"`
}

type CreativeBriefStyle struct {
	Tone        []string `json:"tone,omitempty"`
	VisualStyle []string `json:"visual_style,omitempty"`
	Mood        []string `json:"mood,omitempty"`
}

type CreativeBriefPacing struct {
	CutStyle         string `json:"cut_style,omitempty"`
	FirstSecondsNote string `json:"first_seconds_note,omitempty"`
}

type CreativeBriefCaptions struct {
	Required bool   `json:"required"`
	Style    string `json:"style,omitempty"`
	Position string `json:"position,omitempty"`
}

type CreativeBriefMediaUse struct {
	Role        string `json:"role"`
	Description string `json:"description"`
}

type CreativeBriefRequests struct {
	Script                  bool `json:"script"`
	Voiceover               bool `json:"voiceover"`
	UseTalkingClipNarration bool `json:"use_talking_clip_narration"`
	GeneratedBRoll          bool `json:"generated_broll"`
	GeneratedImage          bool `json:"generated_image"`
	InstagramCaptions       bool `json:"instagram_captions"`
	VisualTransform         bool `json:"visual_transform"`
	StyleTransfer           bool `json:"style_transfer"`
	ObjectRemove            bool `json:"object_remove"`
	BackgroundReplace       bool `json:"background_replace"`
}

type DeliverablesArtifact struct {
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	PlanID        string                `json:"plan_id,omitempty"`
	Deliverables  []CreativeDeliverable `json:"deliverables"`
	Warnings      []string              `json:"warnings,omitempty"`
}

type CreativeDeliverable struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Platform    string         `json:"platform,omitempty"`
	Format      string         `json:"format,omitempty"`
	Duration    int            `json:"duration_seconds,omitempty"`
	Description string         `json:"description"`
	Options     map[string]any `json:"options,omitempty"`
}

type AssetRequirementsArtifact struct {
	SchemaVersion string             `json:"schema_version"`
	CreatedAt     time.Time          `json:"created_at"`
	PlanID        string             `json:"plan_id,omitempty"`
	Requirements  []AssetRequirement `json:"requirements"`
	Warnings      []string           `json:"warnings,omitempty"`
}

type AssetRequirement struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Description  string   `json:"description"`
	Capability   string   `json:"capability"`
	Status       string   `json:"status"`
	Required     bool     `json:"required"`
	Route        string   `json:"route,omitempty"`
	Backend      string   `json:"backend,omitempty"`
	Generated    bool     `json:"generated"`
	DegradedPath string   `json:"degraded_path,omitempty"`
	SourceHint   string   `json:"source_hint,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type AgentOrchestrateOptions struct {
	AgentPlanCommandOptions
	GraphJSON   bool
	WorkersDir  string
	SkipGraph   bool
	WriteReview bool
}

type VisualRequestsOptions struct {
	JSON      bool
	Overwrite bool
}

type VisualRequestsDryRunArtifact struct {
	SchemaVersion       string                    `json:"schema_version"`
	CreatedAt           time.Time                 `json:"created_at"`
	PlanID              string                    `json:"plan_id"`
	Requests            []VisualGenerationRequest `json:"requests"`
	MissingCapabilities []string                  `json:"missing_capabilities,omitempty"`
	Warnings            []string                  `json:"warnings,omitempty"`
}

type VisualGenerationRequest struct {
	ID             string            `json:"id"`
	RequirementID  string            `json:"requirement_id"`
	Kind           string            `json:"kind"`
	Capability     string            `json:"capability"`
	Route          string            `json:"route,omitempty"`
	Backend        string            `json:"backend,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	Model          string            `json:"model,omitempty"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Auth           VisualRequestAuth `json:"auth"`
	Status         string            `json:"status"`
	RequestPreview map[string]any    `json:"request_preview"`
	OutputContract map[string]any    `json:"output_contract"`
	Warnings       []string          `json:"warnings,omitempty"`
}

type VisualRequestAuth struct {
	Type string `json:"type"`
	Env  string `json:"env,omitempty"`
}

func buildCreativePlanningArtifacts(planID string, now time.Time, opts AgentPlanCommandOptions, ctx AgentPlanContextSnapshot) (CreativeBriefArtifact, DeliverablesArtifact, AssetRequirementsArtifact) {
	brief := parseCreativeBrief(planID, now, opts.Goal, opts.Platform)
	deliverables := buildDeliverablesFromBrief(planID, now, brief)
	assets := buildAssetRequirementsFromBrief(planID, now, brief, ctx)
	return brief, deliverables, assets
}

func parseCreativeBrief(planID string, now time.Time, goal string, platformOverride string) CreativeBriefArtifact {
	lower := strings.ToLower(goal)
	brief := CreativeBriefArtifact{
		SchemaVersion: creativeBriefSchema,
		CreatedAt:     now,
		PlanID:        planID,
		RawPrompt:     strings.TrimSpace(goal),
		Platform:      inferBriefPlatform(lower, platformOverride),
		Requests: CreativeBriefRequests{
			Script:                  strings.Contains(lower, "script") || strings.Contains(lower, "hook") || strings.Contains(lower, "caption options") || strings.Contains(lower, "instagram caption"),
			Voiceover:               strings.Contains(lower, "voiceover") || strings.Contains(lower, "narration") || strings.Contains(lower, "spoken"),
			UseTalkingClipNarration: strings.Contains(lower, "talking clip") && (strings.Contains(lower, "narration") || strings.Contains(lower, "voiceover")),
			GeneratedBRoll:          strings.Contains(lower, "generate") && (strings.Contains(lower, "b-roll") || strings.Contains(lower, "broll") || strings.Contains(lower, "visual")),
			GeneratedImage:          (strings.Contains(lower, "generate") || strings.Contains(lower, "create")) && (strings.Contains(lower, "image") || strings.Contains(lower, "reference asset") || strings.Contains(lower, "reference image")),
			InstagramCaptions:       strings.Contains(lower, "instagram caption") || strings.Contains(lower, "caption options"),
			VisualTransform:         strings.Contains(lower, "make me thinner") || strings.Contains(lower, "thinner") || strings.Contains(lower, "remove object") || strings.Contains(lower, "object removal") || strings.Contains(lower, "background replace") || strings.Contains(lower, "replace background") || strings.Contains(lower, "style transfer") || strings.Contains(lower, "make lighting") || strings.Contains(lower, "make it darker"),
			StyleTransfer:           strings.Contains(lower, "style transfer") || strings.Contains(lower, "make lighting") || strings.Contains(lower, "make it darker") || (strings.Contains(lower, "cinematic") && strings.Contains(lower, "lighting")),
			ObjectRemove:            strings.Contains(lower, "remove object") || strings.Contains(lower, "object removal"),
			BackgroundReplace:       strings.Contains(lower, "background replace") || strings.Contains(lower, "replace background"),
		},
	}
	brief.AspectRatio = aspectRatioForPlatform(brief.Platform)
	if sec := extractDurationSeconds(lower); sec > 0 {
		brief.Duration.TargetSeconds = sec
		brief.Duration.MaxSeconds = sec
	}
	brief.Style.Tone = collectKeywords(lower, []string{"luxury", "premium", "cinematic", "fitness", "futuristic", "energetic", "minimal", "editorial"})
	brief.Style.VisualStyle = collectKeywords(lower, []string{"dark", "darker", "city", "gym", "neon", "clean", "moody", "high contrast"})
	brief.Style.Mood = collectKeywords(lower, []string{"emotional", "funny", "technical", "aspirational", "intense"})
	if strings.Contains(lower, "fast cuts") {
		brief.Pacing.CutStyle = "fast_cuts"
	}
	if sec := extractFirstSeconds(lower); sec > 0 {
		brief.Pacing.FirstSecondsNote = fmt.Sprintf("Prioritize fast visual momentum in the first %d seconds.", sec)
	}
	brief.Captions.Required = strings.Contains(lower, "caption") || strings.Contains(lower, "subtitle") || strings.Contains(lower, "text on screen")
	switch {
	case strings.Contains(lower, "lower-third"), strings.Contains(lower, "lower third"):
		brief.Captions.Position = "lower-third"
	case strings.Contains(lower, "captions top"):
		brief.Captions.Position = "top"
	case strings.Contains(lower, "captions center"):
		brief.Captions.Position = "center"
	case strings.Contains(lower, "captions bottom"):
		brief.Captions.Position = "bottom"
	}
	switch {
	case strings.Contains(lower, "bold"):
		brief.Captions.Style = "bold"
	case strings.Contains(lower, "boxed"):
		brief.Captions.Style = "boxed"
	}
	if strings.Contains(lower, "talking clip") {
		brief.SourceMedia = append(brief.SourceMedia, CreativeBriefMediaUse{Role: "narration", Description: "Use the talking clip as narration/source audio."})
	}
	if strings.Contains(lower, "gym clips") || strings.Contains(lower, "gym clip") {
		brief.SourceMedia = append(brief.SourceMedia, CreativeBriefMediaUse{Role: "b_roll", Description: "Use gym clips as b-roll."})
	}
	if brief.Requests.VisualTransform {
		brief.Warnings = append(brief.Warnings, "visual transform requested; this milestone only plans asset requirements and does not edit body shape or pixels")
	}
	if brief.Requests.GeneratedBRoll {
		brief.Constraints = append(brief.Constraints, "generate b-roll only if a local/configured backend exists; otherwise use existing footage")
	}
	brief.Summary = buildBriefSummary(brief)
	return brief
}

func buildDeliverablesFromBrief(planID string, now time.Time, brief CreativeBriefArtifact) DeliverablesArtifact {
	items := []CreativeDeliverable{{
		ID:          "deliverable_0001",
		Type:        "edited_video",
		Title:       titleForPlatform(brief.Platform),
		Platform:    brief.Platform,
		Format:      brief.AspectRatio,
		Duration:    brief.Duration.TargetSeconds,
		Description: brief.Summary,
		Options: map[string]any{
			"caption_style":    brief.Captions.Style,
			"caption_position": brief.Captions.Position,
			"pacing":           brief.Pacing.CutStyle,
		},
	}}
	if brief.Requests.InstagramCaptions {
		items = append(items, CreativeDeliverable{
			ID:          "deliverable_0002",
			Type:        "social_caption_options",
			Title:       "Instagram Caption Options",
			Platform:    "instagram",
			Format:      "text",
			Description: "Generate multiple Instagram caption options aligned to the creative brief.",
		})
	}
	if brief.Requests.Script || brief.Requests.Voiceover {
		items = append(items, CreativeDeliverable{
			ID:          fmt.Sprintf("deliverable_%04d", len(items)+1),
			Type:        "script_or_voiceover_outline",
			Title:       "Narration / Hook Outline",
			Format:      "text",
			Description: "Plan hook, narration, and voiceover text before any provider execution.",
		})
	}
	return DeliverablesArtifact{SchemaVersion: deliverablesSchema, CreatedAt: now, PlanID: planID, Deliverables: items}
}

func buildAssetRequirementsFromBrief(planID string, now time.Time, brief CreativeBriefArtifact, ctx AgentPlanContextSnapshot) AssetRequirementsArtifact {
	reqs := []AssetRequirement{}
	add := func(kind, desc, capability, route, backend string, generated bool, required bool, degraded string) {
		status := "missing"
		if route != "" {
			status = "satisfied"
		}
		reqs = append(reqs, AssetRequirement{
			ID:           fmt.Sprintf("asset_req_%04d", len(reqs)+1),
			Kind:         kind,
			Description:  desc,
			Capability:   capability,
			Status:       status,
			Required:     required,
			Route:        route,
			Backend:      backend,
			Generated:    generated,
			DegradedPath: degraded,
		})
	}
	routes, backends := creativeToolRoutes()
	if brief.Requests.GeneratedBRoll {
		route := firstNonEmpty(routes["creative.broll_generate"], routes["creative.video_generate"], routes["creative.image_generate"], routes["creative.video_broll"], routes["creative.image_broll"], routes["creative.visual_asset"])
		backend := backends[route]
		add("generated_broll", "Generate futuristic gym/city b-roll if a backend is configured.", "creative.broll_generate", route, backend, true, false, "Use existing source media as b-roll.")
	}
	if brief.Requests.GeneratedImage {
		route := firstNonEmpty(routes["creative.image_generate"], routes["creative.visual_asset"])
		backend := backends[route]
		add("generated_image", "Generate image or reference assets if a backend is configured.", "creative.image_generate", route, backend, true, false, "Use existing stills/source frames as reference assets.")
	}
	if brief.Requests.Voiceover && !brief.Requests.UseTalkingClipNarration {
		status := ctx.Capabilities.VoiceGeneration
		route := routes["creative.voiceover"]
		backend := backends[route]
		req := AssetRequirement{
			ID:           fmt.Sprintf("asset_req_%04d", len(reqs)+1),
			Kind:         "voiceover",
			Description:  "Generate or prepare voiceover narration for the creative brief.",
			Capability:   "voice_generation",
			Status:       capabilityToRequirementStatus(status),
			Required:     false,
			Route:        route,
			Backend:      backend,
			Generated:    route != "",
			DegradedPath: "Prepare voiceover text and use the source talking clip or manual recording.",
		}
		reqs = append(reqs, req)
	}
	if brief.Requests.VisualTransform {
		route := firstNonEmpty(routes["creative.visual_transform"], routes["creative.style_transfer"], routes["creative.object_remove"], routes["creative.background_replace"], routes["creative.object_removal"])
		backend := backends[route]
		add("visual_transform", "Plan requested visual transform as a capability gap; no pixel edit is executed.", "creative.visual_transform", route, backend, false, false, "Skip transform and preserve source footage.")
	}
	if brief.Requests.StyleTransfer {
		route := firstNonEmpty(routes["creative.style_transfer"], routes["creative.visual_transform"])
		backend := backends[route]
		add("style_transfer", "Plan cinematic/darker style transfer or lighting adjustment as a dry-run visual request.", "creative.style_transfer", route, backend, false, false, "Keep source lighting or use normal grading controls.")
	}
	if brief.Requests.ObjectRemove {
		route := firstNonEmpty(routes["creative.object_remove"], routes["creative.visual_transform"], routes["creative.object_removal"])
		backend := backends[route]
		add("object_remove", "Plan object removal as a dry-run visual request.", "creative.object_remove", route, backend, false, false, "Keep source footage unchanged.")
	}
	if brief.Requests.BackgroundReplace {
		route := firstNonEmpty(routes["creative.background_replace"], routes["creative.visual_transform"])
		backend := backends[route]
		add("background_replace", "Plan background replacement as a dry-run visual request.", "creative.background_replace", route, backend, false, false, "Keep source background unchanged.")
	}
	if brief.Requests.UseTalkingClipNarration {
		reqs = append(reqs, AssetRequirement{
			ID:           fmt.Sprintf("asset_req_%04d", len(reqs)+1),
			Kind:         "source_audio",
			Description:  "Use talking clip audio as narration.",
			Capability:   "source_media",
			Status:       "satisfied",
			Required:     true,
			Generated:    false,
			SourceHint:   "talking clip",
			DegradedPath: "",
		})
	}
	warnings := []string{}
	for _, req := range reqs {
		if req.Status == "missing" {
			warnings = append(warnings, fmt.Sprintf("%s is missing; %s", req.Capability, req.DegradedPath))
		}
	}
	return AssetRequirementsArtifact{SchemaVersion: assetRequirementsSchema, CreatedAt: now, PlanID: planID, Requirements: reqs, Warnings: dedupeStrings(warnings)}
}

func AgentOrchestrate(stdout io.Writer, opts AgentOrchestrateOptions) error {
	opts.WriteReview = true
	opts.AgentPlanCommandOptions.WriteReview = true
	if opts.DryRun {
		return AgentPlanCommand(stdout, opts.AgentPlanCommandOptions)
	}
	var planOut strings.Builder
	if err := AgentPlanCommand(&planOut, opts.AgentPlanCommandOptions); err != nil {
		return err
	}
	plans, err := listAgentPlans()
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		return fmt.Errorf("agent-orchestrate could not find created plan")
	}
	planID := plans[0].PlanID
	if opts.SkipGraph {
		if opts.JSON {
			data, _ := json.MarshalIndent(map[string]any{"plan_id": planID, "graph_skipped": true}, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Agent orchestration\n  plan id: %s\n  graph:   skipped\n", planID)
		return nil
	}
	graphOpts := AgentGraphRunOptions{JSON: opts.GraphJSON || opts.JSON, WorkersDir: opts.WorkersDir}
	if opts.JSON {
		graphOpts.JSON = true
	}
	var graphOut strings.Builder
	graphErr := AgentGraphRunCommand(&graphOut, planID, graphOpts)
	if opts.JSON {
		payload := map[string]any{"plan_id": planID, "graph_output": strings.TrimSpace(graphOut.String())}
		if graphErr != nil {
			payload["graph_error"] = graphErr.Error()
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return graphErr
	}
	fmt.Fprintln(stdout, "Agent orchestration")
	fmt.Fprintf(stdout, "  plan id: %s\n", planID)
	fmt.Fprintf(stdout, "  path:    %s\n", filepath.Join(agentPlansV1Root, planID))
	if graphErr != nil {
		fmt.Fprintf(stdout, "  graph:   failed (%v)\n", graphErr)
		return graphErr
	}
	fmt.Fprintln(stdout, "  graph:   completed")
	fmt.Fprint(stdout, graphOut.String())
	return nil
}

func VisualRequests(planID string, stdout io.Writer, opts VisualRequestsOptions) error {
	if strings.TrimSpace(planID) == "" {
		return fmt.Errorf("visual-requests requires an agent plan id")
	}
	path := filepath.Join(agentPlansV1Root, planID, visualRequestsArtifact)
	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; use --overwrite", visualRequestsArtifact)
		}
	}
	brief, err := readCreativeBrief(planID)
	if err != nil {
		return err
	}
	assets, err := readAssetRequirements(planID)
	if err != nil {
		return err
	}
	artifact := buildVisualRequestsDryRun(planID, time.Now().UTC(), brief, assets)
	if err := writeJSONFile(path, artifact); err != nil {
		return err
	}
	appendAgentPlanEvent(planID, "AGENT_VISUAL_REQUESTS_WRITTEN", map[string]any{"plan_id": planID, "path": path})
	if opts.JSON {
		data, _ := json.MarshalIndent(artifact, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Visual requests dry-run")
	fmt.Fprintf(stdout, "  plan id: %s\n", planID)
	fmt.Fprintf(stdout, "  requests: %d\n", len(artifact.Requests))
	fmt.Fprintf(stdout, "  path: %s\n", path)
	for _, missing := range artifact.MissingCapabilities {
		fmt.Fprintf(stdout, "  missing: %s\n", missing)
	}
	return nil
}

func readCreativeBrief(planID string) (CreativeBriefArtifact, error) {
	var artifact CreativeBriefArtifact
	err := readJSONFile(filepath.Join(agentPlansV1Root, planID, creativeBriefArtifact), &artifact)
	return artifact, err
}

func readJSONFile(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func readDeliverables(planID string) (DeliverablesArtifact, error) {
	var artifact DeliverablesArtifact
	err := readJSONFile(filepath.Join(agentPlansV1Root, planID, deliverablesArtifact), &artifact)
	return artifact, err
}

func readAssetRequirements(planID string) (AssetRequirementsArtifact, error) {
	var artifact AssetRequirementsArtifact
	err := readJSONFile(filepath.Join(agentPlansV1Root, planID, assetRequirementsArtifact), &artifact)
	return artifact, err
}

func readVisualRequestsDryRun(planID string) (VisualRequestsDryRunArtifact, error) {
	var artifact VisualRequestsDryRunArtifact
	err := readJSONFile(filepath.Join(agentPlansV1Root, planID, visualRequestsArtifact), &artifact)
	return artifact, err
}

func buildVisualRequestsDryRun(planID string, now time.Time, brief CreativeBriefArtifact, assets AssetRequirementsArtifact) VisualRequestsDryRunArtifact {
	artifact := VisualRequestsDryRunArtifact{
		SchemaVersion: visualRequestsDryRunSchema,
		CreatedAt:     now,
		PlanID:        planID,
	}
	cfg, _ := config.Load(config.DefaultPath)
	for _, req := range assets.Requirements {
		if !isVisualAssetRequirement(req) {
			continue
		}
		routeKey := visualRouteForRequirement(req)
		backendName := ""
		backend := config.ToolBackendConfig{}
		backendExists := false
		for _, candidate := range visualRouteCandidates(routeKey, req) {
			if cfg.Tools.Routes != nil {
				if name := cfg.Tools.Routes[candidate]; name != "" {
					backendName = name
					routeKey = candidate
					backend, backendExists = cfg.Tools.Backends[name]
					break
				}
			}
		}
		status := "previewed"
		warnings := []string{}
		if backendName == "" {
			status = "missing_backend"
			msg := fmt.Sprintf("%s missing; configure a route such as %s", req.Capability, routeKey)
			warnings = append(warnings, msg)
			artifact.MissingCapabilities = append(artifact.MissingCapabilities, msg)
		} else if !backendExists {
			status = "missing_backend"
			msg := fmt.Sprintf("%s route %s points to missing backend %s", req.Capability, routeKey, backendName)
			warnings = append(warnings, msg)
			artifact.MissingCapabilities = append(artifact.MissingCapabilities, msg)
		}
		request := VisualGenerationRequest{
			ID:            fmt.Sprintf("visual_req_%04d", len(artifact.Requests)+1),
			RequirementID: req.ID,
			Kind:          req.Kind,
			Capability:    routeKey,
			Route:         routeKey,
			Backend:       backendName,
			Provider:      backend.Provider,
			Model:         backend.Model,
			Endpoint:      backend.Endpoint,
			Auth:          VisualRequestAuth{Type: backend.Auth.Type, Env: backend.Auth.Env},
			Status:        status,
			RequestPreview: map[string]any{
				"prompt":            buildVisualPrompt(brief, req),
				"source_media":      brief.SourceMedia,
				"style":             brief.Style,
				"duration_seconds":  brief.Duration.TargetSeconds,
				"aspect_ratio":      brief.AspectRatio,
				"no_provider_calls": true,
			},
			OutputContract: map[string]any{
				"artifact": fmt.Sprintf("%s_%s_placeholder.json", req.ID, req.Kind),
				"format":   "planned_asset_reference",
			},
			Warnings: warnings,
		}
		artifact.Requests = append(artifact.Requests, request)
	}
	artifact.MissingCapabilities = dedupeStrings(artifact.MissingCapabilities)
	return artifact
}

func isVisualAssetRequirement(req AssetRequirement) bool {
	switch req.Kind {
	case "generated_broll", "generated_image", "visual_transform", "style_transfer", "object_remove", "background_replace":
		return true
	}
	switch req.Capability {
	case "creative.broll_generate", "creative.video_generate", "creative.image_generate", "creative.visual_transform", "creative.style_transfer", "creative.object_remove", "creative.background_replace", "video_generation_or_image_generation", "visual_transform", "image_generation", "video_generation", "style_transfer", "object_removal":
		return true
	}
	return false
}

func visualRouteForRequirement(req AssetRequirement) string {
	switch req.Kind {
	case "generated_broll":
		return "creative.broll_generate"
	case "generated_image":
		return "creative.image_generate"
	case "visual_transform":
		return "creative.visual_transform"
	case "style_transfer":
		return "creative.style_transfer"
	case "object_remove":
		return "creative.object_remove"
	case "background_replace":
		return "creative.background_replace"
	}
	switch req.Capability {
	case "creative.broll_generate", "creative.video_generate", "creative.image_generate", "creative.visual_transform", "creative.style_transfer", "creative.object_remove", "creative.background_replace":
		return req.Capability
	case "image_generation":
		return "creative.image_generate"
	case "video_generation", "video_generation_or_image_generation":
		return "creative.video_generate"
	case "style_transfer":
		return "creative.style_transfer"
	case "object_removal":
		return "creative.object_remove"
	default:
		return "creative.visual_transform"
	}
}

func visualRouteCandidates(primary string, req AssetRequirement) []string {
	candidates := []string{primary}
	switch req.Kind {
	case "generated_broll":
		candidates = append(candidates, "creative.video_generate", "creative.image_generate", "creative.video_broll", "creative.visual_asset")
	case "generated_image":
		candidates = append(candidates, "creative.image_generate", "creative.visual_asset")
	case "visual_transform":
		candidates = append(candidates, "creative.style_transfer", "creative.object_remove", "creative.background_replace", "creative.visual_transform")
	case "style_transfer":
		candidates = append(candidates, "creative.visual_transform")
	case "object_remove":
		candidates = append(candidates, "creative.visual_transform", "creative.object_removal")
	case "background_replace":
		candidates = append(candidates, "creative.visual_transform")
	}
	return dedupeStrings(candidates)
}

func buildVisualPrompt(brief CreativeBriefArtifact, req AssetRequirement) string {
	parts := []string{req.Description}
	if brief.Summary != "" {
		parts = append(parts, brief.Summary)
	}
	if len(brief.Style.Tone)+len(brief.Style.VisualStyle) > 0 {
		parts = append(parts, "Style: "+strings.Join(append(append([]string{}, brief.Style.Tone...), brief.Style.VisualStyle...), ", "))
	}
	if brief.Pacing.FirstSecondsNote != "" {
		parts = append(parts, brief.Pacing.FirstSecondsNote)
	}
	return strings.Join(parts, " ")
}

func enrichAgentActionsWithCreativeBrief(actions []AgentActionV1, brief CreativeBriefArtifact, deliverables DeliverablesArtifact, assets AssetRequirementsArtifact) []AgentActionV1 {
	for i := range actions {
		action := &actions[i]
		if action.Input == nil {
			action.Input = map[string]any{}
		}
		action.Input["creative_brief_ref"] = creativeBriefArtifact
		action.Input["deliverables_ref"] = deliverablesArtifact
		action.Input["asset_requirements_ref"] = assetRequirementsArtifact
		if brief.Duration.TargetSeconds > 0 {
			action.Input["desired_duration_seconds"] = brief.Duration.TargetSeconds
		}
		if brief.Summary != "" {
			action.Input["creative_summary"] = brief.Summary
		}
		if len(brief.Style.Tone) > 0 {
			action.Input["tone"] = brief.Style.Tone
		}
		if len(brief.Style.VisualStyle) > 0 {
			action.Input["visual_style"] = brief.Style.VisualStyle
		}
		if brief.Pacing.CutStyle != "" {
			action.Input["pacing"] = brief.Pacing.CutStyle
		}
		if brief.Pacing.FirstSecondsNote != "" {
			action.Input["opening_note"] = brief.Pacing.FirstSecondsNote
		}
		if brief.Captions.Style != "" {
			action.Input["caption_style"] = normalizeCaptionStyle(brief.Captions.Style)
		}
		if brief.Captions.Position != "" {
			action.Input["caption_position"] = normalizeCaptionPosition(brief.Captions.Position)
		}
		if brief.Requests.InstagramCaptions {
			action.Input["instagram_caption_options"] = true
		}
		if len(assets.Requirements) > 0 {
			action.Input["asset_requirements_count"] = len(assets.Requirements)
		}
		if action.Type == agentActionTypeMake {
			action.Description = makeActionDescriptionFromBrief(brief, deliverables, assets)
		}
	}
	return actions
}

func makeActionDescriptionFromBrief(brief CreativeBriefArtifact, deliverables DeliverablesArtifact, assets AssetRequirementsArtifact) string {
	parts := []string{"Create"}
	if brief.Duration.TargetSeconds > 0 {
		parts = append(parts, fmt.Sprintf("a %d-second", brief.Duration.TargetSeconds))
	} else {
		parts = append(parts, "a")
	}
	if brief.Platform != "" && brief.Platform != "original" {
		parts = append(parts, brief.Platform)
	}
	if len(brief.Style.Tone)+len(brief.Style.VisualStyle) > 0 {
		style := append(append([]string{}, brief.Style.Tone...), brief.Style.VisualStyle...)
		parts = append(parts, strings.Join(style, "/"))
	}
	parts = append(parts, "edit")
	if len(deliverables.Deliverables) > 1 {
		parts = append(parts, fmt.Sprintf("with %d deliverables", len(deliverables.Deliverables)))
	}
	if len(assets.Requirements) > 0 {
		parts = append(parts, fmt.Sprintf("and %d planned asset requirement(s)", len(assets.Requirements)))
	}
	return strings.Join(parts, " ") + "."
}

func normalizeCaptionStyle(style string) string {
	switch style {
	case "bold", "boxed":
		return style
	default:
		return style
	}
}

func normalizeCaptionPosition(position string) string {
	switch position {
	case "lower-third":
		return "bottom"
	default:
		return position
	}
}

func inferBriefPlatform(lower string, override string) string {
	if override != "" {
		return override
	}
	switch {
	case strings.Contains(lower, "instagram"), strings.Contains(lower, "reel"):
		return "instagram-reel"
	case strings.Contains(lower, "tiktok"), strings.Contains(lower, "tik tok"):
		return "tiktok"
	case strings.Contains(lower, "youtube short"), strings.Contains(lower, "shorts"):
		return "youtube-short"
	case strings.Contains(lower, "square"):
		return "square"
	case strings.Contains(lower, "youtube"):
		return "youtube"
	case strings.Contains(lower, "vertical"):
		return "instagram-reel"
	default:
		return "original"
	}
}

func aspectRatioForPlatform(platform string) string {
	switch platform {
	case "instagram-reel", "tiktok", "youtube-short":
		return "9:16"
	case "square":
		return "1:1"
	case "youtube":
		return "16:9"
	default:
		return "source"
	}
}

func extractDurationSeconds(lower string) int {
	re := regexp.MustCompile(`(\d{1,3})[\s-]*(second|seconds|sec|secs|s)\b`)
	m := re.FindStringSubmatch(lower)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func extractFirstSeconds(lower string) int {
	re := regexp.MustCompile(`first\s+(\d{1,2})\s*(second|seconds|sec|secs|s)\b`)
	m := re.FindStringSubmatch(lower)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func collectKeywords(lower string, candidates []string) []string {
	out := []string{}
	for _, c := range candidates {
		if strings.Contains(lower, c) {
			out = append(out, strings.ReplaceAll(c, " ", "_"))
		}
	}
	return out
}

func buildBriefSummary(brief CreativeBriefArtifact) string {
	parts := []string{}
	if brief.Duration.TargetSeconds > 0 {
		parts = append(parts, fmt.Sprintf("%d-second", brief.Duration.TargetSeconds))
	}
	if brief.Platform != "" && brief.Platform != "original" {
		parts = append(parts, brief.Platform)
	}
	style := append(append([]string{}, brief.Style.Tone...), brief.Style.VisualStyle...)
	if len(style) > 0 {
		parts = append(parts, strings.Join(style, " "))
	}
	if len(parts) == 0 {
		return brief.RawPrompt
	}
	return "Plan a " + strings.Join(parts, " ") + " edit from the creator brief."
}

func titleForPlatform(platform string) string {
	switch platform {
	case "instagram-reel":
		return "Instagram Reel"
	case "tiktok":
		return "TikTok Short"
	case "youtube-short":
		return "YouTube Short"
	default:
		return "Edited Video"
	}
}

func creativeToolRoutes() (map[string]string, map[string]string) {
	routes := map[string]string{}
	backends := map[string]string{}
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return routes, backends
	}
	for route, backend := range cfg.Tools.Routes {
		routes[route] = backend
		backends[backend] = backend
	}
	return routes, backends
}

func capabilityToRequirementStatus(cap string) string {
	switch cap {
	case "available":
		return "satisfied"
	case "missing_env":
		return "missing_env"
	case "missing":
		return "missing"
	default:
		return "unknown"
	}
}

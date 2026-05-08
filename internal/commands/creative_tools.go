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

	"github.com/mirelahmd/byom-video/internal/config"
)

const creativePlansRoot = ".byom-video/creative_plans"

type ToolsOptions struct{ JSON bool }
type ToolsValidateOptions struct {
	JSON     bool
	Strict   bool
	CheckEnv bool
}
type ToolsRequirementsOptions struct{ JSON bool }
type CreativePlanOptions struct {
	Goal          string
	JSON          bool
	WriteArtifact bool
	Strict        bool
}
type InspectCreativePlanOptions struct{ JSON bool }
type ReviewCreativePlanOptions struct {
	JSON          bool
	WriteArtifact bool
}

type ToolsSummary struct {
	Enabled  bool                                `json:"enabled"`
	Backends map[string]config.ToolBackendConfig `json:"backends,omitempty"`
	Routes   map[string]string                   `json:"routes,omitempty"`
}

type ToolsValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type CapabilityRequirement struct {
	Capability       string   `json:"capability"`
	Status           string   `json:"status"`
	MatchingRoutes   []string `json:"matching_routes,omitempty"`
	MatchingBackends []string `json:"matching_backends,omitempty"`
	Notes            []string `json:"notes,omitempty"`
	SuggestedRoutes  []string `json:"suggested_routes,omitempty"`
}

type CreativeStep struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Capability  string `json:"capability"`
	Route       string `json:"route,omitempty"`
	Backend     string `json:"backend,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type CreativePlan struct {
	SchemaVersion        string                  `json:"schema_version"`
	PlanID               string                  `json:"plan_id"`
	CreatedAt            time.Time               `json:"created_at"`
	InputPath            string                  `json:"input_path"`
	Goal                 string                  `json:"goal"`
	Mode                 string                  `json:"mode"`
	RequiredCapabilities []CreativeCapabilityRef `json:"required_capabilities"`
	Steps                []CreativeStep          `json:"steps"`
	Warnings             []string                `json:"warnings,omitempty"`
	Safety               CreativePlanSafety      `json:"safety"`
}

type CreativeCapabilityRef struct {
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Route   string `json:"route,omitempty"`
	Backend string `json:"backend,omitempty"`
}

type CreativePlanSafety struct {
	NoProviderCallsDuringPlanning bool `json:"no_provider_calls_during_planning"`
	NoInputFilesModified          bool `json:"no_input_files_modified"`
	MissingCapabilitiesDoNotBlock bool `json:"missing_capabilities_do_not_block_planning"`
}

type CreativePlanReview struct {
	PlanID               string                  `json:"plan_id"`
	Goal                 string                  `json:"goal"`
	InputPath            string                  `json:"input_path"`
	RequiredCapabilities []CreativeCapabilityRef `json:"required_capabilities"`
	Steps                []CreativeStep          `json:"steps"`
	Warnings             []string                `json:"warnings,omitempty"`
	SuggestedFixes       []string                `json:"suggested_fixes,omitempty"`
}

type creativeRequirementRule struct {
	Reason         string
	Kinds          []string
	SuggestedRoute string
}

func Tools(stdout io.Writer, opts ToolsOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	summary := ToolsSummary{
		Enabled:  cfg.Tools.Enabled,
		Backends: cloneToolBackends(cfg.Tools.Backends),
		Routes:   cloneRouting(cfg.Tools.Routes),
	}
	if opts.JSON {
		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Tools")
	fmt.Fprintf(stdout, "  enabled: %t\n", summary.Enabled)
	if !summary.Enabled {
		fmt.Fprintln(stdout, "  status:  tools are disabled")
	}
	if len(summary.Backends) > 0 {
		fmt.Fprintln(stdout, "  backends:")
		for _, name := range sortedToolBackendNames(summary.Backends) {
			backend := summary.Backends[name]
			fmt.Fprintf(stdout, "    - %s: kind=%s provider=%s", name, emptyDash(backend.Kind), emptyDash(backend.Provider))
			if backend.Model != "" {
				fmt.Fprintf(stdout, " model=%s", backend.Model)
			}
			if backend.Endpoint != "" {
				fmt.Fprintf(stdout, " endpoint=%s", backend.Endpoint)
			}
			if backend.Auth.Type != "" {
				fmt.Fprintf(stdout, " auth=%s", backend.Auth.Type)
			}
			if backend.Auth.Env != "" {
				fmt.Fprintf(stdout, " env=%s", backend.Auth.Env)
			}
			fmt.Fprintln(stdout)
		}
	}
	if len(summary.Routes) > 0 {
		fmt.Fprintln(stdout, "  routes:")
		for _, key := range sortedRoutingKeys(summary.Routes) {
			fmt.Fprintf(stdout, "    - %s: %s\n", key, summary.Routes[key])
		}
	}
	return nil
}

func ToolsValidate(stdout io.Writer, opts ToolsValidateOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	result := ValidateToolsConfig(cfg.Tools, opts.Strict, opts.CheckEnv)
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		if !result.Valid {
			return fmt.Errorf("tools config validation failed")
		}
		return nil
	}
	fmt.Fprintln(stdout, "Tools validation")
	if result.Valid {
		fmt.Fprintln(stdout, "  status: ok")
	} else {
		fmt.Fprintln(stdout, "  status: failed")
	}
	if len(result.Errors) > 0 {
		fmt.Fprintln(stdout, "  errors:")
		for _, item := range result.Errors {
			fmt.Fprintf(stdout, "    - %s\n", item)
		}
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintln(stdout, "  warnings:")
		for _, item := range result.Warnings {
			fmt.Fprintf(stdout, "    - %s\n", item)
		}
	}
	if !result.Valid {
		return fmt.Errorf("tools config validation failed")
	}
	return nil
}

func ToolsRequirements(stdout io.Writer, goal string, opts ToolsRequirementsOptions) error {
	if strings.TrimSpace(goal) == "" {
		return fmt.Errorf("--goal is required")
	}
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	reqs := detectCapabilityRequirements(goal, cfg.Tools)
	if opts.JSON {
		data, err := json.MarshalIndent(reqs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Tools requirements")
	fmt.Fprintf(stdout, "  goal: %s\n", goal)
	for _, req := range reqs {
		fmt.Fprintf(stdout, "  - %s: %s\n", req.Capability, req.Status)
		if len(req.MatchingRoutes) > 0 {
			fmt.Fprintf(stdout, "    routes: %s\n", strings.Join(req.MatchingRoutes, ", "))
		}
		if len(req.MatchingBackends) > 0 {
			fmt.Fprintf(stdout, "    backends: %s\n", strings.Join(req.MatchingBackends, ", "))
		}
		for _, note := range req.Notes {
			fmt.Fprintf(stdout, "    note: %s\n", note)
		}
		if len(req.SuggestedRoutes) > 0 {
			fmt.Fprintf(stdout, "    suggested: %s\n", strings.Join(req.SuggestedRoutes, ", "))
		}
	}
	return nil
}

func CreativePlanCommand(inputPath string, stdout io.Writer, opts CreativePlanOptions) error {
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("--goal is required")
	}
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(inputPath)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(creativePlansRoot, 0o755)
	planID := time.Now().UTC().Format("20060102T150405Z") + "-" + shortID(strings.TrimSpace(opts.Goal))
	reqs := detectCapabilityRequirements(opts.Goal, cfg.Tools)
	plan, strictErr := buildCreativePlan(planID, abs, opts.Goal, reqs)
	if strictErr != nil && opts.Strict {
		return strictErr
	}
	path := filepath.Join(creativePlansRoot, planID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	if opts.WriteArtifact || !opts.JSON {
		if err := writeJSONFile(filepath.Join(path, "creative_plan.json"), plan); err != nil {
			return err
		}
	}
	if opts.JSON {
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Creative plan created")
	fmt.Fprintf(stdout, "  plan id:   %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  input:     %s\n", plan.InputPath)
	fmt.Fprintf(stdout, "  goal:      %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  path:      %s\n", filepath.Join(path, "creative_plan.json"))
	fmt.Fprintf(stdout, "  steps:     %d\n", len(plan.Steps))
	for _, warning := range plan.Warnings {
		fmt.Fprintf(stdout, "  warning:   %s\n", warning)
	}
	if strictErr != nil {
		fmt.Fprintf(stdout, "  note:      %s\n", strictErr.Error())
	}
	return nil
}

func CreativePlans(stdout io.Writer) error {
	entries, err := os.ReadDir(creativePlansRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "No creative plans found.")
			return nil
		}
		return err
	}
	type row struct {
		ID        string
		CreatedAt time.Time
		InputPath string
		Goal      string
	}
	rows := []row{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		plan, err := readCreativePlan(entry.Name())
		if err != nil {
			continue
		}
		rows = append(rows, row{ID: plan.PlanID, CreatedAt: plan.CreatedAt, InputPath: plan.InputPath, Goal: plan.Goal})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No creative plans found.")
		return nil
	}
	fmt.Fprintf(stdout, "%-28s %-25s %-24s %s\n", "PLAN ID", "CREATED AT", "INPUT", "GOAL")
	for _, item := range rows {
		fmt.Fprintf(stdout, "%-28s %-25s %-24s %s\n", item.ID, item.CreatedAt.Format(time.RFC3339), truncate(filepath.Base(item.InputPath), 24), item.Goal)
	}
	return nil
}

func InspectCreativePlan(planID string, stdout io.Writer, opts InspectCreativePlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")

	// read as map to capture patched fields (approval_status, execution_status, etc.)
	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	plan, err := readCreativePlan(planID)
	if err != nil {
		return err
	}

	if opts.JSON {
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	approvalStatus, _ := m["approval_status"].(string)
	executionStatus, _ := m["execution_status"].(string)
	previewArtifact, _ := m["request_preview_artifact"].(string)
	if approvalStatus == "" {
		approvalStatus = "pending"
	}
	if executionStatus == "" {
		executionStatus = "not_started"
	}

	eventsPath := filepath.Join(planDir, "events.jsonl")
	eventsExist := ""
	if _, err := os.Stat(eventsPath); err == nil {
		eventsExist = eventsPath
	}

	// read outputs index if present
	outputsDir := filepath.Join(planDir, "outputs")
	outputsIndexPath := filepath.Join(outputsDir, "creative_outputs.json")
	outputsReviewPath := filepath.Join(planDir, "creative_outputs_review.md")
	var outputArtifacts []CreativeOutputArtifact
	if outData, err := os.ReadFile(outputsIndexPath); err == nil {
		var idx CreativeOutputsIndex
		if json.Unmarshal(outData, &idx) == nil {
			outputArtifacts = idx.Artifacts
		}
	}

	// also update step statuses from the map (may have been patched since readCreativePlan)
	stepStatuses := map[string]string{}
	if stepsRaw, ok := m["steps"].([]any); ok {
		for _, s := range stepsRaw {
			if sm, ok := s.(map[string]any); ok {
				id, _ := sm["id"].(string)
				status, _ := sm["status"].(string)
				if id != "" {
					stepStatuses[id] = status
				}
			}
		}
	}

	fmt.Fprintln(stdout, "Creative plan")
	fmt.Fprintf(stdout, "  plan id:          %s\n", plan.PlanID)
	fmt.Fprintf(stdout, "  input:            %s\n", plan.InputPath)
	fmt.Fprintf(stdout, "  goal:             %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  path:             %s\n", planPath)
	fmt.Fprintf(stdout, "  approval_status:  %s\n", approvalStatus)
	fmt.Fprintf(stdout, "  execution_status: %s\n", executionStatus)
	if previewArtifact != "" {
		fmt.Fprintf(stdout, "  preview artifact: %s\n", filepath.Join(planDir, previewArtifact))
	}
	if eventsExist != "" {
		fmt.Fprintf(stdout, "  events:           %s\n", eventsExist)
	}
	if len(outputArtifacts) > 0 {
		fmt.Fprintf(stdout, "  outputs dir:      %s\n", outputsDir)
		fmt.Fprintf(stdout, "  outputs index:    %s\n", outputsIndexPath)
		if _, err := os.Stat(outputsReviewPath); err == nil {
			fmt.Fprintf(stdout, "  outputs review:   %s\n", outputsReviewPath)
		}
		fmt.Fprintf(stdout, "  output artifacts: %d\n", len(outputArtifacts))
	}

	// timeline and render plan
	timelinePath := filepath.Join(outputsDir, "creative_timeline.json")
	if tlData, err := os.ReadFile(timelinePath); err == nil {
		var tl CreativeTimelineArtifact
		if json.Unmarshal(tlData, &tl) == nil {
			fmt.Fprintf(stdout, "  timeline:         %s\n", timelinePath)
			fmt.Fprintf(stdout, "  timeline tracks:  %d\n", len(tl.Tracks))
			fmt.Fprintf(stdout, "  timeline duration:%.2fs\n", tl.TotalDuration)
		}
	}
	renderPlanPath := filepath.Join(outputsDir, "creative_render_plan.json")
	if rpData, err := os.ReadFile(renderPlanPath); err == nil {
		var rp CreativeRenderPlanArtifact
		if json.Unmarshal(rpData, &rp) == nil {
			fmt.Fprintf(stdout, "  render plan:      %s\n", renderPlanPath)
			fmt.Fprintf(stdout, "  render steps:     %d\n", len(rp.Steps))
			fmt.Fprintf(stdout, "  render output:    %s\n", rp.PlannedOutput.PlannedFile)
		}
	}
	assembleResultPath := filepath.Join(outputsDir, "creative_assemble_result.json")
	if arData, err := os.ReadFile(assembleResultPath); err == nil {
		var ar CreativeAssembleResult
		if json.Unmarshal(arData, &ar) == nil {
			fmt.Fprintf(stdout, "  assemble status:  %s\n", ar.Status)
			fmt.Fprintf(stdout, "  assemble mode:    %s\n", ar.Mode)
			finalOut := ar.FinalOutputFile
			if finalOut == "" {
				finalOut = ar.OutputFile
			}
			fmt.Fprintf(stdout, "  draft output:     %s\n", finalOut)
			if _, err := os.Stat(filepath.Join(planDir, finalOut)); err == nil {
				fmt.Fprintf(stdout, "  draft exists:     yes\n")
			}
			if ar.Captions != nil && ar.Captions.Requested {
				fmt.Fprintf(stdout, "  captions:         %s\n", ar.Captions.Status)
			}
			if ar.Voiceover != nil && ar.Voiceover.Requested {
				fmt.Fprintf(stdout, "  voiceover:        %s\n", ar.Voiceover.Status)
			}
			assembleReviewPath := filepath.Join(outputsDir, "creative_assemble_review.md")
			if _, err := os.Stat(assembleReviewPath); err == nil {
				fmt.Fprintf(stdout, "  assemble review:  %s\n", assembleReviewPath)
			}
		}
	}

	fmt.Fprintf(stdout, "  steps:            %d\n", len(plan.Steps))
	for _, step := range plan.Steps {
		status := step.Status
		if s, ok := stepStatuses[step.ID]; ok && s != "" {
			status = s
		}
		fmt.Fprintf(stdout, "    %s  %-35s  %s\n", step.ID, step.Type, status)
	}
	for _, capability := range plan.RequiredCapabilities {
		fmt.Fprintf(stdout, "  - %s: %s\n", capability.Kind, capability.Status)
	}
	return nil
}

func ReviewCreativePlan(planID string, stdout io.Writer, opts ReviewCreativePlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)

	raw, err := os.ReadFile(filepath.Join(planDir, "creative_plan.json"))
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	plan, err := readCreativePlan(planID)
	if err != nil {
		return err
	}

	approvalStatus, _ := m["approval_status"].(string)
	executionStatus, _ := m["execution_status"].(string)
	previewArtifact, _ := m["request_preview_artifact"].(string)
	if approvalStatus == "" {
		approvalStatus = "pending"
	}
	if executionStatus == "" {
		executionStatus = "not_started"
	}

	// read stub outputs if present
	var reviewOutputArtifacts []CreativeOutputArtifact
	if outData, err := os.ReadFile(filepath.Join(planDir, "outputs", "creative_outputs.json")); err == nil {
		var idx CreativeOutputsIndex
		if json.Unmarshal(outData, &idx) == nil {
			reviewOutputArtifacts = idx.Artifacts
		}
	}

	nextCmds := []string{}
	if approvalStatus != "approved" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video approve-creative-plan %s", planID))
	}
	if previewArtifact == "" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-preview %s", planID))
	}
	if executionStatus == "not_started" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video execute-creative-plan %s", planID))
	}
	if executionStatus != "stub_completed" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-execute-stub %s", planID))
	}
	if len(reviewOutputArtifacts) > 0 {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video review-creative-outputs %s", planID))
	}

	review := CreativePlanReview{
		PlanID:               plan.PlanID,
		Goal:                 plan.Goal,
		InputPath:            plan.InputPath,
		RequiredCapabilities: plan.RequiredCapabilities,
		Steps:                plan.Steps,
		Warnings:             plan.Warnings,
		SuggestedFixes:       creativePlanFixes(plan),
	}

	artifactPath := ""
	if opts.WriteArtifact {
		artifactPath = filepath.Join(planDir, "creative_plan_review.md")
		var b strings.Builder
		b.WriteString("# Creative Plan Review\n\n")
		fmt.Fprintf(&b, "- Plan ID: `%s`\n", plan.PlanID)
		fmt.Fprintf(&b, "- Goal: %s\n", plan.Goal)
		fmt.Fprintf(&b, "- Input: `%s`\n", plan.InputPath)
		fmt.Fprintf(&b, "- Approval: `%s`\n", approvalStatus)
		fmt.Fprintf(&b, "- Execution: `%s`\n\n", executionStatus)
		b.WriteString("## Required Capabilities\n\n")
		for _, capability := range plan.RequiredCapabilities {
			fmt.Fprintf(&b, "- `%s` `%s`", capability.Kind, capability.Status)
			if capability.Route != "" {
				fmt.Fprintf(&b, " route=`%s`", capability.Route)
			}
			if capability.Backend != "" {
				fmt.Fprintf(&b, " backend=`%s`", capability.Backend)
			}
			fmt.Fprintf(&b, "\n  - %s\n", capability.Reason)
		}
		b.WriteString("\n## Steps\n\n")
		for _, step := range plan.Steps {
			fmt.Fprintf(&b, "- `%s` `%s` `%s`\n", step.ID, step.Type, step.Capability)
			fmt.Fprintf(&b, "  - %s\n", step.Description)
		}
		if len(review.Warnings) > 0 {
			b.WriteString("\n## Warnings\n\n")
			for _, warning := range review.Warnings {
				fmt.Fprintf(&b, "- %s\n", warning)
			}
		}
		if len(review.SuggestedFixes) > 0 {
			b.WriteString("\n## Suggested Config Fixes\n\n")
			for _, fix := range review.SuggestedFixes {
				fmt.Fprintf(&b, "- %s\n", fix)
			}
		}
		if len(reviewOutputArtifacts) > 0 {
			b.WriteString("\n## Stub Outputs\n\n")
			for _, a := range reviewOutputArtifacts {
				fmt.Fprintf(&b, "- `%s` `%s` (%s)\n", a.Type, a.Path, a.Status)
			}
		}
		if len(nextCmds) > 0 {
			b.WriteString("\n## Next Steps\n\n")
			for _, cmd := range nextCmds {
				fmt.Fprintf(&b, "```sh\n%s\n```\n", cmd)
			}
		}
		if err := os.WriteFile(artifactPath, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}

	if opts.JSON {
		out := map[string]any{
			"plan_id":          review.PlanID,
			"goal":             review.Goal,
			"input_path":       review.InputPath,
			"approval_status":  approvalStatus,
			"execution_status": executionStatus,
			"preview_artifact": previewArtifact,
			"output_artifacts": len(reviewOutputArtifacts),
			"capabilities":     review.RequiredCapabilities,
			"steps":            review.Steps,
			"warnings":         review.Warnings,
			"suggested_fixes":  review.SuggestedFixes,
			"next_commands":    nextCmds,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Creative plan review")
	fmt.Fprintf(stdout, "  plan id:          %s\n", review.PlanID)
	fmt.Fprintf(stdout, "  input:            %s\n", review.InputPath)
	fmt.Fprintf(stdout, "  goal:             %s\n", review.Goal)
	fmt.Fprintf(stdout, "  approval_status:  %s\n", approvalStatus)
	fmt.Fprintf(stdout, "  execution_status: %s\n", executionStatus)
	if previewArtifact != "" {
		fmt.Fprintf(stdout, "  preview artifact: %s\n", filepath.Join(planDir, previewArtifact))
	}
	if len(reviewOutputArtifacts) > 0 {
		fmt.Fprintf(stdout, "  output artifacts: %d\n", len(reviewOutputArtifacts))
		for _, a := range reviewOutputArtifacts {
			fmt.Fprintf(stdout, "    %s: %s\n", a.Type, a.Path)
		}
	}
	for _, capability := range review.RequiredCapabilities {
		fmt.Fprintf(stdout, "  - %s: %s\n", capability.Kind, capability.Status)
	}
	for _, warning := range review.Warnings {
		fmt.Fprintf(stdout, "  warning:          %s\n", warning)
	}
	for _, fix := range review.SuggestedFixes {
		fmt.Fprintf(stdout, "  fix:              %s\n", fix)
	}
	if artifactPath != "" {
		fmt.Fprintf(stdout, "  artifact:         %s\n", artifactPath)
	}
	if len(nextCmds) > 0 {
		fmt.Fprintln(stdout, "  next:")
		for _, cmd := range nextCmds {
			fmt.Fprintf(stdout, "    %s\n", cmd)
		}
	}
	return nil
}

func ValidateToolsConfig(tools config.ToolsConfig, strict bool, checkEnv bool) ToolsValidationResult {
	result := ToolsValidationResult{Valid: true}
	knownKinds := knownToolCapabilityKinds()
	for name, backend := range tools.Backends {
		if strings.TrimSpace(name) == "" {
			result.Errors = append(result.Errors, "tool backend logical name must be non-empty")
		}
		if strings.TrimSpace(backend.Kind) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q kind is required", name))
		} else if !knownKinds[backend.Kind] {
			msg := fmt.Sprintf("tool backend %q has unknown kind %q", name, backend.Kind)
			if strict {
				result.Errors = append(result.Errors, msg)
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
		}
		if strings.TrimSpace(backend.Provider) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q provider is required", name))
		}
		if backend.Endpoint == "" && backend.Provider != "" && backend.Provider != "local-command" && backend.Kind != "local_command" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("tool backend %q has no endpoint", name))
		}
		if backend.Model == "" && generationLikeToolKind(backend.Kind) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("tool backend %q has no model configured", name))
		}
		validateToolAuth(name, backend.Auth, strict, checkEnv, &result)
	}
	for route, backendName := range tools.Routes {
		if strings.TrimSpace(route) == "" {
			result.Errors = append(result.Errors, "tool route key must be non-empty")
			continue
		}
		if _, ok := tools.Backends[backendName]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("tool route %q points to missing backend %q", route, backendName))
		}
	}
	result.Valid = len(result.Errors) == 0
	return result
}

func detectCapabilityRequirements(goal string, tools config.ToolsConfig) []CapabilityRequirement {
	lower := strings.ToLower(strings.TrimSpace(goal))
	rules := []creativeRequirementRule{}
	add := func(reason string, suggested string, kinds ...string) {
		rules = append(rules, creativeRequirementRule{Reason: reason, SuggestedRoute: suggested, Kinds: kinds})
	}
	if strings.Contains(lower, "narration") || strings.Contains(lower, "voiceover") || strings.Contains(lower, "voice over") {
		add("Draft narration/script.", "creative.script", "text_generation")
		add("Generate voiceover audio.", "creative.voiceover", "voice_generation")
		add("Compose the final timed output.", "creative.render", "render_composition")
	}
	if strings.Contains(lower, "cinematic") || strings.Contains(lower, "short") {
		add("Compose the final timed output.", "creative.render", "render_composition")
	}
	if strings.Contains(lower, "b-roll") || strings.Contains(lower, "broll") {
		add("Generate supporting visual assets.", "creative.video_broll", "video_generation", "image_generation")
	}
	if strings.Contains(lower, "captions") || strings.Contains(lower, "caption") {
		add("Generate captions or caption variants.", "creative.captions", "caption_generation", "text_generation")
	}
	if strings.Contains(lower, "remove object") || strings.Contains(lower, "object removal") {
		add("Remove objects from visuals.", "creative.object_removal", "object_removal")
	}
	if strings.Contains(lower, "translate") || strings.Contains(lower, "spanish") {
		add("Translate text or captions.", "creative.translation", "translation")
	}
	if len(rules) == 0 {
		add("Draft script or creative text.", "creative.script", "text_generation")
	}
	reqs := make([]CapabilityRequirement, 0, len(rules))
	for _, rule := range rules {
		req := CapabilityRequirement{
			Capability:      strings.Join(rule.Kinds, " or "),
			Status:          "missing",
			SuggestedRoutes: []string{rule.SuggestedRoute},
			Notes:           []string{rule.Reason},
		}
		for routeKey, backendName := range tools.Routes {
			backend, ok := tools.Backends[backendName]
			if !ok {
				continue
			}
			for _, kind := range rule.Kinds {
				if backend.Kind == kind {
					req.MatchingRoutes = append(req.MatchingRoutes, routeKey)
					req.MatchingBackends = append(req.MatchingBackends, backendName)
				}
			}
		}
		req.MatchingRoutes = dedupeStrings(req.MatchingRoutes)
		req.MatchingBackends = dedupeStrings(req.MatchingBackends)
		switch {
		case len(req.MatchingRoutes) > 0:
			req.Status = "satisfied"
		case tools.Enabled && hasAnyBackendForKinds(tools.Backends, rule.Kinds):
			req.Status = "partial"
			req.Notes = append(req.Notes, "matching backend exists but no route is configured")
		default:
			req.Notes = append(req.Notes, fmt.Sprintf("%s is missing. Configure a backend and route such as %s.", primaryKind(rule.Kinds), rule.SuggestedRoute))
		}
		reqs = append(reqs, req)
	}
	merged := map[string]CapabilityRequirement{}
	order := []string{}
	for _, req := range reqs {
		existing, ok := merged[req.Capability]
		if !ok {
			req.MatchingRoutes = dedupeStrings(req.MatchingRoutes)
			req.MatchingBackends = dedupeStrings(req.MatchingBackends)
			req.Notes = dedupeStrings(req.Notes)
			req.SuggestedRoutes = dedupeStrings(req.SuggestedRoutes)
			merged[req.Capability] = req
			order = append(order, req.Capability)
			continue
		}
		existing.MatchingRoutes = dedupeStrings(append(existing.MatchingRoutes, req.MatchingRoutes...))
		existing.MatchingBackends = dedupeStrings(append(existing.MatchingBackends, req.MatchingBackends...))
		existing.Notes = dedupeStrings(append(existing.Notes, req.Notes...))
		existing.SuggestedRoutes = dedupeStrings(append(existing.SuggestedRoutes, req.SuggestedRoutes...))
		switch {
		case existing.Status == "satisfied" || req.Status == "satisfied":
			existing.Status = "satisfied"
		case existing.Status == "partial" || req.Status == "partial":
			existing.Status = "partial"
		default:
			existing.Status = "missing"
		}
		merged[req.Capability] = existing
	}
	out := make([]CapabilityRequirement, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func buildCreativePlan(planID string, inputPath string, goal string, reqs []CapabilityRequirement) (CreativePlan, error) {
	plan := CreativePlan{
		SchemaVersion: "creative_plan.v1",
		PlanID:        planID,
		CreatedAt:     time.Now().UTC(),
		InputPath:     inputPath,
		Goal:          goal,
		Mode:          "deterministic_planning",
		Safety: CreativePlanSafety{
			NoProviderCallsDuringPlanning: true,
			NoInputFilesModified:          true,
			MissingCapabilitiesDoNotBlock: true,
		},
	}
	missing := []string{}
	for i, req := range reqs {
		stepType := stepTypeForCapability(primaryKind(strings.Split(req.Capability, " or ")))
		route := ""
		backend := ""
		if len(req.MatchingRoutes) > 0 {
			route = req.MatchingRoutes[0]
		}
		if len(req.MatchingBackends) > 0 {
			backend = req.MatchingBackends[0]
		}
		kind := primaryKind(strings.Split(req.Capability, " or "))
		plan.RequiredCapabilities = append(plan.RequiredCapabilities, CreativeCapabilityRef{
			Kind:    kind,
			Reason:  firstNonEmpty(req.Notes...),
			Status:  req.Status,
			Route:   route,
			Backend: backend,
		})
		plan.Steps = append(plan.Steps, CreativeStep{
			ID:          fmt.Sprintf("step_%04d", i+1),
			Type:        stepType,
			Capability:  kind,
			Route:       route,
			Backend:     backend,
			Status:      "planned",
			Description: stepDescription(stepType, goal),
		})
		if req.Status != "satisfied" {
			missing = append(missing, fmt.Sprintf("%s is missing; configure %s", kind, firstNonEmpty(req.SuggestedRoutes...)))
		}
	}
	plan.Warnings = dedupeStrings(missing)
	if len(plan.Warnings) > 0 {
		return plan, fmt.Errorf("creative plan has missing capabilities")
	}
	return plan, nil
}

func readCreativePlan(planID string) (CreativePlan, error) {
	data, err := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	if err != nil {
		return CreativePlan{}, err
	}
	var plan CreativePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return CreativePlan{}, err
	}
	return plan, nil
}

func creativePlanFixes(plan CreativePlan) []string {
	fixes := []string{}
	for _, warning := range plan.Warnings {
		fixes = append(fixes, warning)
	}
	return dedupeStrings(fixes)
}

func validateToolAuth(name string, auth config.ToolAuthConfig, strict bool, checkEnv bool, result *ToolsValidationResult) {
	switch auth.Type {
	case "", "none":
	case "bearer_env", "header_env", "query_env":
		if strings.TrimSpace(auth.Env) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q auth env is required for auth type %q", name, auth.Type))
		}
		if auth.Type == "header_env" && strings.TrimSpace(auth.Header) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q auth header is required for auth type header_env", name))
		}
	case "basic_env":
		if strings.TrimSpace(auth.Username) == "" || strings.TrimSpace(auth.Password) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q basic_env requires username and password env names", name))
		}
	default:
		result.Errors = append(result.Errors, fmt.Sprintf("tool backend %q auth type %q is invalid", name, auth.Type))
	}
	if !checkEnv {
		return
	}
	for _, envName := range []string{auth.Env, auth.Username, auth.Password} {
		if strings.TrimSpace(envName) == "" {
			continue
		}
		if _, ok := os.LookupEnv(envName); !ok {
			msg := fmt.Sprintf("referenced env var %q is not set", envName)
			if strict {
				result.Errors = append(result.Errors, msg)
			} else {
				result.Warnings = append(result.Warnings, msg)
			}
		}
	}
}

func knownToolCapabilityKinds() map[string]bool {
	return map[string]bool{
		"text_generation":         true,
		"voice_generation":        true,
		"image_generation":        true,
		"video_generation":        true,
		"caption_generation":      true,
		"audio_generation":        true,
		"music_generation":        true,
		"sound_effect_generation": true,
		"audio_cleanup":           true,
		"object_removal":          true,
		"style_transfer":          true,
		"translation":             true,
		"render_composition":      true,
		"local_command":           true,
		"custom":                  true,
	}
}

func generationLikeToolKind(kind string) bool {
	switch kind {
	case "text_generation", "voice_generation", "image_generation", "video_generation", "caption_generation", "audio_generation", "music_generation", "sound_effect_generation", "translation":
		return true
	default:
		return false
	}
}

func hasAnyBackendForKinds(backends map[string]config.ToolBackendConfig, kinds []string) bool {
	for _, backend := range backends {
		for _, kind := range kinds {
			if backend.Kind == kind {
				return true
			}
		}
	}
	return false
}

func primaryKind(kinds []string) string {
	if len(kinds) == 0 {
		return ""
	}
	return strings.TrimSpace(kinds[0])
}

func stepTypeForCapability(kind string) string {
	switch kind {
	case "text_generation":
		return "generate_script"
	case "voice_generation":
		return "generate_voiceover"
	case "image_generation", "video_generation":
		return "generate_visual_asset"
	case "caption_generation":
		return "generate_captions_or_caption_variants"
	case "render_composition":
		return "render_draft"
	case "music_generation", "audio_generation", "sound_effect_generation":
		return "generate_audio_asset"
	case "object_removal", "style_transfer":
		return "visual_transform"
	case "translation":
		return "translate_text"
	default:
		return "custom_step"
	}
}

func stepDescription(stepType string, goal string) string {
	switch stepType {
	case "generate_script":
		return "Generate a short script from the user goal."
	case "generate_voiceover":
		return "Generate narration or voiceover for the goal."
	case "generate_visual_asset":
		return "Generate supporting visual assets for the goal."
	case "generate_captions_or_caption_variants":
		return "Generate captions or caption variants for the goal."
	case "render_draft":
		return "Render a draft composition for the goal."
	case "generate_audio_asset":
		return "Generate supporting audio assets for the goal."
	case "visual_transform":
		return "Apply a visual transform required by the goal."
	case "translate_text":
		return "Translate text required by the goal."
	default:
		return "Plan a creative step for the user goal: " + goal
	}
}

func shortID(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, value)
	if value == "" {
		return "creative"
	}
	if len(value) > 16 {
		return value[:16]
	}
	return value
}

func cloneToolBackends(in map[string]config.ToolBackendConfig) map[string]config.ToolBackendConfig {
	out := map[string]config.ToolBackendConfig{}
	for key, value := range in {
		if value.Options != nil {
			value.Options = cloneAnyMap(value.Options)
		}
		if value.ResponseMapping != nil {
			value.ResponseMapping = cloneAnyMap(value.ResponseMapping)
		}
		out[key] = value
	}
	return out
}

func sortedToolBackendNames(backends map[string]config.ToolBackendConfig) []string {
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

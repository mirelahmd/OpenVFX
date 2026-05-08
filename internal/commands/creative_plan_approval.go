package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
)

// ---- extended CreativePlan fields (patched onto disk) ----

type CreativePlanPatch struct {
	ApprovalStatus         string     `json:"approval_status,omitempty"`
	ApprovedAt             *time.Time `json:"approved_at,omitempty"`
	ApprovalMode           string     `json:"approval_mode,omitempty"`
	ExecutionStatus        string     `json:"execution_status,omitempty"`
	RequestPreviewArtifact string     `json:"request_preview_artifact,omitempty"`
	ReviewArtifact         string     `json:"review_artifact,omitempty"`
}

// ---- creative request preview types ----

type CreativeRequestsPreview struct {
	SchemaVersion       string                `json:"schema_version"`
	CreatedAt           time.Time             `json:"created_at"`
	CreativePlanID      string                `json:"creative_plan_id"`
	Goal                string                `json:"goal"`
	InputPath           string                `json:"input_path"`
	Requests            []CreativeRequestItem `json:"requests"`
	MissingCapabilities []string              `json:"missing_capabilities,omitempty"`
	Warnings            []string              `json:"warnings,omitempty"`
}

type CreativeRequestItem struct {
	StepID         string                `json:"step_id"`
	StepType       string                `json:"step_type"`
	Capability     string                `json:"capability"`
	Route          string                `json:"route"`
	Backend        string                `json:"backend"`
	Provider       string                `json:"provider"`
	Model          string                `json:"model"`
	Endpoint       string                `json:"endpoint"`
	Auth           CreativeRequestAuth   `json:"auth"`
	Status         string                `json:"status"`
	RequestPreview CreativeRequestPayload `json:"request_preview"`
	Warnings       []string              `json:"warnings,omitempty"`
}

type CreativeRequestAuth struct {
	Type string `json:"type"`
	Env  string `json:"env"`
}

type CreativeRequestPayload struct {
	Instruction    string                `json:"instruction"`
	InputSummary   string                `json:"input_summary"`
	OutputContract CreativeOutputContract `json:"output_contract"`
}

type CreativeOutputContract struct {
	Artifact string `json:"artifact"`
	Format   string `json:"format"`
}

// ---- options ----

type ApproveCreativePlanOptions struct{}

type CreativePlanEventsOptions struct{ JSON bool }

type CreativePreviewOptions struct {
	JSON      bool
	Strict    bool
	Overwrite bool
	CheckEnv  bool
}

type ExecuteCreativePlanOptions struct {
	Yes      bool
	DryRun   bool
	Strict   bool
	CheckEnv bool
	JSON     bool
}

type CreativeResultOptions struct {
	JSON          bool
	WriteArtifact bool
}

type ValidateCreativePlanOptions struct{ JSON bool }

// ---- approve-creative-plan ----

func ApproveCreativePlan(planID string, stdout io.Writer, _ ApproveCreativePlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")
	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}
	if err := validateCreativePlanMap(m); err != nil {
		return fmt.Errorf("creative plan validation failed: %w", err)
	}
	if status, _ := m["approval_status"].(string); status == "approved" {
		fmt.Fprintf(stdout, "Creative plan %s is already approved.\n", planID)
		return nil
	}
	now := time.Now().UTC()
	m["approval_status"] = "approved"
	m["approved_at"] = now.Format(time.RFC3339)
	m["approval_mode"] = "manual"
	if err := writeJSONFile(planPath, m); err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_PLAN_APPROVED", map[string]any{"plan_id": planID, "approved_at": now})
		_ = log.Close()
	}
	fmt.Fprintf(stdout, "Creative plan approved: %s\n", planID)
	fmt.Fprintf(stdout, "  approved_at:    %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(stdout, "  approval_mode:  manual\n")
	return nil
}

// ---- creative-plan-events ----

func CreativePlanEvents(planID string, stdout io.Writer, opts CreativePlanEventsOptions) error {
	eventsPath := filepath.Join(creativePlansRoot, planID, "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "No events recorded for this creative plan.")
			return nil
		}
		return err
	}
	if opts.JSON {
		fmt.Fprint(stdout, string(data))
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			fmt.Fprintf(stdout, "  [malformed] %s\n", line)
			continue
		}
		t, _ := ev["time"].(string)
		typ, _ := ev["type"].(string)
		fmt.Fprintf(stdout, "  %s  %s\n", t, typ)
		count++
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No events recorded.")
	}
	return nil
}

// ---- creative-preview ----

func CreativePreview(planID string, stdout io.Writer, opts CreativePreviewOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	plan, err := readCreativePlan(planID)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	outPath := filepath.Join(planDir, "creative_requests.dryrun.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("creative_requests.dryrun.json already exists; use --overwrite")
		}
	}
	cfg, cfgErr := config.Load(config.DefaultPath)
	if cfgErr != nil {
		cfg = config.Config{}
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_PREVIEW_STARTED", map[string]any{"plan_id": planID})
	}

	preview := CreativeRequestsPreview{
		SchemaVersion:  "creative_requests.dryrun.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Goal:           plan.Goal,
		InputPath:      plan.InputPath,
	}

	for _, step := range plan.Steps {
		item := buildRequestItem(step, cfg.Tools, opts.Strict, opts.CheckEnv)
		preview.Requests = append(preview.Requests, item)
		if item.Status == "missing_backend" {
			preview.MissingCapabilities = append(preview.MissingCapabilities, fmt.Sprintf("%s: no backend/route configured", step.Capability))
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("step %s has no route configured", step.ID))
		}
	}
	preview.MissingCapabilities = dedupeStrings(preview.MissingCapabilities)
	preview.Warnings = dedupeStrings(preview.Warnings)

	if opts.Strict && len(preview.MissingCapabilities) > 0 {
		if log != nil {
			_ = log.Write("CREATIVE_PREVIEW_FAILED", map[string]any{"plan_id": planID, "reason": "missing backends in strict mode"})
			_ = log.Close()
		}
		return fmt.Errorf("creative preview failed: missing backends: %s", strings.Join(preview.MissingCapabilities, "; "))
	}

	if err := writeJSONFile(outPath, preview); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_PREVIEW_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	if log != nil {
		_ = log.Write("CREATIVE_PREVIEW_COMPLETED", map[string]any{"plan_id": planID, "requests": len(preview.Requests)})
		_ = log.Close()
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(preview, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Creative preview")
	fmt.Fprintf(stdout, "  plan id:    %s\n", planID)
	fmt.Fprintf(stdout, "  goal:       %s\n", plan.Goal)
	fmt.Fprintf(stdout, "  requests:   %d\n", len(preview.Requests))
	fmt.Fprintf(stdout, "  artifact:   %s\n", outPath)
	for _, req := range preview.Requests {
		fmt.Fprintf(stdout, "  step %s: %s -> backend=%s status=%s\n", req.StepID, req.StepType, emptyDash(req.Backend), req.Status)
	}
	for _, w := range preview.Warnings {
		fmt.Fprintf(stdout, "  warning:    %s\n", w)
	}
	return nil
}

// ---- execute-creative-plan ----

func ExecuteCreativePlan(planID string, stdout io.Writer, opts ExecuteCreativePlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")

	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	approvalStatus, _ := m["approval_status"].(string)
	if approvalStatus != "approved" && !opts.Yes {
		return fmt.Errorf("creative plan %s is not approved; run approve-creative-plan first or use --yes", planID)
	}

	if opts.DryRun {
		fmt.Fprintf(stdout, "Dry run: execute-creative-plan %s\n", planID)
		fmt.Fprintln(stdout, "  would run creative preview and update execution_status to dry_run_completed")
		fmt.Fprintln(stdout, "  no files written (--dry-run)")
		return nil
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_EXECUTION_STARTED", map[string]any{"plan_id": planID})
	}

	if opts.Yes && approvalStatus != "approved" {
		now := time.Now().UTC()
		m["approval_status"] = "approved"
		m["approved_at"] = now.Format(time.RFC3339)
		m["approval_mode"] = "yes_flag"
	}

	previewOpts := CreativePreviewOptions{
		Strict:    opts.Strict,
		CheckEnv:  opts.CheckEnv,
		Overwrite: true,
	}
	plan, _ := readCreativePlan(planID)
	cfg, cfgErr := config.Load(config.DefaultPath)
	if cfgErr != nil {
		cfg = config.Config{}
	}

	preview := CreativeRequestsPreview{
		SchemaVersion:  "creative_requests.dryrun.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		Goal:           plan.Goal,
		InputPath:      plan.InputPath,
	}
	for _, step := range plan.Steps {
		item := buildRequestItem(step, cfg.Tools, previewOpts.Strict, previewOpts.CheckEnv)
		preview.Requests = append(preview.Requests, item)
		if item.Status == "missing_backend" {
			preview.MissingCapabilities = append(preview.MissingCapabilities, fmt.Sprintf("%s: no backend/route configured", step.Capability))
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("step %s has no route", step.ID))
		}
		if log != nil {
			if item.Status == "previewed" {
				_ = log.Write("CREATIVE_STEP_PREVIEWED", map[string]any{"step_id": step.ID})
			} else {
				_ = log.Write("CREATIVE_STEP_SKIPPED", map[string]any{"step_id": step.ID, "reason": "missing_backend"})
			}
		}
	}
	preview.MissingCapabilities = dedupeStrings(preview.MissingCapabilities)
	preview.Warnings = dedupeStrings(preview.Warnings)

	outPath := filepath.Join(planDir, "creative_requests.dryrun.json")
	if err := writeJSONFile(outPath, preview); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_EXECUTION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	m["execution_status"] = "dry_run_completed"
	m["request_preview_artifact"] = "creative_requests.dryrun.json"
	if err := writeJSONFile(planPath, m); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_EXECUTION_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	if log != nil {
		_ = log.Write("CREATIVE_EXECUTION_COMPLETED", map[string]any{"plan_id": planID, "execution_status": "dry_run_completed"})
		_ = log.Close()
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(map[string]any{
			"plan_id":          planID,
			"execution_status": "dry_run_completed",
			"requests":         len(preview.Requests),
			"preview_artifact": outPath,
		}, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Creative plan executed (dry run): %s\n", planID)
	fmt.Fprintf(stdout, "  execution_status: dry_run_completed\n")
	fmt.Fprintf(stdout, "  requests:         %d\n", len(preview.Requests))
	fmt.Fprintf(stdout, "  preview artifact: %s\n", outPath)
	for _, w := range preview.Warnings {
		fmt.Fprintf(stdout, "  warning:          %s\n", w)
	}
	return nil
}

// ---- creative-result ----

func CreativeResult(planID string, stdout io.Writer, opts CreativeResultOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	raw, err := os.ReadFile(filepath.Join(planDir, "creative_plan.json"))
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	planID2, _ := m["plan_id"].(string)
	goal, _ := m["goal"].(string)
	approvalStatus, _ := m["approval_status"].(string)
	executionStatus, _ := m["execution_status"].(string)
	previewArtifact, _ := m["request_preview_artifact"].(string)
	if approvalStatus == "" {
		approvalStatus = "pending"
	}
	if executionStatus == "" {
		executionStatus = "not_started"
	}

	// collect missing capabilities from warnings
	var missing []string
	if wList, ok := m["warnings"].([]any); ok {
		for _, w := range wList {
			if s, ok := w.(string); ok {
				missing = append(missing, s)
			}
		}
	}

	// read stub outputs if present
	outputsDir := filepath.Join(planDir, "outputs")
	var outputArtifacts []CreativeOutputArtifact
	if outData, err := os.ReadFile(filepath.Join(outputsDir, "creative_outputs.json")); err == nil {
		var idx CreativeOutputsIndex
		if json.Unmarshal(outData, &idx) == nil {
			outputArtifacts = idx.Artifacts
		}
	}

	// next commands
	nextCmds := []string{}
	if approvalStatus != "approved" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video approve-creative-plan %s", planID2))
	}
	if previewArtifact == "" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-preview %s", planID2))
	}
	if executionStatus == "not_started" || executionStatus == "" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video execute-creative-plan %s", planID2))
	}
	if executionStatus != "stub_completed" {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-execute-stub %s", planID2))
	}
	if len(outputArtifacts) > 0 {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video review-creative-outputs %s", planID2))
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video validate-creative-plan %s", planID2))
	}
	timelineArtifactPath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	renderPlanArtifactPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	assembleResultArtifactPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")
	_, hasTimeline := os.Stat(timelineArtifactPath)
	_, hasRenderPlan := os.Stat(renderPlanArtifactPath)
	_, hasAssembleResult := os.Stat(assembleResultArtifactPath)
	if hasTimeline != nil {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-timeline %s", planID2))
	} else if hasRenderPlan != nil {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-render-plan %s", planID2))
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video review-creative-timeline %s", planID2))
	} else if hasAssembleResult != nil {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video creative-assemble %s", planID2))
	} else {
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video validate-creative-assemble %s", planID2))
		nextCmds = append(nextCmds, fmt.Sprintf("byom-video review-creative-assemble %s", planID2))
	}

	// read assemble result for draft path and media enrichments
	var draftPath string
	var captionsStatus, voiceoverStatus string
	if hasAssembleResult == nil {
		arData, _ := os.ReadFile(assembleResultArtifactPath)
		var ar CreativeAssembleResult
		if json.Unmarshal(arData, &ar) == nil {
			if ar.FinalOutputFile != "" {
				draftPath = ar.FinalOutputFile
			} else {
				draftPath = ar.OutputFile
			}
			if ar.Captions != nil && ar.Captions.Requested {
				captionsStatus = ar.Captions.Status
			}
			if ar.Voiceover != nil && ar.Voiceover.Requested {
				voiceoverStatus = ar.Voiceover.Status
			}
		}
	}

	result := map[string]any{
		"plan_id":           planID2,
		"goal":              goal,
		"approval_status":   approvalStatus,
		"execution_status":  executionStatus,
		"preview_artifact":  previewArtifact,
		"output_artifacts":  len(outputArtifacts),
		"draft_path":        draftPath,
		"captions_status":   captionsStatus,
		"voiceover_status":  voiceoverStatus,
		"missing":           missing,
		"next_commands":     nextCmds,
	}

	if opts.WriteArtifact {
		var b strings.Builder
		b.WriteString("# Creative Result\n\n")
		fmt.Fprintf(&b, "- Plan ID: `%s`\n", planID2)
		fmt.Fprintf(&b, "- Goal: %s\n", goal)
		fmt.Fprintf(&b, "- Approval: `%s`\n", approvalStatus)
		fmt.Fprintf(&b, "- Execution: `%s`\n", executionStatus)
		if previewArtifact != "" {
			fmt.Fprintf(&b, "- Preview: `%s`\n", previewArtifact)
		}
		if draftPath != "" {
			fmt.Fprintf(&b, "- Draft: `%s`\n", draftPath)
		}
		if captionsStatus != "" {
			fmt.Fprintf(&b, "- Captions: `%s`\n", captionsStatus)
		}
		if voiceoverStatus != "" {
			fmt.Fprintf(&b, "- Voiceover: `%s`\n", voiceoverStatus)
		}
		if len(outputArtifacts) > 0 {
			fmt.Fprintf(&b, "- Output artifacts: %d\n\n", len(outputArtifacts))
			b.WriteString("## Stub Outputs\n\n")
			for _, a := range outputArtifacts {
				fmt.Fprintf(&b, "- `%s` `%s` (%s)\n", a.Type, a.Path, a.Status)
			}
		}
		if len(missing) > 0 {
			b.WriteString("\n## Warnings\n\n")
			for _, w := range missing {
				fmt.Fprintf(&b, "- %s\n", w)
			}
		}
		if len(nextCmds) > 0 {
			b.WriteString("\n## Next Steps\n\n")
			for _, cmd := range nextCmds {
				fmt.Fprintf(&b, "```sh\n%s\n```\n", cmd)
			}
		}
		artifactPath := filepath.Join(planDir, "creative_result.md")
		if err := os.WriteFile(artifactPath, []byte(b.String()), 0o644); err != nil {
			return err
		}
		result["artifact"] = artifactPath
		fmt.Fprintf(stdout, "  artifact: %s\n", artifactPath)
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Creative result")
	fmt.Fprintf(stdout, "  plan id:          %s\n", planID2)
	fmt.Fprintf(stdout, "  goal:             %s\n", goal)
	fmt.Fprintf(stdout, "  approval_status:  %s\n", approvalStatus)
	fmt.Fprintf(stdout, "  execution_status: %s\n", executionStatus)
	if previewArtifact != "" {
		fmt.Fprintf(stdout, "  preview artifact: %s\n", previewArtifact)
	}
	if len(outputArtifacts) > 0 {
		fmt.Fprintf(stdout, "  output artifacts: %d\n", len(outputArtifacts))
		for _, a := range outputArtifacts {
			fmt.Fprintf(stdout, "    %s: %s\n", a.Type, a.Path)
		}
	}
	if draftPath != "" {
		fmt.Fprintf(stdout, "  draft:            %s\n", draftPath)
	}
	for _, w := range missing {
		fmt.Fprintf(stdout, "  warning:          %s\n", w)
	}
	if len(nextCmds) > 0 {
		fmt.Fprintln(stdout, "  next:")
		for _, cmd := range nextCmds {
			fmt.Fprintf(stdout, "    %s\n", cmd)
		}
	}
	return nil
}

// ---- validate-creative-plan ----

func ValidateCreativePlan(planID string, stdout io.Writer, opts ValidateCreativePlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	errs := []string{}
	warnings := []string{}

	// validate creative_plan.json
	planPath := filepath.Join(planDir, "creative_plan.json")
	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		errs = append(errs, "creative_plan.json is not valid JSON")
	} else if err := validateCreativePlanMap(m); err != nil {
		errs = append(errs, err.Error())
	}

	// validate creative_requests.dryrun.json if present
	previewPath := filepath.Join(planDir, "creative_requests.dryrun.json")
	if data, err := os.ReadFile(previewPath); err == nil {
		var p map[string]any
		if err := json.Unmarshal(data, &p); err != nil {
			errs = append(errs, "creative_requests.dryrun.json is not valid JSON")
		} else {
			sv, _ := p["schema_version"].(string)
			if sv != "creative_requests.dryrun.v1" {
				errs = append(errs, fmt.Sprintf("creative_requests.dryrun.json: unexpected schema_version %q", sv))
			}
			if _, ok := p["requests"]; !ok {
				errs = append(errs, "creative_requests.dryrun.json: missing field \"requests\"")
			}
		}
	}

	// validate events.jsonl if present
	eventsPath := filepath.Join(planDir, "events.jsonl")
	if data, err := os.ReadFile(eventsPath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var ev map[string]any
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				warnings = append(warnings, fmt.Sprintf("events.jsonl line %d is malformed", lineNum))
			}
		}
	}

	// validate outputs/creative_outputs.json if present
	outputsIndexPath := filepath.Join(planDir, "outputs", "creative_outputs.json")
	if data, err := os.ReadFile(outputsIndexPath); err == nil {
		var idx map[string]any
		if err := json.Unmarshal(data, &idx); err != nil {
			errs = append(errs, "outputs/creative_outputs.json is not valid JSON")
		} else {
			sv, _ := idx["schema_version"].(string)
			if sv != "creative_outputs.v1" {
				errs = append(errs, fmt.Sprintf("outputs/creative_outputs.json: unexpected schema_version %q", sv))
			}
			if _, ok := idx["artifacts"]; !ok {
				errs = append(errs, "outputs/creative_outputs.json: missing field \"artifacts\"")
			}
			if artifactsRaw, ok := idx["artifacts"].([]any); ok {
				for _, a := range artifactsRaw {
					am, ok := a.(map[string]any)
					if !ok {
						continue
					}
					artPath, _ := am["path"].(string)
					if artPath == "" {
						continue
					}
					fullPath := filepath.Join(planDir, artPath)
					if _, err := os.Stat(fullPath); err != nil {
						errs = append(errs, fmt.Sprintf("artifact path not found: %s", artPath))
						continue
					}
					artData, err := os.ReadFile(fullPath)
					if err != nil {
						continue
					}
					var artMap map[string]any
					if err := json.Unmarshal(artData, &artMap); err != nil {
						warnings = append(warnings, fmt.Sprintf("artifact %s is not valid JSON", artPath))
						continue
					}
					artSV, _ := artMap["schema_version"].(string)
					artType, _ := am["type"].(string)
					expectedSV := expectedArtifactSchemaVersion(artType)
					if expectedSV != "" && artSV != expectedSV {
						errs = append(errs, fmt.Sprintf("artifact %s: schema_version %q != expected %q", artPath, artSV, expectedSV))
					}
				}
			}
		}
	}

	// validate outputs/creative_timeline.json if present
	timelinePath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	if data, err := os.ReadFile(timelinePath); err == nil {
		var tl map[string]any
		if err := json.Unmarshal(data, &tl); err != nil {
			errs = append(errs, "outputs/creative_timeline.json is not valid JSON")
		} else {
			sv, _ := tl["schema_version"].(string)
			if sv != "creative_timeline.v1" {
				errs = append(errs, fmt.Sprintf("outputs/creative_timeline.json: unexpected schema_version %q", sv))
			}
			if _, ok := tl["tracks"]; !ok {
				errs = append(errs, "outputs/creative_timeline.json: missing field \"tracks\"")
			}
			tlStart, _ := tl["total_duration_seconds"].(float64)
			if tsEnd, ok := tl["total_duration_seconds"].(float64); ok && tsEnd < tlStart {
				errs = append(errs, "outputs/creative_timeline.json: total_duration_seconds is negative")
			}
		}
	}

	// validate outputs/creative_render_plan.json if present
	renderPlanPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	if data, err := os.ReadFile(renderPlanPath); err == nil {
		var rp map[string]any
		if err := json.Unmarshal(data, &rp); err != nil {
			errs = append(errs, "outputs/creative_render_plan.json is not valid JSON")
		} else {
			sv, _ := rp["schema_version"].(string)
			if sv != "creative_render_plan.v1" {
				errs = append(errs, fmt.Sprintf("outputs/creative_render_plan.json: unexpected schema_version %q", sv))
			}
			if po, ok := rp["planned_output"].(map[string]any); !ok {
				errs = append(errs, "outputs/creative_render_plan.json: missing field \"planned_output\"")
			} else {
				pf, _ := po["planned_file"].(string)
				if pf == "" {
					errs = append(errs, "outputs/creative_render_plan.json: planned_output.planned_file is empty")
				}
			}
			if _, ok := rp["steps"]; !ok {
				errs = append(errs, "outputs/creative_render_plan.json: missing field \"steps\"")
			}
		}
	}

	// validate outputs/creative_assemble_result.json if present
	assembleResultPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")
	if data, err := os.ReadFile(assembleResultPath); err == nil {
		var ar map[string]any
		if err := json.Unmarshal(data, &ar); err != nil {
			errs = append(errs, "outputs/creative_assemble_result.json is not valid JSON")
		} else {
			sv, _ := ar["schema_version"].(string)
			if sv != "creative_assemble_result.v1" {
				errs = append(errs, fmt.Sprintf("outputs/creative_assemble_result.json: unexpected schema_version %q", sv))
			}
			outFile, _ := ar["output_file"].(string)
			if outFile != "" {
				fullOutPath := filepath.Join(planDir, outFile)
				if status, _ := ar["status"].(string); status == "completed" || status == "partial" {
					if _, err := os.Stat(fullOutPath); err != nil {
						errs = append(errs, fmt.Sprintf("outputs/creative_assemble_result.json: output_file not found: %s", outFile))
					}
				}
			}
			if clipsRaw, ok := ar["clips"].([]any); ok {
				for _, c := range clipsRaw {
					cm, ok := c.(map[string]any)
					if !ok {
						continue
					}
					clipStatus, _ := cm["status"].(string)
					workFile, _ := cm["work_file"].(string)
					if clipStatus == "completed" && workFile != "" {
						if _, err := os.Stat(filepath.Join(planDir, workFile)); err != nil {
							warnings = append(warnings, fmt.Sprintf("assemble work clip not found: %s", workFile))
						}
					}
				}
			}
		}
	}

	valid := len(errs) == 0
	result := map[string]any{
		"valid":    valid,
		"errors":   errs,
		"warnings": warnings,
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Validate creative plan")
	fmt.Fprintf(stdout, "  plan id: %s\n", planID)
	if valid {
		fmt.Fprintln(stdout, "  status:  ok")
	} else {
		fmt.Fprintln(stdout, "  status:  failed")
	}
	for _, e := range errs {
		fmt.Fprintf(stdout, "  error:   %s\n", e)
	}
	for _, w := range warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	if !valid {
		return fmt.Errorf("creative plan validation failed")
	}
	return nil
}

// ---- helpers ----

func buildRequestItem(step CreativeStep, tools config.ToolsConfig, strict bool, checkEnv bool) CreativeRequestItem {
	item := CreativeRequestItem{
		StepID:     step.ID,
		StepType:   step.Type,
		Capability: step.Capability,
		Route:      step.Route,
		Backend:    step.Backend,
	}

	backendName := step.Backend
	routeKey := step.Route
	if backendName == "" && routeKey != "" {
		backendName = tools.Routes[routeKey]
	}

	if backendName == "" {
		item.Status = "missing_backend"
		item.Warnings = append(item.Warnings, "no backend configured for this capability")
		item.RequestPreview = CreativeRequestPayload{
			Instruction:  instructionForStep(step.Type, ""),
			InputSummary: "not available — no backend configured",
			OutputContract: CreativeOutputContract{
				Artifact: artifactNameForStep(step.Type),
				Format:   formatForStep(step.Type),
			},
		}
		return item
	}

	backend, ok := tools.Backends[backendName]
	if !ok {
		item.Status = "missing_backend"
		item.Warnings = append(item.Warnings, fmt.Sprintf("backend %q not found in tools config", backendName))
		return item
	}

	item.Provider = backend.Provider
	item.Model = backend.Model
	item.Endpoint = backend.Endpoint
	item.Auth = CreativeRequestAuth{
		Type: backend.Auth.Type,
		Env:  backend.Auth.Env,
	}
	item.Status = "previewed"

	if checkEnv && backend.Auth.Env != "" {
		if _, set := os.LookupEnv(backend.Auth.Env); !set {
			msg := fmt.Sprintf("env var %q is not set", backend.Auth.Env)
			if strict {
				item.Status = "missing_backend"
				item.Warnings = append(item.Warnings, msg)
			} else {
				item.Warnings = append(item.Warnings, msg)
			}
		}
	}

	item.RequestPreview = CreativeRequestPayload{
		Instruction:  instructionForStep(step.Type, item.Provider),
		InputSummary: fmt.Sprintf("input from creative plan step %s", step.ID),
		OutputContract: CreativeOutputContract{
			Artifact: artifactNameForStep(step.Type),
			Format:   formatForStep(step.Type),
		},
	}
	return item
}

func instructionForStep(stepType, provider string) string {
	switch stepType {
	case "generate_script":
		return "Draft a short script for the requested creative goal."
	case "generate_voiceover":
		return "Generate narration or voiceover audio for the script."
	case "generate_visual_asset":
		return "Generate supporting visual assets for the creative goal."
	case "generate_captions_or_caption_variants":
		return "Generate captions or caption variants for the content."
	case "render_draft":
		return "Render a draft composition from the provided assets."
	case "generate_audio_asset":
		return "Generate supporting audio or music for the creative goal."
	case "visual_transform":
		return "Apply a visual transformation as required by the goal."
	case "translate_text":
		return "Translate the provided text according to the goal."
	default:
		return "Execute the creative step for the user goal."
	}
}

func artifactNameForStep(stepType string) string {
	switch stepType {
	case "generate_script":
		return "script_draft.txt"
	case "generate_voiceover":
		return "voiceover_draft.mp3"
	case "generate_visual_asset":
		return "visual_asset_draft.mp4"
	case "generate_captions_or_caption_variants":
		return "caption_variants.json"
	case "render_draft":
		return "render_draft.mp4"
	case "generate_audio_asset":
		return "audio_asset_draft.mp3"
	case "visual_transform":
		return "visual_transform_draft.mp4"
	case "translate_text":
		return "translated_text.txt"
	default:
		return "creative_step_output.json"
	}
}

func formatForStep(stepType string) string {
	switch stepType {
	case "generate_script", "translate_text":
		return "text"
	case "generate_captions_or_caption_variants":
		return "json"
	case "generate_voiceover", "generate_audio_asset":
		return "audio"
	case "generate_visual_asset", "render_draft", "visual_transform":
		return "video"
	default:
		return "json"
	}
}

func validateCreativePlanMap(m map[string]any) error {
	sv, _ := m["schema_version"].(string)
	if sv != "creative_plan.v1" {
		return fmt.Errorf("unexpected schema_version %q", sv)
	}
	if _, ok := m["plan_id"]; !ok {
		return fmt.Errorf("missing field \"plan_id\"")
	}
	if _, ok := m["goal"]; !ok {
		return fmt.Errorf("missing field \"goal\"")
	}
	if _, ok := m["steps"]; !ok {
		return fmt.Errorf("missing field \"steps\"")
	}
	return nil
}

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"byom-video/internal/config"
	"byom-video/internal/events"
	"byom-video/internal/highlights"
	"byom-video/internal/manifest"
	"byom-video/internal/roughcut"
	"byom-video/internal/runstore"
)

type MaskPlanOptions struct {
	Intent          string
	Tone            string
	MaxCaptionWords int
	TopK            int
	Overwrite       bool
}

type MaskTemplateSummary struct {
	RunID string   `json:"run_id"`
	Files []string `json:"files"`
}

type InspectMaskOptions struct{ JSON bool }
type MaskValidateOptions struct{ JSON bool }
type ReviewMaskOptions struct {
	JSON          bool
	WriteArtifact bool
}
type ExpansionPlanOptions struct {
	Overwrite           bool
	CaptionVariants     int
	LabelMaxWords       int
	DescriptionMaxWords int
}
type VerificationPlanOptions struct{ Overwrite bool }

type MaskInspection struct {
	RunID string             `json:"run_id"`
	Items []MaskArtifactItem `json:"items"`
}

type MaskArtifactItem struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Valid  *bool  `json:"valid,omitempty"`
}

type InferenceMask struct {
	SchemaVersion string          `json:"schema_version"`
	Source        MaskSource      `json:"source"`
	Intent        string          `json:"intent"`
	Constraints   MaskConstraints `json:"constraints"`
	Decisions     []MaskDecision  `json:"decisions"`
	CreatedAt     time.Time       `json:"created_at"`
}

type MaskSource struct {
	ChunksArtifact     string `json:"chunks_artifact,omitempty"`
	HighlightsArtifact string `json:"highlights_artifact,omitempty"`
	RoughcutArtifact   string `json:"roughcut_artifact,omitempty"`
	Mode               string `json:"mode"`
	Reasoner           string `json:"reasoner"`
}

type MaskConstraints struct {
	MustInclude     []string `json:"must_include"`
	MustNotInclude  []string `json:"must_not_include"`
	Tone            string   `json:"tone"`
	MaxCaptionWords int      `json:"max_caption_words"`
}

type MaskDecision struct {
	ID            string  `json:"id"`
	HighlightID   string  `json:"highlight_id,omitempty"`
	ClipID        string  `json:"clip_id,omitempty"`
	ChunkID       string  `json:"chunk_id,omitempty"`
	SourceChunkID string  `json:"source_chunk_id,omitempty"`
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	Decision      string  `json:"decision"`
	Reason        string  `json:"reason"`
	TextPreview   string  `json:"text_preview"`
}

type ExpansionTasks struct {
	SchemaVersion string          `json:"schema_version"`
	Source        map[string]any  `json:"source"`
	Tasks         []ExpansionTask `json:"tasks"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ExpansionTask struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	ModelRoute     string         `json:"model_route"`
	InputRefs      []string       `json:"input_refs"`
	OutputContract map[string]any `json:"output_contract"`
}

type VerificationPlan struct {
	SchemaVersion string              `json:"schema_version"`
	Source        map[string]any      `json:"source"`
	Status        string              `json:"status"`
	Checks        []VerificationCheck `json:"checks"`
	CreatedAt     time.Time           `json:"created_at"`
}

type VerificationCheck struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type MaskValidationResult struct {
	RunID  string               `json:"run_id"`
	Valid  bool                 `json:"valid"`
	Files  []MaskFileValidation `json:"files"`
	Errors []string             `json:"errors,omitempty"`
}

type MaskFileValidation struct {
	Type   string   `json:"type"`
	Path   string   `json:"path"`
	Exists bool     `json:"exists"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type MaskReview struct {
	RunID       string          `json:"run_id"`
	Intent      string          `json:"intent"`
	Source      MaskSource      `json:"source"`
	Constraints MaskConstraints `json:"constraints"`
	Decisions   []MaskDecision  `json:"decisions"`
}

func MaskPlan(runID string, stdout io.Writer, opts MaskPlanOptions) error {
	if opts.Intent == "" {
		opts.Intent = "create_short_highlights"
	}
	if opts.Tone == "" {
		opts.Tone = "concise, useful, editor-friendly"
	}
	if opts.MaxCaptionWords == 0 {
		opts.MaxCaptionWords = 18
	}
	if opts.TopK == 0 {
		opts.TopK = 10
	}
	if opts.MaxCaptionWords <= 0 {
		return fmt.Errorf("--max-caption-words must be positive")
	}
	if opts.TopK <= 0 {
		return fmt.Errorf("--top-k must be positive")
	}
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("MASK_PLAN_STARTED", map[string]any{"run_id": runID})
	}
	maskPath := filepath.Join(runDir, "inference_mask.json")
	if !opts.Overwrite {
		if _, err := os.Stat(maskPath); err == nil {
			writeMaskFailure(log, "MASK_PLAN_FAILED", "inference_mask.json already exists; pass --overwrite")
			return fmt.Errorf("inference_mask.json already exists; pass --overwrite")
		}
	}
	mask, err := buildInferenceMask(runDir, opts)
	if err != nil {
		writeMaskFailure(log, "MASK_PLAN_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(maskPath, mask); err != nil {
		writeMaskFailure(log, "MASK_PLAN_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "inference_mask", "inference_mask.json"); err != nil {
		writeMaskFailure(log, "MASK_PLAN_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("MASK_PLAN_COMPLETED", map[string]any{"path": "inference_mask.json", "decisions": len(mask.Decisions)})
	}
	fmt.Fprintln(stdout, "Inference mask planned")
	fmt.Fprintf(stdout, "  run id:    %s\n", runID)
	fmt.Fprintf(stdout, "  path:      %s\n", maskPath)
	fmt.Fprintf(stdout, "  decisions: %d\n", len(mask.Decisions))
	return nil
}

func MaskTemplate(runID string, stdout io.Writer) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	source := maskSource(runDir)
	files := map[string]any{
		"inference_mask.template.json":  inferenceMaskTemplate(source),
		"expansion_tasks.template.json": expansionTasksTemplate(),
		"verification.template.json":    verificationTemplate(),
	}
	written := []string{}
	for name, value := range files {
		path := filepath.Join(runDir, name)
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		written = append(written, filepath.Join(runDir, name))
	}
	fmt.Fprintln(stdout, "Mask templates written")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	for _, path := range written {
		fmt.Fprintf(stdout, "  - %s\n", path)
	}
	return nil
}

func InspectMask(runID string, stdout io.Writer, opts InspectMaskOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	inspection := MaskInspection{RunID: runID}
	validation := validateMaskArtifacts(runID, runDir)
	validationByPath := map[string]bool{}
	for _, file := range validation.Files {
		validationByPath[file.Path] = file.Valid
	}
	for _, name := range maskArtifactNames() {
		path := filepath.Join(runDir, name)
		_, err := os.Stat(path)
		item := MaskArtifactItem{Path: path, Exists: err == nil}
		if valid, ok := validationByPath[path]; ok {
			item.Valid = &valid
		}
		inspection.Items = append(inspection.Items, item)
	}
	if opts.JSON {
		data, err := json.MarshalIndent(inspection, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Mask inspection")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	for _, item := range inspection.Items {
		status := "missing"
		if item.Exists {
			status = "present"
		}
		if item.Valid != nil {
			if *item.Valid {
				status += ", valid"
			} else {
				status += ", invalid"
			}
		}
		fmt.Fprintf(stdout, "  - %s: %s\n", filepath.Base(item.Path), status)
	}
	return nil
}

func MaskValidate(runID string, stdout io.Writer, opts MaskValidateOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	result := validateMaskArtifacts(runID, runDir)
	if opts.JSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		if !result.Valid {
			return fmt.Errorf("mask validation failed")
		}
		return nil
	}
	fmt.Fprintln(stdout, "Mask validation")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	if result.Valid {
		fmt.Fprintln(stdout, "  status: ok")
	} else {
		fmt.Fprintln(stdout, "  status: failed")
	}
	for _, file := range result.Files {
		status := "missing"
		if file.Exists && file.Valid {
			status = "ok"
		} else if file.Exists {
			status = "failed"
		}
		fmt.Fprintf(stdout, "  - %s: %s (%s)\n", file.Type, status, file.Path)
		for _, err := range file.Errors {
			fmt.Fprintf(stdout, "    - %s\n", err)
		}
	}
	if !result.Valid {
		return fmt.Errorf("mask validation failed")
	}
	return nil
}

func ReviewMask(runID string, stdout io.Writer, opts ReviewMaskOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		return err
	}
	review := MaskReview{RunID: runID, Intent: mask.Intent, Source: mask.Source, Constraints: mask.Constraints, Decisions: mask.Decisions}
	if opts.JSON {
		data, err := json.MarshalIndent(review, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Mask review")
	fmt.Fprintf(stdout, "  run id:    %s\n", runID)
	fmt.Fprintf(stdout, "  intent:    %s\n", review.Intent)
	fmt.Fprintf(stdout, "  source:    chunks=%s highlights=%s roughcut=%s mode=%s reasoner=%s\n", emptyDash(review.Source.ChunksArtifact), emptyDash(review.Source.HighlightsArtifact), emptyDash(review.Source.RoughcutArtifact), review.Source.Mode, review.Source.Reasoner)
	fmt.Fprintf(stdout, "  tone:      %s\n", review.Constraints.Tone)
	fmt.Fprintf(stdout, "  max words: %d\n", review.Constraints.MaxCaptionWords)
	fmt.Fprintf(stdout, "  decisions: %d\n", len(review.Decisions))
	for _, decision := range review.Decisions {
		fmt.Fprintf(stdout, "    - %s %.3f-%.3f %s: %s\n", decision.ID, decision.Start, decision.End, decision.Decision, decision.Reason)
		if decision.TextPreview != "" {
			fmt.Fprintf(stdout, "      text: %s\n", decision.TextPreview)
		}
	}
	if opts.WriteArtifact {
		path := filepath.Join(runDir, "mask_review.md")
		if err := writeMaskReview(path, review); err != nil {
			return err
		}
		if err := addManifestArtifact(runDir, "mask_review", "mask_review.md"); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "  artifact: %s\n", path)
	}
	return nil
}

func ExpansionPlanCommand(runID string, stdout io.Writer, opts ExpansionPlanOptions) error {
	if opts.CaptionVariants == 0 {
		opts.CaptionVariants = 3
	}
	if opts.LabelMaxWords == 0 {
		opts.LabelMaxWords = 8
	}
	if opts.DescriptionMaxWords == 0 {
		opts.DescriptionMaxWords = 80
	}
	if opts.CaptionVariants <= 0 || opts.LabelMaxWords <= 0 || opts.DescriptionMaxWords <= 0 {
		return fmt.Errorf("expansion plan numeric flags must be positive")
	}
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPANSION_PLAN_STARTED", map[string]any{"run_id": runID})
	}
	path := filepath.Join(runDir, "expansion_tasks.json")
	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			writeMaskFailure(log, "EXPANSION_PLAN_FAILED", "expansion_tasks.json already exists; pass --overwrite")
			return fmt.Errorf("expansion_tasks.json already exists; pass --overwrite")
		}
	}
	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		writeMaskFailure(log, "EXPANSION_PLAN_FAILED", err.Error())
		return err
	}
	tasks := buildExpansionTasks(mask, opts)
	if err := writeJSONFile(path, tasks); err != nil {
		writeMaskFailure(log, "EXPANSION_PLAN_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "expansion_tasks", "expansion_tasks.json"); err != nil {
		writeMaskFailure(log, "EXPANSION_PLAN_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("EXPANSION_PLAN_COMPLETED", map[string]any{"path": "expansion_tasks.json", "tasks": len(tasks.Tasks)})
	}
	fmt.Fprintln(stdout, "Expansion plan written")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  path:   %s\n", path)
	fmt.Fprintf(stdout, "  tasks:  %d\n", len(tasks.Tasks))
	return nil
}

func VerificationPlanCommand(runID string, stdout io.Writer, opts VerificationPlanOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("VERIFICATION_PLAN_STARTED", map[string]any{"run_id": runID})
	}
	path := filepath.Join(runDir, "verification.json")
	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			writeMaskFailure(log, "VERIFICATION_PLAN_FAILED", "verification.json already exists; pass --overwrite")
			return fmt.Errorf("verification.json already exists; pass --overwrite")
		}
	}
	if _, err := os.Stat(filepath.Join(runDir, "inference_mask.json")); err != nil {
		writeMaskFailure(log, "VERIFICATION_PLAN_FAILED", "inference_mask.json is required")
		return fmt.Errorf("inference_mask.json is required; run mask-plan first")
	}
	plan := buildVerificationPlan(runDir)
	if err := writeJSONFile(path, plan); err != nil {
		writeMaskFailure(log, "VERIFICATION_PLAN_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "verification", "verification.json"); err != nil {
		writeMaskFailure(log, "VERIFICATION_PLAN_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("VERIFICATION_PLAN_COMPLETED", map[string]any{"path": "verification.json", "checks": len(plan.Checks)})
	}
	fmt.Fprintln(stdout, "Verification plan written")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  path:   %s\n", path)
	fmt.Fprintf(stdout, "  checks: %d\n", len(plan.Checks))
	return nil
}

func maskSource(runDir string) map[string]any {
	source := map[string]any{
		"mode":     "template",
		"reasoner": "premium_reasoner",
	}
	manifestPath := filepath.Join(runDir, "manifest.json")
	if m, err := manifest.Read(manifestPath); err == nil {
		for _, artifact := range m.Artifacts {
			switch artifact.Path {
			case "chunks.json":
				source["chunks_artifact"] = artifact.Path
			case "highlights.json":
				source["highlights_artifact"] = artifact.Path
			case "roughcut.json":
				source["roughcut_artifact"] = artifact.Path
			}
		}
	}
	for _, name := range []string{"chunks.json", "highlights.json", "roughcut.json"} {
		if _, ok := source[artifactSourceKey(name)]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(runDir, name)); err == nil {
			source[artifactSourceKey(name)] = name
		}
	}
	return source
}

func artifactSourceKey(name string) string {
	switch name {
	case "chunks.json":
		return "chunks_artifact"
	case "highlights.json":
		return "highlights_artifact"
	case "roughcut.json":
		return "roughcut_artifact"
	default:
		return name
	}
}

func inferenceMaskTemplate(source map[string]any) map[string]any {
	return map[string]any{
		"schema_version": "inference_mask.v1",
		"source":         source,
		"intent":         "create_short_highlights",
		"constraints": map[string]any{
			"must_include":      []string{},
			"must_not_include":  []string{},
			"tone":              "technical, concise",
			"max_caption_words": 18,
		},
		"decisions": []map[string]any{
			{
				"id":           "decision_0001",
				"highlight_id": "",
				"start":        0.0,
				"end":          0.0,
				"decision":     "keep",
				"reason":       "Template decision placeholder.",
			},
		},
		"created_at": time.Now().UTC(),
	}
}

func expansionTasksTemplate() map[string]any {
	return map[string]any{
		"schema_version": "expansion_tasks.v1",
		"source": map[string]any{
			"inference_mask_artifact": "inference_mask.json",
		},
		"tasks": []map[string]any{
			{
				"id":          "task_0001",
				"type":        "caption_variants",
				"model_route": "caption_expansion",
				"input_refs":  []string{"decision_0001"},
				"output_contract": map[string]any{
					"max_items": 3,
					"max_words": 18,
				},
			},
		},
	}
}

func verificationTemplate() map[string]any {
	return map[string]any{
		"schema_version": "verification.v1",
		"source": map[string]any{
			"inference_mask_artifact": "inference_mask.json",
			"expansion_artifacts":     []string{},
		},
		"status": "pending",
		"checks": []map[string]any{
			{
				"id":      "check_0001",
				"type":    "must_not_include",
				"status":  "pending",
				"message": "",
			},
		},
	}
}

func maskArtifactNames() []string {
	return []string{
		"inference_mask.json",
		"inference_mask.template.json",
		"expansion_tasks.json",
		"expansion_tasks.template.json",
		"verification.json",
		"verification.template.json",
		"model_requests.dryrun.json",
		"model_requests.executed.json",
		"model_requests_review.md",
		"mask_review.md",
		"expansions/caption_variants.json",
		"expansions/timeline_labels.json",
		"expansions/short_descriptions.json",
		"expansions_review.md",
		"verification_results.json",
		"verification_review.md",
	}
}

func buildInferenceMask(runDir string, opts MaskPlanOptions) (InferenceMask, error) {
	source := MaskSource{
		Mode:     "deterministic",
		Reasoner: "deterministic_mask_planner_v1",
	}
	if _, err := os.Stat(filepath.Join(runDir, "chunks.json")); err == nil {
		source.ChunksArtifact = "chunks.json"
	}
	if _, err := os.Stat(filepath.Join(runDir, "highlights.json")); err == nil {
		source.HighlightsArtifact = "highlights.json"
	}
	if _, err := os.Stat(filepath.Join(runDir, "roughcut.json")); err == nil {
		source.RoughcutArtifact = "roughcut.json"
	}
	decisions, err := maskDecisionsFromRoughcut(runDir)
	if err != nil {
		return InferenceMask{}, err
	}
	if len(decisions) == 0 {
		decisions, err = maskDecisionsFromHighlights(runDir, opts.TopK)
		if err != nil {
			return InferenceMask{}, err
		}
	}
	if len(decisions) == 0 {
		return InferenceMask{}, fmt.Errorf("no roughcut or highlights found; run highlights/roughcut first")
	}
	return InferenceMask{
		SchemaVersion: "inference_mask.v1",
		Source:        source,
		Intent:        opts.Intent,
		Constraints: MaskConstraints{
			MustInclude:     []string{},
			MustNotInclude:  []string{"unsupported claims", "invented timestamps", "invented speakers"},
			Tone:            opts.Tone,
			MaxCaptionWords: opts.MaxCaptionWords,
		},
		Decisions: decisions,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func maskDecisionsFromRoughcut(runDir string) ([]MaskDecision, error) {
	path := filepath.Join(runDir, "roughcut.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read roughcut: %w", err)
	}
	var doc roughcut.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode roughcut: %w", err)
	}
	decisions := make([]MaskDecision, 0, len(doc.Clips))
	for index, clip := range doc.Clips {
		decisions = append(decisions, MaskDecision{
			ID:            fmt.Sprintf("decision_%04d", index+1),
			HighlightID:   clip.HighlightID,
			ClipID:        clip.ID,
			ChunkID:       clip.SourceChunkID,
			SourceChunkID: clip.SourceChunkID,
			Start:         clip.Start,
			End:           clip.End,
			Decision:      "keep",
			Reason:        nonEmptyString(clip.EditIntent, "Selected by deterministic roughcut."),
			TextPreview:   trimPreview(clip.Text),
		})
	}
	return decisions, nil
}

func maskDecisionsFromHighlights(runDir string, topK int) ([]MaskDecision, error) {
	path := filepath.Join(runDir, "highlights.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read highlights: %w", err)
	}
	var doc highlights.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode highlights: %w", err)
	}
	limit := len(doc.Highlights)
	if topK > 0 && topK < limit {
		limit = topK
	}
	decisions := make([]MaskDecision, 0, limit)
	for index, highlight := range doc.Highlights[:limit] {
		decisions = append(decisions, MaskDecision{
			ID:            fmt.Sprintf("decision_%04d", index+1),
			HighlightID:   highlight.ID,
			ChunkID:       highlight.ChunkID,
			SourceChunkID: highlight.ChunkID,
			Start:         highlight.Start,
			End:           highlight.End,
			Decision:      "candidate_keep",
			Reason:        nonEmptyString(highlight.Reason, "Selected by deterministic highlight score."),
			TextPreview:   trimPreview(highlight.Text),
		})
	}
	return decisions, nil
}

func buildExpansionTasks(mask InferenceMask, opts ExpansionPlanOptions) ExpansionTasks {
	decisionIDs := make([]string, 0, len(mask.Decisions))
	for _, decision := range mask.Decisions {
		decisionIDs = append(decisionIDs, decision.ID)
	}
	descriptionRoute := "caption_expansion"
	if cfg, err := config.Load(config.DefaultPath); err == nil {
		if _, ok := cfg.Models.Routes["description_expansion"]; ok {
			descriptionRoute = "description_expansion"
		}
	}
	return ExpansionTasks{
		SchemaVersion: "expansion_tasks.v1",
		Source: map[string]any{
			"inference_mask_artifact": "inference_mask.json",
		},
		Tasks: []ExpansionTask{
			{
				ID:         "task_0001",
				Type:       "caption_variants",
				ModelRoute: "caption_expansion",
				InputRefs:  decisionIDs,
				OutputContract: map[string]any{
					"max_items": opts.CaptionVariants,
					"max_words": mask.Constraints.MaxCaptionWords,
					"style":     mask.Constraints.Tone,
				},
			},
			{
				ID:         "task_0002",
				Type:       "timeline_labels",
				ModelRoute: "timeline_labeling",
				InputRefs:  decisionIDs,
				OutputContract: map[string]any{
					"max_items": len(decisionIDs),
					"max_words": opts.LabelMaxWords,
					"style":     "short timeline label",
				},
			},
			{
				ID:         "task_0003",
				Type:       "short_descriptions",
				ModelRoute: descriptionRoute,
				InputRefs:  decisionIDs,
				OutputContract: map[string]any{
					"max_items": len(decisionIDs),
					"max_words": opts.DescriptionMaxWords,
					"style":     mask.Constraints.Tone,
				},
			},
		},
		CreatedAt: time.Now().UTC(),
	}
}

func buildVerificationPlan(runDir string) VerificationPlan {
	source := map[string]any{
		"inference_mask_artifact": "inference_mask.json",
		"expansion_artifacts":     []string{},
	}
	if _, err := os.Stat(filepath.Join(runDir, "expansion_tasks.json")); err == nil {
		source["expansion_tasks_artifact"] = "expansion_tasks.json"
	}
	checkTypes := []string{"must_not_include", "timestamp_drift", "missing_required_decisions", "output_contract_compliance"}
	checks := make([]VerificationCheck, 0, len(checkTypes))
	for index, checkType := range checkTypes {
		checks = append(checks, VerificationCheck{
			ID:      fmt.Sprintf("check_%04d", index+1),
			Type:    checkType,
			Status:  "pending",
			Message: "",
		})
	}
	return VerificationPlan{
		SchemaVersion: "verification.v1",
		Source:        source,
		Status:        "pending",
		Checks:        checks,
		CreatedAt:     time.Now().UTC(),
	}
}

func readInferenceMask(path string) (InferenceMask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InferenceMask{}, fmt.Errorf("read inference mask: %w", err)
	}
	var mask InferenceMask
	if err := json.Unmarshal(data, &mask); err != nil {
		return InferenceMask{}, fmt.Errorf("decode inference mask: %w", err)
	}
	return mask, nil
}

func writeMaskReview(path string, review MaskReview) error {
	var builder strings.Builder
	builder.WriteString("# Mask Review\n\n")
	builder.WriteString(fmt.Sprintf("- generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- run_id: %s\n", review.RunID))
	builder.WriteString(fmt.Sprintf("- intent: %s\n", review.Intent))
	builder.WriteString(fmt.Sprintf("- tone: %s\n", review.Constraints.Tone))
	builder.WriteString(fmt.Sprintf("- max_caption_words: %d\n", review.Constraints.MaxCaptionWords))
	builder.WriteString(fmt.Sprintf("- source.chunks_artifact: %s\n", emptyDash(review.Source.ChunksArtifact)))
	builder.WriteString(fmt.Sprintf("- source.highlights_artifact: %s\n", emptyDash(review.Source.HighlightsArtifact)))
	builder.WriteString(fmt.Sprintf("- source.roughcut_artifact: %s\n", emptyDash(review.Source.RoughcutArtifact)))
	builder.WriteString("\n## Decisions\n\n")
	for _, decision := range review.Decisions {
		builder.WriteString(fmt.Sprintf("- %s %.3f-%.3f `%s`: %s\n", decision.ID, decision.Start, decision.End, decision.Decision, decision.Reason))
		if decision.TextPreview != "" {
			builder.WriteString(fmt.Sprintf("  - text: %s\n", decision.TextPreview))
		}
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write mask review: %w", err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func addManifestArtifact(runDir string, name string, path string) error {
	manifestPath := filepath.Join(runDir, "manifest.json")
	m, err := manifest.Read(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	m.AddArtifact(name, path)
	return manifest.Write(manifestPath, m)
}

func writeMaskFailure(log *events.Log, eventType string, message string) {
	if log != nil {
		_ = log.Write(eventType, map[string]any{"error": message})
	}
}

func nonEmptyString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func trimPreview(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 180 {
		return value
	}
	return value[:177] + "..."
}

func validateMaskArtifacts(runID string, runDir string) MaskValidationResult {
	result := MaskValidationResult{RunID: runID, Valid: true}
	specs := []struct {
		kind     string
		paths    []string
		validate func(map[string]any) []string
	}{
		{"inference_mask", []string{"inference_mask.json", "inference_mask.template.json"}, validateInferenceMaskShape},
		{"expansion_tasks", []string{"expansion_tasks.json", "expansion_tasks.template.json"}, validateExpansionTasksShape},
		{"verification", []string{"verification.json", "verification.template.json"}, validateVerificationShape},
		{"model_requests_dryrun", []string{"model_requests.dryrun.json"}, validateModelRequestsDryRunShape},
		{"model_requests_executed", []string{"model_requests.executed.json"}, validateExecutedModelRequestsShape},
		{"verification_results", []string{"verification_results.json"}, validateVerificationResultsShape},
	}
	for index, spec := range specs {
		path, exists := firstExisting(runDir, spec.paths)
		file := MaskFileValidation{Type: spec.kind, Path: path, Exists: exists, Valid: exists}
		if !exists {
			file.Errors = append(file.Errors, "artifact or template is missing")
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				file.Errors = append(file.Errors, "read file: "+err.Error())
			} else {
				var payload map[string]any
				if err := json.Unmarshal(data, &payload); err != nil {
					file.Errors = append(file.Errors, "invalid JSON: "+err.Error())
				} else {
					file.Errors = append(file.Errors, spec.validate(payload)...)
				}
			}
		}
		file.Valid = file.Exists && len(file.Errors) == 0
		if !file.Valid {
			if file.Exists || index == 0 {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("%s validation failed", spec.kind))
			}
		}
		result.Files = append(result.Files, file)
	}
	if m, err := manifest.Read(filepath.Join(runDir, "manifest.json")); err == nil {
		for _, artifact := range m.Artifacts {
			if artifact.Path == "model_requests_review.md" {
				path := filepath.Join(runDir, artifact.Path)
				file := MaskFileValidation{Type: "model_requests_review", Path: path}
				if _, err := os.Stat(path); err == nil {
					file.Exists = true
					file.Valid = true
				} else {
					file.Exists = false
					file.Valid = false
					file.Errors = append(file.Errors, "artifact listed in manifest but file is missing")
					result.Valid = false
					result.Errors = append(result.Errors, "model_requests_review validation failed")
				}
				result.Files = append(result.Files, file)
				break
			}
		}
	}
	return result
}

func firstExisting(runDir string, names []string) (string, bool) {
	for _, name := range names {
		path := filepath.Join(runDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return filepath.Join(runDir, names[0]), false
}

func validateInferenceMaskShape(payload map[string]any) []string {
	errs := []string{}
	requireStringValue(&errs, payload, "schema_version", "inference_mask.v1")
	requireMap(&errs, payload, "source")
	requireNonEmptyString(&errs, payload, "intent")
	requireMap(&errs, payload, "constraints")
	requireArray(&errs, payload, "decisions")
	if decisions, ok := payload["decisions"].([]any); ok {
		for index, raw := range decisions {
			decision, ok := raw.(map[string]any)
			if !ok {
				errs = append(errs, fmt.Sprintf("decisions[%d] must be an object", index))
				continue
			}
			requireDecisionString(&errs, decision, "id", fmt.Sprintf("decisions[%d].id", index))
			requireDecisionString(&errs, decision, "decision", fmt.Sprintf("decisions[%d].decision", index))
			requireDecisionString(&errs, decision, "reason", fmt.Sprintf("decisions[%d].reason", index))
			start, hasStart, startOK := numericField(decision, "start")
			end, hasEnd, endOK := numericField(decision, "end")
			if hasStart && !startOK {
				errs = append(errs, fmt.Sprintf("decisions[%d].start must be numeric", index))
			}
			if hasEnd && !endOK {
				errs = append(errs, fmt.Sprintf("decisions[%d].end must be numeric", index))
			}
			if hasStart && hasEnd && startOK && endOK && end < start {
				errs = append(errs, fmt.Sprintf("decisions[%d].end must be greater than or equal to start", index))
			}
			if preview, ok := decision["text_preview"]; ok {
				if _, ok := preview.(string); !ok {
					errs = append(errs, fmt.Sprintf("decisions[%d].text_preview must be a string", index))
				}
			}
		}
	}
	return errs
}

func validateExpansionTasksShape(payload map[string]any) []string {
	errs := []string{}
	requireStringValue(&errs, payload, "schema_version", "expansion_tasks.v1")
	requireMap(&errs, payload, "source")
	requireArray(&errs, payload, "tasks")
	return errs
}

func validateVerificationShape(payload map[string]any) []string {
	errs := []string{}
	requireStringValue(&errs, payload, "schema_version", "verification.v1")
	requireMap(&errs, payload, "source")
	requireNonEmptyString(&errs, payload, "status")
	requireArray(&errs, payload, "checks")
	return errs
}

func requireStringValue(errs *[]string, payload map[string]any, key string, want string) {
	got, ok := payload[key].(string)
	if !ok || got != want {
		*errs = append(*errs, fmt.Sprintf("%s must be %s", key, want))
	}
}

func requireNonEmptyString(errs *[]string, payload map[string]any, key string) {
	got, ok := payload[key].(string)
	if !ok || got == "" {
		*errs = append(*errs, key+" must be a non-empty string")
	}
}

func requireDecisionString(errs *[]string, payload map[string]any, key string, label string) {
	got, ok := payload[key].(string)
	if !ok || got == "" {
		*errs = append(*errs, label+" must be a non-empty string")
	}
}

func requireMap(errs *[]string, payload map[string]any, key string) {
	if _, ok := payload[key].(map[string]any); !ok {
		*errs = append(*errs, key+" must be an object")
	}
}

func requireArray(errs *[]string, payload map[string]any, key string) {
	if _, ok := payload[key].([]any); !ok {
		*errs = append(*errs, key+" must be an array")
	}
}

func numericField(payload map[string]any, key string) (float64, bool, bool) {
	value, ok := payload[key]
	if !ok {
		return 0, false, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true, true
	case int:
		return float64(typed), true, true
	default:
		return 0, true, false
	}
}

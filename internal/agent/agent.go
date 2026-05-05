package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"byom-video/internal/events"
	"byom-video/internal/runctx"
)

const PlansRoot = ".byom-video/plans"

type Plan struct {
	SchemaVersion    string     `json:"schema_version"`
	PlanID           string     `json:"plan_id"`
	CreatedAt        time.Time  `json:"created_at"`
	InputPath        string     `json:"input_path"`
	Goal             string     `json:"goal"`
	Mode             string     `json:"mode"`
	TargetType       string     `json:"target_type"`
	Preset           string     `json:"preset"`
	Status           string     `json:"status"`
	ValidationStatus string     `json:"validation_status,omitempty"`
	ValidationErrors []string   `json:"validation_errors,omitempty"`
	ApprovalStatus   string     `json:"approval_status,omitempty"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	ApprovalMode     string     `json:"approval_mode,omitempty"`
	ReviewStatus     string     `json:"review_status,omitempty"`
	Actions          []Action   `json:"actions"`
	Safety           Safety     `json:"safety"`
}

type Action struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	Description    string         `json:"description"`
	CommandPreview string         `json:"command_preview"`
	Options        map[string]any `json:"options,omitempty"`
	RunID          string         `json:"run_id,omitempty"`
	BatchID        string         `json:"batch_id,omitempty"`
	Error          string         `json:"error,omitempty"`
}

type Safety struct {
	ExportsRequireExplicitExecution bool  `json:"exports_require_explicit_execution"`
	NoInputFilesModified            bool  `json:"no_input_files_modified"`
	Present                         *bool `json:"-"`
}

type GoalOptions struct {
	PresetOverride string
	MaxClips       int
	WithExport     bool
	WithValidate   bool
	WithReport     bool
	WithReportSet  bool
	Mode           string
	Recursive      bool
	Once           bool
	Limit          int
}

func NewPlan(inputPath string, goal string, opts GoalOptions, now time.Time) (Plan, error) {
	if strings.TrimSpace(goal) == "" {
		return Plan{}, fmt.Errorf("goal is required")
	}
	abs, err := filepath.Abs(inputPath)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve input path: %w", err)
	}
	planID, err := runctx.NewRunID(now)
	if err != nil {
		return Plan{}, err
	}
	targetType, err := InferTargetType(abs, goal, opts.Mode)
	if err != nil {
		return Plan{}, err
	}
	preset, runOptions, err := ParseGoal(goal, opts)
	if err != nil {
		return Plan{}, err
	}
	if opts.PresetOverride != "" {
		preset = opts.PresetOverride
		runOptions = optionsForPreset(preset, opts)
	}
	actionType := "run_pipeline"
	if targetType == "batch" {
		actionType = "batch_pipeline"
	}
	if targetType == "watch" {
		actionType = "watch_pipeline"
	}
	runOptions["recursive"] = opts.Recursive
	runOptions["once"] = opts.Once
	if opts.Limit > 0 {
		runOptions["limit"] = opts.Limit
	}
	actions := []Action{{
		ID:             "act_0001",
		Type:           actionType,
		Status:         "planned",
		Description:    descriptionForPreset(preset),
		CommandPreview: CommandPreviewForOptions(actionType, abs, preset, "", runOptions),
		Options:        runOptions,
	}}
	next := 2
	if opts.WithExport {
		actions = append(actions, Action{ID: actionID(next), Type: "export_run", Status: "planned", Description: "Run explicit export", CommandPreview: "./byom-video export <run_id>", Options: map[string]any{}})
		next++
	}
	if opts.WithValidate {
		actions = append(actions, Action{ID: actionID(next), Type: "validate_run", Status: "planned", Description: "Validate run artifacts", CommandPreview: "./byom-video validate <run_id>", Options: map[string]any{}})
	}
	present := true
	return Plan{
		SchemaVersion:  "agent_plan.v1",
		PlanID:         planID,
		CreatedAt:      now.UTC(),
		InputPath:      abs,
		Goal:           goal,
		Mode:           "deterministic",
		TargetType:     targetType,
		Preset:         preset,
		Status:         "planned",
		ApprovalStatus: "pending",
		ReviewStatus:   "not_reviewed",
		Actions:        actions,
		Safety: Safety{
			ExportsRequireExplicitExecution: true,
			NoInputFilesModified:            true,
			Present:                         &present,
		},
	}, nil
}

func NormalizePlan(plan *Plan) {
	if plan.ApprovalStatus == "" {
		plan.ApprovalStatus = "pending"
	}
	if plan.ReviewStatus == "" {
		plan.ReviewStatus = "not_reviewed"
	}
	if plan.TargetType == "" {
		plan.TargetType = "file"
	}
	for i := range plan.Actions {
		if plan.Actions[i].Options == nil {
			plan.Actions[i].Options = map[string]any{}
		}
	}
}

func optionsForPreset(preset string, opts GoalOptions) map[string]any {
	if preset == "metadata" {
		return map[string]any{}
	}
	maxClips := opts.MaxClips
	if maxClips == 0 {
		maxClips = 5
	}
	withReport := opts.WithReport
	if !opts.WithReportSet {
		withReport = true
	}
	return map[string]any{
		"with_transcript":    true,
		"with_captions":      true,
		"with_chunks":        true,
		"with_highlights":    true,
		"with_roughcut":      true,
		"with_ffmpeg_script": true,
		"with_report":        withReport,
		"roughcut_max_clips": maxClips,
	}
}

func InferTargetType(inputPath string, goal string, override string) (string, error) {
	if override != "" {
		if override == "file" || override == "batch" || override == "watch" {
			return override, nil
		}
		return "", fmt.Errorf("unknown plan mode %q; supported values: file, batch, watch", override)
	}
	g := strings.ToLower(goal)
	info, err := os.Stat(inputPath)
	isDir := err == nil && info.IsDir()
	switch {
	case isDir && (strings.Contains(g, "watch") || strings.Contains(g, "monitor") || strings.Contains(g, "keep processing")):
		return "watch", nil
	case isDir:
		return "batch", nil
	default:
		return "file", nil
	}
}

func ParseGoal(goal string, opts GoalOptions) (string, map[string]any, error) {
	g := strings.ToLower(strings.TrimSpace(goal))
	maxClips := opts.MaxClips
	if maxClips == 0 {
		maxClips = numberBefore(g, "shorts")
	}
	if maxClips == 0 {
		maxClips = numberBefore(g, "clips")
	}
	if maxClips == 0 {
		maxClips = 5
	}
	withReport := opts.WithReport
	if !opts.WithReportSet {
		withReport = true
	}
	switch {
	case strings.Contains(g, "metadata"):
		return "metadata", map[string]any{}, nil
	case strings.Contains(g, "caption"):
		return "custom", map[string]any{"with_transcript": true, "with_captions": true}, nil
	case strings.Contains(g, "transcrib"):
		return "custom", map[string]any{"with_transcript": true}, nil
	case strings.Contains(g, "highlight"):
		return "custom", map[string]any{"with_transcript": true, "with_chunks": true, "with_highlights": true}, nil
	case strings.Contains(g, "roughcut") || strings.Contains(g, "clip") || strings.Contains(g, "short") || strings.Contains(g, "process"):
		return "shorts", map[string]any{
			"with_transcript":    true,
			"with_captions":      true,
			"with_chunks":        true,
			"with_highlights":    true,
			"with_roughcut":      true,
			"with_ffmpeg_script": true,
			"with_report":        withReport,
			"roughcut_max_clips": maxClips,
		}, nil
	default:
		return "", nil, fmt.Errorf("unknown goal %q; examples: make 5 shorts, create clips, find highlights, transcribe this, metadata only, make captions", goal)
	}
}

func ValidatePlan(plan Plan) []string {
	errs := []string{}
	if plan.SchemaVersion != "agent_plan.v1" {
		errs = append(errs, "schema_version must be agent_plan.v1")
	}
	if strings.TrimSpace(plan.PlanID) == "" {
		errs = append(errs, "plan_id is required")
	}
	if strings.TrimSpace(plan.InputPath) == "" {
		errs = append(errs, "input_path is required")
	} else if requiresInputPath(plan.TargetType) {
		if _, err := os.Stat(plan.InputPath); err != nil {
			errs = append(errs, fmt.Sprintf("input_path is unavailable: %v", err))
		}
	}
	if strings.TrimSpace(plan.Goal) == "" {
		errs = append(errs, "goal is required")
	}
	if len(plan.Actions) == 0 {
		errs = append(errs, "actions must not be empty")
	}
	if !plan.Safety.ExportsRequireExplicitExecution {
		errs = append(errs, "safety.exports_require_explicit_execution must be true")
	}
	if !plan.Safety.NoInputFilesModified {
		errs = append(errs, "safety.no_input_files_modified must be true")
	}
	for i, action := range plan.Actions {
		prefix := fmt.Sprintf("action %d", i)
		if action.ID == "" {
			errs = append(errs, prefix+": id is required")
		}
		if !allowedActionType(action.Type) {
			errs = append(errs, prefix+": unsupported action type "+action.Type)
		}
		if !allowedStatus(action.Status) {
			errs = append(errs, prefix+": invalid status "+action.Status)
		}
		if action.Description == "" {
			errs = append(errs, prefix+": description is required")
		}
		if action.Options == nil {
			errs = append(errs, prefix+": options is required")
		}
	}
	return errs
}

func CommandPreview(actionType string, path string, preset string, runID string, opts GoalOptions) string {
	quoted := fmt.Sprintf("%q", path)
	switch actionType {
	case "run_pipeline":
		return fmt.Sprintf("./byom-video pipeline %s --preset %s", quoted, presetForCommand(preset))
	case "batch_pipeline":
		cmd := fmt.Sprintf("./byom-video batch %s --preset %s", quoted, presetForCommand(preset))
		if opts.Recursive {
			cmd += " --recursive"
		}
		if opts.Limit > 0 {
			cmd += fmt.Sprintf(" --limit %d", opts.Limit)
		}
		return cmd
	case "watch_pipeline":
		cmd := fmt.Sprintf("./byom-video watch %s --preset %s", quoted, presetForCommand(preset))
		if opts.Recursive {
			cmd += " --recursive"
		}
		if opts.Once {
			cmd += " --once"
		}
		if opts.Limit > 0 {
			cmd += fmt.Sprintf(" --limit %d", opts.Limit)
		}
		return cmd
	case "export_run":
		if runID == "" {
			runID = "<run_id>"
		}
		return "./byom-video export " + runID
	case "validate_run":
		if runID == "" {
			runID = "<run_id>"
		}
		return "./byom-video validate " + runID
	default:
		return ""
	}
}

func CommandPreviewForOptions(actionType string, path string, preset string, runID string, options map[string]any) string {
	quoted := fmt.Sprintf("%q", path)
	switch actionType {
	case "run_pipeline":
		if len(options) == 0 {
			return fmt.Sprintf("./byom-video run %s", quoted)
		}
		cmd := fmt.Sprintf("./byom-video run %s", quoted)
		if boolMapOption(options, "with_transcript") {
			cmd += " --with-transcript"
		}
		if boolMapOption(options, "with_captions") {
			cmd += " --with-captions"
		}
		if boolMapOption(options, "with_chunks") {
			cmd += " --with-chunks"
		}
		if boolMapOption(options, "with_highlights") {
			cmd += " --with-highlights"
		}
		if boolMapOption(options, "with_roughcut") {
			cmd += " --with-roughcut"
		}
		if boolMapOption(options, "with_ffmpeg_script") {
			cmd += " --with-ffmpeg-script"
		}
		if boolMapOption(options, "with_report") {
			cmd += " --with-report"
		}
		if boolMapOption(options, "with_transcript") {
			cmd += " --transcript-model-size " + stringMapOption(options, "transcript_model_size", "tiny")
		}
		if value, ok := optionValue(options, "chunk_target_seconds"); ok {
			cmd += " --chunk-target-seconds " + value
		}
		if value, ok := optionValue(options, "chunk_max_gap_seconds"); ok {
			cmd += " --chunk-max-gap-seconds " + value
		}
		if value, ok := optionValue(options, "highlight_top_k"); ok {
			cmd += " --highlight-top-k " + value
		}
		if value, ok := optionValue(options, "highlight_min_duration_seconds"); ok {
			cmd += " --highlight-min-duration-seconds " + value
		}
		if value, ok := optionValue(options, "highlight_max_duration_seconds"); ok {
			cmd += " --highlight-max-duration-seconds " + value
		}
		if value, ok := optionValue(options, "roughcut_max_clips"); ok {
			cmd += " --roughcut-max-clips " + value
		}
		if value, ok := optionValue(options, "ffmpeg_output_format"); ok {
			cmd += " --ffmpeg-output-format " + value
		}
		return cmd
	case "batch_pipeline":
		cmd := fmt.Sprintf("./byom-video batch %s --preset %s", quoted, presetForCommand(preset))
		if boolMapOption(options, "recursive") {
			cmd += " --recursive"
		}
		if value, ok := optionValue(options, "limit"); ok {
			cmd += " --limit " + value
		}
		return cmd
	case "watch_pipeline":
		cmd := fmt.Sprintf("./byom-video watch %s --preset %s", quoted, presetForCommand(preset))
		if boolMapOption(options, "recursive") {
			cmd += " --recursive"
		}
		if boolMapOption(options, "once") {
			cmd += " --once"
		}
		if value, ok := optionValue(options, "limit"); ok {
			cmd += " --limit " + value
		}
		return cmd
	case "export_run":
		if runID == "" {
			runID = "<run_id>"
		}
		return "./byom-video export " + runID
	case "validate_run":
		if runID == "" {
			runID = "<run_id>"
		}
		return "./byom-video validate " + runID
	default:
		return ""
	}
}

func boolMapOption(options map[string]any, key string) bool {
	value, _ := options[key].(bool)
	return value
}

func stringMapOption(options map[string]any, key string, fallback string) string {
	value, ok := options[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func optionValue(options map[string]any, key string) (string, bool) {
	value, ok := options[key]
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	case int:
		return fmt.Sprintf("%d", typed), true
	case int64:
		return fmt.Sprintf("%d", typed), true
	case float64:
		return fmt.Sprintf("%g", typed), true
	case float32:
		return fmt.Sprintf("%g", typed), true
	default:
		return fmt.Sprint(typed), true
	}
}

func allowedActionType(value string) bool {
	switch value {
	case "run_pipeline", "batch_pipeline", "watch_pipeline", "export_run", "validate_run":
		return true
	default:
		return false
	}
}

func allowedStatus(value string) bool {
	switch value {
	case "planned", "running", "completed", "failed", "skipped":
		return true
	default:
		return false
	}
}

func requiresInputPath(targetType string) bool {
	return targetType == "" || targetType == "file" || targetType == "batch" || targetType == "watch"
}

func presetForCommand(preset string) string {
	if preset == "custom" {
		return "shorts"
	}
	return preset
}

func WritePlan(plan Plan) error {
	dir := filepath.Join(PlansRoot, plan.PlanID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create plan directory: %w", err)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "agent_plan.json"), data, 0o644); err != nil {
		return fmt.Errorf("write agent plan: %w", err)
	}
	return nil
}

func ReadPlan(planID string) (Plan, error) {
	if planID == "" || planID != filepath.Base(planID) || strings.Contains(planID, "..") {
		return Plan{}, fmt.Errorf("invalid plan id %q", planID)
	}
	data, err := os.ReadFile(filepath.Join(PlansRoot, planID, "agent_plan.json"))
	if err != nil {
		return Plan{}, fmt.Errorf("read agent plan: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("decode agent plan: %w", err)
	}
	NormalizePlan(&plan)
	return plan, nil
}

func ListPlans() ([]Plan, error) {
	entries, err := os.ReadDir(PlansRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []Plan{}, nil
		}
		return nil, fmt.Errorf("read plans directory: %w", err)
	}
	plans := []Plan{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		plan, err := ReadPlan(entry.Name())
		if err == nil {
			plans = append(plans, plan)
		}
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].CreatedAt.After(plans[j].CreatedAt) })
	return plans, nil
}

func OpenActionLog(planID string) (*events.Log, error) {
	return events.Open(filepath.Join(PlansRoot, planID, "actions.jsonl"))
}

func PlanDir(planID string) string {
	return filepath.Join(PlansRoot, planID)
}

func actionID(n int) string {
	return fmt.Sprintf("act_%04d", n)
}

func descriptionForPreset(preset string) string {
	if preset == "metadata" {
		return "Run local metadata pipeline"
	}
	return "Run local shorts pipeline"
}

func numberBefore(goal string, word string) int {
	re := regexp.MustCompile(`(\d+)\s+` + regexp.QuoteMeta(word))
	match := re.FindStringSubmatch(goal)
	if len(match) != 2 {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(match[1], "%d", &n)
	return n
}

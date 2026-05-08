package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/modelrouter"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type ExpandDryRunOptions struct {
	JSON     bool
	Strict   bool
	TaskType string
}

type ExpandLocalStubOptions struct {
	Overwrite bool
	JSON      bool
	TaskType  string
}

type ExpandOptions struct {
	Overwrite bool
	JSON      bool
	TaskType  string
	Strict    bool
	DryRun    bool
	MaxTasks  int
	FailFast  bool
}

type ModelRequestsDryRun struct {
	SchemaVersion string                `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	RunID         string                `json:"run_id"`
	Requests      []modelrouter.Request `json:"requests"`
	Warnings      []string              `json:"warnings,omitempty"`
}

type ExpandDryRunSummary struct {
	RunID    string                `json:"run_id"`
	Artifact string                `json:"artifact"`
	Requests []modelrouter.Request `json:"requests"`
	Warnings []string              `json:"warnings,omitempty"`
}

type ExpandSummary struct {
	RunID      string         `json:"run_id"`
	Mode       string         `json:"mode"`
	Files      []string       `json:"files"`
	ItemCounts map[string]int `json:"item_counts"`
	Warnings   []string       `json:"warnings,omitempty"`
	Failures   int            `json:"failures,omitempty"`
}

type ExecutedModelRequestEntry struct {
	TaskID         string                     `json:"task_id"`
	DecisionID     string                     `json:"decision_id"`
	TaskType       string                     `json:"task_type"`
	ModelRoute     string                     `json:"model_route"`
	ModelEntry     string                     `json:"model_entry"`
	Provider       string                     `json:"provider"`
	Model          string                     `json:"model"`
	Status         string                     `json:"status"`
	RequestPreview modelrouter.RequestPreview `json:"request_preview"`
	ResponseMode   string                     `json:"response_mode,omitempty"`
	Error          string                     `json:"error,omitempty"`
}

type ExecutedModelRequests struct {
	SchemaVersion string                      `json:"schema_version"`
	CreatedAt     time.Time                   `json:"created_at"`
	RunID         string                      `json:"run_id"`
	Requests      []ExecutedModelRequestEntry `json:"requests"`
}

type ReviewModelRequestsOptions struct {
	JSON          bool
	WriteArtifact bool
}

type ModelRequestsReview struct {
	RunID            string         `json:"run_id"`
	DryRunCount      int            `json:"dry_run_count"`
	ExecutedCount    int            `json:"executed_count"`
	Providers        map[string]int `json:"providers"`
	Models           map[string]int `json:"models"`
	TaskTypes        map[string]int `json:"task_types"`
	Statuses         map[string]int `json:"statuses"`
	ResponseModes    map[string]int `json:"response_modes"`
	Failures         []string       `json:"failures,omitempty"`
	DryRunArtifact   string         `json:"dry_run_artifact,omitempty"`
	ExecutedArtifact string         `json:"executed_artifact,omitempty"`
	ReviewArtifact   string         `json:"review_artifact,omitempty"`
}

func ExpandDryRun(runID string, stdout io.Writer, opts ExpandDryRunOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPAND_DRY_RUN_STARTED", map[string]any{"run_id": runID})
	}

	cfg := config.Config{}
	if c, loadErr := config.Load(config.DefaultPath); loadErr == nil {
		cfg = c
	}
	mask, tasks, requests, warnings, err := buildAdapterRequests(runDir, cfg, opts.TaskType, "dry-run")
	if err != nil {
		writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", err.Error())
		return err
	}
	_ = mask
	_ = tasks

	if opts.Strict {
		for _, req := range requests {
			if len(req.Warnings) > 0 || req.ModelEntryName == "" || req.Provider == "" || req.Model == "" {
				writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", "missing routes or entries (strict mode)")
				return fmt.Errorf("expand-dry-run: missing routes or entries (--strict)")
			}
		}
	}

	adapter, ok := modelrouter.DefaultRegistry().ByName("dry-run")
	if !ok {
		return fmt.Errorf("dry-run adapter is not registered")
	}
	for i, req := range requests {
		built, buildErr := adapter.BuildRequest(req)
		if buildErr != nil {
			writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", buildErr.Error())
			return buildErr
		}
		if _, execErr := adapter.Execute(built); execErr != nil {
			writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", execErr.Error())
			return execErr
		}
		requests[i] = built
	}

	artifact := ModelRequestsDryRun{
		SchemaVersion: "model_requests.dryrun.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Requests:      requests,
		Warnings:      dedupeStrings(warnings),
	}
	artifactPath := filepath.Join(runDir, "model_requests.dryrun.json")
	if err := writeJSONFile(artifactPath, artifact); err != nil {
		writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "model_requests_dryrun", "model_requests.dryrun.json"); err != nil {
		writeMaskFailure(log, "EXPAND_DRY_RUN_FAILED", err.Error())
		return err
	}

	if log != nil {
		_ = log.Write("EXPAND_DRY_RUN_COMPLETED", map[string]any{
			"run_id":   runID,
			"requests": len(requests),
			"warnings": len(artifact.Warnings),
		})
	}

	summary := ExpandDryRunSummary{
		RunID:    runID,
		Artifact: "model_requests.dryrun.json",
		Requests: requests,
		Warnings: artifact.Warnings,
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Expansion dry run")
	fmt.Fprintf(stdout, "  run id:   %s\n", runID)
	fmt.Fprintf(stdout, "  requests: %d\n", len(requests))
	fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "model_requests.dryrun.json"))
	for _, req := range requests {
		fmt.Fprintf(stdout, "  - %s (%s)\n", req.TaskID, req.TaskType)
		fmt.Fprintf(stdout, "    route:    %s\n", req.RouteName)
		if req.ModelEntryName != "" {
			fmt.Fprintf(stdout, "    entry:    %s\n", req.ModelEntryName)
		}
		if req.Provider != "" {
			fmt.Fprintf(stdout, "    provider: %s\n", req.Provider)
		}
		if req.Model != "" {
			fmt.Fprintf(stdout, "    model:    %s\n", req.Model)
		}
		fmt.Fprintf(stdout, "    preview:  %s\n", req.RequestPreview.User)
		for _, warning := range req.Warnings {
			fmt.Fprintf(stdout, "    warning:  %s\n", warning)
		}
	}
	for _, warning := range artifact.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
	return nil
}

func ExpandLocalStub(runID string, stdout io.Writer, opts ExpandLocalStubOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPAND_LOCAL_STUB_STARTED", map[string]any{"run_id": runID})
	}

	cfg := config.Config{}
	if c, loadErr := config.Load(config.DefaultPath); loadErr == nil {
		cfg = c
	}
	mask, tasks, requests, warnings, err := buildAdapterRequests(runDir, cfg, opts.TaskType, "stub")
	if err != nil {
		writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", err.Error())
		return err
	}
	adapter, ok := modelrouter.DefaultRegistry().ByName("stub")
	if !ok {
		return fmt.Errorf("stub adapter is not registered")
	}

	if err := os.MkdirAll(filepath.Join(runDir, "expansions"), 0o755); err != nil {
		writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", err.Error())
		return fmt.Errorf("create expansions dir: %w", err)
	}

	decisionMap := buildDecisionMap(mask.Decisions)
	rejectedIDs := map[string]bool{}
	for _, d := range mask.Decisions {
		if d.Decision == "reject" {
			rejectedIDs[d.ID] = true
		}
	}

	grouped := groupExpansionTasks(tasks.Tasks, opts.TaskType)
	if opts.TaskType != "" && len(grouped) == 0 {
		msg := fmt.Sprintf("no tasks found for task type %q", opts.TaskType)
		writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", msg)
		return fmt.Errorf("%s", msg)
	}

	summary := ExpandStubSummary{
		RunID:      runID,
		Mode:       "local_stub_adapter",
		ItemCounts: map[string]int{},
		Warnings:   warnings,
	}

	requestByType := map[string]modelrouter.Request{}
	for _, req := range requests {
		requestByType[req.TaskType] = req
	}

	for _, group := range grouped {
		outPath := filepath.Join(runDir, "expansions", group.taskType+".json")
		if !opts.Overwrite {
			if _, statErr := os.Stat(outPath); statErr == nil {
				msg := fmt.Sprintf("expansions/%s.json already exists; pass --overwrite", group.taskType)
				writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", msg)
				return fmt.Errorf("%s", msg)
			}
		}

		req, ok := requestByType[group.taskType]
		if !ok {
			msg := fmt.Sprintf("missing built request for task type %q", group.taskType)
			writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", msg)
			return fmt.Errorf("%s", msg)
		}
		built, buildErr := adapter.BuildRequest(req)
		if buildErr != nil {
			writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", buildErr.Error())
			return buildErr
		}
		if _, execErr := adapter.Execute(built); execErr != nil {
			writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", execErr.Error())
			return execErr
		}

		output, taskWarnings := buildStubOutput(group.taskType, group.tasks, mask, decisionMap, rejectedIDs)
		output.Mode = "local_stub_adapter"
		taskIDs := make([]string, 0, len(group.tasks))
		for _, task := range group.tasks {
			taskIDs = append(taskIDs, task.ID)
		}
		output.Source = ExpansionOutputSource{
			InferenceMaskArtifact:  "inference_mask.json",
			ExpansionTasksArtifact: "expansion_tasks.json",
			TaskIDs:                taskIDs,
		}
		output.Warnings = append(output.Warnings, taskWarnings...)
		if err := writeJSONFile(outPath, output); err != nil {
			writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", err.Error())
			return err
		}
		if err := addManifestArtifact(runDir, "expansion_"+group.taskType, filepath.Join("expansions", group.taskType+".json")); err != nil {
			writeMaskFailure(log, "EXPAND_LOCAL_STUB_FAILED", err.Error())
			return err
		}
		relPath := filepath.Join("expansions", group.taskType+".json")
		summary.Files = append(summary.Files, relPath)
		summary.ItemCounts[group.taskType] = len(output.Items)
		summary.Warnings = append(summary.Warnings, taskWarnings...)
	}

	if log != nil {
		_ = log.Write("EXPAND_LOCAL_STUB_COMPLETED", map[string]any{
			"run_id": runID,
			"files":  summary.Files,
		})
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Local stub expansion completed")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  mode:   local_stub_adapter\n")
	for _, relPath := range summary.Files {
		taskType := strings.TrimSuffix(filepath.Base(relPath), ".json")
		fmt.Fprintf(stdout, "  - %-25s  %d items\n", relPath, summary.ItemCounts[taskType])
	}
	for _, warning := range dedupeStrings(summary.Warnings) {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
	return nil
}

func Expand(runID string, stdout io.Writer, opts ExpandOptions) error {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return err
	}
	if !cfg.Models.Enabled {
		return fmt.Errorf("models.enabled is false; use expand-stub or enable models in byom-video.yaml")
	}
	if opts.MaxTasks < 0 {
		return fmt.Errorf("--max-tasks must be positive")
	}
	if opts.MaxTasks == 0 {
		opts.MaxTasks = -1
	}
	if opts.DryRun {
		return ExpandDryRun(runID, stdout, ExpandDryRunOptions{
			JSON:     opts.JSON,
			Strict:   opts.Strict,
			TaskType: opts.TaskType,
		})
	}

	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("EXPAND_STARTED", map[string]any{"run_id": runID})
	}

	mask, tasks, requests, warnings, err := buildAdapterRequests(runDir, cfg, opts.TaskType, "provider")
	if err != nil {
		writeMaskFailure(log, "EXPAND_FAILED", err.Error())
		return err
	}
	if opts.Strict {
		for _, req := range requests {
			if len(req.Warnings) > 0 || req.ModelEntryName == "" || req.Provider == "" || req.Model == "" {
				writeMaskFailure(log, "EXPAND_FAILED", "missing routes, entries, or providers (strict mode)")
				return fmt.Errorf("expand: missing routes, entries, or providers (--strict)")
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(runDir, "expansions"), 0o755); err != nil {
		writeMaskFailure(log, "EXPAND_FAILED", err.Error())
		return fmt.Errorf("create expansions dir: %w", err)
	}

	decisionMap := buildDecisionMap(mask.Decisions)
	rejectedIDs := map[string]bool{}
	for _, d := range mask.Decisions {
		if d.Decision == "reject" {
			rejectedIDs[d.ID] = true
		}
	}
	grouped := groupExpansionTasks(tasks.Tasks, opts.TaskType)
	requestByType := map[string]modelrouter.Request{}
	for _, req := range requests {
		requestByType[req.TaskType] = req
	}

	summary := ExpandSummary{
		RunID:      runID,
		Mode:       "provider",
		Files:      []string{},
		ItemCounts: map[string]int{},
		Warnings:   warnings,
	}
	executedTasks := 0
	executedLog := ExecutedModelRequests{
		SchemaVersion: "model_requests.executed.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Requests:      []ExecutedModelRequestEntry{},
	}
	hadFailure := false

	for _, group := range grouped {
		outPath := filepath.Join(runDir, "expansions", group.taskType+".json")
		if !opts.Overwrite {
			if _, statErr := os.Stat(outPath); statErr == nil {
				msg := fmt.Sprintf("expansions/%s.json already exists; pass --overwrite", group.taskType)
				writeMaskFailure(log, "EXPAND_FAILED", msg)
				return fmt.Errorf("%s", msg)
			}
		}

		req, ok := requestByType[group.taskType]
		if !ok {
			msg := fmt.Sprintf("missing request for task type %q", group.taskType)
			if opts.Strict {
				writeMaskFailure(log, "EXPAND_FAILED", msg)
				return fmt.Errorf("%s", msg)
			}
			summary.Warnings = append(summary.Warnings, msg)
			continue
		}
		adapter, ok := modelrouter.DefaultRegistry().ForProvider(req.Provider)
		if !ok {
			msg := fmt.Sprintf("no adapter registered for provider %q", req.Provider)
			if opts.Strict {
				writeMaskFailure(log, "EXPAND_FAILED", msg)
				return fmt.Errorf("%s", msg)
			}
			summary.Warnings = append(summary.Warnings, msg)
			continue
		}

		output, groupWarnings, taskCalls, entries, execErr := executeProviderGroup(log, group.taskType, group.tasks, mask, decisionMap, rejectedIDs, req, adapter, opts.MaxTasks, executedTasks, opts.FailFast)
		executedLog.Requests = append(executedLog.Requests, entries...)
		executedTasks += taskCalls
		if execErr != nil {
			hadFailure = true
			summary.Failures++
			summary.Warnings = append(summary.Warnings, execErr.Error())
			if opts.FailFast {
				break
			}
		}
		output.Source = ExpansionOutputSource{
			InferenceMaskArtifact:  "inference_mask.json",
			ExpansionTasksArtifact: "expansion_tasks.json",
			TaskIDs:                collectTaskIDs(group.tasks),
		}
		output.Mode = "provider"
		output.TaskType = group.taskType
		output.Warnings = append(output.Warnings, groupWarnings...)
		if err := writeJSONFile(outPath, output); err != nil {
			writeMaskFailure(log, "EXPAND_FAILED", err.Error())
			return err
		}
		if err := addManifestArtifact(runDir, "expansion_"+group.taskType, filepath.Join("expansions", group.taskType+".json")); err != nil {
			writeMaskFailure(log, "EXPAND_FAILED", err.Error())
			return err
		}
		relPath := filepath.Join("expansions", group.taskType+".json")
		summary.Files = append(summary.Files, relPath)
		summary.ItemCounts[group.taskType] = len(output.Items)
		summary.Warnings = append(summary.Warnings, groupWarnings...)
	}

	executedArtifactPath := filepath.Join(runDir, "model_requests.executed.json")
	if err := writeJSONFile(executedArtifactPath, executedLog); err != nil {
		writeMaskFailure(log, "EXPAND_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "model_requests_executed", "model_requests.executed.json"); err != nil {
		writeMaskFailure(log, "EXPAND_FAILED", err.Error())
		return err
	}

	if log != nil {
		eventName := "EXPAND_COMPLETED"
		if hadFailure {
			eventName = "EXPAND_FAILED"
		}
		_ = log.Write(eventName, map[string]any{
			"run_id":   runID,
			"files":    summary.Files,
			"failures": summary.Failures,
		})
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Provider expansion completed")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  mode:   provider\n")
	for _, relPath := range summary.Files {
		taskType := strings.TrimSuffix(filepath.Base(relPath), ".json")
		fmt.Fprintf(stdout, "  - %-25s  %d items\n", relPath, summary.ItemCounts[taskType])
	}
	for _, warning := range dedupeStrings(summary.Warnings) {
		fmt.Fprintf(stdout, "  warning: %s\n", warning)
	}
	if hadFailure {
		return fmt.Errorf("expand completed with %d failed request(s)", summary.Failures)
	}
	return nil
}

type expansionTaskGroup struct {
	taskType string
	tasks    []ExpansionTask
}

func groupExpansionTasks(tasks []ExpansionTask, filter string) []expansionTaskGroup {
	seen := map[string]bool{}
	var groups []expansionTaskGroup
	for _, task := range tasks {
		if filter != "" && task.Type != filter {
			continue
		}
		if !seen[task.Type] {
			seen[task.Type] = true
			groups = append(groups, expansionTaskGroup{taskType: task.Type})
		}
		for i := range groups {
			if groups[i].taskType == task.Type {
				groups[i].tasks = append(groups[i].tasks, task)
				break
			}
		}
	}
	return groups
}

func buildAdapterRequests(runDir string, cfg config.Config, taskType string, adapterName string) (InferenceMask, ExpansionTasks, []modelrouter.Request, []string, error) {
	mask, err := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	if err != nil {
		return InferenceMask{}, ExpansionTasks{}, nil, nil, err
	}

	taskData, err := os.ReadFile(filepath.Join(runDir, "expansion_tasks.json"))
	if err != nil {
		return InferenceMask{}, ExpansionTasks{}, nil, nil, fmt.Errorf("expansion_tasks.json is required; run expansion-plan first")
	}
	var tasks ExpansionTasks
	if err := json.Unmarshal(taskData, &tasks); err != nil {
		return InferenceMask{}, ExpansionTasks{}, nil, nil, fmt.Errorf("decode expansion_tasks.json: %w", err)
	}

	decisionMap := buildDecisionMap(mask.Decisions)
	requests := []modelrouter.Request{}
	warnings := []string{}
	for _, task := range tasks.Tasks {
		if taskType != "" && task.Type != taskType {
			continue
		}
		req, reqWarnings := buildModelRouterRequest(task, mask, cfg, decisionMap, adapterName)
		requests = append(requests, req)
		warnings = append(warnings, reqWarnings...)
	}
	if taskType != "" && len(requests) == 0 {
		return InferenceMask{}, ExpansionTasks{}, nil, nil, fmt.Errorf("no tasks found for task type %q", taskType)
	}
	return mask, tasks, requests, dedupeStrings(warnings), nil
}

func buildModelRouterRequest(task ExpansionTask, mask InferenceMask, cfg config.Config, decisionMap map[string]MaskDecision, adapterName string) (modelrouter.Request, []string) {
	route := resolveRouteEntry(task.ID, task.Type, task.ModelRoute, cfg)
	entry := cfg.Models.Entries[route.ResolvedEntry]
	request := modelrouter.Request{
		TaskID:         task.ID,
		TaskType:       task.Type,
		RouteName:      task.ModelRoute,
		ModelEntryName: route.ResolvedEntry,
		Provider:       route.Provider,
		Model:          route.Model,
		Role:           route.Role,
		BaseURL:        entry.BaseURL,
		Options:        cloneAnyMap(entry.Options),
		Status:         adapterName,
		Warnings:       append([]string{}, route.Warnings...),
		Input: modelrouter.RequestInput{
			Decisions:      []modelrouter.DecisionInput{},
			Constraints:    maskConstraintsMap(mask.Constraints),
			OutputContract: cloneAnyMap(task.OutputContract),
		},
	}
	for _, ref := range task.InputRefs {
		if decision, ok := decisionMap[ref]; ok {
			request.Input.Decisions = append(request.Input.Decisions, modelrouter.DecisionInput{
				ID:          decision.ID,
				Start:       decision.Start,
				End:         decision.End,
				Decision:    decision.Decision,
				Reason:      decision.Reason,
				TextPreview: decision.TextPreview,
			})
		} else {
			request.Warnings = append(request.Warnings, fmt.Sprintf("decision %q not found in inference_mask.json", ref))
		}
	}
	return request, dedupeStrings(request.Warnings)
}

func maskConstraintsMap(constraints MaskConstraints) map[string]any {
	return map[string]any{
		"must_include":      append([]string{}, constraints.MustInclude...),
		"must_not_include":  append([]string{}, constraints.MustNotInclude...),
		"tone":              constraints.Tone,
		"max_caption_words": constraints.MaxCaptionWords,
	}
}

func validateModelRequestsDryRunShape(payload map[string]any) []string {
	errs := []string{}
	requireStringValue(&errs, payload, "schema_version", "model_requests.dryrun.v1")
	requireNonEmptyString(&errs, payload, "run_id")
	requireArray(&errs, payload, "requests")
	requests, ok := payload["requests"].([]any)
	if !ok {
		return errs
	}
	for index, raw := range requests {
		req, ok := raw.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("requests[%d] must be an object", index))
			continue
		}
		requireDecisionString(&errs, req, "task_id", fmt.Sprintf("requests[%d].task_id", index))
		requireDecisionString(&errs, req, "task_type", fmt.Sprintf("requests[%d].task_type", index))
		requireDecisionString(&errs, req, "route_name", fmt.Sprintf("requests[%d].route_name", index))
		requireMap(&errs, req, "request_preview")
		requireDecisionString(&errs, req, "status", fmt.Sprintf("requests[%d].status", index))
	}
	return errs
}

func executeProviderGroup(log *events.Log, taskType string, tasks []ExpansionTask, mask InferenceMask, decisionMap map[string]MaskDecision, rejectedIDs map[string]bool, baseReq modelrouter.Request, adapter modelrouter.Adapter, maxTasks int, alreadyExecuted int, failFast bool) (ExpansionOutput, []string, int, []ExecutedModelRequestEntry, error) {
	output := ExpansionOutput{
		SchemaVersion: "expansion_output.v1",
		CreatedAt:     time.Now().UTC(),
		Mode:          "provider",
		TaskType:      taskType,
		Items:         []ExpansionOutputItem{},
	}
	warnings := []string{}
	executed := 0
	requestEntries := []ExecutedModelRequestEntry{}
	var groupErr error

	seenRefs := map[string]bool{}
	for _, task := range tasks {
		for _, ref := range task.InputRefs {
			key := task.ID + ":" + ref
			if seenRefs[key] {
				continue
			}
			seenRefs[key] = true
			if rejectedIDs[ref] {
				continue
			}
			decision, ok := decisionMap[ref]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("decision %q not found in inference_mask.json", ref))
				continue
			}
			if maxTasks > 0 && alreadyExecuted+executed >= maxTasks {
				return output, append(warnings, fmt.Sprintf("stopped after --max-tasks=%d", maxTasks)), executed, requestEntries, nil
			}

			req := baseReq
			req.TaskID = task.ID
			req.RouteName = task.ModelRoute
			req.TaskType = taskType
			req.Input.OutputContract = cloneAnyMap(task.OutputContract)
			req.Input.Decisions = []modelrouter.DecisionInput{{
				ID:          decision.ID,
				Start:       decision.Start,
				End:         decision.End,
				Decision:    decision.Decision,
				Reason:      decision.Reason,
				TextPreview: decision.TextPreview,
			}}
			entry := ExecutedModelRequestEntry{
				TaskID:     task.ID,
				DecisionID: decision.ID,
				TaskType:   taskType,
				ModelRoute: task.ModelRoute,
				ModelEntry: req.ModelEntryName,
				Provider:   req.Provider,
				Model:      req.Model,
				Status:     "failed",
			}
			built, err := adapter.BuildRequest(req)
			if err != nil {
				entry.Error = err.Error()
				requestEntries = append(requestEntries, entry)
				groupErr = err
				if failFast {
					return output, warnings, executed, requestEntries, groupErr
				}
				continue
			}
			entry.RequestPreview = built.RequestPreview
			if log != nil {
				_ = log.Write("MODEL_REQUEST_STARTED", map[string]any{
					"task_id":     task.ID,
					"decision_id": decision.ID,
					"task_type":   taskType,
					"provider":    built.Provider,
					"model":       built.Model,
				})
			}
			resp, err := adapter.Execute(built)
			if err != nil {
				entry.Error = err.Error()
				requestEntries = append(requestEntries, entry)
				if log != nil {
					_ = log.Write("MODEL_REQUEST_FAILED", map[string]any{
						"task_id":     task.ID,
						"decision_id": decision.ID,
						"error":       err.Error(),
					})
				}
				groupErr = err
				if failFast {
					return output, warnings, executed, requestEntries, groupErr
				}
				continue
			}
			entry.Status = "completed"
			entry.ResponseMode = resp.Mode
			requestEntries = append(requestEntries, entry)
			if log != nil {
				_ = log.Write("MODEL_REQUEST_COMPLETED", map[string]any{
					"task_id":       task.ID,
					"decision_id":   decision.ID,
					"response_mode": resp.Mode,
				})
			}
			items, itemWarnings := providerResponseToItems(taskType, task, decision, resp, built)
			output.Items = append(output.Items, items...)
			warnings = append(warnings, resp.Warnings...)
			warnings = append(warnings, itemWarnings...)
			executed++
		}
	}
	if len(output.Items) == 0 {
		warnings = append(warnings, fmt.Sprintf("no provider expansion items were generated for task type %q", taskType))
	}
	return output, dedupeStrings(warnings), executed, requestEntries, groupErr
}

func collectTaskIDs(tasks []ExpansionTask) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.ID)
	}
	return out
}

func providerResponseToItems(taskType string, task ExpansionTask, decision MaskDecision, resp modelrouter.Response, req modelrouter.Request) ([]ExpansionOutputItem, []string) {
	var warnings []string
	texts := resp.Texts
	contract := req.Input.OutputContract
	if len(texts) == 0 {
		texts = []string{decision.TextPreview}
		warnings = append(warnings, "provider returned no text; using text preview fallback")
	}
	switch taskType {
	case "caption_variants":
		texts = normalizeProviderTexts(texts, contractInt(contract, "max_items", 3))
		maxWords := contractInt(contract, "max_words", 18)
		items := make([]ExpansionOutputItem, 0, len(texts))
		for index, text := range texts {
			truncatedText, truncated := truncateToWords(strings.TrimSpace(text), maxWords)
			if truncated {
				warnings = append(warnings, fmt.Sprintf("truncated caption variant for decision %s to %d words", decision.ID, maxWords))
			}
			items = append(items, ExpansionOutputItem{
				ID:         fmt.Sprintf("cap_%s_%04d", decision.ID, index+1),
				TaskID:     task.ID,
				DecisionID: decision.ID,
				Text:       truncatedText,
				Start:      decision.Start,
				End:        decision.End,
				Metadata:   providerMetadata(req, resp.Mode, truncated, index+1),
			})
		}
		return items, warnings
	case "timeline_labels":
		text := firstProviderText(texts)
		maxWords := contractInt(contract, "max_words", 8)
		truncatedText, truncated := truncateToWords(strings.TrimSpace(text), maxWords)
		if truncated {
			warnings = append(warnings, fmt.Sprintf("truncated timeline label for decision %s to %d words", decision.ID, maxWords))
		}
		return []ExpansionOutputItem{{
			ID:         fmt.Sprintf("lbl_%s", decision.ID),
			TaskID:     task.ID,
			DecisionID: decision.ID,
			Text:       truncatedText,
			Start:      decision.Start,
			End:        decision.End,
			Metadata:   providerMetadata(req, resp.Mode, truncated, 0),
		}}, warnings
	case "short_descriptions":
		text := firstProviderText(texts)
		maxWords := contractInt(contract, "max_words", 80)
		truncatedText, truncated := truncateToWords(strings.TrimSpace(text), maxWords)
		if truncated {
			warnings = append(warnings, fmt.Sprintf("truncated short description for decision %s to %d words", decision.ID, maxWords))
		}
		return []ExpansionOutputItem{{
			ID:         fmt.Sprintf("desc_%s", decision.ID),
			TaskID:     task.ID,
			DecisionID: decision.ID,
			Text:       truncatedText,
			Start:      decision.Start,
			End:        decision.End,
			Metadata:   providerMetadata(req, resp.Mode, truncated, 0),
		}}, warnings
	default:
		return []ExpansionOutputItem{{
			ID:         fmt.Sprintf("item_%s", decision.ID),
			TaskID:     task.ID,
			DecisionID: decision.ID,
			Text:       strings.TrimSpace(firstProviderText(texts)),
			Start:      decision.Start,
			End:        decision.End,
			Metadata:   providerMetadata(req, resp.Mode, false, 0),
		}}, warnings
	}
}

func providerMetadata(req modelrouter.Request, responseMode string, truncated bool, variant int) map[string]any {
	metadata := map[string]any{
		"provider":      req.Provider,
		"model":         req.Model,
		"model_route":   req.RouteName,
		"model_entry":   req.ModelEntryName,
		"response_mode": responseMode,
		"truncated":     truncated,
	}
	if variant > 0 {
		metadata["variant"] = variant
	}
	return metadata
}

func truncateToWords(s string, n int) (string, bool) {
	if n <= 0 {
		return s, false
	}
	words := strings.Fields(s)
	if len(words) <= n {
		return s, false
	}
	return strings.Join(words[:n], " ") + "...", true
}

func validateExecutedModelRequestsShape(payload map[string]any) []string {
	errs := []string{}
	requireStringValue(&errs, payload, "schema_version", "model_requests.executed.v1")
	requireNonEmptyString(&errs, payload, "run_id")
	requireArray(&errs, payload, "requests")
	requests, ok := payload["requests"].([]any)
	if !ok {
		return errs
	}
	for index, raw := range requests {
		req, ok := raw.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("requests[%d] must be an object", index))
			continue
		}
		requireDecisionString(&errs, req, "task_id", fmt.Sprintf("requests[%d].task_id", index))
		requireDecisionString(&errs, req, "decision_id", fmt.Sprintf("requests[%d].decision_id", index))
		requireDecisionString(&errs, req, "task_type", fmt.Sprintf("requests[%d].task_type", index))
		requireDecisionString(&errs, req, "model_route", fmt.Sprintf("requests[%d].model_route", index))
		requireDecisionString(&errs, req, "provider", fmt.Sprintf("requests[%d].provider", index))
		requireDecisionString(&errs, req, "model", fmt.Sprintf("requests[%d].model", index))
		requireDecisionString(&errs, req, "status", fmt.Sprintf("requests[%d].status", index))
		requireMap(&errs, req, "request_preview")
	}
	return errs
}

func ReviewModelRequests(runID string, stdout io.Writer, opts ReviewModelRequestsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	review := ModelRequestsReview{
		RunID:         runID,
		Providers:     map[string]int{},
		Models:        map[string]int{},
		TaskTypes:     map[string]int{},
		Statuses:      map[string]int{},
		ResponseModes: map[string]int{},
		Failures:      []string{},
	}

	dryRunPath := filepath.Join(runDir, "model_requests.dryrun.json")
	if data, err := os.ReadFile(dryRunPath); err == nil {
		var doc ModelRequestsDryRun
		if json.Unmarshal(data, &doc) == nil {
			review.DryRunArtifact = "model_requests.dryrun.json"
			review.DryRunCount = len(doc.Requests)
			for _, req := range doc.Requests {
				review.Providers[req.Provider]++
				review.Models[req.Model]++
				review.TaskTypes[req.TaskType]++
				review.Statuses[req.Status]++
			}
		}
	}

	executedPath := filepath.Join(runDir, "model_requests.executed.json")
	if data, err := os.ReadFile(executedPath); err == nil {
		var doc ExecutedModelRequests
		if json.Unmarshal(data, &doc) == nil {
			review.ExecutedArtifact = "model_requests.executed.json"
			review.ExecutedCount = len(doc.Requests)
			for _, req := range doc.Requests {
				review.Providers[req.Provider]++
				review.Models[req.Model]++
				review.TaskTypes[req.TaskType]++
				review.Statuses[req.Status]++
				if req.ResponseMode != "" {
					review.ResponseModes[req.ResponseMode]++
				}
				if req.Error != "" {
					review.Failures = append(review.Failures, fmt.Sprintf("%s/%s: %s", req.TaskID, req.DecisionID, req.Error))
				}
			}
		}
	}

	if review.DryRunCount == 0 && review.ExecutedCount == 0 {
		return fmt.Errorf("no model request artifacts found; run expand-dry-run or expand first")
	}

	if opts.WriteArtifact {
		reviewPath := filepath.Join(runDir, "model_requests_review.md")
		if err := writeModelRequestsReviewMarkdown(reviewPath, review); err != nil {
			return err
		}
		review.ReviewArtifact = "model_requests_review.md"
		if err := addManifestArtifact(runDir, "model_requests_review", "model_requests_review.md"); err != nil {
			return err
		}
	}

	if opts.JSON {
		data, _ := json.MarshalIndent(review, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Model request review")
	fmt.Fprintf(stdout, "  run id:         %s\n", runID)
	fmt.Fprintf(stdout, "  dry run count:  %d\n", review.DryRunCount)
	fmt.Fprintf(stdout, "  executed count: %d\n", review.ExecutedCount)
	printStringIntMap(stdout, "providers", review.Providers)
	printStringIntMap(stdout, "models", review.Models)
	printStringIntMap(stdout, "task types", review.TaskTypes)
	printStringIntMap(stdout, "statuses", review.Statuses)
	if len(review.ResponseModes) > 0 {
		printStringIntMap(stdout, "response modes", review.ResponseModes)
	}
	for _, failure := range review.Failures {
		fmt.Fprintf(stdout, "  failure: %s\n", failure)
	}
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "model_requests_review.md"))
	}
	return nil
}

func printStringIntMap(stdout io.Writer, label string, values map[string]int) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(stdout, "  %s:\n", label)
	for key, value := range values {
		fmt.Fprintf(stdout, "    - %s: %d\n", emptyDash(key), value)
	}
}

func writeModelRequestsReviewMarkdown(path string, review ModelRequestsReview) error {
	var builder strings.Builder
	builder.WriteString("# Model Requests Review\n\n")
	builder.WriteString(fmt.Sprintf("- run_id: %s\n", review.RunID))
	builder.WriteString(fmt.Sprintf("- dry_run_count: %d\n", review.DryRunCount))
	builder.WriteString(fmt.Sprintf("- executed_count: %d\n", review.ExecutedCount))
	builder.WriteString("\n## Statuses\n\n")
	for key, value := range review.Statuses {
		builder.WriteString(fmt.Sprintf("- %s: %d\n", key, value))
	}
	if len(review.Failures) > 0 {
		builder.WriteString("\n## Failures\n\n")
		for _, failure := range review.Failures {
			builder.WriteString(fmt.Sprintf("- %s\n", failure))
		}
	}
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}

func normalizeProviderTexts(texts []string, maxItems int) []string {
	out := []string{}
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		var stringList []string
		if err := json.Unmarshal([]byte(text), &stringList); err == nil {
			for _, item := range stringList {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
			continue
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(text), &object); err == nil {
			for _, key := range []string{"items", "variants", "labels", "descriptions"} {
				if values, ok := object[key].([]any); ok {
					for _, raw := range values {
						switch value := raw.(type) {
						case string:
							if strings.TrimSpace(value) != "" {
								out = append(out, strings.TrimSpace(value))
							}
						case map[string]any:
							if textValue, ok := value["text"].(string); ok && strings.TrimSpace(textValue) != "" {
								out = append(out, strings.TrimSpace(textValue))
							}
						}
					}
				}
			}
			if len(out) > 0 {
				continue
			}
		}
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
			if line != "" {
				out = append(out, line)
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	if maxItems > 0 && len(out) > maxItems {
		out = out[:maxItems]
	}
	return out
}

func firstProviderText(texts []string) string {
	normalized := normalizeProviderTexts(texts, 1)
	if len(normalized) == 0 {
		return ""
	}
	return normalized[0]
}

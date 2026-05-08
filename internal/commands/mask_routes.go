package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type RoutesPlanOptions struct {
	JSON          bool
	WriteArtifact bool
	Strict        bool
}

type RouteEntry struct {
	TaskID        string   `json:"task_id"`
	TaskType      string   `json:"task_type"`
	ModelRoute    string   `json:"model_route"`
	ResolvedEntry string   `json:"resolved_entry,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	Model         string   `json:"model,omitempty"`
	Role          string   `json:"role,omitempty"`
	Status        string   `json:"status"`
	Warnings      []string `json:"warnings"`
}

type RoutesPlan struct {
	SchemaVersion string       `json:"schema_version"`
	CreatedAt     time.Time    `json:"created_at"`
	RunID         string       `json:"run_id"`
	ModelsEnabled bool         `json:"models_enabled"`
	Routes        []RouteEntry `json:"routes"`
	Warnings      []string     `json:"warnings"`
}

func RoutesPlanCommand(runID string, stdout io.Writer, opts RoutesPlanOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}

	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("ROUTES_PLAN_STARTED", map[string]any{"run_id": runID})
	}

	cfg := config.Config{}
	if c, loadErr := config.Load(config.DefaultPath); loadErr == nil {
		cfg = c
	}

	routes := collectRoutesForRun(runDir, cfg)
	allWarnings := []string{}
	for _, r := range routes {
		allWarnings = append(allWarnings, r.Warnings...)
	}
	if len(routes) == 0 {
		allWarnings = append(allWarnings, "no expansion_tasks.json or verification.json found; run expansion-plan and verification-plan first")
	}

	plan := RoutesPlan{
		SchemaVersion: "routes_plan.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		ModelsEnabled: cfg.Models.Enabled,
		Routes:        routes,
		Warnings:      dedupeStrings(allWarnings),
	}

	if opts.WriteArtifact {
		path := filepath.Join(runDir, "routes_plan.json")
		if err := writeJSONFile(path, plan); err != nil {
			writeMaskFailure(log, "ROUTES_PLAN_FAILED", err.Error())
			return err
		}
		if err := addManifestArtifact(runDir, "routes_plan", "routes_plan.json"); err != nil {
			writeMaskFailure(log, "ROUTES_PLAN_FAILED", err.Error())
			return err
		}
	}

	if log != nil {
		_ = log.Write("ROUTES_PLAN_COMPLETED", map[string]any{"routes": len(routes), "warnings": len(plan.Warnings)})
	}

	if opts.Strict {
		for _, r := range routes {
			if r.Status == "missing_route" || r.Status == "missing_entry" {
				writeMaskFailure(log, "ROUTES_PLAN_FAILED", "missing routes or entries (strict mode)")
				return fmt.Errorf("routes-plan: missing routes or entries (--strict)")
			}
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

	printRoutesPlan(stdout, plan)
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "routes_plan.json"))
	}
	return nil
}

func collectRoutesForRun(runDir string, cfg config.Config) []RouteEntry {
	routes := []RouteEntry{}

	var tasks ExpansionTasks
	if data, err := os.ReadFile(filepath.Join(runDir, "expansion_tasks.json")); err == nil {
		if json.Unmarshal(data, &tasks) == nil {
			for _, task := range tasks.Tasks {
				routes = append(routes, resolveRouteEntry(task.ID, task.Type, task.ModelRoute, cfg))
			}
		}
	}

	var verPlan VerificationPlan
	if data, err := os.ReadFile(filepath.Join(runDir, "verification.json")); err == nil {
		if json.Unmarshal(data, &verPlan) == nil && len(verPlan.Checks) > 0 {
			routes = append(routes, resolveRouteEntry("verify_0001", "verification", "verification", cfg))
		}
	}

	return routes
}

func resolveRouteEntry(taskID, taskType, modelRoute string, cfg config.Config) RouteEntry {
	entry := RouteEntry{
		TaskID:     taskID,
		TaskType:   taskType,
		ModelRoute: modelRoute,
		Warnings:   []string{},
	}

	entryName, ok := cfg.Models.Routes[modelRoute]
	if !ok {
		entry.Status = "missing_route"
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("route %q is not configured in models.routes", modelRoute))
		return entry
	}

	modelEntry, ok := cfg.Models.Entries[entryName]
	if !ok {
		entry.ResolvedEntry = entryName
		entry.Status = "missing_entry"
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("model entry %q is not configured in models.entries", entryName))
		return entry
	}

	entry.ResolvedEntry = entryName
	entry.Provider = modelEntry.Provider
	entry.Model = modelEntry.Model
	entry.Role = modelEntry.Role

	if !cfg.Models.Enabled {
		entry.Status = "models_disabled"
	} else {
		entry.Status = "configured"
	}

	return entry
}

func printRoutesPlan(stdout io.Writer, plan RoutesPlan) {
	fmt.Fprintln(stdout, "Routes plan")
	fmt.Fprintf(stdout, "  run id:         %s\n", plan.RunID)
	fmt.Fprintf(stdout, "  models enabled: %v\n", plan.ModelsEnabled)
	fmt.Fprintf(stdout, "  routes:         %d\n", len(plan.Routes))
	for _, r := range plan.Routes {
		fmt.Fprintf(stdout, "  - task:     %s (%s)\n", r.TaskID, r.TaskType)
		fmt.Fprintf(stdout, "    route:    %s\n", r.ModelRoute)
		if r.ResolvedEntry != "" {
			fmt.Fprintf(stdout, "    entry:    %s\n", r.ResolvedEntry)
		}
		if r.Provider != "" {
			fmt.Fprintf(stdout, "    provider: %s\n", r.Provider)
		}
		if r.Model != "" {
			fmt.Fprintf(stdout, "    model:    %s\n", r.Model)
		}
		if r.Role != "" {
			fmt.Fprintf(stdout, "    role:     %s\n", r.Role)
		}
		fmt.Fprintf(stdout, "    status:   %s\n", r.Status)
		for _, w := range r.Warnings {
			fmt.Fprintf(stdout, "    warning:  %s\n", w)
		}
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

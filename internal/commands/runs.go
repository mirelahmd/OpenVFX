package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"byom-video/internal/runinfo"
)

type RunsOptions struct {
	Limit int
	All   bool
}

type InspectOptions struct {
	JSON bool
}

type ArtifactsOptions struct {
	Type string
}

func Runs(stdout io.Writer, opts RunsOptions) error {
	rows, err := runinfo.ListRuns(runinfo.RunListOptions{Limit: opts.Limit, All: opts.All})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No runs found. Run `byom-video pipeline <input-file> --preset shorts` first.")
		return nil
	}
	fmt.Fprintf(stdout, "%-28s %-10s %-25s %-24s %-9s %-12s\n", "RUN ID", "STATUS", "CREATED AT", "INPUT", "ARTIFACTS", "EXPORT")
	for _, row := range rows {
		fmt.Fprintf(stdout, "%-28s %-10s %-25s %-24s %-9d %-12s\n",
			row.RunID,
			row.Status,
			timeDisplay(row.CreatedAt),
			truncate(row.InputBasename, 24),
			row.ArtifactCount,
			emptyDisplay(row.ExportStatus),
		)
	}
	return nil
}

func Inspect(runID string, stdout io.Writer, opts InspectOptions) error {
	summary, err := runinfo.Inspect(runID)
	if err != nil {
		return err
	}
	if opts.JSON {
		encoded, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("encode inspect summary: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
		return nil
	}
	fmt.Fprintln(stdout, "Run inspection")
	fmt.Fprintf(stdout, "  run id:      %s\n", summary.RunID)
	fmt.Fprintf(stdout, "  status:      %s\n", summary.Status)
	fmt.Fprintf(stdout, "  input path:  %s\n", summary.InputPath)
	fmt.Fprintf(stdout, "  created at:  %s\n", summary.CreatedAt)
	if summary.ErrorMessage != "" {
		fmt.Fprintf(stdout, "  error:       %s\n", summary.ErrorMessage)
	}
	if summary.ExportStatus != "" {
		fmt.Fprintf(stdout, "  export:      %s\n", summary.ExportStatus)
	}
	if summary.ExportValidationStatus != "" {
		fmt.Fprintf(stdout, "  validation:  %s\n", summary.ExportValidationStatus)
	}
	if summary.ReportPath != "" {
		fmt.Fprintf(stdout, "  report path: %s\n", summary.ReportPath)
	}
	fmt.Fprintln(stdout, "  counts:")
	printOptionalCount(stdout, "transcript segments", summary.TranscriptSegmentCount)
	printOptionalCount(stdout, "chunks", summary.ChunkCount)
	printOptionalCount(stdout, "highlights", summary.HighlightCount)
	printOptionalCount(stdout, "roughcut clips", summary.RoughcutClipCount)
	printOptionalCount(stdout, "clip cards", summary.ClipCardCount)
	printOptionalCount(stdout, "enhanced roughcut", summary.EnhancedRoughcutCount)
	printOptionalCount(stdout, "selected clips", summary.SelectedClipCount)
	fmt.Fprintf(stdout, "    exported files:      %d\n", summary.ExportedFileCount)
	if summary.ExportManifestSummary != nil {
		fmt.Fprintf(stdout, "    export manifest:     planned=%d exported=%d validated=%d missing=%d\n",
			summary.ExportManifestSummary.Planned,
			summary.ExportManifestSummary.Exported,
			summary.ExportManifestSummary.Validated,
			summary.ExportManifestSummary.Missing,
		)
	}
	if summary.ConcatPlanPresent {
		fmt.Fprintln(stdout, "    concat plan:         present")
	}
	fmt.Fprintln(stdout, "  artifacts:")
	for _, artifact := range summary.Artifacts {
		fmt.Fprintf(stdout, "    - %s: %s\n", artifact.Name, artifact.Path)
	}
	if len(summary.ExportedFiles) > 0 {
		fmt.Fprintln(stdout, "  exported files:")
		for _, file := range summary.ExportedFiles {
			fmt.Fprintf(stdout, "    - %s\n", file)
		}
	}
	if len(summary.Warnings) > 0 {
		fmt.Fprintln(stdout, "  warnings:")
		for _, warning := range summary.Warnings {
			fmt.Fprintf(stdout, "    - %s\n", warning)
		}
	}
	return nil
}

func Artifacts(runID string, stdout io.Writer, opts ArtifactsOptions) error {
	paths, err := runinfo.ArtifactPaths(runID, opts.Type)
	if err != nil {
		return err
	}
	for _, path := range paths {
		fmt.Fprintln(stdout, path)
	}
	return nil
}

func timeDisplay(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format(time.RFC3339)
}

func emptyDisplay(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func truncate(value string, limit int) string {
	if value == "" {
		return "-"
	}
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func printOptionalCount(stdout io.Writer, label string, value *int) {
	if value == nil {
		fmt.Fprintf(stdout, "    %-20s unknown\n", label+":")
		return
	}
	fmt.Fprintf(stdout, "    %-20s %d\n", label+":", *value)
}

func ReportPath(runID string) (string, error) {
	paths, err := runinfo.ArtifactPaths(runID, "report")
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("report artifact is missing for run %s", runID)
	}
	return filepath.Clean(paths[0]), nil
}

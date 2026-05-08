package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mirelahmd/byom-video/internal/exporter"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/report"
)

func Export(runID string, stdout io.Writer) error {
	summary, err := exporter.Run(runID, stdout)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(filepath.Join(summary.RunDir, "report.html")); statErr == nil {
		m, readErr := manifest.Read(filepath.Join(summary.RunDir, "manifest.json"))
		if readErr == nil {
			_, _ = report.Write(summary.RunDir, m)
		}
	}
	fmt.Fprintln(stdout, "Export completed")
	fmt.Fprintf(stdout, "  run id:         %s\n", summary.RunID)
	fmt.Fprintf(stdout, "  run directory:  %s\n", summary.RunDir)
	fmt.Fprintf(stdout, "  script path:    %s\n", summary.ScriptPath)
	fmt.Fprintf(stdout, "  exports dir:    %s\n", summary.ExportsDir)
	fmt.Fprintf(stdout, "  exported files: %d\n", len(summary.ExportedFiles))
	for _, file := range summary.ExportedFiles {
		fmt.Fprintf(stdout, "    - %s\n", file)
	}
	if summary.ExportValidationStatus != "" {
		fmt.Fprintf(stdout, "  validation:     %s\n", summary.ExportValidationStatus)
	}
	if summary.ExportValidationError != "" {
		fmt.Fprintf(stdout, "  validation error: %s\n", summary.ExportValidationError)
	}
	return nil
}

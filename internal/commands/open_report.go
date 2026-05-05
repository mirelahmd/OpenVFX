package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"byom-video/internal/runstore"
)

func OpenReport(runID string, stdout io.Writer, open bool) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	reportPath := filepath.Join(runDir, "report.html")
	if info, err := os.Stat(reportPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("report artifact is missing: %s", reportPath)
		}
		return fmt.Errorf("stat report artifact: %w", err)
	} else if info.IsDir() {
		return fmt.Errorf("report path is a directory: %s", reportPath)
	}
	fmt.Fprintln(stdout, "Report path")
	fmt.Fprintf(stdout, "  run id:      %s\n", runID)
	fmt.Fprintf(stdout, "  report path: %s\n", reportPath)
	if open {
		command, args, ok := OpenReportCommand(reportPath, runtime.GOOS)
		if !ok {
			fmt.Fprintf(stdout, "  open:        unsupported on %s\n", runtime.GOOS)
			return nil
		}
		if err := exec.Command(command, args...).Start(); err != nil {
			fmt.Fprintf(stdout, "  open:        failed: %v\n", err)
			return nil
		}
		fmt.Fprintln(stdout, "  open:        attempted")
	}
	return nil
}

func OpenReportCommand(path string, goos string) (string, []string, bool) {
	switch goos {
	case "darwin":
		return "open", []string{path}, true
	case "linux":
		return "xdg-open", []string{path}, true
	case "windows":
		return "cmd", []string{"/c", "start", "", path}, true
	default:
		return "", nil, false
	}
}

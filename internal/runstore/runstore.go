package runstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const RunsRoot = ".byom-video/runs"

func ResolveRunDir(runID string) (string, error) {
	if runID == "" {
		return "", fmt.Errorf("run id is required")
	}
	if runID != filepath.Base(runID) || strings.Contains(runID, "..") {
		return "", fmt.Errorf("invalid run id %q", runID)
	}
	rootAbs, err := filepath.Abs(RunsRoot)
	if err != nil {
		return "", fmt.Errorf("resolve runs root: %w", err)
	}
	runAbs := filepath.Join(rootAbs, runID)
	rel, err := filepath.Rel(rootAbs, runAbs)
	if err != nil {
		return "", fmt.Errorf("resolve run path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("run path escapes runs root")
	}
	return filepath.Join(RunsRoot, runID), nil
}

func RequireRunDir(runID string) (string, error) {
	runDir, err := ResolveRunDir(runID)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(runDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("run directory does not exist: %s", runDir)
		}
		return "", fmt.Errorf("stat run directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("run path is not a directory: %s", runDir)
	}
	return runDir, nil
}

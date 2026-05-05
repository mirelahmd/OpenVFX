package cleanup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"byom-video/internal/manifest"
	"byom-video/internal/runstore"
)

type Options struct {
	Failed          bool
	StaleRunning    bool
	MissingManifest bool
	OlderThan       time.Duration
	Limit           int
	Now             time.Time
}

type Candidate struct {
	RunID  string `json:"run_id"`
	RunDir string `json:"run_dir"`
	Reason string `json:"reason"`
	Status string `json:"status,omitempty"`
}

func FindCandidates(opts Options) ([]Candidate, error) {
	if !opts.Failed && !opts.StaleRunning && !opts.MissingManifest {
		opts.Failed = true
		opts.StaleRunning = true
		opts.MissingManifest = true
	}
	if opts.OlderThan <= 0 {
		opts.OlderThan = 24 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	entries, err := os.ReadDir(runstore.RunsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []Candidate{}, nil
		}
		return nil, fmt.Errorf("read runs directory: %w", err)
	}
	candidates := []Candidate{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		runDir, err := runstore.ResolveRunDir(runID)
		if err != nil {
			continue
		}
		m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && opts.MissingManifest {
				candidates = append(candidates, Candidate{RunID: runID, RunDir: runDir, Reason: "missing-manifest"})
			}
			continue
		}
		switch {
		case opts.Failed && m.Status == manifest.StatusFailed:
			candidates = append(candidates, Candidate{RunID: runID, RunDir: runDir, Reason: "failed", Status: m.Status})
		case opts.StaleRunning && m.Status == manifest.StatusRunning && opts.Now.Sub(m.CreatedAt) >= opts.OlderThan:
			candidates = append(candidates, Candidate{RunID: runID, RunDir: runDir, Reason: "stale-running", Status: m.Status})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RunID < candidates[j].RunID
	})
	if opts.Limit > 0 && len(candidates) > opts.Limit {
		candidates = candidates[:opts.Limit]
	}
	return candidates, nil
}

func DeleteCandidate(candidate Candidate) error {
	runDir, err := runstore.ResolveRunDir(candidate.RunID)
	if err != nil {
		return err
	}
	if filepath.Clean(runDir) != filepath.Clean(candidate.RunDir) {
		return fmt.Errorf("candidate run dir mismatch for %s", candidate.RunID)
	}
	rootAbs, err := filepath.Abs(runstore.RunsRoot)
	if err != nil {
		return err
	}
	runAbs, err := filepath.Abs(runDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, runAbs)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to delete path outside runs root: %s", candidate.RunDir)
	}
	if err := os.RemoveAll(runDir); err != nil {
		return fmt.Errorf("delete run directory: %w", err)
	}
	return nil
}

func MarshalCandidates(candidates []Candidate) ([]byte, error) {
	data, err := json.MarshalIndent(candidates, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

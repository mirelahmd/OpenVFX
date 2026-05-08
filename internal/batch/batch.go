package batch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/runctx"
)

type Options struct {
	InputDir          string
	Preset            string
	Recursive         bool
	Limit             int
	FailFast          bool
	DryRun            bool
	Validate          bool
	Export            bool
	ExportAndValidate bool
}

type Runner func(inputPath string, stdout io.Writer) error
type Exporter func(runID string, stdout io.Writer) error
type Validator func(runID string, stdout io.Writer) error

type Hooks struct {
	Run      Runner
	Export   Exporter
	Validate Validator
	Now      func() time.Time
}

type Summary struct {
	SchemaVersion string    `json:"schema_version"`
	BatchID       string    `json:"batch_id"`
	CreatedAt     time.Time `json:"created_at"`
	InputDir      string    `json:"input_dir"`
	Preset        string    `json:"preset"`
	Recursive     bool      `json:"recursive"`
	DryRun        bool      `json:"dry_run"`
	Totals        Totals    `json:"totals"`
	Items         []Item    `json:"items"`
}

type Totals struct {
	Discovered int `json:"discovered"`
	Attempted  int `json:"attempted"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

type Item struct {
	InputPath string `json:"input_path"`
	Status    string `json:"status"`
	RunID     string `json:"run_id,omitempty"`
	RunDir    string `json:"run_dir,omitempty"`
	Error     string `json:"error"`
}

func DiscoverMediaFiles(inputDir string, recursive bool) ([]string, error) {
	root, err := filepath.Abs(inputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve input directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat input directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("input path is not a directory: %s", root)
	}
	files := []string{}
	if recursive {
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() || !IsMediaFile(entry.Name()) {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan input directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("read input directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !IsMediaFile(entry.Name()) {
				continue
			}
			files = append(files, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func IsMediaFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mov", ".m4v", ".mp3", ".wav", ".m4a", ".aac", ".flac", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func Run(opts Options, hooks Hooks, stdout io.Writer) (Summary, error) {
	if opts.Preset == "" {
		opts.Preset = "shorts"
	}
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if opts.Limit < 0 {
		return Summary{}, fmt.Errorf("--limit must be positive")
	}
	if hooks.Run == nil {
		return Summary{}, fmt.Errorf("batch run hook is required")
	}
	now := time.Now
	if hooks.Now != nil {
		now = hooks.Now
	}
	createdAt := now().UTC()
	batchID, err := runctx.NewRunID(createdAt)
	if err != nil {
		return Summary{}, err
	}
	inputDirAbs, err := filepath.Abs(opts.InputDir)
	if err != nil {
		return Summary{}, fmt.Errorf("resolve input directory: %w", err)
	}
	files, err := DiscoverMediaFiles(opts.InputDir, opts.Recursive)
	if err != nil {
		return Summary{}, err
	}
	discovered := len(files)
	if opts.Limit > 0 && len(files) > opts.Limit {
		files = files[:opts.Limit]
	}
	summary := Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       batchID,
		CreatedAt:     createdAt,
		InputDir:      inputDirAbs,
		Preset:        opts.Preset,
		Recursive:     opts.Recursive,
		DryRun:        opts.DryRun,
		Totals:        Totals{Discovered: discovered, Skipped: discovered - len(files)},
		Items:         []Item{},
	}
	if opts.DryRun {
		for _, file := range files {
			summary.Items = append(summary.Items, Item{InputPath: file, Status: "dry_run"})
		}
		PrintSummary(stdout, summary)
		return summary, nil
	}
	for _, file := range files {
		item := Item{InputPath: file, Status: "completed"}
		summary.Totals.Attempted++
		var runOut bytes.Buffer
		err := hooks.Run(file, &runOut)
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			summary.Totals.Failed++
			summary.Items = append(summary.Items, item)
			if opts.FailFast {
				break
			}
			continue
		}
		item.RunID = ParseRunID(runOut.String())
		if item.RunID != "" {
			item.RunDir = filepath.Join(".byom-video", "runs", item.RunID)
		}
		if opts.Export && hooks.Export != nil && item.RunID != "" {
			if err := hooks.Export(item.RunID, stdout); err != nil {
				item.Status = "failed"
				item.Error = err.Error()
				summary.Totals.Failed++
				summary.Items = append(summary.Items, item)
				if opts.FailFast {
					break
				}
				continue
			}
		}
		if opts.Validate && hooks.Validate != nil && item.RunID != "" {
			if err := hooks.Validate(item.RunID, stdout); err != nil {
				item.Status = "failed"
				item.Error = err.Error()
				summary.Totals.Failed++
				summary.Items = append(summary.Items, item)
				if opts.FailFast {
					break
				}
				continue
			}
		}
		summary.Totals.Succeeded++
		summary.Items = append(summary.Items, item)
	}
	if err := WriteSummary(summary); err != nil {
		return summary, err
	}
	PrintSummary(stdout, summary)
	return summary, nil
}

func ParseRunID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "run id:"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func WriteSummary(summary Summary) error {
	dir := filepath.Join(".byom-video", "batches", summary.BatchID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create batch directory: %w", err)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("encode batch summary: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "batch_summary.json"), data, 0o644); err != nil {
		return fmt.Errorf("write batch summary: %w", err)
	}
	return nil
}

func ReadSummary(batchID string) (Summary, error) {
	if batchID == "" || batchID != filepath.Base(batchID) || strings.Contains(batchID, "..") {
		return Summary{}, fmt.Errorf("invalid batch id %q", batchID)
	}
	path := filepath.Join(".byom-video", "batches", batchID, "batch_summary.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read batch summary: %w", err)
	}
	var summary Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		return Summary{}, fmt.Errorf("decode batch summary: %w", err)
	}
	return summary, nil
}

func ListSummaries() ([]Summary, error) {
	root := filepath.Join(".byom-video", "batches")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Summary{}, nil
		}
		return nil, fmt.Errorf("read batches directory: %w", err)
	}
	summaries := []Summary{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		summary, err := ReadSummary(entry.Name())
		if err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})
	return summaries, nil
}

func PrintSummary(stdout io.Writer, summary Summary) {
	fmt.Fprintln(stdout, "Batch completed")
	fmt.Fprintf(stdout, "  batch id:    %s\n", summary.BatchID)
	fmt.Fprintf(stdout, "  input dir:   %s\n", summary.InputDir)
	fmt.Fprintf(stdout, "  preset:      %s\n", summary.Preset)
	fmt.Fprintf(stdout, "  discovered:  %d\n", summary.Totals.Discovered)
	fmt.Fprintf(stdout, "  attempted:   %d\n", summary.Totals.Attempted)
	fmt.Fprintf(stdout, "  succeeded:   %d\n", summary.Totals.Succeeded)
	fmt.Fprintf(stdout, "  failed:      %d\n", summary.Totals.Failed)
	fmt.Fprintf(stdout, "  skipped:     %d\n", summary.Totals.Skipped)
	if len(summary.Items) > 0 {
		fmt.Fprintln(stdout, "  items:")
		for _, item := range summary.Items {
			if item.RunID != "" {
				fmt.Fprintf(stdout, "    - %s: %s (%s)\n", item.Status, item.InputPath, item.RunID)
			} else if item.Error != "" {
				fmt.Fprintf(stdout, "    - %s: %s (%s)\n", item.Status, item.InputPath, item.Error)
			} else {
				fmt.Fprintf(stdout, "    - %s: %s\n", item.Status, item.InputPath)
			}
		}
	}
}

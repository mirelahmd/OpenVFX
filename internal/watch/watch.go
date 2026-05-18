package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/batch"
)

const RegistryPath = ".byom-video/watch/processed.json"

type Options struct {
	InputDir          string
	Preset            string
	Recursive         bool
	Once              bool
	Limit             int
	FailFast          bool
	Interval          time.Duration
	Validate          bool
	Export            bool
	ExportAndValidate bool
	IgnoreRegistry    bool
}

type Runner func(inputPath string, stdout io.Writer) error
type Exporter func(runID string, stdout io.Writer) error
type Validator func(runID string, stdout io.Writer) error

type Hooks struct {
	Run      Runner
	Export   Exporter
	Validate Validator
	Now      func() time.Time
	Sleep    func(context.Context, time.Duration) error
}

type Registry struct {
	SchemaVersion string         `json:"schema_version"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Items         []RegistryItem `json:"items"`
}

type RegistryItem struct {
	InputPath   string    `json:"input_path"`
	Fingerprint string    `json:"fingerprint"`
	ProcessedAt time.Time `json:"processed_at"`
	RunID       string    `json:"run_id"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
}

type FileState struct {
	Path    string
	Size    int64
	ModTime time.Time
}

func Run(ctx context.Context, opts Options, hooks Hooks, stdout io.Writer) error {
	if opts.Preset == "" {
		opts.Preset = "shorts"
	}
	if opts.Interval <= 0 {
		return fmt.Errorf("--interval-seconds must be positive")
	}
	if opts.Limit < 0 {
		return fmt.Errorf("--limit must be positive")
	}
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if hooks.Run == nil {
		return fmt.Errorf("watch run hook is required")
	}
	now := time.Now
	if hooks.Now != nil {
		now = hooks.Now
	}
	sleep := defaultSleep
	if hooks.Sleep != nil {
		sleep = hooks.Sleep
	}
	registry, err := LoadRegistry()
	if err != nil {
		return err
	}
	seen := map[string]FileState{}
	processedThisInvocation := 0
	fmt.Fprintf(stdout, "Watching %s with preset %s\n", opts.InputDir, opts.Preset)
	for {
		processed, err := scanAndProcess(ctx, opts, hooks, stdout, &registry, seen, now, &processedThisInvocation)
		if err != nil {
			return err
		}
		if opts.Once {
			if processed == 0 {
				fmt.Fprintln(stdout, "Watch scan completed; no new stable files processed")
			}
			return nil
		}
		if opts.Limit > 0 && processedThisInvocation >= opts.Limit {
			fmt.Fprintf(stdout, "Watch limit reached: %d\n", opts.Limit)
			return nil
		}
		if err := sleep(ctx, opts.Interval); err != nil {
			fmt.Fprintln(stdout, "Watch stopped")
			return nil
		}
	}
}

func scanAndProcess(ctx context.Context, opts Options, hooks Hooks, stdout io.Writer, registry *Registry, seen map[string]FileState, now func() time.Time, processedThisInvocation *int) (int, error) {
	files, err := batch.DiscoverMediaFiles(opts.InputDir, opts.Recursive)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return processed, nil
		}
		if opts.Limit > 0 && *processedThisInvocation >= opts.Limit {
			return processed, nil
		}
		state, err := InspectFile(file)
		if err != nil {
			fmt.Fprintf(stdout, "Skipping unreadable file: %s (%v)\n", file, err)
			continue
		}
		fingerprint := Fingerprint(state)
		if !opts.IgnoreRegistry && registry.Has(fingerprint) {
			continue
		}
		prev, hadPrev := seen[file]
		if !IsStable(prev, state, hadPrev, opts.Interval, now()) {
			seen[file] = state
			fmt.Fprintf(stdout, "Seen but not stable yet: %s\n", file)
			continue
		}
		fmt.Fprintf(stdout, "Processing: %s\n", file)
		item := RegistryItem{
			InputPath:   file,
			Fingerprint: fingerprint,
			ProcessedAt: now().UTC(),
			Status:      "completed",
		}
		var runOut bytes.Buffer
		err = hooks.Run(file, &runOut)
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			registry.Upsert(item, now())
			if writeErr := SaveRegistry(registry); writeErr != nil {
				return processed, writeErr
			}
			fmt.Fprintf(stdout, "Failed: %s (%s)\n", file, err.Error())
			(*processedThisInvocation)++
			processed++
			if opts.FailFast {
				return processed, nil
			}
			continue
		}
		item.RunID = batch.ParseRunID(runOut.String())
		if opts.Export && hooks.Export != nil && item.RunID != "" {
			if err := hooks.Export(item.RunID, stdout); err != nil {
				item.Status = "failed"
				item.Error = err.Error()
			}
		}
		if item.Status == "completed" && opts.Validate && hooks.Validate != nil && item.RunID != "" {
			if err := hooks.Validate(item.RunID, stdout); err != nil {
				item.Status = "failed"
				item.Error = err.Error()
			}
		}
		registry.Upsert(item, now())
		if err := SaveRegistry(registry); err != nil {
			return processed, err
		}
		if item.Status == "completed" {
			fmt.Fprintf(stdout, "Completed: %s", file)
			if item.RunID != "" {
				fmt.Fprintf(stdout, " (%s)", item.RunID)
			}
			fmt.Fprintln(stdout)
		} else {
			fmt.Fprintf(stdout, "Failed: %s (%s)\n", file, item.Error)
			if opts.FailFast {
				(*processedThisInvocation)++
				processed++
				return processed, nil
			}
		}
		(*processedThisInvocation)++
		processed++
	}
	return processed, nil
}

func InspectFile(path string) (FileState, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return FileState{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return FileState{}, err
	}
	if info.IsDir() {
		return FileState{}, fmt.Errorf("path is a directory")
	}
	return FileState{Path: abs, Size: info.Size(), ModTime: info.ModTime().UTC()}, nil
}

func Fingerprint(state FileState) string {
	return fmt.Sprintf("%s:%d:%d", state.Path, state.Size, state.ModTime.UnixNano())
}

func IsStable(previous FileState, current FileState, hadPrevious bool, interval time.Duration, now time.Time) bool {
	if hadPrevious {
		return previous.Size == current.Size && previous.ModTime.Equal(current.ModTime)
	}
	return now.Sub(current.ModTime) >= interval
}

func LoadRegistry() (Registry, error) {
	data, err := os.ReadFile(RegistryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Registry{SchemaVersion: "watch_processed.v1", Items: []RegistryItem{}}, nil
		}
		return Registry{}, fmt.Errorf("read watch registry: %w", err)
	}
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return Registry{}, fmt.Errorf("decode watch registry: %w", err)
	}
	if registry.SchemaVersion == "" {
		registry.SchemaVersion = "watch_processed.v1"
	}
	if registry.Items == nil {
		registry.Items = []RegistryItem{}
	}
	return registry, nil
}

func SaveRegistry(registry *Registry) error {
	if registry.SchemaVersion == "" {
		registry.SchemaVersion = "watch_processed.v1"
	}
	if err := os.MkdirAll(filepath.Dir(RegistryPath), 0o755); err != nil {
		return fmt.Errorf("create watch directory: %w", err)
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode watch registry: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(RegistryPath, data, 0o644); err != nil {
		return fmt.Errorf("write watch registry: %w", err)
	}
	return nil
}

func (r *Registry) Has(fingerprint string) bool {
	for _, item := range r.Items {
		if item.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func (r *Registry) Upsert(item RegistryItem, now time.Time) {
	r.UpdatedAt = now.UTC()
	for index, existing := range r.Items {
		if existing.Fingerprint == item.Fingerprint {
			r.Items[index] = item
			return
		}
	}
	r.Items = append(r.Items, item)
	sort.Slice(r.Items, func(i, j int) bool {
		return r.Items[i].ProcessedAt.Before(r.Items[j].ProcessedAt)
	})
}

func Status() (Registry, error) {
	return LoadRegistry()
}

func PrintStatus(stdout io.Writer, registry Registry) {
	total := len(registry.Items)
	completed := 0
	failed := 0
	for _, item := range registry.Items {
		switch item.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	fmt.Fprintln(stdout, "Watch status")
	fmt.Fprintf(stdout, "  total processed: %d\n", total)
	fmt.Fprintf(stdout, "  completed:       %d\n", completed)
	fmt.Fprintf(stdout, "  failed:          %d\n", failed)
	if total == 0 {
		return
	}
	fmt.Fprintln(stdout, "  latest:")
	start := total - 10
	if start < 0 {
		start = 0
	}
	for i := total - 1; i >= start; i-- {
		item := registry.Items[i]
		parts := []string{item.Status, item.InputPath}
		if item.RunID != "" {
			parts = append(parts, item.RunID)
		}
		if item.Error != "" {
			parts = append(parts, item.Error)
		}
		fmt.Fprintf(stdout, "    - %s\n", strings.Join(parts, " | "))
	}
}

func defaultSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/batch"
	"github.com/mirelahmd/byom-video/internal/cleanup"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/runctx"
	"github.com/mirelahmd/byom-video/internal/runstore"
	"github.com/mirelahmd/byom-video/internal/watch"
)

type RetryBatchOptions struct {
	Limit             int
	FailFast          bool
	DryRun            bool
	Export            bool
	Validate          bool
	ExportAndValidate bool
}

type RetryWatchOptions struct {
	Preset            string
	RunOptions        RunOptions
	Limit             int
	FailFast          bool
	DryRun            bool
	Export            bool
	Validate          bool
	ExportAndValidate bool
}

type RerunOptions struct {
	Preset            string
	RunOptions        RunOptions
	PresetOverride    bool
	DryRun            bool
	Export            bool
	Validate          bool
	ExportAndValidate bool
}

type CleanupOptions struct {
	Failed          bool
	StaleRunning    bool
	MissingManifest bool
	OlderThanHours  int
	Delete          bool
	Limit           int
	JSON            bool
	Yes             bool
	ConfirmInput    io.Reader
}

func RetryBatch(batchID string, stdout io.Writer, opts RetryBatchOptions) error {
	original, err := batch.ReadSummary(batchID)
	if err != nil {
		return err
	}
	failed := failedBatchItems(original.Items, opts.Limit)
	if opts.DryRun {
		fmt.Fprintln(stdout, "Retry batch dry run")
		fmt.Fprintf(stdout, "  original batch: %s\n", original.BatchID)
		fmt.Fprintf(stdout, "  failed items:   %d\n", len(failed))
		for _, item := range failed {
			fmt.Fprintf(stdout, "    - %s\n", item.InputPath)
		}
		return nil
	}
	runOpts := presetOptions(original.Preset)
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if opts.Export && !runOpts.WithFFmpegScript {
		return fmt.Errorf("--export requires a preset that generates ffmpeg_commands.sh")
	}
	summary := batch.Summary{
		SchemaVersion: "batch_summary.v1",
		BatchID:       retryBatchID(),
		CreatedAt:     time.Now().UTC(),
		InputDir:      original.InputDir,
		Preset:        original.Preset,
		Recursive:     original.Recursive,
		Totals:        batch.Totals{Discovered: len(failed)},
		Items:         []batch.Item{},
	}
	for _, old := range failed {
		item := retryInput(old.InputPath, runOpts, opts.Export, opts.Validate, stdout)
		summary.Totals.Attempted++
		if item.Status == "completed" {
			summary.Totals.Succeeded++
		} else {
			summary.Totals.Failed++
		}
		summary.Items = append(summary.Items, item)
		fmt.Fprintf(stdout, "%s -> %s", old.InputPath, item.Status)
		if item.RunID != "" {
			fmt.Fprintf(stdout, " (%s)", item.RunID)
		}
		if item.Error != "" {
			fmt.Fprintf(stdout, " error=%s", item.Error)
		}
		fmt.Fprintln(stdout)
		if opts.FailFast && item.Status == "failed" {
			break
		}
	}
	if err := batch.WriteSummary(summary); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Retry batch summary: .byom-video/batches/%s/batch_summary.json\n", summary.BatchID)
	return nil
}

func RetryWatch(stdout io.Writer, opts RetryWatchOptions) error {
	if opts.Preset == "" {
		opts.Preset = "shorts"
	}
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if opts.Export && !opts.RunOptions.WithFFmpegScript {
		return fmt.Errorf("--export requires a preset that generates ffmpeg_commands.sh")
	}
	registry, err := watch.LoadRegistry()
	if err != nil {
		return err
	}
	failed := failedWatchItems(registry.Items, opts.Limit)
	if opts.DryRun {
		fmt.Fprintln(stdout, "Retry watch dry run")
		fmt.Fprintf(stdout, "  failed items: %d\n", len(failed))
		for _, item := range failed {
			fmt.Fprintf(stdout, "    - %s\n", item.InputPath)
		}
		return nil
	}
	for _, old := range failed {
		item := watch.RegistryItem{
			InputPath:   old.InputPath,
			Fingerprint: old.Fingerprint,
			ProcessedAt: time.Now().UTC(),
			Status:      "completed",
		}
		if _, err := os.Stat(old.InputPath); err != nil {
			item.Status = "failed"
			item.Error = fmt.Sprintf("input file unavailable: %v", err)
		} else {
			retried := retryInput(old.InputPath, opts.RunOptions, opts.Export, opts.Validate, stdout)
			item.RunID = retried.RunID
			item.Status = retried.Status
			item.Error = retried.Error
			if state, err := watch.InspectFile(old.InputPath); err == nil {
				item.Fingerprint = watch.Fingerprint(state)
			}
		}
		registry.Upsert(item, time.Now().UTC())
		if err := watch.SaveRegistry(&registry); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s -> %s", old.InputPath, item.Status)
		if item.RunID != "" {
			fmt.Fprintf(stdout, " (%s)", item.RunID)
		}
		if item.Error != "" {
			fmt.Fprintf(stdout, " error=%s", item.Error)
		}
		fmt.Fprintln(stdout)
		if opts.FailFast && item.Status == "failed" {
			break
		}
	}
	return nil
}

func Rerun(runID string, stdout io.Writer, opts RerunOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		return err
	}
	preset := opts.Preset
	if preset == "" {
		preset = InferPreset(m)
	}
	runOpts := opts.RunOptions
	if !opts.PresetOverride {
		runOpts = presetOptions(preset)
	}
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if opts.Export && !runOpts.WithFFmpegScript {
		return fmt.Errorf("--export requires a preset that generates ffmpeg_commands.sh")
	}
	if opts.DryRun {
		fmt.Fprintln(stdout, "Rerun dry run")
		fmt.Fprintf(stdout, "  old run id: %s\n", runID)
		fmt.Fprintf(stdout, "  input:      %s\n", m.InputPath)
		fmt.Fprintf(stdout, "  preset:     %s\n", preset)
		return nil
	}
	item := retryInput(m.InputPath, runOpts, opts.Export, opts.Validate, stdout)
	if item.Status == "failed" {
		return fmt.Errorf("rerun failed: %s", item.Error)
	}
	fmt.Fprintf(stdout, "old run id: %s\n", runID)
	fmt.Fprintf(stdout, "new run id: %s\n", item.RunID)
	return nil
}

func Cleanup(stdout io.Writer, opts CleanupOptions) error {
	older := time.Duration(opts.OlderThanHours) * time.Hour
	if opts.OlderThanHours == 0 {
		older = 24 * time.Hour
	}
	candidates, err := cleanup.FindCandidates(cleanup.Options{
		Failed:          opts.Failed,
		StaleRunning:    opts.StaleRunning,
		MissingManifest: opts.MissingManifest,
		OlderThan:       older,
		Limit:           opts.Limit,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		data, err := cleanup.MarshalCandidates(candidates)
		if err != nil {
			return err
		}
		_, err = stdout.Write(data)
		return err
	}
	fmt.Fprintln(stdout, "Cleanup candidates")
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "  - %s [%s] %s\n", candidate.RunID, candidate.Reason, candidate.RunDir)
	}
	if !opts.Delete {
		fmt.Fprintln(stdout, "Dry run only. Pass --delete to remove candidates.")
		return nil
	}
	if len(candidates) == 0 {
		return nil
	}
	if !opts.Yes {
		input := opts.ConfirmInput
		if input == nil {
			input = os.Stdin
		}
		fmt.Fprint(stdout, "Delete these run directories? Type yes to continue: ")
		var answer string
		_, _ = fmt.Fscan(input, &answer)
		if answer != "yes" {
			return fmt.Errorf("cleanup deletion cancelled")
		}
	}
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "Deleting %s\n", candidate.RunDir)
		if err := cleanup.DeleteCandidate(candidate); err != nil {
			return err
		}
	}
	return nil
}

func InferPreset(m manifest.Manifest) string {
	for _, artifact := range m.Artifacts {
		switch artifact.Path {
		case "roughcut.json", "report.html", "ffmpeg_commands.sh":
			return "shorts"
		}
	}
	return "metadata"
}

func failedBatchItems(items []batch.Item, limit int) []batch.Item {
	out := []batch.Item{}
	for _, item := range items {
		if item.Status != "failed" {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func failedWatchItems(items []watch.RegistryItem, limit int) []watch.RegistryItem {
	out := []watch.RegistryItem{}
	for _, item := range items {
		if item.Status != "failed" {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func retryInput(inputPath string, opts RunOptions, export bool, validate bool, stdout io.Writer) batch.Item {
	item := batch.Item{InputPath: inputPath, Status: "completed"}
	if _, err := os.Stat(inputPath); err != nil {
		item.Status = "failed"
		item.Error = fmt.Sprintf("input file unavailable: %v", err)
		return item
	}
	var runOut bytes.Buffer
	if err := Run(inputPath, &runOut, opts); err != nil {
		item.Status = "failed"
		item.Error = err.Error()
		return item
	}
	item.RunID = batch.ParseRunID(runOut.String())
	if item.RunID != "" {
		item.RunDir = filepath.Join(".byom-video", "runs", item.RunID)
	}
	if export && item.RunID != "" {
		if err := Export(item.RunID, stdout); err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			return item
		}
	}
	if validate && item.RunID != "" {
		if err := Validate(item.RunID, stdout, ValidateOptions{}); err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			return item
		}
	}
	return item
}

func presetOptions(preset string) RunOptions {
	opts := RunOptions{
		TranscriptModelSize:  "tiny",
		ChunkTargetSeconds:   30,
		ChunkMaxGapSeconds:   2,
		HighlightTopK:        10,
		HighlightMinDuration: 3,
		HighlightMaxDuration: 90,
		RoughcutMaxClips:     5,
		FFmpegOutputFormat:   "mp4",
	}
	if preset == "shorts" {
		opts.WithTranscript = true
		opts.WithCaptions = true
		opts.WithChunks = true
		opts.WithHighlights = true
		opts.WithRoughcut = true
		opts.WithFFmpegScript = true
		opts.WithReport = true
	}
	return opts
}

func retryBatchID() string {
	id, err := runctx.NewRunID(time.Now().UTC())
	if err != nil {
		return time.Now().UTC().Format("20060102T150405Z") + "-retry"
	}
	return id
}

func ValidateRecoveryPreset(preset string) error {
	switch strings.TrimSpace(preset) {
	case "shorts", "metadata":
		return nil
	default:
		return fmt.Errorf("unknown pipeline preset %q; supported values: shorts, metadata", preset)
	}
}

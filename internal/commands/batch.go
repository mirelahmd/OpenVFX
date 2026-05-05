package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"byom-video/internal/batch"
)

type BatchOptions struct {
	Preset            string
	RunOptions        RunOptions
	Recursive         bool
	Limit             int
	FailFast          bool
	DryRun            bool
	Validate          bool
	Export            bool
	ExportAndValidate bool
}

type InspectBatchOptions struct {
	JSON bool
}

func Batch(inputDir string, stdout io.Writer, opts BatchOptions) error {
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
	_, err := batch.Run(batch.Options{
		InputDir:          inputDir,
		Preset:            opts.Preset,
		Recursive:         opts.Recursive,
		Limit:             opts.Limit,
		FailFast:          opts.FailFast,
		DryRun:            opts.DryRun,
		Validate:          opts.Validate,
		Export:            opts.Export,
		ExportAndValidate: opts.ExportAndValidate,
	}, batch.Hooks{
		Run: func(inputPath string, out io.Writer) error {
			return Run(inputPath, out, opts.RunOptions)
		},
		Export: func(runID string, out io.Writer) error {
			return Export(runID, out)
		},
		Validate: func(runID string, out io.Writer) error {
			return Validate(runID, out, ValidateOptions{})
		},
	}, stdout)
	return err
}

func Batches(stdout io.Writer) error {
	summaries, err := batch.ListSummaries()
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "No batches found. Run `byom-video batch <input-dir> --preset metadata` first.")
		return nil
	}
	fmt.Fprintf(stdout, "%-28s %-25s %-10s %-9s %-9s %-9s %s\n", "BATCH ID", "CREATED AT", "PRESET", "SUCCEEDED", "FAILED", "ATTEMPTED", "INPUT DIR")
	for _, summary := range summaries {
		fmt.Fprintf(stdout, "%-28s %-25s %-10s %-9d %-9d %-9d %s\n",
			summary.BatchID,
			summary.CreatedAt.Format(time.RFC3339),
			summary.Preset,
			summary.Totals.Succeeded,
			summary.Totals.Failed,
			summary.Totals.Attempted,
			summary.InputDir,
		)
	}
	return nil
}

func InspectBatch(batchID string, stdout io.Writer, opts InspectBatchOptions) error {
	summary, err := batch.ReadSummary(batchID)
	if err != nil {
		return err
	}
	if opts.JSON {
		encoded, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("encode batch summary: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
		return nil
	}
	fmt.Fprintln(stdout, "Batch inspection")
	fmt.Fprintf(stdout, "  batch id:   %s\n", summary.BatchID)
	fmt.Fprintf(stdout, "  created at: %s\n", summary.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "  input dir:  %s\n", summary.InputDir)
	fmt.Fprintf(stdout, "  preset:     %s\n", summary.Preset)
	fmt.Fprintf(stdout, "  recursive:  %t\n", summary.Recursive)
	fmt.Fprintf(stdout, "  dry run:    %t\n", summary.DryRun)
	fmt.Fprintf(stdout, "  totals:     discovered=%d attempted=%d succeeded=%d failed=%d skipped=%d\n",
		summary.Totals.Discovered,
		summary.Totals.Attempted,
		summary.Totals.Succeeded,
		summary.Totals.Failed,
		summary.Totals.Skipped,
	)
	fmt.Fprintln(stdout, "  items:")
	for _, item := range summary.Items {
		fmt.Fprintf(stdout, "    - %s: %s", item.Status, item.InputPath)
		if item.RunID != "" {
			fmt.Fprintf(stdout, " run=%s", item.RunID)
		}
		if item.Error != "" {
			fmt.Fprintf(stdout, " error=%s", item.Error)
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

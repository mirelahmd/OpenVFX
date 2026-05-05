package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"byom-video/internal/watch"
)

type WatchOptions struct {
	Preset            string
	RunOptions        RunOptions
	Recursive         bool
	Once              bool
	Limit             int
	FailFast          bool
	IntervalSeconds   int
	Validate          bool
	Export            bool
	ExportAndValidate bool
	IgnoreRegistry    bool
}

type WatchStatusOptions struct {
	JSON bool
}

func Watch(inputDir string, stdout io.Writer, opts WatchOptions) error {
	if opts.Preset == "" {
		opts.Preset = "shorts"
	}
	if opts.IntervalSeconds <= 0 {
		return fmt.Errorf("--interval-seconds must be positive")
	}
	if opts.ExportAndValidate {
		opts.Export = true
		opts.Validate = true
	}
	if opts.Export && !opts.RunOptions.WithFFmpegScript {
		return fmt.Errorf("--export requires a preset that generates ffmpeg_commands.sh")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return watch.Run(ctx, watch.Options{
		InputDir:          inputDir,
		Preset:            opts.Preset,
		Recursive:         opts.Recursive,
		Once:              opts.Once,
		Limit:             opts.Limit,
		FailFast:          opts.FailFast,
		Interval:          time.Duration(opts.IntervalSeconds) * time.Second,
		Validate:          opts.Validate,
		Export:            opts.Export,
		ExportAndValidate: opts.ExportAndValidate,
		IgnoreRegistry:    opts.IgnoreRegistry,
	}, watch.Hooks{
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
}

func WatchStatus(stdout io.Writer, opts WatchStatusOptions) error {
	registry, err := watch.Status()
	if err != nil {
		return err
	}
	if opts.JSON {
		encoded, err := json.MarshalIndent(registry, "", "  ")
		if err != nil {
			return fmt.Errorf("encode watch status: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
		return nil
	}
	watch.PrintStatus(stdout, registry)
	return nil
}

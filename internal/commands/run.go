package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"byom-video/internal/captions"
	"byom-video/internal/chunks"
	"byom-video/internal/events"
	"byom-video/internal/exportscript"
	"byom-video/internal/fsx"
	"byom-video/internal/highlights"
	"byom-video/internal/manifest"
	"byom-video/internal/media"
	"byom-video/internal/report"
	"byom-video/internal/roughcut"
	"byom-video/internal/runctx"
	"byom-video/internal/transcript"
	"byom-video/internal/workers"
)

type RunOptions struct {
	WithTranscriptStub      bool
	WithTranscript          bool
	WithCaptions            bool
	TranscriptModelSize     string
	TranscriptModelSizeSet  bool
	WithChunks              bool
	ChunkTargetSeconds      float64
	ChunkTargetSecondsSet   bool
	ChunkMaxGapSeconds      float64
	ChunkMaxGapSecondsSet   bool
	WithHighlights          bool
	HighlightTopK           int
	HighlightTopKSet        bool
	HighlightMinDuration    float64
	HighlightMinDurationSet bool
	HighlightMaxDuration    float64
	HighlightMaxDurationSet bool
	WithRoughcut            bool
	RoughcutMaxClips        int
	RoughcutMaxClipsSet     bool
	WithFFmpegScript        bool
	FFmpegOutputFormat      string
	FFmpegOutputFormatSet   bool
	FFmpegMode              string
	FFmpegModeSet           bool
	WithReport              bool
	PythonInterpreter       string
}

func ValidateTranscriptModelSize(modelSize string) error {
	switch modelSize {
	case "tiny", "base", "small", "medium", "large-v3":
		return nil
	default:
		return fmt.Errorf("unsupported transcript model size %q; supported values: tiny, base, small, medium, large-v3", modelSize)
	}
}

func Run(inputFile string, stdout io.Writer, opts RunOptions) error {
	if opts.TranscriptModelSize == "" {
		opts.TranscriptModelSize = "tiny"
	}
	if opts.ChunkTargetSeconds == 0 {
		opts.ChunkTargetSeconds = 30
	}
	if opts.ChunkMaxGapSeconds == 0 && !opts.ChunkMaxGapSecondsSet {
		opts.ChunkMaxGapSeconds = 2
	}
	if opts.HighlightTopK == 0 {
		opts.HighlightTopK = 10
	}
	if opts.HighlightMinDuration == 0 {
		opts.HighlightMinDuration = 3
	}
	if opts.HighlightMaxDuration == 0 {
		opts.HighlightMaxDuration = 90
	}
	if opts.RoughcutMaxClips == 0 {
		opts.RoughcutMaxClips = 5
	}
	if opts.FFmpegOutputFormat == "" {
		opts.FFmpegOutputFormat = "mp4"
	}
	if opts.FFmpegMode == "" {
		opts.FFmpegMode = "stream-copy"
	}
	inputPath, err := filepath.Abs(inputFile)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	if err := fsx.RequireFile(inputPath); err != nil {
		return err
	}

	ctx, err := runctx.New(inputPath, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := fsx.EnsureDir(ctx.Dir); err != nil {
		return err
	}

	eventLog, err := events.Open(filepath.Join(ctx.Dir, "events.jsonl"))
	if err != nil {
		return err
	}
	defer eventLog.Close()

	m := manifest.New(ctx.RunID, inputPath, ctx.CreatedAt)
	if err := eventLog.Write("RUN_STARTED", map[string]any{"input_path": inputPath, "run_dir": ctx.Dir}); err != nil {
		return err
	}

	manifestPath := filepath.Join(ctx.Dir, "manifest.json")
	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("events", "events.jsonl")
	if err := manifest.Write(manifestPath, m); err != nil {
		return err
	}
	if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "manifest.json"}); err != nil {
		return err
	}
	if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "events.jsonl"}); err != nil {
		return err
	}

	metadataPath := filepath.Join(ctx.Dir, "metadata.json")
	if err := eventLog.Write("FFPROBE_STARTED", map[string]any{"input_path": inputPath}); err != nil {
		return err
	}

	rawMetadata, err := media.Probe(inputPath)
	if err != nil {
		m.Status = manifest.StatusFailed
		m.ErrorMessage = err.Error()
		if version, versionErr := media.ToolVersion("ffprobe"); versionErr == nil {
			m.ToolVersions["ffprobe"] = version
		}
		_ = manifest.Write(manifestPath, m)
		_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
		if errors.Is(err, media.ErrFFprobeMissing) {
			return fmt.Errorf("%w; run `byom-video doctor` for install guidance", err)
		}
		return err
	}
	if err := os.WriteFile(metadataPath, rawMetadata, 0o644); err != nil {
		m.Status = manifest.StatusFailed
		m.ErrorMessage = err.Error()
		if version, versionErr := media.ToolVersion("ffprobe"); versionErr == nil {
			m.ToolVersions["ffprobe"] = version
		}
		_ = manifest.Write(manifestPath, m)
		_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
		return fmt.Errorf("write metadata artifact: %w", err)
	}
	m.AddArtifact("metadata", "metadata.json")
	if err := eventLog.Write("FFPROBE_COMPLETED", map[string]any{"path": "metadata.json"}); err != nil {
		return err
	}
	if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "metadata.json"}); err != nil {
		return err
	}

	if version, err := media.ToolVersion("ffprobe"); err == nil {
		m.ToolVersions["ffprobe"] = version
	}

	artifacts := []string{"manifest.json", "events.jsonl", "metadata.json"}
	var transcriptSummary *transcript.Summary
	var highlightsSummary *highlights.Summary
	var roughcutSummary *roughcut.Summary
	var captionsSummary *captions.Summary
	var ffmpegSummary *exportscript.Summary
	var reportSummary *report.Summary
	transcriptValidated := false
	if opts.WithTranscriptStub {
		if err := eventLog.Write("TRANSCRIBE_STUB_STARTED", map[string]any{"input_path": inputPath}); err != nil {
			return err
		}
		if err := workers.RunTranscribeStubWithPython(opts.PythonInterpreter, inputPath, ctx.Dir); err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("TRANSCRIBE_STUB_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		m.AddArtifact("transcript", "transcript.json")
		artifacts = append(artifacts, "transcript.json")
		if err := eventLog.Write("TRANSCRIBE_STUB_COMPLETED", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
	}
	if opts.WithTranscript {
		if err := eventLog.Write("TRANSCRIBE_STARTED", map[string]any{"input_path": inputPath}); err != nil {
			return err
		}
		if err := workers.RunTranscribeWithPython(opts.PythonInterpreter, inputPath, ctx.Dir, opts.TranscriptModelSize); err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("TRANSCRIBE_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		transcriptPath := filepath.Join(ctx.Dir, "transcript.json")
		if err := eventLog.Write("TRANSCRIPT_VALIDATION_STARTED", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
		validatedSummary, err := transcript.ValidateFile(transcriptPath)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("TRANSCRIPT_VALIDATION_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		transcriptSummary = &validatedSummary
		transcriptValidated = true
		if transcriptSummary.ModelSize == "" {
			transcriptSummary.ModelSize = opts.TranscriptModelSize
		}
		if err := eventLog.Write("TRANSCRIPT_VALIDATION_COMPLETED", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
		m.AddArtifact("transcript", "transcript.json")
		artifacts = append(artifacts, "transcript.json")
		if err := eventLog.Write("TRANSCRIBE_COMPLETED", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "transcript.json"}); err != nil {
			return err
		}
	}
	if opts.WithCaptions {
		if err := eventLog.Write("CAPTIONS_STARTED", map[string]any{"transcript_path": "transcript.json"}); err != nil {
			return err
		}
		generatedSummary, err := captions.WriteFromTranscript(filepath.Join(ctx.Dir, "transcript.json"), filepath.Join(ctx.Dir, "captions.srt"))
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("CAPTIONS_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		captionsSummary = &generatedSummary
		m.AddArtifact("captions", "captions.srt")
		artifacts = append(artifacts, "captions.srt")
		if err := eventLog.Write("CAPTIONS_COMPLETED", map[string]any{"path": "captions.srt"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "captions.srt"}); err != nil {
			return err
		}
	}
	var chunksSummary *chunks.Summary
	if opts.WithChunks {
		transcriptPath := filepath.Join(ctx.Dir, "transcript.json")
		if !transcriptValidated {
			if err := eventLog.Write("TRANSCRIPT_VALIDATION_STARTED", map[string]any{"path": "transcript.json"}); err != nil {
				return err
			}
			validatedSummary, err := transcript.ValidateFile(transcriptPath)
			if err != nil {
				m.Status = manifest.StatusFailed
				m.ErrorMessage = err.Error()
				_ = manifest.Write(manifestPath, m)
				_ = eventLog.Write("TRANSCRIPT_VALIDATION_FAILED", map[string]any{"error": err.Error()})
				_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
				return err
			}
			if opts.WithTranscript {
				transcriptSummary = &validatedSummary
			}
			if err := eventLog.Write("TRANSCRIPT_VALIDATION_COMPLETED", map[string]any{"path": "transcript.json"}); err != nil {
				return err
			}
		}
		if err := eventLog.Write("CHUNKING_STARTED", map[string]any{"transcript_path": "transcript.json"}); err != nil {
			return err
		}
		chunkPath := filepath.Join(ctx.Dir, "chunks.json")
		generatedSummary, err := chunks.WriteFromTranscript(transcriptPath, chunkPath, chunks.Options{TargetSeconds: opts.ChunkTargetSeconds, MaxGapSeconds: opts.ChunkMaxGapSeconds})
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("CHUNKING_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		if err := eventLog.Write("CHUNKS_VALIDATION_STARTED", map[string]any{"path": "chunks.json"}); err != nil {
			return err
		}
		validatedSummary, err := chunks.ValidateFile(chunkPath)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("CHUNKS_VALIDATION_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		generatedSummary.ChunkCount = validatedSummary.ChunkCount
		chunksSummary = &generatedSummary
		m.AddArtifact("chunks", "chunks.json")
		artifacts = append(artifacts, "chunks.json")
		if err := eventLog.Write("CHUNKS_VALIDATION_COMPLETED", map[string]any{"path": "chunks.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("CHUNKING_COMPLETED", map[string]any{"path": "chunks.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "chunks.json"}); err != nil {
			return err
		}
	}
	if opts.WithHighlights {
		if err := eventLog.Write("HIGHLIGHTS_STARTED", map[string]any{"chunks_path": "chunks.json"}); err != nil {
			return err
		}
		highlightsPath := filepath.Join(ctx.Dir, "highlights.json")
		generatedSummary, err := highlights.WriteFromChunks(filepath.Join(ctx.Dir, "chunks.json"), highlightsPath, highlights.Options{
			MinDurationSeconds: opts.HighlightMinDuration,
			MaxDurationSeconds: opts.HighlightMaxDuration,
			TopK:               opts.HighlightTopK,
		})
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("HIGHLIGHTS_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		if err := eventLog.Write("HIGHLIGHTS_VALIDATION_STARTED", map[string]any{"path": "highlights.json"}); err != nil {
			return err
		}
		validatedSummary, err := highlights.ValidateFile(highlightsPath)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("HIGHLIGHTS_VALIDATION_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		generatedSummary.Count = validatedSummary.Count
		generatedSummary.TopScore = validatedSummary.TopScore
		generatedSummary.TopStart = validatedSummary.TopStart
		generatedSummary.TopEnd = validatedSummary.TopEnd
		highlightsSummary = &generatedSummary
		m.AddArtifact("highlights", "highlights.json")
		artifacts = append(artifacts, "highlights.json")
		if err := eventLog.Write("HIGHLIGHTS_VALIDATION_COMPLETED", map[string]any{"path": "highlights.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("HIGHLIGHTS_COMPLETED", map[string]any{"path": "highlights.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "highlights.json"}); err != nil {
			return err
		}
	}
	if opts.WithRoughcut {
		if err := eventLog.Write("ROUGHCUT_STARTED", map[string]any{"highlights_path": "highlights.json"}); err != nil {
			return err
		}
		roughcutPath := filepath.Join(ctx.Dir, "roughcut.json")
		generatedSummary, err := roughcut.WriteFromHighlights(filepath.Join(ctx.Dir, "highlights.json"), roughcutPath, roughcut.Options{MaxClips: opts.RoughcutMaxClips})
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("ROUGHCUT_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		if err := eventLog.Write("ROUGHCUT_VALIDATION_STARTED", map[string]any{"path": "roughcut.json"}); err != nil {
			return err
		}
		validatedSummary, err := roughcut.ValidateFile(roughcutPath)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("ROUGHCUT_VALIDATION_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		generatedSummary.ClipCount = validatedSummary.ClipCount
		generatedSummary.TotalDurationSeconds = validatedSummary.TotalDurationSeconds
		roughcutSummary = &generatedSummary
		m.AddArtifact("roughcut", "roughcut.json")
		artifacts = append(artifacts, "roughcut.json")
		if err := eventLog.Write("ROUGHCUT_VALIDATION_COMPLETED", map[string]any{"path": "roughcut.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ROUGHCUT_COMPLETED", map[string]any{"path": "roughcut.json"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "roughcut.json"}); err != nil {
			return err
		}
	}
	if opts.WithFFmpegScript {
		if err := eventLog.Write("FFMPEG_SCRIPT_STARTED", map[string]any{"roughcut_path": "roughcut.json"}); err != nil {
			return err
		}
		generatedSummary, err := exportscript.WriteFFmpegScript(filepath.Join(ctx.Dir, "roughcut.json"), filepath.Join(ctx.Dir, "ffmpeg_commands.sh"), inputPath, opts.FFmpegOutputFormat, opts.FFmpegMode)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("FFMPEG_SCRIPT_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		ffmpegSummary = &generatedSummary
		m.AddArtifact("ffmpeg_script", "ffmpeg_commands.sh")
		artifacts = append(artifacts, "ffmpeg_commands.sh")
		if err := eventLog.Write("FFMPEG_SCRIPT_COMPLETED", map[string]any{"path": "ffmpeg_commands.sh"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "ffmpeg_commands.sh"}); err != nil {
			return err
		}
	}

	if opts.WithReport {
		if err := eventLog.Write("REPORT_STARTED", map[string]any{"run_dir": ctx.Dir}); err != nil {
			return err
		}
		m.Status = manifest.StatusCompleted
		m.AddArtifact("report", "report.html")
		generatedSummary, err := report.Write(ctx.Dir, m)
		if err != nil {
			m.Status = manifest.StatusFailed
			m.ErrorMessage = err.Error()
			_ = manifest.Write(manifestPath, m)
			_ = eventLog.Write("REPORT_FAILED", map[string]any{"error": err.Error()})
			_ = eventLog.Write("RUN_FAILED", map[string]any{"error": err.Error()})
			return err
		}
		reportSummary = &generatedSummary
		artifacts = append(artifacts, "report.html")
		if err := eventLog.Write("REPORT_COMPLETED", map[string]any{"path": "report.html"}); err != nil {
			return err
		}
		if err := eventLog.Write("ARTIFACT_WRITTEN", map[string]any{"path": "report.html"}); err != nil {
			return err
		}
	}

	m.Status = manifest.StatusCompleted
	if err := manifest.Write(manifestPath, m); err != nil {
		return err
	}
	if err := eventLog.Write("RUN_COMPLETED", map[string]any{"status": m.Status}); err != nil {
		return err
	}

	summary := summarizeMetadata(rawMetadata)
	printRunSummary(stdout, ctx, summary, artifacts, transcriptSummary, captionsSummary, chunksSummary, highlightsSummary, roughcutSummary, ffmpegSummary, reportSummary)
	return nil
}

type probeSummary struct {
	Duration     string
	VideoStreams *int
	AudioStreams *int
	TotalStreams *int
}

func summarizeMetadata(raw []byte) probeSummary {
	var doc struct {
		Format *struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams *[]struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}

	var out probeSummary
	if err := json.Unmarshal(raw, &doc); err != nil {
		return out
	}
	if doc.Format != nil {
		out.Duration = doc.Format.Duration
	}
	if doc.Streams == nil {
		return out
	}
	videoStreams := 0
	audioStreams := 0
	for _, stream := range *doc.Streams {
		switch stream.CodecType {
		case "video":
			videoStreams++
		case "audio":
			audioStreams++
		}
	}
	totalStreams := len(*doc.Streams)
	out.VideoStreams = &videoStreams
	out.AudioStreams = &audioStreams
	out.TotalStreams = &totalStreams
	return out
}

func printRunSummary(stdout io.Writer, ctx runctx.Context, summary probeSummary, artifacts []string, transcriptSummary *transcript.Summary, captionsSummary *captions.Summary, chunksSummary *chunks.Summary, highlightsSummary *highlights.Summary, roughcutSummary *roughcut.Summary, ffmpegSummary *exportscript.Summary, reportSummary *report.Summary) {
	fmt.Fprintln(stdout, "Run completed")
	fmt.Fprintf(stdout, "  run id:        %s\n", ctx.RunID)
	fmt.Fprintf(stdout, "  input file:    %s\n", ctx.InputPath)
	fmt.Fprintf(stdout, "  run directory: %s\n", ctx.Dir)
	fmt.Fprintf(stdout, "  duration:      %s\n", durationDisplay(summary.Duration))
	fmt.Fprintf(stdout, "  streams:       video=%s audio=%s total=%s\n", countDisplay(summary.VideoStreams), countDisplay(summary.AudioStreams), countDisplay(summary.TotalStreams))
	if transcriptSummary != nil {
		fmt.Fprintln(stdout, "  transcript:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", transcriptSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    language:    %s\n", transcriptSummary.Language)
		fmt.Fprintf(stdout, "    segments:    %d\n", transcriptSummary.SegmentCount)
		fmt.Fprintf(stdout, "    duration:    %s\n", transcriptDurationDisplay(transcriptSummary.DurationSeconds))
		fmt.Fprintf(stdout, "    model size:  %s\n", transcriptSummary.ModelSize)
	}
	if captionsSummary != nil {
		fmt.Fprintln(stdout, "  captions:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", captionsSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    cues:        %d\n", captionsSummary.CueCount)
	}
	if chunksSummary != nil {
		fmt.Fprintln(stdout, "  chunks:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", chunksSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    count:       %d\n", chunksSummary.ChunkCount)
		fmt.Fprintf(stdout, "    target sec:  %s\n", floatDisplay(chunksSummary.TargetSeconds))
		fmt.Fprintf(stdout, "    max gap sec: %s\n", floatDisplay(chunksSummary.MaxGapSeconds))
	}
	if highlightsSummary != nil {
		fmt.Fprintln(stdout, "  highlights:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", highlightsSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    count:       %d\n", highlightsSummary.Count)
		fmt.Fprintf(stdout, "    top score:   %s\n", optionalFloatDisplay(highlightsSummary.TopScore))
		fmt.Fprintf(stdout, "    top range:   %s-%s\n", optionalFloatDisplay(highlightsSummary.TopStart), optionalFloatDisplay(highlightsSummary.TopEnd))
	}
	if roughcutSummary != nil {
		fmt.Fprintln(stdout, "  roughcut:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", roughcutSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    clips:       %d\n", roughcutSummary.ClipCount)
		fmt.Fprintf(stdout, "    duration:    %.6f seconds\n", roughcutSummary.TotalDurationSeconds)
	}
	if ffmpegSummary != nil {
		fmt.Fprintln(stdout, "  ffmpeg script:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", ffmpegSummary.ArtifactPath)
		fmt.Fprintf(stdout, "    commands:    %d\n", ffmpegSummary.CommandCount)
		fmt.Fprintf(stdout, "    mode:        %s\n", ffmpegSummary.Mode)
		fmt.Fprintln(stdout, "    note:        script generated only; not executed")
	}
	if reportSummary != nil {
		fmt.Fprintln(stdout, "  report:")
		fmt.Fprintf(stdout, "    artifact:    %s\n", reportSummary.ArtifactPath)
	}
	fmt.Fprintln(stdout, "  artifacts:")
	for _, artifact := range artifacts {
		fmt.Fprintf(stdout, "    - %s\n", artifact)
	}
}

func optionalFloatDisplay(value *float64) string {
	if value == nil {
		return "none"
	}
	return fmt.Sprintf("%.6f", *value)
}

func floatDisplay(value float64) string {
	return fmt.Sprintf("%.3f", value)
}

func durationDisplay(duration string) string {
	if duration == "" {
		return "unknown"
	}
	return duration + " seconds"
}

func countDisplay(count *int) string {
	if count == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d", *count)
}

func transcriptDurationDisplay(duration *float64) string {
	if duration == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.6f seconds", *duration)
}

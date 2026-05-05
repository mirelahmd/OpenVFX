package cli

import (
	"os"
	"testing"
)

func TestParseRunArgsMetadataOnly(t *testing.T) {
	input, opts, err := parseRunArgs([]string{"input.mp4"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if input != "input.mp4" {
		t.Fatalf("input = %q, want input.mp4", input)
	}
	if opts.WithTranscriptStub {
		t.Fatal("WithTranscriptStub = true, want false")
	}
	if opts.TranscriptModelSize != "tiny" {
		t.Fatalf("TranscriptModelSize = %q, want tiny", opts.TranscriptModelSize)
	}
}

func TestParseRunArgsWithTranscriptStub(t *testing.T) {
	input, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript-stub"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if input != "input.mp4" {
		t.Fatalf("input = %q, want input.mp4", input)
	}
	if !opts.WithTranscriptStub {
		t.Fatal("WithTranscriptStub = false, want true")
	}
}

func TestParseRunArgsWithTranscript(t *testing.T) {
	input, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if input != "input.mp4" {
		t.Fatalf("input = %q, want input.mp4", input)
	}
	if !opts.WithTranscript {
		t.Fatal("WithTranscript = false, want true")
	}
}

func TestParseRunArgsWithTranscriptModelSize(t *testing.T) {
	input, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--transcript-model-size", "base"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if input != "input.mp4" {
		t.Fatalf("input = %q, want input.mp4", input)
	}
	if opts.TranscriptModelSize != "base" {
		t.Fatalf("TranscriptModelSize = %q, want base", opts.TranscriptModelSize)
	}
}

func TestParseRunArgsRejectsModelSizeWithoutTranscript(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--transcript-model-size", "base"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsDefaultModelSizeWithoutTranscript(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--transcript-model-size", "tiny"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsUnsupportedModelSize(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--transcript-model-size", "huge"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsTranscriptModesTogether(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-transcript-stub"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsMissingInput(t *testing.T) {
	_, _, err := parseRunArgs([]string{"--with-transcript-stub"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseBatchArgsDefaultsToShorts(t *testing.T) {
	input, opts, err := parseBatchArgs([]string{"media"})
	if err != nil {
		t.Fatalf("parseBatchArgs returned error: %v", err)
	}
	if input != "media" || opts.Preset != "shorts" || !opts.RunOptions.WithFFmpegScript {
		t.Fatalf("input=%q opts=%#v", input, opts)
	}
}

func TestParseBatchArgsRejectsInvalidPreset(t *testing.T) {
	_, _, err := parseBatchArgs([]string{"media", "--preset", "unknown"})
	if err == nil {
		t.Fatal("parseBatchArgs returned nil error")
	}
}

func TestParseBatchArgsRejectsZeroLimit(t *testing.T) {
	_, _, err := parseBatchArgs([]string{"media", "--limit", "0"})
	if err == nil {
		t.Fatal("parseBatchArgs returned nil error")
	}
}

func TestParseWatchArgsDefaults(t *testing.T) {
	input, opts, err := parseWatchArgs([]string{"media"})
	if err != nil {
		t.Fatalf("parseWatchArgs returned error: %v", err)
	}
	if input != "media" || opts.Preset != "shorts" || opts.IntervalSeconds != 5 || !opts.RunOptions.WithFFmpegScript {
		t.Fatalf("input=%q opts=%#v", input, opts)
	}
}

func TestParseWatchArgsRejectsInvalidInterval(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"media", "--interval-seconds", "0"})
	if err == nil {
		t.Fatal("parseWatchArgs returned nil error")
	}
}

func TestParseWatchArgsRejectsInvalidPreset(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"media", "--preset", "unknown"})
	if err == nil {
		t.Fatal("parseWatchArgs returned nil error")
	}
}

func TestParseRunArgsRejectsUnknownFlag(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--unknown"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsWithChunks(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--chunk-target-seconds", "12.5", "--chunk-max-gap-seconds", "1.5"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithChunks {
		t.Fatal("WithChunks = false, want true")
	}
	if opts.ChunkTargetSeconds != 12.5 {
		t.Fatalf("ChunkTargetSeconds = %v, want 12.5", opts.ChunkTargetSeconds)
	}
	if opts.ChunkMaxGapSeconds != 1.5 {
		t.Fatalf("ChunkMaxGapSeconds = %v, want 1.5", opts.ChunkMaxGapSeconds)
	}
}

func TestParseRunArgsRejectsChunksWithoutTranscriptMode(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-chunks"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsChunkTargetWithoutChunks(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--chunk-target-seconds", "30"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsChunkMaxGapWithoutChunks(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--chunk-max-gap-seconds", "2"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsInvalidChunkNumbers(t *testing.T) {
	cases := [][]string{
		{"input.mp4", "--with-transcript", "--with-chunks", "--chunk-target-seconds", "0"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--chunk-target-seconds", "-1"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--chunk-max-gap-seconds", "-1"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--chunk-target-seconds", "abc"},
	}
	for _, args := range cases {
		_, _, err := parseRunArgs(args)
		if err == nil {
			t.Fatalf("parseRunArgs(%v) returned nil error", args)
		}
	}
}

func TestParseRunArgsWithHighlights(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-highlights", "--highlight-top-k", "3"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithHighlights {
		t.Fatal("WithHighlights = false, want true")
	}
	if opts.HighlightTopK != 3 {
		t.Fatalf("HighlightTopK = %d, want 3", opts.HighlightTopK)
	}
}

func TestParseRunArgsRejectsHighlightsWithoutChunks(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-highlights"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsInvalidHighlightFlags(t *testing.T) {
	cases := [][]string{
		{"input.mp4", "--with-transcript", "--with-chunks", "--with-highlights", "--highlight-top-k", "0"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--with-highlights", "--highlight-min-duration-seconds", "-1"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--with-highlights", "--highlight-min-duration-seconds", "5", "--highlight-max-duration-seconds", "5"},
		{"input.mp4", "--with-transcript", "--with-chunks", "--highlight-top-k", "3"},
	}
	for _, args := range cases {
		_, _, err := parseRunArgs(args)
		if err == nil {
			t.Fatalf("parseRunArgs(%v) returned nil error", args)
		}
	}
}

func TestParseRunArgsRoughcutEnablesHighlights(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-roughcut", "--roughcut-max-clips", "2"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithHighlights {
		t.Fatal("WithHighlights = false, want true")
	}
	if !opts.WithRoughcut {
		t.Fatal("WithRoughcut = false, want true")
	}
	if opts.RoughcutMaxClips != 2 {
		t.Fatalf("RoughcutMaxClips = %d, want 2", opts.RoughcutMaxClips)
	}
}

func TestParseRunArgsRejectsRoughcutWithoutChunks(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-roughcut"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsRoughcutFlagsWithoutRoughcut(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-highlights", "--roughcut-max-clips", "2"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsWithCaptions(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-captions"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithCaptions {
		t.Fatal("WithCaptions = false, want true")
	}
}

func TestParseRunArgsRejectsCaptionsWithoutTranscriptMode(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-captions"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsWithFFmpegScript(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-roughcut", "--with-ffmpeg-script", "--ffmpeg-output-format", "mp4"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithFFmpegScript {
		t.Fatal("WithFFmpegScript = false, want true")
	}
}

func TestParseRunArgsRejectsFFmpegScriptWithoutRoughcut(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-ffmpeg-script"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsRejectsUnsupportedFFmpegFormat(t *testing.T) {
	_, _, err := parseRunArgs([]string{"input.mp4", "--with-transcript", "--with-chunks", "--with-roughcut", "--with-ffmpeg-script", "--ffmpeg-output-format", "mov"})
	if err == nil {
		t.Fatal("parseRunArgs returned nil error")
	}
}

func TestParseRunArgsWithReport(t *testing.T) {
	_, opts, err := parseRunArgs([]string{"input.mp4", "--with-report"})
	if err != nil {
		t.Fatalf("parseRunArgs returned error: %v", err)
	}
	if !opts.WithReport {
		t.Fatal("WithReport = false, want true")
	}
}

func TestConfiguredRunOptionsLoadsConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	data := []byte("python:\n  interpreter: .venv/bin/python\ntranscription:\n  enabled: true\n  model_size: base\nreport:\n  enabled: true\n")
	if err := os.WriteFile("byom-video.yaml", data, 0o644); err != nil {
		t.Fatal(err)
	}
	opts, err := configuredRunOptions(true)
	if err != nil {
		t.Fatalf("configuredRunOptions returned error: %v", err)
	}
	if !opts.WithTranscript || !opts.WithReport {
		t.Fatalf("configured enabled flags not applied: %#v", opts)
	}
	if opts.TranscriptModelSize != "base" {
		t.Fatalf("TranscriptModelSize = %q", opts.TranscriptModelSize)
	}
	if opts.PythonInterpreter != ".venv/bin/python" {
		t.Fatalf("PythonInterpreter = %q", opts.PythonInterpreter)
	}
}

func TestParseRunArgsCLIOverridesConfigValues(t *testing.T) {
	base := defaultRunOptions()
	base.TranscriptModelSize = "base"
	_, opts, err := parseRunArgsWithBase([]string{"input.mp4", "--with-transcript", "--transcript-model-size", "small"}, base)
	if err != nil {
		t.Fatalf("parseRunArgsWithBase returned error: %v", err)
	}
	if opts.TranscriptModelSize != "small" {
		t.Fatalf("TranscriptModelSize = %q, want small", opts.TranscriptModelSize)
	}
}

func TestPresetShortsMapsToRunOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	opts, err := presetRunOptions("shorts")
	if err != nil {
		t.Fatalf("presetRunOptions returned error: %v", err)
	}
	if !opts.WithTranscript || !opts.WithCaptions || !opts.WithChunks || !opts.WithHighlights || !opts.WithRoughcut || !opts.WithFFmpegScript || !opts.WithReport {
		t.Fatalf("shorts preset did not enable full pipeline: %#v", opts)
	}
}

func TestPipelineRejectsUnknownPreset(t *testing.T) {
	t.Chdir(t.TempDir())
	_, _, err := parsePipelineArgs([]string{"input.mp4", "--preset", "unknown"})
	if err == nil {
		t.Fatal("parsePipelineArgs returned nil error")
	}
}

func TestPresetMetadataDoesNotEnablePipeline(t *testing.T) {
	t.Chdir(t.TempDir())
	data := []byte("transcription:\n  enabled: true\nreport:\n  enabled: true\n")
	if err := os.WriteFile("byom-video.yaml", data, 0o644); err != nil {
		t.Fatal(err)
	}
	opts, err := presetRunOptions("metadata")
	if err != nil {
		t.Fatalf("presetRunOptions returned error: %v", err)
	}
	if opts.WithTranscript || opts.WithReport {
		t.Fatalf("metadata preset enabled pipeline flags: %#v", opts)
	}
}

func TestParseRunsArgs(t *testing.T) {
	opts, err := parseRunsArgs([]string{"--limit", "5"})
	if err != nil {
		t.Fatalf("parseRunsArgs returned error: %v", err)
	}
	if opts.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", opts.Limit)
	}
}

func TestParseRunsArgsRejectsInvalidLimit(t *testing.T) {
	if _, err := parseRunsArgs([]string{"--limit", "0"}); err == nil {
		t.Fatal("parseRunsArgs returned nil error")
	}
}

func TestParseInspectArgsJSON(t *testing.T) {
	runID, opts, err := parseInspectArgs([]string{"run-1", "--json"})
	if err != nil {
		t.Fatalf("parseInspectArgs returned error: %v", err)
	}
	if runID != "run-1" || !opts.JSON {
		t.Fatalf("runID=%q opts=%#v", runID, opts)
	}
}

func TestParseArtifactsArgsType(t *testing.T) {
	runID, opts, err := parseArtifactsArgs([]string{"run-1", "--type", "metadata"})
	if err != nil {
		t.Fatalf("parseArtifactsArgs returned error: %v", err)
	}
	if runID != "run-1" || opts.Type != "metadata" {
		t.Fatalf("runID=%q opts=%#v", runID, opts)
	}
}

func TestParseOpenReportArgs(t *testing.T) {
	runID, open, err := parseOpenReportArgs([]string{"run-1", "--open"})
	if err != nil {
		t.Fatalf("parseOpenReportArgs returned error: %v", err)
	}
	if runID != "run-1" || !open {
		t.Fatalf("runID=%q open=%v", runID, open)
	}
}

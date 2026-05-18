package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- NormalizeCaptionPosition ----

func TestNormalizeCaptionPosition_Empty(t *testing.T) {
	pos, err := NormalizeCaptionPosition("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos != "auto" {
		t.Errorf("expected 'auto' for empty input, got %q", pos)
	}
}

func TestNormalizeCaptionPosition_Valid(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"bottom", "bottom"},
		{"BOTTOM", "bottom"},
		{"center", "center"},
		{"top", "top"},
		{"auto", "auto"},
		{" bottom ", "bottom"},
	} {
		got, err := NormalizeCaptionPosition(tc.in)
		if err != nil {
			t.Errorf("NormalizeCaptionPosition(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeCaptionPosition(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeCaptionPosition_Invalid(t *testing.T) {
	_, err := NormalizeCaptionPosition("middle")
	if err == nil {
		t.Fatal("expected error for unknown position, got nil")
	}
	if !strings.Contains(err.Error(), "middle") {
		t.Errorf("error should mention the bad value, got: %v", err)
	}
}

// ---- NormalizeCaptionStyle ----

func TestNormalizeCaptionStyle_Empty(t *testing.T) {
	s, err := NormalizeCaptionStyle("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "default" {
		t.Errorf("expected 'default' for empty input, got %q", s)
	}
}

func TestNormalizeCaptionStyle_Valid(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"default", "default"},
		{"DEFAULT", "default"},
		{"bold", "bold"},
		{"boxed", "boxed"},
		{" bold ", "bold"},
	} {
		got, err := NormalizeCaptionStyle(tc.in)
		if err != nil {
			t.Errorf("NormalizeCaptionStyle(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeCaptionStyle(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeCaptionStyle_Invalid(t *testing.T) {
	_, err := NormalizeCaptionStyle("outline")
	if err == nil {
		t.Fatal("expected error for unknown style, got nil")
	}
}

// ---- ResolveCaptionPosition ----

func TestResolveCaptionPosition_Auto(t *testing.T) {
	got := ResolveCaptionPosition("auto", "tiktok")
	if got != "bottom" {
		t.Errorf("expected 'bottom' for auto, got %q", got)
	}
}

func TestResolveCaptionPosition_Empty(t *testing.T) {
	got := ResolveCaptionPosition("", "instagram-reel")
	if got != "bottom" {
		t.Errorf("expected 'bottom' for empty, got %q", got)
	}
}

func TestResolveCaptionPosition_Explicit(t *testing.T) {
	for _, pos := range []string{"bottom", "center", "top"} {
		got := ResolveCaptionPosition(pos, "youtube")
		if got != pos {
			t.Errorf("ResolveCaptionPosition(%q, youtube): got %q", pos, got)
		}
	}
}

// ---- DefaultCaptionMargin ----

func TestDefaultCaptionMargin(t *testing.T) {
	cases := []struct {
		platform string
		want     int
	}{
		{"tiktok", 160},
		{"instagram-reel", 160},
		{"youtube-short", 160},
		{"square", 100},
		{"youtube", 80},
		{"original", 80},
		{"", 80},
	}
	for _, tc := range cases {
		got := DefaultCaptionMargin(tc.platform)
		if got != tc.want {
			t.Errorf("DefaultCaptionMargin(%q): got %d, want %d", tc.platform, got, tc.want)
		}
	}
}

// ---- buildForceStyleArg ----

func TestBuildForceStyleArg_BottomDefault(t *testing.T) {
	got := buildForceStyleArg("bottom", 80, "default")
	if !strings.Contains(got, "Alignment=2") {
		t.Errorf("expected Alignment=2 for bottom, got: %q", got)
	}
	if !strings.Contains(got, "MarginV=80") {
		t.Errorf("expected MarginV=80, got: %q", got)
	}
	if strings.Contains(got, "Bold") || strings.Contains(got, "BorderStyle") {
		t.Errorf("default style should not add Bold or BorderStyle, got: %q", got)
	}
}

func TestBuildForceStyleArg_Center(t *testing.T) {
	got := buildForceStyleArg("center", 50, "default")
	if !strings.Contains(got, "Alignment=5") {
		t.Errorf("expected Alignment=5 for center, got: %q", got)
	}
}

func TestBuildForceStyleArg_Top(t *testing.T) {
	got := buildForceStyleArg("top", 40, "default")
	if !strings.Contains(got, "Alignment=8") {
		t.Errorf("expected Alignment=8 for top, got: %q", got)
	}
}

func TestBuildForceStyleArg_Bold(t *testing.T) {
	got := buildForceStyleArg("bottom", 80, "bold")
	if !strings.Contains(got, "Bold=1") {
		t.Errorf("expected Bold=1 for bold style, got: %q", got)
	}
}

func TestBuildForceStyleArg_Boxed(t *testing.T) {
	got := buildForceStyleArg("bottom", 80, "boxed")
	if !strings.Contains(got, "BorderStyle=3") {
		t.Errorf("expected BorderStyle=3 for boxed style, got: %q", got)
	}
	if !strings.Contains(got, "BackColour=&H80000000") {
		t.Errorf("expected BackColour in boxed style, got: %q", got)
	}
	if strings.Contains(got, "Bold=1") {
		t.Errorf("boxed style should not set Bold, got: %q", got)
	}
}

func TestBuildForceStyleArg_TikTokDefaults(t *testing.T) {
	margin := DefaultCaptionMargin("tiktok")
	pos := ResolveCaptionPosition("auto", "tiktok")
	got := buildForceStyleArg(pos, margin, "default")
	if !strings.Contains(got, "Alignment=2") {
		t.Errorf("expected bottom alignment for tiktok auto, got: %q", got)
	}
	if !strings.Contains(got, "MarginV=160") {
		t.Errorf("expected MarginV=160 for tiktok, got: %q", got)
	}
}

// ---- buildCaptionFilterString ----

func TestBuildCaptionFilterString_NoForceStyle(t *testing.T) {
	got := buildCaptionFilterString("/path/to/subs.srt", "")
	if got != "subtitles=/path/to/subs.srt" {
		t.Errorf("expected plain subtitles= form, got: %q", got)
	}
}

func TestBuildCaptionFilterString_WithForceStyle(t *testing.T) {
	got := buildCaptionFilterString("/path/subs.srt", "Alignment=2,MarginV=80")
	expected := "subtitles=/path/subs.srt:force_style='Alignment=2,MarginV=80'"
	if got != expected {
		t.Errorf("got: %q\nwant: %q", got, expected)
	}
}

// ---- buildCaptionArgs with forceStyle ----

func TestBuildCaptionArgs_WithForceStyle(t *testing.T) {
	forceStyle := buildForceStyleArg("top", 40, "bold")
	args := buildCaptionArgs("/in/video.mp4", "/in/caps.srt", "/out/final.mp4", forceStyle)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "force_style=") {
		t.Errorf("expected force_style= in args: %v", args)
	}
	if !strings.Contains(joined, "Alignment=8") {
		t.Errorf("expected Alignment=8 in force_style: %v", args)
	}
	if !strings.Contains(joined, "Bold=1") {
		t.Errorf("expected Bold=1 in force_style: %v", args)
	}
	for _, a := range args {
		if strings.ContainsAny(a, "&;|") {
			t.Errorf("shell operator in arg: %q", a)
		}
	}
}

func TestBuildCaptionArgs_NoForceStyle(t *testing.T) {
	args := buildCaptionArgs("/in/video.mp4", "/in/caps.srt", "/out/final.mp4", "")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "force_style") {
		t.Errorf("unexpected force_style= in args when forceStyle empty: %v", args)
	}
	if !strings.Contains(joined, "subtitles=") {
		t.Errorf("expected subtitles= in args: %v", args)
	}
}

// ---- integration: assemble dry-run shows caption position/style ----

func makeCaptionTestPlan(t *testing.T) (planID, srtPath string) {
	t.Helper()
	dir := t.TempDir()
	mustChdir(t, dir)
	inputPath := filepath.Join(dir, "test.mov")
	if err := os.WriteFile(inputPath, []byte("stub-video"), 0o644); err != nil {
		t.Fatal(err)
	}
	planID = "cap-test-001"
	outDir := filepath.Join(creativePlansRoot, planID, "outputs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a fake SRT so caption burn path is triggered in dry-run
	fakeSRT := "1\n00:00:00,000 --> 00:00:02,000\nHello world\n\n"
	srtPath = filepath.Join(outDir, "captions.srt")
	if err := os.WriteFile(srtPath, []byte(fakeSRT), 0o644); err != nil {
		t.Fatal(err)
	}
	tl := CreativeTimelineArtifact{
		SchemaVersion:  "creative_timeline.v1",
		CreativePlanID: planID,
		InputPath:      inputPath,
		Tracks: []CreativeTimelineTrack{
			{
				ID:   "track_video_main",
				Kind: "video",
				Items: []CreativeTimelineItem{
					{ID: "clip_0001", Kind: "source_clip", TimelineStart: 0, TimelineEnd: 5, SourceStart: 0, SourceEnd: 5},
				},
			},
		},
		TotalDuration: 5,
	}
	if err := writeJSONFile(filepath.Join(outDir, "creative_timeline.json"), tl); err != nil {
		t.Fatal(err)
	}
	rp := CreativeRenderPlanArtifact{
		SchemaVersion:  "creative_render_plan.v1",
		CreativePlanID: planID,
		Steps:          []CreativeRenderStep{},
	}
	if err := writeJSONFile(filepath.Join(outDir, "creative_render_plan.json"), rp); err != nil {
		t.Fatal(err)
	}
	return planID, srtPath
}

func TestCreativeAssembleDryRun_CaptionPositionShown(t *testing.T) {
	planID, srtPath := makeCaptionTestPlan(t)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:          true,
		BurnCaptions:    true,
		CaptionsPath:    srtPath,
		CaptionPosition: "top",
		CaptionStyle:    "bold",
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "top") {
		t.Errorf("dry-run should show 'top' position, got:\n%s", out)
	}
	if !strings.Contains(out, "bold") {
		t.Errorf("dry-run should show 'bold' style, got:\n%s", out)
	}
}

func TestCreativeAssembleDryRun_CaptionForceStyleInCommand(t *testing.T) {
	planID, srtPath := makeCaptionTestPlan(t)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:          true,
		BurnCaptions:    true,
		CaptionsPath:    srtPath,
		CaptionPosition: "center",
		CaptionStyle:    "boxed",
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	out := buf.String()
	// force_style appears in the ffmpeg command line via buildCaptionArgs
	if !strings.Contains(out, "Alignment=5") {
		t.Errorf("expected Alignment=5 (center) in dry-run output, got:\n%s", out)
	}
	if !strings.Contains(out, "BorderStyle=3") {
		t.Errorf("expected BorderStyle=3 (boxed) in dry-run output, got:\n%s", out)
	}
}

func TestCreativeAssembleDryRun_DefaultMarginByPlatform(t *testing.T) {
	planID, srtPath := makeCaptionTestPlan(t)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:          true,
		BurnCaptions:    true,
		CaptionsPath:    srtPath,
		Platform:        "tiktok",
		CaptionPosition: "auto",
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "160") {
		t.Errorf("expected margin=160 for tiktok platform, got:\n%s", out)
	}
}

func TestCreativeAssemble_InvalidCaptionPositionRejected(t *testing.T) {
	planID, _ := makeCaptionTestPlan(t)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:          true,
		BurnCaptions:    true,
		CaptionPosition: "diagonal",
	}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error for unknown caption position, got nil")
	}
	if !strings.Contains(err.Error(), "diagonal") {
		t.Errorf("error should mention the bad value, got: %v", err)
	}
}

func TestCreativeAssemble_InvalidCaptionStyleRejected(t *testing.T) {
	planID, _ := makeCaptionTestPlan(t)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:       true,
		BurnCaptions: true,
		CaptionStyle: "neon",
	}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error for unknown caption style, got nil")
	}
}

// ---- voiceover stability/similarity validation ----

func makeCaptionVoicePlan(t *testing.T) (planDir string, planID string) {
	t.Helper()
	planID = "cap-voice-001"
	dir := t.TempDir()
	planDir = filepath.Join(dir, ".byom-video", "creative_plans", planID)
	if err := os.MkdirAll(filepath.Join(planDir, "outputs"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{
		"schema_version": "creative_plan.v1",
		"plan_id":        planID,
		"goal":           "test",
		"steps":          []any{},
	}
	writeTestJSON(t, filepath.Join(planDir, "creative_plan.json"), plan)
	writeTestVoiceoverText(t, planDir, "test narration text for voiceover generation")
	mustChdir(t, dir)
	return planDir, planID
}

func TestGenerateVoiceover_InvalidStabilityRejected(t *testing.T) {
	_, planID := makeCaptionVoicePlan(t)

	// stability > 1 should fail after route resolution — write a minimal config
	if err := os.WriteFile("byom-video.yaml", []byte(`tools:
  enabled: true
  backends:
    vb:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: http://localhost:9999
      auth:
        type: header_env
        header: xi-api-key
        env: TEST_STAB_KEY
  routes:
    creative.voiceover: vb
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_STAB_KEY", "dummy-key")

	var buf bytes.Buffer
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{
		DryRun:    true,
		Stability: 1.5, // > 1
	}, nil)
	if err == nil {
		t.Fatal("expected error for stability > 1, got nil")
	}
	if !strings.Contains(err.Error(), "stability") {
		t.Errorf("error should mention stability, got: %v", err)
	}
}

func TestGenerateVoiceover_InvalidSimilarityBoostRejected(t *testing.T) {
	_, planID := makeCaptionVoicePlan(t)

	if err := os.WriteFile("byom-video.yaml", []byte(`tools:
  enabled: true
  backends:
    vb:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: http://localhost:9999
      auth:
        type: header_env
        header: xi-api-key
        env: TEST_SIM_KEY
  routes:
    creative.voiceover: vb
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SIM_KEY", "dummy-key")

	var buf bytes.Buffer
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{
		DryRun:          true,
		Stability:       0.5,
		SimilarityBoost: 1.5, // > 1 (invalid)
	}, nil)
	if err == nil {
		t.Fatal("expected error for similarity-boost > 1, got nil")
	}
	if !strings.Contains(err.Error(), "similarity") {
		t.Errorf("error should mention similarity, got: %v", err)
	}
}

func TestGenerateVoiceover_ValidStabilityPassesThrough(t *testing.T) {
	_, planID := makeCaptionVoicePlan(t)

	if err := os.WriteFile("byom-video.yaml", []byte(`tools:
  enabled: true
  backends:
    vb:
      kind: voice_generation
      provider: elevenlabs-compatible
      model: eleven_multilingual_v2
      endpoint: http://localhost:9999
      auth:
        type: header_env
        header: xi-api-key
        env: TEST_VALID_KEY
  routes:
    creative.voiceover: vb
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_VALID_KEY", "dummy-key")

	var buf bytes.Buffer
	err := generateVoiceoverWithClient(planID, &buf, GenerateVoiceoverOptions{
		DryRun:          true,
		Stability:       0.3,
		SimilarityBoost: 0.9,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error for valid stability/similarity: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0.30") {
		t.Errorf("expected stability 0.30 in output, got:\n%s", out)
	}
	if !strings.Contains(out, "0.90") {
		t.Errorf("expected similarity 0.90 in output, got:\n%s", out)
	}
}

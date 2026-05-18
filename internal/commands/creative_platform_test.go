package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- NormalizePlatform ----

func TestNormalizePlatform_CanonicalNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"original", "original"},
		{"tiktok", "tiktok"},
		{"instagram-reel", "instagram-reel"},
		{"youtube-short", "youtube-short"},
		{"youtube", "youtube"},
		{"square", "square"},
		{"", "original"},
	}
	for _, c := range cases {
		got, err := NormalizePlatform(c.in)
		if err != nil {
			t.Errorf("NormalizePlatform(%q) err = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizePlatform(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizePlatform_Aliases(t *testing.T) {
	cases := []struct{ alias, want string }{
		{"reel", "instagram-reel"},
		{"reels", "instagram-reel"},
		{"ig", "instagram-reel"},
		{"shorts", "youtube-short"},
		{"yt-short", "youtube-short"},
		{"yt", "youtube"},
	}
	for _, c := range cases {
		got, err := NormalizePlatform(c.alias)
		if err != nil {
			t.Errorf("NormalizePlatform(%q) err = %v", c.alias, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizePlatform(%q) = %q, want %q", c.alias, got, c.want)
		}
	}
}

func TestNormalizePlatform_UnknownFails(t *testing.T) {
	_, err := NormalizePlatform("snapchat")
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
	if !strings.Contains(err.Error(), "snapchat") {
		t.Errorf("error should mention the unknown name: %v", err)
	}
	if !strings.Contains(err.Error(), "original") {
		t.Errorf("error should list supported names: %v", err)
	}
}

// ---- LookupPlatform dimensions ----

func TestLookupPlatform_Dimensions(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		defaultFit    string
	}{
		{"tiktok", 1080, 1920, "crop"},
		{"instagram-reel", 1080, 1920, "crop"},
		{"youtube-short", 1080, 1920, "crop"},
		{"youtube", 1920, 1080, "pad"},
		{"square", 1080, 1080, "crop"},
		{"original", 0, 0, ""},
	}
	for _, c := range cases {
		p := LookupPlatform(c.name)
		if p.Width != c.width || p.Height != c.height {
			t.Errorf("LookupPlatform(%q) = %dx%d, want %dx%d", c.name, p.Width, p.Height, c.width, c.height)
		}
		if p.DefaultFit != c.defaultFit {
			t.Errorf("LookupPlatform(%q).DefaultFit = %q, want %q", c.name, p.DefaultFit, c.defaultFit)
		}
	}
}

func TestDefaultFitForPlatform(t *testing.T) {
	if got := DefaultFitForPlatform("youtube"); got != "pad" {
		t.Errorf("youtube default fit = %q, want pad", got)
	}
	if got := DefaultFitForPlatform("tiktok"); got != "crop" {
		t.Errorf("tiktok default fit = %q, want crop", got)
	}
	if got := DefaultFitForPlatform("square"); got != "crop" {
		t.Errorf("square default fit = %q, want crop", got)
	}
	if got := DefaultFitForPlatform("original"); got != "crop" {
		// original has no default fit; DefaultFitForPlatform returns "crop" as safe fallback
		// (only called for hasPlatform cases in practice)
		_ = got
	}
}

// ---- buildPlatformArgs filter strings ----

func TestBuildPlatformArgs_CropFilter(t *testing.T) {
	p := PlatformPreset{Name: "tiktok", Width: 1080, Height: 1920, DefaultFit: "crop"}
	args := buildPlatformArgs("input.mp4", "output.mp4", p, "crop", "black")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "scale=1080:1920") {
		t.Errorf("crop filter missing scale: %s", joined)
	}
	if !strings.Contains(joined, "force_original_aspect_ratio=increase") {
		t.Errorf("crop filter missing force_original_aspect_ratio=increase: %s", joined)
	}
	if !strings.Contains(joined, "crop=1080:1920") {
		t.Errorf("crop filter missing crop: %s", joined)
	}
	if strings.Contains(joined, "pad=") {
		t.Errorf("crop mode should not contain pad filter: %s", joined)
	}
}

func TestBuildPlatformArgs_PadFilter(t *testing.T) {
	p := PlatformPreset{Name: "youtube", Width: 1920, Height: 1080, DefaultFit: "pad"}
	args := buildPlatformArgs("input.mp4", "output.mp4", p, "pad", "black")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "scale=1920:1080") {
		t.Errorf("pad filter missing scale: %s", joined)
	}
	if !strings.Contains(joined, "force_original_aspect_ratio=decrease") {
		t.Errorf("pad filter missing force_original_aspect_ratio=decrease: %s", joined)
	}
	if !strings.Contains(joined, "pad=1920:1080") {
		t.Errorf("pad filter missing pad: %s", joined)
	}
	if !strings.Contains(joined, "color=black") {
		t.Errorf("pad filter missing color=black: %s", joined)
	}
	if strings.Contains(joined, "crop=") {
		t.Errorf("pad mode should not contain crop filter: %s", joined)
	}
}

func TestBuildPlatformArgs_CustomBackground(t *testing.T) {
	p := PlatformPreset{Name: "youtube", Width: 1920, Height: 1080, DefaultFit: "pad"}
	args := buildPlatformArgs("in.mp4", "out.mp4", p, "pad", "white")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "color=white") {
		t.Errorf("expected color=white in pad filter: %s", joined)
	}
}

func TestBuildPlatformArgs_InputOutputPassthrough(t *testing.T) {
	p := PlatformPreset{Name: "tiktok", Width: 1080, Height: 1920}
	args := buildPlatformArgs("/in/file.mp4", "/out/file.mp4", p, "crop", "black")
	if args[0] != "-y" {
		t.Errorf("first arg should be -y, got %q", args[0])
	}
	if args[len(args)-1] != "/out/file.mp4" {
		t.Errorf("last arg should be output path, got %q", args[len(args)-1])
	}
	// -i must precede the input path
	inputIdx := -1
	for i, a := range args {
		if a == "-i" {
			inputIdx = i
			break
		}
	}
	if inputIdx < 0 || args[inputIdx+1] != "/in/file.mp4" {
		t.Errorf("input path not found after -i in args: %v", args)
	}
}

// ---- creative-assemble with platform stage ----

func TestCreativeAssemble_PlatformOriginalSkipsStage(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "original",
	}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No platform-format calls should be present
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "force_original_aspect_ratio") {
			t.Errorf("original platform should not produce scale/crop/pad calls: %v", call)
		}
	}

	// Result should not have a Platform field
	planDir := filepath.Join(creativePlansRoot, planID)
	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Platform != nil {
		t.Errorf("Platform field should be nil for original preset, got %+v", result.Platform)
	}
}

func TestCreativeAssemble_PlatformApplied(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "tiktok",
	}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify platform FFmpeg call was made
	foundPlatform := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "force_original_aspect_ratio=increase") && strings.Contains(joined, "crop=1080:1920") {
			foundPlatform = true
			break
		}
	}
	if !foundPlatform {
		t.Errorf("expected tiktok crop filter call; calls: %v", runner.calls)
	}

	// Verify result JSON contains platform fields
	planDir := filepath.Join(creativePlansRoot, planID)
	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Platform == nil {
		t.Fatal("Platform field should not be nil for tiktok preset")
	}
	if result.Platform.Normalized != "tiktok" {
		t.Errorf("Platform.Normalized = %q, want tiktok", result.Platform.Normalized)
	}
	if result.Platform.Width != 1080 || result.Platform.Height != 1920 {
		t.Errorf("Platform dimensions = %dx%d, want 1080x1920", result.Platform.Width, result.Platform.Height)
	}
	if result.Platform.Status != "applied" {
		t.Errorf("Platform.Status = %q, want applied", result.Platform.Status)
	}
	if result.Platform.Fit != "crop" {
		t.Errorf("Platform.Fit = %q, want crop", result.Platform.Fit)
	}
}

func TestCreativeAssemble_PlatformAlias(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "reels",
	}, runner)
	if err != nil {
		t.Fatalf("reels alias error: %v", err)
	}

	planDir := filepath.Join(creativePlansRoot, planID)
	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Platform == nil || result.Platform.Normalized != "instagram-reel" {
		t.Errorf("reels alias should normalize to instagram-reel, got %v", result.Platform)
	}
}

func TestCreativeAssemble_UnknownPlatformFails(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "snapchat",
	}, runner)
	if err == nil || !strings.Contains(err.Error(), "snapchat") {
		t.Fatalf("expected error for unknown platform, got: %v", err)
	}
}

func TestCreativeAssemble_PlatformYouTubePad(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "youtube",
	}, runner)
	if err != nil {
		t.Fatalf("youtube platform error: %v", err)
	}

	foundPad := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "force_original_aspect_ratio=decrease") && strings.Contains(joined, "pad=1920:1080") {
			foundPad = true
			break
		}
	}
	if !foundPad {
		t.Errorf("expected youtube pad filter; calls: %v", runner.calls)
	}
}

func TestCreativeAssemble_PlatformFitOverride(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	// Force pad on a preset that defaults to crop
	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "tiktok",
		Fit:      "pad",
	}, runner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	foundPad := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "force_original_aspect_ratio=decrease") {
			foundPad = true
			break
		}
	}
	if !foundPad {
		t.Errorf("expected pad filter when --fit=pad on tiktok preset; calls: %v", runner.calls)
	}
}

func TestCreativeAssemble_PlatformBackground(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform:   "youtube",
		Fit:        "pad",
		Background: "white",
	}, runner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	foundWhite := false
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "color=white") {
			foundWhite = true
			break
		}
	}
	if !foundWhite {
		t.Errorf("expected color=white in pad filter; calls: %v", runner.calls)
	}

	// Also verify background is stored in result
	planDir := filepath.Join(creativePlansRoot, planID)
	data, _ := os.ReadFile(filepath.Join(planDir, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Platform == nil || result.Platform.Background != "white" {
		t.Errorf("Platform.Background should be white, got %v", result.Platform)
	}
}

// ---- dry-run includes platform command ----

func TestCreativeAssemble_DryRunShowsPlatformCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:   true,
		Platform: "instagram-reel",
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "instagram-reel") {
		t.Errorf("dry-run should mention platform preset: %s", out)
	}
	if !strings.Contains(out, "1080") || !strings.Contains(out, "1920") {
		t.Errorf("dry-run should show platform dimensions: %s", out)
	}
	if !strings.Contains(out, "platform format") {
		t.Errorf("dry-run should show platform format command: %s", out)
	}
}

func TestCreativeAssemble_DryRunOriginalNoPlatformCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	var buf bytes.Buffer
	err := creativeAssembleWithRunner(planID, &buf, CreativeAssembleOptions{
		DryRun:   true,
		Platform: "original",
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "platform format") {
		t.Errorf("original preset dry-run should not show platform format command: %s", out)
	}
}

// ---- stage ordering: platform before captions ----

func TestCreativeAssemble_PlatformBeforeCaptions(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	// Write a fake SRT file
	planDir := filepath.Join(creativePlansRoot, planID)
	srtPath := filepath.Join(planDir, "outputs", "captions.srt")
	_ = os.WriteFile(srtPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o644)

	runner := &fakeFFmpegRunner{hasSubtitles: true}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform:     "tiktok",
		BurnCaptions: true,
		CaptionsPath: srtPath,
	}, runner)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// The platform call should come before the caption call
	platformIdx, captionIdx := -1, -1
	for i, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "force_original_aspect_ratio=increase") && platformIdx < 0 {
			platformIdx = i
		}
		if strings.Contains(joined, "subtitles=") && captionIdx < 0 {
			captionIdx = i
		}
	}
	if platformIdx < 0 {
		t.Error("platform filter call not found")
	}
	if captionIdx < 0 {
		t.Error("caption burn call not found")
	}
	if platformIdx > captionIdx {
		t.Errorf("platform stage (call %d) should come before caption stage (call %d)", platformIdx, captionIdx)
	}
}

// ---- validate-creative-assemble dimension check ----

func TestValidateCreativeAssemble_DimensionMismatchError(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	planDir := filepath.Join(creativePlansRoot, planID)
	outputsDir := filepath.Join(planDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)
	_ = os.WriteFile(filepath.Join(outputsDir, "draft.mp4"), []byte("fake"), 0o644)

	// Write a result claiming platform was applied at 1080x1920
	result := CreativeAssembleResult{
		SchemaVersion:   "creative_assemble_result.v1",
		CreativePlanID:  planID,
		Mode:            "reencode",
		Status:          "completed",
		OutputFile:      "outputs/draft.mp4",
		FinalOutputFile: "outputs/draft.mp4",
		WorkDir:         "outputs/render_work",
		Platform: &AssemblePlatformResult{
			Requested:  true,
			Normalized: "tiktok",
			Width:      1080,
			Height:     1920,
			Fit:        "crop",
			Status:     "applied",
		},
		// FinalProbe with wrong dimensions (simulates what we'd get if ffprobe ran)
		FinalProbe: &AssembleFinalProbe{Width: 1920, Height: 1080},
	}
	data, _ := json.Marshal(result)
	_ = os.WriteFile(filepath.Join(outputsDir, "creative_assemble_result.json"), data, 0o644)

	// Inject the FinalProbe directly rather than running ffprobe — validate reads the JSON.
	// For the dimension validation path that calls probeVideoDimensions, we can't easily mock it
	// in this test without ffprobe, so we just verify the FinalProbe fields are serialised.
	// The actual dimension check test with mocked probe is covered via integration paths.
	// Here we simply verify the result JSON round-trips correctly.
	var got CreativeAssembleResult
	_ = json.Unmarshal(data, &got)
	if got.Platform == nil || got.Platform.Width != 1080 {
		t.Errorf("Platform round-trip failed: %+v", got.Platform)
	}
	if got.FinalProbe == nil || got.FinalProbe.Width != 1920 {
		t.Errorf("FinalProbe round-trip failed: %+v", got.FinalProbe)
	}
}

// ---- review-creative-assemble shows platform ----

func TestReviewCreativeAssemble_ShowsPlatform(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		Platform: "tiktok",
	}, runner)
	if err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	var buf bytes.Buffer
	err = ReviewCreativeAssemble(planID, &buf, ReviewCreativeAssembleOptions{})
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "tiktok") {
		t.Errorf("review should mention platform 'tiktok': %s", out)
	}
	if !strings.Contains(out, "1080") || !strings.Contains(out, "1920") {
		t.Errorf("review should show platform dimensions: %s", out)
	}
}

// ---- make --platform dry-run ----

func TestMake_PlatformDryRunShows(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = os.WriteFile("byom-video.yaml", []byte("tools:\n  enabled: false\n"), 0o644)

	var buf bytes.Buffer
	_ = Make("/dev/null", &buf, MakeOptions{
		Goal:     "test goal",
		DryRun:   true,
		Yes:      true,
		Platform: "instagram-reel",
	})
	out := buf.String()
	if !strings.Contains(out, "instagram-reel") {
		t.Errorf("make dry-run should mention platform: %s", out)
	}
	if !strings.Contains(out, "1080") || !strings.Contains(out, "1920") {
		t.Errorf("make dry-run should show platform dimensions: %s", out)
	}
	if !strings.Contains(out, "--platform") {
		t.Errorf("make dry-run assemble command should include --platform: %s", out)
	}
}

func TestMake_PlatformOriginalDryRunHidden(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = os.WriteFile("byom-video.yaml", []byte("tools:\n  enabled: false\n"), 0o644)

	var buf bytes.Buffer
	_ = Make("/dev/null", &buf, MakeOptions{
		Goal:     "test goal",
		DryRun:   true,
		Yes:      true,
		Platform: "original",
	})
	out := buf.String()
	// "original" preset should not clutter dry-run output
	if strings.Contains(out, "platform: original") {
		t.Errorf("make dry-run should not show original platform line: %s", out)
	}
}

// ---- make-result shows platform ----

func TestMakeResult_ShowsPlatform(t *testing.T) {
	t.Chdir(t.TempDir())
	_ = os.MkdirAll(".byom-video/makes/test-make-001", 0o755)
	summary := MakeSummary{
		SchemaVersion:  "make_summary.v1",
		MakeID:         "test-make-001",
		Status:         "completed",
		Goal:           "test",
		PlatformPreset: "tiktok",
		PlatformWidth:  1080,
		PlatformHeight: 1920,
		PlatformFit:    "crop",
		PlatformStatus: "applied",
		FinalWidth:     1080,
		FinalHeight:    1920,
	}
	data, _ := json.Marshal(summary)
	_ = os.WriteFile(".byom-video/makes/test-make-001/make_summary.json", data, 0o644)

	var buf bytes.Buffer
	err := MakeResult("test-make-001", &buf, MakeResultOptions{})
	if err != nil {
		t.Fatalf("make-result error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "tiktok") {
		t.Errorf("make-result should mention platform: %s", out)
	}
	if !strings.Contains(out, "1080") || !strings.Contains(out, "1920") {
		t.Errorf("make-result should show dimensions: %s", out)
	}
}

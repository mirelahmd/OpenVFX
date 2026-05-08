package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- fake ffmpeg runner ----

type fakeFFmpegRunner struct {
	calls [][]string
	err   error
}

func (f *fakeFFmpegRunner) Run(args []string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if f.err != nil {
		return []byte("fake ffmpeg error"), f.err
	}
	// create the output file so existence checks pass
	if len(args) > 0 {
		out := args[len(args)-1]
		if strings.HasSuffix(out, ".mp4") {
			_ = os.MkdirAll(filepath.Dir(out), 0o755)
			_ = os.WriteFile(out, []byte("fake-mp4"), 0o644)
		}
	}
	return []byte("ok"), nil
}

// makeTimelineWithClips builds an approved plan with stub outputs and a timeline with real source clips.
func makeTimelineWithClips(t *testing.T, inputPath string) string {
	t.Helper()
	planID := makeApprovedStubPlan(t, "make a cinematic short with narration")
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatalf("stub execution error: %v", err)
	}

	// Build a timeline manually with source_clip items pointing at inputPath
	tl := CreativeTimelineArtifact{
		SchemaVersion:  "creative_timeline.v1",
		CreativePlanID: planID,
		Goal:           "test",
		InputPath:      inputPath,
		Mode:           "stub",
		Source:         CreativeTimelineSource{ClipCount: 2, StubOutputs: true},
		Tracks: []CreativeTimelineTrack{
			{
				ID:   "track_video_main",
				Kind: "video",
				Items: []CreativeTimelineItem{
					{ID: "clip_0001", Kind: "source_clip", TimelineStart: 0, TimelineEnd: 5, SourceStart: 0, SourceEnd: 5, Text: "hello"},
					{ID: "clip_0002", Kind: "source_clip", TimelineStart: 5, TimelineEnd: 12, SourceStart: 10, SourceEnd: 17, Text: "world"},
				},
			},
			{ID: "track_voiceover", Kind: "audio", Items: []CreativeTimelineItem{{ID: "vo_main", Kind: "voiceover_placeholder", TimelineStart: 0, TimelineEnd: 12}}},
			{ID: "track_captions", Kind: "text", Items: []CreativeTimelineItem{}},
			{ID: "track_visual_overlays", Kind: "visual", Items: []CreativeTimelineItem{}},
		},
		TotalDuration: 12,
	}
	outDir := filepath.Join(creativePlansRoot, planID, "outputs")
	_ = os.MkdirAll(outDir, 0o755)
	if err := writeJSONFile(filepath.Join(outDir, "creative_timeline.json"), tl); err != nil {
		t.Fatalf("write timeline: %v", err)
	}

	// Build a render plan
	rp := CreativeRenderPlanArtifact{
		SchemaVersion:  "creative_render_plan.v1",
		CreativePlanID: planID,
		Mode:           "stub",
		Source:         CreativeRenderPlanSource{TimelineArtifact: "outputs/creative_timeline.json"},
		PlannedOutput:  CreativeRenderOutput{PlannedFile: "outputs/draft.mp4"},
		Steps:          []CreativeRenderStep{{StepIndex: 0, Operation: "cut_source_clip"}},
	}
	if err := writeJSONFile(filepath.Join(outDir, "creative_render_plan.json"), rp); err != nil {
		t.Fatalf("write render plan: %v", err)
	}

	return planID
}

func TestCreativeAssemble_DryRunWritesNothing(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	var out bytes.Buffer
	err := creativeAssembleWithRunner(planID, &out, CreativeAssembleOptions{DryRun: true}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "no files written") {
		t.Fatalf("expected 'no files written' in dry-run output: %s", text)
	}
	if !strings.Contains(text, "ffmpeg") {
		t.Fatalf("expected ffmpeg commands in dry-run output: %s", text)
	}
	// no draft or work_dir should be created
	planDir := filepath.Join(creativePlansRoot, planID)
	if _, err := os.Stat(filepath.Join(planDir, "outputs", "draft.mp4")); err == nil {
		t.Fatal("draft.mp4 should not exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(planDir, "outputs", "render_work")); err == nil {
		t.Fatal("render_work/ should not exist after dry-run")
	}
}

func TestCreativeAssemble_RefusesOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	// first run
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	// second run without --overwrite
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error without --overwrite")
	}
	if !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_OverwriteSucceeds(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, runner); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	runner2 := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{Overwrite: true}, runner2); err != nil {
		t.Fatalf("overwrite run error: %v", err)
	}
}

func TestCreativeAssemble_NoSourceClipsFails(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a cinematic short")
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	// build timeline with empty video track
	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeRenderPlan(planID, ioDiscard{}, CreativeRenderPlanOptions{}); err != nil {
		t.Fatal(err)
	}

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error for empty video track")
	}
	if !strings.Contains(err.Error(), "no source clips") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_WritesResultJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	resultPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal("creative_assemble_result.json not created")
	}
	var result CreativeAssembleResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("result JSON invalid: %v", err)
	}
	if result.SchemaVersion != "creative_assemble_result.v1" {
		t.Fatalf("schema_version = %v", result.SchemaVersion)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %v", result.Status)
	}
	if result.OutputFile != "outputs/draft.mp4" {
		t.Fatalf("output_file = %v", result.OutputFile)
	}
	if len(result.Clips) == 0 {
		t.Fatal("expected clips in result")
	}
}

func TestCreativeAssemble_UsesArgsNotShellString(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, runner); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	if len(runner.calls) == 0 {
		t.Fatal("expected ffmpeg calls")
	}
	for _, call := range runner.calls {
		// Each call must be a slice of args, not a single shell string
		for _, arg := range call {
			if strings.Contains(arg, " && ") || strings.Contains(arg, ";") || strings.Contains(arg, "|") {
				t.Fatalf("shell injection detected in args: %v", call)
			}
		}
		// First arg should not be "ffmpeg" (that's the command, provided by runner)
		if len(call) > 0 && call[0] == "ffmpeg" {
			t.Fatalf("args should not include 'ffmpeg' itself: %v", call)
		}
	}
}

func TestCreativeAssemble_MaxClips(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{MaxClips: 1}, runner); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	// 1 clip cut + 1 remux = 2 ffmpeg calls
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 ffmpeg calls for 1 clip, got %d", len(runner.calls))
	}
	// result should have 1 clip
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if len(result.Clips) != 1 {
		t.Fatalf("expected 1 clip in result, got %d", len(result.Clips))
	}
}

func TestCreativeAssemble_UpdatesOutputsIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json"))
	var idx CreativeOutputsIndex
	_ = json.Unmarshal(data, &idx)
	found := map[string]bool{}
	for _, a := range idx.Artifacts {
		found[a.Type] = true
	}
	if !found["creative_assemble_result"] {
		t.Fatal("creative_assemble_result not in outputs index")
	}
	if !found["draft_video"] {
		t.Fatal("draft_video not in outputs index")
	}
}

func TestValidateCreativeAssemble_MissingDraftFails(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	// delete the draft.mp4
	_ = os.Remove(filepath.Join(creativePlansRoot, planID, "outputs", "draft.mp4"))

	err := ValidateCreativeAssemble(planID, ioDiscard{}, ValidateCreativeAssembleOptions{})
	if err == nil {
		t.Fatal("expected validation error for missing draft.mp4")
	}
}

func TestValidateCreativeAssemble_ValidPass(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	if err := ValidateCreativeAssemble(planID, ioDiscard{}, ValidateCreativeAssembleOptions{}); err != nil {
		t.Fatalf("validate error: %v", err)
	}
}

func TestReviewCreativeAssemble_WritesMarkdown(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	var out bytes.Buffer
	if err := ReviewCreativeAssemble(planID, &out, ReviewCreativeAssembleOptions{WriteArtifact: true}); err != nil {
		t.Fatalf("review error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "# Creative Assemble Review") {
		t.Fatalf("missing header: %s", text)
	}
	reviewPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_review.md")
	if _, err := os.Stat(reviewPath); err != nil {
		t.Fatal("creative_assemble_review.md not created")
	}
}

func TestInspectCreativePlan_ShowsAssembleResult(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	var out bytes.Buffer
	if err := InspectCreativePlan(planID, &out, InspectCreativePlanOptions{}); err != nil {
		t.Fatalf("inspect error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "assemble status:") {
		t.Fatalf("expected assemble status in inspect: %s", text)
	}
	if !strings.Contains(text, "draft output:") {
		t.Fatalf("expected draft output in inspect: %s", text)
	}
}

func TestCreativeResult_ShowsDraftPath(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	var out bytes.Buffer
	if err := CreativeResult(planID, &out, CreativeResultOptions{}); err != nil {
		t.Fatalf("creative-result error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "draft:") {
		t.Fatalf("expected draft path in creative-result: %s", text)
	}
}

func TestCreativeAssemble_RequiresTimeline(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a cinematic short")

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error when timeline missing")
	}
	if !strings.Contains(err.Error(), "creative-timeline") && !strings.Contains(err.Error(), "creative_timeline.json") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_RequiresRenderPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	planID := makeApprovedStubPlan(t, "make a cinematic short with narration")
	if err := CreativeExecuteStub(planID, ioDiscard{}, CreativeExecuteStubOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := CreativeTimeline(planID, ioDiscard{}, CreativeTimelineOptions{}); err != nil {
		t.Fatal(err)
	}
	// no render plan

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error when render plan missing")
	}
	if !strings.Contains(err.Error(), "creative-render-plan") && !strings.Contains(err.Error(), "creative_render_plan.json") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_DryRunPrintsAllClipCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	var out bytes.Buffer
	err := creativeAssembleWithRunner(planID, &out, CreativeAssembleOptions{DryRun: true, Mode: "stream-copy"}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	// should show 2 clip commands (timeline has 2 clips)
	clipCount := strings.Count(text, "ffmpeg")
	if clipCount < 2 {
		t.Fatalf("expected at least 2 ffmpeg commands, got %d: %s", clipCount, text)
	}
	if !strings.Contains(text, "stream-copy") && !strings.Contains(text, "-c copy") {
		t.Fatalf("expected stream-copy args in dry-run output: %s", text)
	}
}

func TestCreativeAssemble_PatchesPlanExecutionStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	planRaw, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "creative_plan.json"))
	var pm map[string]any
	_ = json.Unmarshal(planRaw, &pm)
	if pm["execution_status"] != "assembled" {
		t.Fatalf("execution_status = %v, want assembled", pm["execution_status"])
	}
}

func TestCreativeAssemble_ModeReencode(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{Mode: "reencode"}, runner); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	// check clip args contain libx264
	found := false
	for _, call := range runner.calls {
		for _, arg := range call {
			if arg == "libx264" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected libx264 in reencode mode args; calls: %v", runner.calls)
	}
}

func TestCreativeAssemble_ModeStreamCopy(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	runner := &fakeFFmpegRunner{}
	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{Mode: "stream-copy"}, runner); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	// check clip args contain -c copy but NOT libx264
	hasCopy := false
	hasLibx264 := false
	for _, call := range runner.calls {
		for _, arg := range call {
			if arg == "copy" {
				hasCopy = true
			}
			if arg == "libx264" {
				hasLibx264 = true
			}
		}
	}
	if !hasCopy {
		t.Fatalf("expected '-c copy' in stream-copy mode; calls: %v", runner.calls)
	}
	if hasLibx264 {
		t.Fatalf("expected no libx264 in stream-copy mode; calls: %v", runner.calls)
	}
}

func TestValidateCreativePlan_ValidatesAssembleResult(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	// valid — should pass
	if err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{}); err != nil {
		t.Fatalf("validate error: %v", err)
	}

	// corrupt schema_version in assemble result
	resultPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json")
	raw, _ := os.ReadFile(resultPath)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["schema_version"] = "wrong.v0"
	data, _ := json.Marshal(m)
	_ = os.WriteFile(resultPath, data, 0o644)

	err := ValidateCreativePlan(planID, ioDiscard{}, ValidateCreativePlanOptions{})
	if err == nil {
		t.Fatal("expected validation error for wrong assemble result schema_version")
	}
}

// --- Prompt 045: captions and voiceover tests ---

func TestEscapeFilterPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple.srt", "simple.srt"},
		{"/path/to/file.srt", "/path/to/file.srt"},
		{"/path with:colon.srt", "/path with\\:colon.srt"},
		{"/path with 'quotes'.srt", "/path with \\'quotes\\'.srt"},
		{`C:\windows\path.srt`, `C\:\\windows\\path.srt`},
	}
	for _, c := range cases {
		got := escapeFilterPath(c.in)
		if got != c.want {
			t.Errorf("escapeFilterPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildVoiceoverArgs_UsesAmix(t *testing.T) {
	args := buildVoiceoverArgs("/in/video.mp4", "/in/vo.wav", "/out/audio.mp4")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "amix") {
		t.Fatalf("expected amix filter in voiceover args: %v", args)
	}
	if !strings.Contains(joined, "-map") {
		t.Fatalf("expected -map in voiceover args: %v", args)
	}
	// must not contain shell operators
	for _, a := range args {
		if strings.ContainsAny(a, "&;|") {
			t.Fatalf("shell operator in arg: %q", a)
		}
	}
}

func TestBuildCaptionArgs_UsesSubtitlesFilter(t *testing.T) {
	args := buildCaptionArgs("/in/video.mp4", "/in/caps.srt", "/out/final.mp4")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "subtitles=") {
		t.Fatalf("expected subtitles= filter in caption args: %v", args)
	}
	// must not contain shell operators
	for _, a := range args {
		if strings.ContainsAny(a, "&;|") {
			t.Fatalf("shell operator in arg: %q", a)
		}
	}
}

func TestCreativeAssemble_BurnCaptionsNoFileFails(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		BurnCaptions: true,
		// no CaptionsPath, no AllowMissingCaptions
	}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error for missing captions file")
	}
	if !strings.Contains(err.Error(), "caption") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_BurnCaptionsAllowMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	var out bytes.Buffer
	err := creativeAssembleWithRunner(planID, &out, CreativeAssembleOptions{
		BurnCaptions:         true,
		AllowMissingCaptions: true,
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("expected success with --allow-missing-captions: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Captions == nil {
		t.Fatal("expected captions field in result")
	}
	if !result.Captions.Requested {
		t.Fatal("expected captions.requested=true")
	}
	if result.Captions.Status != "skipped" {
		t.Fatalf("expected captions.status=skipped, got %s", result.Captions.Status)
	}
}

func TestCreativeAssemble_MixVoiceoverNoFileFails(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		MixVoiceover: true,
		// no VoiceoverPath, no AllowMissingVoiceover
	}, &fakeFFmpegRunner{})
	if err == nil {
		t.Fatal("expected error for missing voiceover file")
	}
	if !strings.Contains(err.Error(), "voiceover") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreativeAssemble_MixVoiceoverAllowMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		MixVoiceover:          true,
		AllowMissingVoiceover: true,
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("expected success with --allow-missing-voiceover: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Voiceover == nil {
		t.Fatal("expected voiceover field in result")
	}
	if result.Voiceover.Status != "skipped" {
		t.Fatalf("expected voiceover.status=skipped, got %s", result.Voiceover.Status)
	}
}

func TestCreativeAssemble_WithVoiceoverFile(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	// write a fake voiceover file
	voPath := filepath.Join(t.TempDir(), "vo.wav")
	_ = os.WriteFile(voPath, []byte("fake-audio"), 0o644)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		MixVoiceover:  true,
		VoiceoverPath: voPath,
	}, runner)
	if err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Voiceover == nil || result.Voiceover.Status != "applied" {
		t.Fatalf("expected voiceover.status=applied, got %+v", result.Voiceover)
	}
	// amix must appear in runner calls
	foundAmix := false
	for _, call := range runner.calls {
		for _, a := range call {
			if strings.Contains(a, "amix") {
				foundAmix = true
			}
		}
	}
	if !foundAmix {
		t.Fatalf("expected amix in ffmpeg calls; calls: %v", runner.calls)
	}
}

func TestCreativeAssemble_WithCaptionsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	captPath := filepath.Join(t.TempDir(), "captions.srt")
	_ = os.WriteFile(captPath, []byte("1\n00:00:00,000 --> 00:00:05,000\nHello\n"), 0o644)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		BurnCaptions: true,
		CaptionsPath: captPath,
	}, runner)
	if err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)
	if result.Captions == nil || result.Captions.Status != "applied" {
		t.Fatalf("expected captions.status=applied, got %+v", result.Captions)
	}
	foundSubtitles := false
	for _, call := range runner.calls {
		for _, a := range call {
			if strings.Contains(a, "subtitles=") {
				foundSubtitles = true
			}
		}
	}
	if !foundSubtitles {
		t.Fatalf("expected subtitles= in ffmpeg calls; calls: %v", runner.calls)
	}
}

func TestCreativeAssemble_StagedResult_BothVoiceoverAndCaptions(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	voPath := filepath.Join(t.TempDir(), "vo.wav")
	_ = os.WriteFile(voPath, []byte("fake-audio"), 0o644)
	captPath := filepath.Join(t.TempDir(), "captions.srt")
	_ = os.WriteFile(captPath, []byte("1\n00:00:00,000 --> 00:00:05,000\nHello\n"), 0o644)

	runner := &fakeFFmpegRunner{}
	err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		MixVoiceover:  true,
		VoiceoverPath: voPath,
		BurnCaptions:  true,
		CaptionsPath:  captPath,
	}, runner)
	if err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(creativePlansRoot, planID, "outputs", "creative_assemble_result.json"))
	var result CreativeAssembleResult
	_ = json.Unmarshal(data, &result)

	if result.Voiceover == nil || result.Voiceover.Status != "applied" {
		t.Fatalf("expected voiceover applied, got %+v", result.Voiceover)
	}
	if result.Captions == nil || result.Captions.Status != "applied" {
		t.Fatalf("expected captions applied, got %+v", result.Captions)
	}
	if len(result.Stages) < 3 {
		t.Fatalf("expected 3 stages (assembled_video, voiceover_mix, caption_burn), got %d: %v", len(result.Stages), result.Stages)
	}
	// final file should be draft.mp4
	if result.FinalOutputFile != "outputs/draft.mp4" {
		t.Fatalf("expected final_output_file=outputs/draft.mp4, got %s", result.FinalOutputFile)
	}
}

func TestCreativeAssemble_DryRun_ShowsCaptionStage(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	captPath := filepath.Join(t.TempDir(), "captions.srt")
	_ = os.WriteFile(captPath, []byte("1\n00:00:00,000 --> 00:00:05,000\nHello\n"), 0o644)

	var out bytes.Buffer
	err := creativeAssembleWithRunner(planID, &out, CreativeAssembleOptions{
		DryRun:       true,
		BurnCaptions: true,
		CaptionsPath: captPath,
	}, &fakeFFmpegRunner{})
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "caption") && !strings.Contains(text, "subtitles") {
		t.Fatalf("expected caption stage in dry-run output: %s", text)
	}
}

func TestReviewCreativeAssemble_ShowsCaptionsVoiceover(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "test.mov")
	_ = os.WriteFile(input, []byte("stub-video"), 0o644)
	planID := makeTimelineWithClips(t, input)

	voPath := filepath.Join(t.TempDir(), "vo.wav")
	_ = os.WriteFile(voPath, []byte("fake-audio"), 0o644)

	if err := creativeAssembleWithRunner(planID, ioDiscard{}, CreativeAssembleOptions{
		MixVoiceover:  true,
		VoiceoverPath: voPath,
	}, &fakeFFmpegRunner{}); err != nil {
		t.Fatalf("assemble error: %v", err)
	}
	var out bytes.Buffer
	if err := ReviewCreativeAssemble(planID, &out, ReviewCreativeAssembleOptions{}); err != nil {
		t.Fatalf("review error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "voiceover") {
		t.Fatalf("expected voiceover in review: %s", text)
	}
}

// helper to create temp input files with unique names
func init() {
	_ = fmt.Sprintf // suppress import warning
}

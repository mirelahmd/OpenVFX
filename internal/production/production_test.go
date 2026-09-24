package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

var testNow = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

// ---- fakes ----

type fakeProber struct {
	binaries  map[string]string
	filters   map[string]bool
	modules   map[string]bool
	filterErr error
}

func (f fakeProber) LookPath(name string) (string, error) {
	if p, ok := f.binaries[name]; ok {
		return p, nil
	}
	return "", errNotFound{name}
}
func (f fakeProber) Version(name string) (string, error) { return name + " 9.9", nil }
func (f fakeProber) FFmpegFilters() (map[string]bool, error) {
	if f.filterErr != nil {
		return nil, f.filterErr
	}
	return f.filters, nil
}
func (f fakeProber) PythonModule(module string) (bool, string) {
	if f.modules[module] {
		return true, "/fake/python"
	}
	return false, "import " + module + " failed"
}

type errNotFound struct{ name string }

func (e errNotFound) Error() string { return e.name + " not found" }

// fullProber is a machine where everything works.
func fullProber() fakeProber {
	return fakeProber{
		binaries: map[string]string{"ffmpeg": "/usr/bin/ffmpeg", "ffprobe": "/usr/bin/ffprobe"},
		filters: map[string]bool{
			"scale": true, "pad": true, "amix": true, "subtitles": true, "drawtext": true,
		},
		modules: map[string]bool{"faster_whisper": true},
	}
}

// noLibassProber reproduces the real condition on the audit machine: a working
// ffmpeg with no libass and no freetype.
func noLibassProber() fakeProber {
	p := fullProber()
	p.filters = map[string]bool{"scale": true, "pad": true, "amix": true}
	return p
}

func fakeProbeJSON(durationSec float64, w, h int, audio bool) []byte {
	doc := map[string]any{
		"format": map[string]any{"duration": formatFloat(durationSec)},
		"streams": []map[string]any{
			{"codec_type": "video", "codec_name": "h264", "width": w, "height": h, "r_frame_rate": "25/1"},
		},
	}
	if audio {
		streams := doc["streams"].([]map[string]any)
		doc["streams"] = append(streams, map[string]any{"codec_type": "audio", "codec_name": "aac"})
	}
	b, _ := json.Marshal(doc)
	return b
}

func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return strings.Trim(string(b), `"`)
}

func testObservation() Observation {
	return Observation{
		SchemaVersion: ObservationSchemaVersion,
		InputPath:     "/tmp/assets",
		Assets: []Asset{
			{ID: "asset_0001", Path: "/tmp/assets/a.mp4", DurationSeconds: 6, Width: 640, Height: 360, HasVideo: true, HasAudio: true},
			{ID: "asset_0002", Path: "/tmp/assets/b.mp4", DurationSeconds: 6, Width: 640, Height: 360, HasVideo: true, HasAudio: true},
		},
		Totals: Totals{AssetCount: 2, DurationSeconds: 12, WithAudio: 2, WithVideo: 2},
	}
}

// ---- capability probe ----

func TestProbeCapabilitiesReportsMissingFilterAsUnavailable(t *testing.T) {
	caps := ProbeCapabilities(noLibassProber(), config.Config{})

	if !caps.IsAvailable(CapFilterScale) {
		t.Fatalf("scale should be available")
	}
	if caps.IsAvailable(CapFilterSubtitles) {
		t.Fatalf("subtitles must be unavailable when absent from the filter list")
	}
	c, ok := caps.Get(CapFilterSubtitles)
	if !ok {
		t.Fatalf("subtitles capability was not probed at all")
	}
	if !strings.Contains(c.Detail, "libass") {
		t.Fatalf("detail should explain the cause, got %q", c.Detail)
	}
	if c.ProbedAt.IsZero() {
		t.Fatalf("probed_at must be set")
	}
}

func TestProbeCapabilitiesSidecarAlwaysAvailable(t *testing.T) {
	caps := ProbeCapabilities(fakeProber{}, config.Config{})
	if !caps.IsAvailable(CapSidecarSRT) {
		t.Fatalf("sidecar.srt is pure Go and must always be available")
	}
}

func TestProbeCapabilitiesWithoutFFmpegMarksFiltersUnavailable(t *testing.T) {
	caps := ProbeCapabilities(fakeProber{}, config.Config{})
	for _, id := range []string{CapFFmpeg, CapFilterScale, CapFilterAmix} {
		if caps.IsAvailable(id) {
			t.Fatalf("%s should be unavailable when ffmpeg is missing", id)
		}
	}
}

func TestParseFilterListReadsSecondField(t *testing.T) {
	out := " ... scale             V->V       Scale the input video.\n TS subtitles  V->V  Render subtitles.\n"
	filters := parseFilterList(out)
	if !filters["scale"] || !filters["subtitles"] {
		t.Fatalf("expected scale and subtitles, got %v", filters)
	}
}

// ---- observation ----

func TestObserveParsesProbeOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.mp4"), "x")
	writeFile(t, filepath.Join(dir, "b.mov"), "x")
	writeFile(t, filepath.Join(dir, "notes.txt"), "x")

	obs, err := Observe(dir, func(path string) ([]byte, error) {
		return fakeProbeJSON(6, 1920, 1080, strings.HasSuffix(path, "a.mp4")), nil
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if obs.Totals.AssetCount != 2 {
		t.Fatalf("expected 2 assets, got %d", obs.Totals.AssetCount)
	}
	if obs.Totals.WithAudio != 1 {
		t.Fatalf("expected 1 asset with audio, got %d", obs.Totals.WithAudio)
	}
	if len(obs.Skipped) != 1 || !strings.Contains(obs.Skipped[0].Reason, ".txt") {
		t.Fatalf("non-media file should be skipped with a reason, got %+v", obs.Skipped)
	}
	if obs.Totals.DurationSeconds != 12 {
		t.Fatalf("expected 12s total, got %v", obs.Totals.DurationSeconds)
	}
}

func TestObserveRecordsProbeFailureRatherThanAborting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good.mp4"), "x")
	writeFile(t, filepath.Join(dir, "bad.mp4"), "x")

	obs, err := Observe(dir, func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "bad.mp4") {
			return nil, errNotFound{"ffprobe"}
		}
		return fakeProbeJSON(4, 640, 360, true), nil
	})
	if err != nil {
		t.Fatalf("Observe should succeed when at least one asset probes: %v", err)
	}
	if obs.Totals.AssetCount != 1 {
		t.Fatalf("expected 1 usable asset, got %d", obs.Totals.AssetCount)
	}
	if len(obs.Skipped) != 1 {
		t.Fatalf("failed probe must be recorded as skipped, got %+v", obs.Skipped)
	}
}

// ---- intent ----

func TestParseIntentExtractsDurationAspectAndCaptions(t *testing.T) {
	obs := testObservation()
	intent := ParseIntent("Cut these into a 20-second vertical teaser with burned-in captions", obs)

	if intent.TargetDurationSeconds != 20 {
		t.Fatalf("expected 20s target, got %v", intent.TargetDurationSeconds)
	}
	if intent.AspectRatio != "9:16" || intent.Width != 1080 || intent.Height != 1920 {
		t.Fatalf("expected 9:16 1080x1920, got %s %dx%d", intent.AspectRatio, intent.Width, intent.Height)
	}
	if !intent.WantsCaptions || !intent.WantsBurnedCaptions {
		t.Fatalf("expected burned-in captions, got %+v", intent)
	}
	if !intent.WantsAudio {
		t.Fatalf("expected audio when sources carry audio")
	}
}

func TestParseIntentMinutesAndSquare(t *testing.T) {
	intent := ParseIntent("make a 2 minute square cut", testObservation())
	if intent.TargetDurationSeconds != 120 {
		t.Fatalf("expected 120s, got %v", intent.TargetDurationSeconds)
	}
	if intent.AspectRatio != "1:1" {
		t.Fatalf("expected 1:1, got %q", intent.AspectRatio)
	}
}

func TestParseIntentSidecarRequestIsNotBurnIn(t *testing.T) {
	intent := ParseIntent("30 second cut with captions as a sidecar srt file", testObservation())
	if !intent.WantsCaptions {
		t.Fatalf("captions should be requested")
	}
	if intent.WantsBurnedCaptions {
		t.Fatalf("explicit sidecar request must not be read as burn-in")
	}
}

func TestParseIntentSilentSuppressesAudioExpectation(t *testing.T) {
	intent := ParseIntent("10 second silent vertical clip", testObservation())
	if intent.WantsAudio {
		t.Fatalf("'silent' must suppress the audio expectation")
	}
}

// ---- planner ----

func TestPlanProducesValidatedStageDAG(t *testing.T) {
	plan, err := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("planner emitted an invalid plan: %v", err)
	}
	if plan.PlanVersion != 1 {
		t.Fatalf("first plan must be version 1, got %d", plan.PlanVersion)
	}
	types := map[string]bool{}
	for _, s := range plan.Stages {
		types[s.Type] = true
		if s.Requires == "" {
			t.Fatalf("stage %s declares no capability", s.ID)
		}
		if s.Requirements.Runtime == "" {
			t.Fatalf("stage %s declares no runtime requirement", s.ID)
		}
		if s.Binding != nil {
			t.Fatalf("stage %s was bound at plan time; binding must be late", s.ID)
		}
	}
	for _, want := range []string{StageTypeProbeAssets, StageTypeTranscribe, StageTypeSelectClips,
		StageTypeAssembleVideo, StageTypeFormatOutput, StageTypeTextOverlay} {
		if !types[want] {
			t.Fatalf("plan is missing a %s stage", want)
		}
	}
}

func TestPlanOmitsTranscriptionWithoutCaptionRequest(t *testing.T) {
	plan, err := Plan("prod_1", "make a 10 second vertical clip", testObservation(), testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, s := range plan.Stages {
		if s.Type == StageTypeTranscribe || s.Type == StageTypeTextOverlay {
			t.Fatalf("unexpected %s stage when captions were not requested", s.Type)
		}
	}
}

func TestPlanWarnsWhenCaptionsRequestedWithoutAudio(t *testing.T) {
	obs := testObservation()
	for i := range obs.Assets {
		obs.Assets[i].HasAudio = false
	}
	obs.Totals.WithAudio = 0

	plan, err := Plan("prod_1", "10 second clip with captions", obs, testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Warnings) == 0 {
		t.Fatalf("expected a warning when captions are impossible")
	}
}

func TestPlanValidateRejectsForwardDependency(t *testing.T) {
	plan := ProductionPlan{
		SchemaVersion: PlanSchemaVersion, ProductionID: "p", PlanVersion: 1,
		Stages: []Stage{
			{ID: "stage_0001", Type: "a", Requires: "c", Requirements: Requirements{Runtime: "r"}, DependsOn: []string{"stage_0002"}},
			{ID: "stage_0002", Type: "b", Requires: "c", Requirements: Requirements{Runtime: "r"}},
		},
	}
	if err := plan.Validate(); err == nil {
		t.Fatalf("expected a forward-dependency error")
	}
}

// ---- binding ----

func TestBindRefusesToSubstituteSilently(t *testing.T) {
	caps := ProbeCapabilities(noLibassProber(), config.Config{})
	e := &Executor{Caps: caps}

	stage := &Stage{ID: "stage_0006", Type: StageTypeTextOverlay, Requires: CapFilterSubtitles}
	if _, err := e.bind(stage); err == nil {
		t.Fatalf("executor must NOT silently fall back; substitution is the reviser's job")
	}
}

func TestBindMarksRevisedStageAsFallback(t *testing.T) {
	caps := ProbeCapabilities(noLibassProber(), config.Config{})
	e := &Executor{Caps: caps}

	// After revision the stage requires the substitute.
	stage := &Stage{ID: "stage_0006", Type: StageTypeTextOverlay, Requires: CapSidecarSRT}
	b, err := e.bind(stage)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if !b.Fallback {
		t.Fatalf("a substituted binding must be labelled as a fallback")
	}
	if b.Backend != CapSidecarSRT {
		t.Fatalf("expected sidecar binding, got %q", b.Backend)
	}
}

// ---- reviser ----

func TestReviseForCapabilitySubstitutesAndPreservesOriginal(t *testing.T) {
	caps := ProbeCapabilities(noLibassProber(), config.Config{})
	v1, err := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// Simulate everything before the overlay having completed.
	for i := range v1.Stages {
		if v1.Stages[i].Type != StageTypeTextOverlay {
			v1.Stages[i].Status = StageStatusCompleted
		}
	}
	overlay := ""
	for _, s := range v1.Stages {
		if s.Type == StageTypeTextOverlay {
			overlay = s.ID
		}
	}

	v2, rev, err := ReviseForCapability(v1, caps, overlay, testNow)
	if err != nil {
		t.Fatalf("ReviseForCapability: %v", err)
	}

	if v1.Stage(overlay).Requires != CapFilterSubtitles {
		t.Fatalf("plan v1 must not be mutated by revision")
	}
	if v2.Stage(overlay).Requires != CapSidecarSRT {
		t.Fatalf("expected substitution to sidecar.srt, got %q", v2.Stage(overlay).Requires)
	}
	if v2.PlanVersion != 2 || v2.DerivedFrom != 1 {
		t.Fatalf("expected v2 derived from v1, got version=%d derived=%d", v2.PlanVersion, v2.DerivedFrom)
	}
	if rev.Trigger != TriggerCapabilityUnavailable {
		t.Fatalf("unexpected trigger %q", rev.Trigger)
	}
	if !strings.Contains(rev.Reason, "libass") {
		t.Fatalf("revision reason must carry the probed cause, got %q", rev.Reason)
	}
	if len(rev.ReusedStages) != len(v1.Stages)-1 {
		t.Fatalf("expected %d reused stages, got %d", len(v1.Stages)-1, len(rev.ReusedStages))
	}
	// The substituted stage must declare its new, cheaper footprint.
	if v2.Stage(overlay).Requirements.CPUCores >= v1.Stage(overlay).Requirements.CPUCores {
		t.Fatalf("a cheaper backend should declare a smaller footprint")
	}
}

func TestReviseForCapabilityFailsWhenNoSubstituteExists(t *testing.T) {
	caps := CapabilitySet{Capabilities: []Capability{
		{ID: CapFFmpeg, Status: CapUnavailable},
	}}
	plan := ProductionPlan{
		SchemaVersion: PlanSchemaVersion, ProductionID: "p", PlanVersion: 1,
		Stages: []Stage{{
			ID: "stage_0001", Type: StageTypeAssembleVideo, Requires: CapFFmpeg,
			Requirements: Requirements{Runtime: "ffmpeg>=6"}, Status: StageStatusBlocked,
		}},
	}
	if _, _, err := ReviseForCapability(plan, caps, "stage_0001", testNow); err == nil {
		t.Fatalf("expected an error when no substitute capability is available")
	}
}

func TestReviseForValidationExtendsShortOutput(t *testing.T) {
	v1, err := Plan("prod_1", "20-second vertical clip", testObservation(), testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for i := range v1.Stages {
		v1.Stages[i].Status = StageStatusCompleted
	}
	failure := Assertion{ID: "duration_within_tolerance", Status: AssertFail,
		Expected: "20.00s ±10%", Actual: "12.000s (off by 8.000s)"}

	v2, rev, err := ReviseForValidation(v1, failure, 12, testNow)
	if err != nil {
		t.Fatalf("ReviseForValidation: %v", err)
	}
	var sel *Stage
	for i := range v2.Stages {
		if v2.Stages[i].Type == StageTypeSelectClips {
			sel = &v2.Stages[i]
		}
	}
	if sel == nil || !sel.Params.ExtendToTarget {
		t.Fatalf("expected the selection stage to be authorised to extend, got %+v", sel)
	}
	if sel.Status != StageStatusPlanned {
		t.Fatalf("the corrected stage must be re-planned, got %q", sel.Status)
	}
	if rev.Trigger != TriggerValidationFailed {
		t.Fatalf("unexpected trigger %q", rev.Trigger)
	}
}

func TestReviseForValidationRefusesUnknownFailure(t *testing.T) {
	v1, _ := Plan("prod_1", "20-second vertical clip", testObservation(), testNow)
	failure := Assertion{ID: "has_audio_stream", Status: AssertFail}
	if _, _, err := ReviseForValidation(v1, failure, 0, testNow); err == nil {
		t.Fatalf("expected a refusal: no corrective pass is defined for a missing audio stream")
	}
}

func TestRevisionBudgetIsBounded(t *testing.T) {
	if MaxRevisions > 2 {
		t.Fatalf("revision budget must stay small and bounded, got %d", MaxRevisions)
	}
}

// ---- validator ----

func TestValidateAssertsAgainstRealProbe(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "final.mp4")
	writeFile(t, out, "fake-bytes")

	plan, _ := Plan("prod_1", "12-second vertical teaser", testObservation(), testNow)
	res := Validate(plan, out, func(string) ([]byte, error) {
		return fakeProbeJSON(12, 1080, 1920, true), nil
	})

	if !res.OK() {
		t.Fatalf("expected all assertions to pass, got %+v", res.Assertions)
	}
	if res.Status != "passed" {
		t.Fatalf("expected status passed, got %q", res.Status)
	}
}

func TestValidateFailsOnDurationMismatch(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "final.mp4")
	writeFile(t, out, "fake-bytes")

	plan, _ := Plan("prod_1", "20-second vertical clip", testObservation(), testNow)
	res := Validate(plan, out, func(string) ([]byte, error) {
		return fakeProbeJSON(12, 1080, 1920, true), nil
	})

	failure, ok := res.FirstFailure()
	if !ok || failure.ID != "duration_within_tolerance" {
		t.Fatalf("expected a duration failure, got %+v", res.Assertions)
	}
	// Both sides of the gap must be recorded for a reader to act on.
	if !strings.Contains(failure.Expected, "20") || !strings.Contains(failure.Actual, "12") {
		t.Fatalf("assertion must record expected and actual, got %+v", failure)
	}
}

func TestValidateMarksSidecarCaptionsDegraded(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "final.mp4")
	writeFile(t, out, "fake-bytes")

	plan, _ := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	for i := range plan.Stages {
		if plan.Stages[i].Type == StageTypeTextOverlay {
			plan.Stages[i].Status = StageStatusCompleted
			plan.Stages[i].Requires = CapSidecarSRT
			plan.Stages[i].Produced = []string{"captions.srt"}
			plan.Stages[i].Binding = &Binding{Backend: CapSidecarSRT, Fallback: true}
		}
	}
	res := Validate(plan, out, func(string) ([]byte, error) {
		return fakeProbeJSON(12, 1080, 1920, true), nil
	})

	var captions Assertion
	for _, a := range res.Assertions {
		if a.ID == "captions_delivered" {
			captions = a
		}
	}
	if captions.Status != AssertPass {
		t.Fatalf("sidecar delivery still delivers captions, got %q", captions.Status)
	}
	if !captions.Degraded {
		t.Fatalf("sidecar delivery against a burn-in brief must be marked degraded")
	}
	if res.Status != "passed_degraded" {
		t.Fatalf("expected passed_degraded, got %q", res.Status)
	}
}

func TestValidateFailsWhenCaptionStageProducedNothing(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "final.mp4")
	writeFile(t, out, "fake-bytes")

	plan, _ := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	for i := range plan.Stages {
		if plan.Stages[i].Type == StageTypeTextOverlay {
			plan.Stages[i].Status = StageStatusCompleted
			plan.Stages[i].Binding = &Binding{Backend: CapSidecarSRT}
			plan.Stages[i].Notes = []string{"no speech detected in the source audio"}
		}
	}
	res := Validate(plan, out, func(string) ([]byte, error) {
		return fakeProbeJSON(12, 1080, 1920, true), nil
	})

	for _, a := range res.Assertions {
		if a.ID == "captions_delivered" {
			if a.Status != AssertFail {
				t.Fatalf("a stage that produced nothing delivered nothing, got %q", a.Status)
			}
			if !strings.Contains(a.Actual, "no speech") {
				t.Fatalf("the reason should surface in the assertion, got %q", a.Actual)
			}
			return
		}
	}
	t.Fatalf("captions_delivered assertion missing")
}

func TestValidateFailsWhenOutputMissing(t *testing.T) {
	plan, _ := Plan("prod_1", "12-second vertical clip", testObservation(), testNow)
	res := Validate(plan, filepath.Join(t.TempDir(), "nope.mp4"), func(string) ([]byte, error) {
		return nil, errNotFound{"ffprobe"}
	})
	if res.OK() {
		t.Fatalf("a missing output must fail validation")
	}
}

// ---- layout / io ----

func TestLayoutKeepsEverythingUnderOneRoot(t *testing.T) {
	l := NewLayout("prod_1")
	paths := []string{
		l.ObservationPath(), l.CapabilitiesPath(), l.EventsPath(), l.RunRecordPath(),
		l.PlanPath(1), l.PlanPath(2), l.ExecutionPath("stage_0001"),
		l.ValidationPath(), l.RevisionPath(1), l.OutputDir(),
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, filepath.Join(ProductionsRoot, "prod_1")) {
			t.Fatalf("path %q escapes the production root", p)
		}
	}
	if l.PlanPath(1) == l.PlanPath(2) {
		t.Fatalf("each plan version must have its own path")
	}
}

func TestPlanCloneIsDeep(t *testing.T) {
	plan, _ := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	clone := plan.Clone()
	clone.Stages[0].Requires = "mutated"
	clone.Stages[0].Binding = &Binding{Backend: "x"}

	if plan.Stages[0].Requires == "mutated" {
		t.Fatalf("Clone must not share stage storage")
	}
	if plan.Stages[0].Binding != nil {
		t.Fatalf("Clone must not share binding pointers")
	}
}

func TestWriteAndReadPlanRoundTrip(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	plan, _ := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	if err := WritePlan(l, plan); err != nil {
		t.Fatalf("WritePlan: %v", err)
	}
	got, err := ReadPlan(l, 1)
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if len(got.Stages) != len(plan.Stages) || got.Brief != plan.Brief {
		t.Fatalf("round trip lost data")
	}
}

// ---- run record ----

func TestBuildRunRecordCountsZeroEgressForLocalRun(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	caps := ProbeCapabilities(noLibassProber(), config.Config{})
	plan, _ := Plan("prod_1", "12-second vertical teaser", testObservation(), testNow)
	for i := range plan.Stages {
		plan.Stages[i].Status = StageStatusCompleted
		plan.Stages[i].Binding = &Binding{Backend: plan.Stages[i].Requires}
	}
	validation := ValidationResult{Status: "passed"}

	rec := BuildRunRecord(l, "prod_1", testObservation(), []ProductionPlan{plan}, nil,
		&validation, caps, "output/final.mp4", nil, testNow)

	if rec.NetworkEgress != 0 {
		t.Fatalf("a fully local production must report zero egress, got %d", rec.NetworkEgress)
	}
	if len(rec.CapabilityGaps) == 0 {
		t.Fatalf("probed gaps must be recorded in the run record")
	}
	if rec.Status != "completed" {
		t.Fatalf("expected completed, got %q", rec.Status)
	}
}

func TestHandoffMentionsRevisionAndProvenance(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	caps := ProbeCapabilities(noLibassProber(), config.Config{})
	plan, _ := Plan("prod_1", "12-second vertical teaser with burned-in captions", testObservation(), testNow)
	for i := range plan.Stages {
		plan.Stages[i].Status = StageStatusCompleted
		plan.Stages[i].Binding = &Binding{Backend: plan.Stages[i].Requires}
	}
	rev := Revision{RevisionID: "rev_0001", Trigger: TriggerCapabilityUnavailable,
		FromPlan: 1, ToPlan: 2, Change: "substituted", Reason: "libass missing"}
	validation := ValidationResult{Status: "passed_degraded"}

	rec := BuildRunRecord(l, "prod_1", testObservation(), []ProductionPlan{plan},
		[]Revision{rev}, &validation, caps, "output/final.mp4", nil, testNow)
	doc := Handoff(rec, []ProductionPlan{plan})

	for _, want := range []string{"rev_0001", "libass missing", "external network calls", "execution.json"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("handoff is missing %q", want)
		}
	}
}

// ---- helpers ----

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPlannerDoesNotAutoExtendOnFirstPass(t *testing.T) {
	plan, err := Plan("prod_1", "make a 30-second vertical cut", testObservation(), testNow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, s := range plan.Stages {
		if s.Type != StageTypeSelectClips {
			continue
		}
		if s.Params.ExtendToTarget {
			t.Fatalf("v1 must not loop footage silently; the shortfall has to surface as a failed assertion first")
		}
		return
	}
	t.Fatalf("no select_clips stage found")
}

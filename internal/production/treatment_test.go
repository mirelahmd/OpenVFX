package production

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// richTreatment is the shape a competent Creative Director produces for a
// layered creative request — not a keyword translation.
func richTreatment() CreativeTreatment {
	return CreativeTreatment{
		SchemaVersion: TreatmentSchemaVersion,
		TreatmentID:   "prod_1-treatment",
		ProductionID:  "prod_1",
		CreatedAt:     testNow,
		Reasoning: ReasoningProvenance{
			RequestedMode: ReasoningLLM, EffectiveMode: ReasoningLLM,
			Provider: "ollama", Model: "llama3", SemanticReasoning: true,
		},
		Brief:                 "25-second cinematic reel, open aggressively, captions bold and low",
		Objective:             "A premium, dramatic vertical teaser that opens hard and lands on one spoken line.",
		Platform:              "instagram_reel",
		TargetDurationSeconds: 10,
		AspectRatio:           "9:16",
		Tone:                  []string{"cinematic", "premium", "dramatic"},
		OpeningStrategy:       "Open at full intensity on the silent coverage.",
		NarrativeStructure: []NarrativeBeat{
			{ID: "beat_0001", Name: "cold open", Purpose: "Start at peak", ApproxSeconds: 4},
			{ID: "beat_0002", Name: "release", Purpose: "Land the line", ApproxSeconds: 6},
		},
		PacingStrategy: []PacingPhase{
			{ID: "phase_0001", FromSeconds: 0, ToSeconds: 4, Intent: "Accelerate", CutStyle: "fast"},
			{ID: "phase_0002", FromSeconds: 4, ToSeconds: 10, Intent: "Settle", CutStyle: "slow"},
		},
		AssetRoles: []AssetRole{
			{AssetID: "asset_0001", Role: RoleHook, Confidence: ConfidenceHigh, Rationale: "silent coverage"},
			{AssetID: "asset_0002", Role: RolePrimaryNarration, Confidence: ConfidenceHigh, Rationale: "only audio"},
		},
		Segments: []SegmentIntent{
			{ID: "seg_0002", Order: 2, AssetID: "asset_0002", SourceIn: 1, SourceOut: 5,
				Purpose: "strongest delivery", BeatRef: "beat_0002", DecisionID: "dec_0002"},
			{ID: "seg_0001", Order: 1, AssetID: "asset_0001", SourceIn: 0, SourceOut: 4,
				Purpose: "aggressive cold open", BeatRef: "beat_0001", DecisionID: "dec_0001"},
		},
		AudioStrategy:   AudioStrategy{UseSourceAudio: true, NarrationPlan: "final line only"},
		CaptionStrategy: CaptionStrategy{Required: true, BurnIn: true, Style: "bold", Position: "bottom"},
		GeneratedAssetNeeds: []GeneratedNeed{
			{ID: "need_0001", Kind: "generated_broll", Description: "atmospheric insert",
				Required: false, Reason: "no atmospheric coverage supplied"},
		},
		DegradedAlternatives: []DegradedAlternative{
			{ID: "alt_0001", ForNeed: "need_0001", Approach: "hold the wide longer", Impact: "less variety"},
		},
		SuccessCriteria: []SuccessCriterion{
			{ID: "crit_0001", Assertion: "duration_within_tolerance", Target: "10s +/-10%", Seconds: 10},
			{ID: "crit_0002", Assertion: "aspect_ratio_matches", Target: "9:16"},
		},
		Decisions: []TreatmentDecision{
			{ID: "dec_0001", Summary: "Open on the silent coverage and hold it.", Rationale: "creator asked to open aggressively"},
			{ID: "dec_0002", Summary: "Use the talking clip only at the end.", Rationale: "only where delivery is strongest"},
		},
		Critique:      &CritiqueRecord{Performed: true, Passes: 1, Findings: []string{}},
		Uncertainties: []string{"which part of the talking clip is strongest"},
		Assumptions:   []string{"silent coverage is the visually strongest material"},
	}
}

// treatmentObservation matches the asset ids used by richTreatment.
func treatmentObservation() Observation {
	return Observation{
		SchemaVersion: ObservationSchemaVersion,
		InputPath:     "/tmp/assets",
		Assets: []Asset{
			{ID: "asset_0001", Path: "/tmp/assets/wide.mp4", DurationSeconds: 12,
				Width: 1920, Height: 1080, HasVideo: true, HasAudio: false},
			{ID: "asset_0002", Path: "/tmp/assets/talking.mp4", DurationSeconds: 20,
				Width: 1920, Height: 1080, HasVideo: true, HasAudio: true},
		},
		Totals: Totals{AssetCount: 2, DurationSeconds: 32, WithAudio: 1, WithVideo: 2},
	}
}

// ---- asset observations ----

func TestBuildAssetObservationsCarriesFactsNotPayloads(t *testing.T) {
	obs := treatmentObservation()
	obs.Assets[0].FrameRate = "30000/1001"
	obs.Skipped = []SkipNote{{Path: "/tmp/assets/notes.txt", Reason: "unrecognised media extension .txt"}}

	got := BuildAssetObservations("prod_1", obs, "", testNow)

	if got.SchemaVersion != AssetObservationsSchemaVersion {
		t.Fatalf("schema = %q", got.SchemaVersion)
	}
	if len(got.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(got.Assets))
	}
	if got.Assets[0].AspectRatio != "16:9" {
		t.Fatalf("aspect = %q, want 16:9", got.Assets[0].AspectRatio)
	}
	if got.Assets[0].FrameRate != "29.970" {
		t.Fatalf("frame rate = %q, want 29.970", got.Assets[0].FrameRate)
	}
	if got.Assets[0].MediaType != "video_only" || got.Assets[1].MediaType != "audiovisual" {
		t.Fatalf("media types = %q / %q", got.Assets[0].MediaType, got.Assets[1].MediaType)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("skipped files must surface as warnings, got %v", got.Warnings)
	}
	// The artifact goes into a prompt; it must not carry transcript bodies.
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "segments") {
		t.Fatalf("asset observations must not embed transcript bodies")
	}
}

func TestBuildAssetObservationsReferencesTranscriptWithoutEmbedding(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, "stages", "stage_0002")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := map[string]any{
		"source":   map[string]any{"input_path": "/tmp/assets/talking.mp4"},
		"segments": []map[string]any{{"text": "hello there"}},
	}
	if err := WriteJSON(filepath.Join(stage, "transcript.json"), transcript); err != nil {
		t.Fatal(err)
	}

	got := BuildAssetObservations("prod_1", treatmentObservation(), dir, testNow)

	var talking ObservedAsset
	for _, a := range got.Assets {
		if a.ID == "asset_0002" {
			talking = a
		}
	}
	if !talking.HasTranscript {
		t.Fatalf("transcript should have been detected")
	}
	if talking.TranscriptRef == "" {
		t.Fatalf("transcript reference is empty")
	}
	if talking.TranscriptChars != len("hello there") {
		t.Fatalf("transcript chars = %d", talking.TranscriptChars)
	}
}

func TestAspectRatioLabelCollapsesNearStandardRatios(t *testing.T) {
	cases := map[string][2]int{
		"16:9": {1920, 1080},
		"9:16": {1080, 1920},
		"1:1":  {1080, 1080},
		"4:5":  {1080, 1350},
	}
	for want, dims := range cases {
		if got := aspectRatioLabel(dims[0], dims[1]); got != want {
			t.Fatalf("aspectRatioLabel(%d,%d) = %q, want %q", dims[0], dims[1], got, want)
		}
	}
}

// ---- treatment validation ----

func TestValidTreatmentPasses(t *testing.T) {
	if err := richTreatment().Validate(treatmentObservation()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestTreatmentRejectsHallucinatedAssetID(t *testing.T) {
	tr := richTreatment()
	tr.Segments[0].AssetID = "asset_9999"
	err := tr.Validate(treatmentObservation())
	if err == nil || !strings.Contains(err.Error(), "unknown asset_id") {
		t.Fatalf("expected unknown asset rejection, got %v", err)
	}
}

func TestTreatmentRejectsSegmentPastEndOfClip(t *testing.T) {
	tr := richTreatment()
	tr.Segments[1].SourceOut = 400
	err := tr.Validate(treatmentObservation())
	if err == nil || !strings.Contains(err.Error(), "exceeds asset") {
		t.Fatalf("expected overrun rejection, got %v", err)
	}
}

func TestTreatmentRejectsInvertedSegment(t *testing.T) {
	tr := richTreatment()
	tr.Segments[0].SourceIn, tr.Segments[0].SourceOut = 5, 1
	if err := tr.Validate(treatmentObservation()); err == nil {
		t.Fatalf("expected inverted segment rejection")
	}
}

func TestTreatmentRejectsDanglingDecisionReference(t *testing.T) {
	tr := richTreatment()
	tr.Segments[0].DecisionID = "dec_9999"
	err := tr.Validate(treatmentObservation())
	if err == nil || !strings.Contains(err.Error(), "unknown decision") {
		t.Fatalf("expected dangling decision rejection, got %v", err)
	}
}

// The honesty contract, enforced structurally.
func TestTreatmentRejectsRulesOutputClaimingSemanticReasoning(t *testing.T) {
	tr := richTreatment()
	tr.Reasoning.EffectiveMode = ReasoningDeterministic
	tr.Reasoning.SemanticReasoning = true
	err := tr.Validate(treatmentObservation())
	if err == nil || !strings.Contains(err.Error(), "semantic_reasoning") {
		t.Fatalf("a rules-based treatment must never claim semantic reasoning, got %v", err)
	}
}

func TestTreatmentRejectsUnknownReasoningMode(t *testing.T) {
	tr := richTreatment()
	tr.Reasoning.EffectiveMode = "vibes"
	if err := tr.Validate(treatmentObservation()); err == nil {
		t.Fatalf("expected unknown mode rejection")
	}
}

func TestTreatmentRejectsDuplicateRoleAssignment(t *testing.T) {
	tr := richTreatment()
	tr.AssetRoles = append(tr.AssetRoles, AssetRole{AssetID: "asset_0001", Role: RoleBRoll, Confidence: ConfidenceLow})
	if err := tr.Validate(treatmentObservation()); err == nil {
		t.Fatalf("expected duplicate role rejection")
	}
}

func TestTreatmentAllowsGenerationSegmentWithoutAsset(t *testing.T) {
	tr := richTreatment()
	tr.Segments = append(tr.Segments, SegmentIntent{
		ID: "seg_0003", Order: 3, NeedsGeneration: true, Purpose: "atmospheric insert",
	})
	if err := tr.Validate(treatmentObservation()); err != nil {
		t.Fatalf("a generation-pending segment need not bind an asset: %v", err)
	}
	usable, pending := tr.UsableSegments()
	if len(usable) != 2 || len(pending) != 1 {
		t.Fatalf("usable=%d pending=%d", len(usable), len(pending))
	}
}

func TestOrderedSegmentsSortsByDeclaredOrder(t *testing.T) {
	ordered := richTreatment().OrderedSegments()
	if ordered[0].ID != "seg_0001" || ordered[1].ID != "seg_0002" {
		t.Fatalf("segments out of order: %s, %s", ordered[0].ID, ordered[1].ID)
	}
}

// ---- treatment -> plan bridge ----

func TestPlanFromTreatmentCarriesSegmentsIntoRealCuts(t *testing.T) {
	obs := treatmentObservation()
	plan, err := PlanFromTreatment("prod_1", "brief", obs, richTreatment(), testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("invalid plan: %v", err)
	}
	if plan.Planner != "creative_director.llm" {
		t.Fatalf("planner = %q", plan.Planner)
	}

	var sel *Stage
	for i := range plan.Stages {
		if plan.Stages[i].Type == StageTypeSelectClips {
			sel = &plan.Stages[i]
		}
	}
	if sel == nil {
		t.Fatalf("no select_clips stage")
	}
	if len(sel.Params.Segments) != 2 {
		t.Fatalf("expected 2 cuts from the treatment, got %d", len(sel.Params.Segments))
	}
	// Order must follow the treatment, not the observation order.
	if sel.Params.Segments[0].ID != "seg_0001" {
		t.Fatalf("cut order did not follow the treatment: %s first", sel.Params.Segments[0].ID)
	}
	if sel.Params.Segments[0].SourcePath != "/tmp/assets/wide.mp4" {
		t.Fatalf("cut resolved to the wrong file: %s", sel.Params.Segments[0].SourcePath)
	}
	if sel.Params.Segments[1].SourceIn != 1 || sel.Params.Segments[1].SourceOut != 5 {
		t.Fatalf("in/out points were not preserved: %+v", sel.Params.Segments[1])
	}
}

func TestPlanFromTreatmentStampsDecisionProvenanceOnStages(t *testing.T) {
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), richTreatment(), testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	found := false
	for _, s := range plan.Stages {
		if s.Type != StageTypeSelectClips {
			continue
		}
		found = true
		if s.TreatmentDecisionID == "" {
			t.Fatalf("select_clips carries no treatment_decision_id")
		}
		if s.TreatmentRef == "" {
			t.Fatalf("select_clips carries no human-readable treatment ref")
		}
	}
	if !found {
		t.Fatalf("no select_clips stage")
	}
	// At least one other stage should cite the treatment too.
	refs := 0
	for _, s := range plan.Stages {
		if s.TreatmentRef != "" {
			refs++
		}
	}
	if refs < 2 {
		t.Fatalf("expected several stages to cite the treatment, got %d", refs)
	}
}

func TestPlanFromTreatmentPrefersNarrationAssetForTranscription(t *testing.T) {
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), richTreatment(), testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	for _, s := range plan.Stages {
		if s.Type != StageTypeTranscribe {
			continue
		}
		if s.Params.SourcePath != "/tmp/assets/talking.mp4" {
			t.Fatalf("transcription should follow the primary_narration role, got %s", s.Params.SourcePath)
		}
		if !strings.Contains(s.TreatmentRef, "primary_narration") {
			t.Fatalf("transcription stage should cite the role decision, got %q", s.TreatmentRef)
		}
		return
	}
	t.Fatalf("no transcribe stage")
}

func TestPlanFromTreatmentSurfacesUnmetGapsAsWarnings(t *testing.T) {
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), richTreatment(), testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	joined := strings.Join(plan.Warnings, " | ")
	if !strings.Contains(joined, "generated_broll") {
		t.Fatalf("unmet generation needs must surface on the plan, got %v", plan.Warnings)
	}
}

func TestPlanFromTreatmentDropsGenerationSegmentsWithAWarning(t *testing.T) {
	tr := richTreatment()
	tr.Segments = append(tr.Segments, SegmentIntent{ID: "seg_0003", Order: 3, NeedsGeneration: true})
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), tr, testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	if !strings.Contains(strings.Join(plan.Warnings, " | "), "seg_0003") {
		t.Fatalf("dropped segment must be reported, got %v", plan.Warnings)
	}
}

func TestPlanFromTreatmentWarnsWhenReasoningFellBack(t *testing.T) {
	tr := richTreatment()
	tr.Reasoning = ReasoningProvenance{
		RequestedMode: ReasoningLLM, EffectiveMode: ReasoningDeterministic,
		FallbackUsed: true, FallbackReason: "connection refused",
	}
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), tr, testNow)
	if err != nil {
		t.Fatalf("PlanFromTreatment: %v", err)
	}
	if !strings.Contains(strings.Join(plan.Warnings, " | "), "connection refused") {
		t.Fatalf("plan must record that reasoning fell back, got %v", plan.Warnings)
	}
	if plan.Planner != "creative_director.deterministic" {
		t.Fatalf("planner = %q", plan.Planner)
	}
}

func TestPlanFromTreatmentFailsWithNoUsableSegments(t *testing.T) {
	tr := richTreatment()
	tr.Segments = []SegmentIntent{{ID: "seg_0001", Order: 1, NeedsGeneration: true}}
	if _, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), tr, testNow); err == nil {
		t.Fatalf("expected an error when nothing can actually be cut")
	}
}

func TestIntentFromTreatmentDrivesTheValidator(t *testing.T) {
	intent := IntentFromTreatment(richTreatment(), treatmentObservation())
	if intent.TargetDurationSeconds != 10 {
		t.Fatalf("target = %v", intent.TargetDurationSeconds)
	}
	if intent.Width != 1080 || intent.Height != 1920 {
		t.Fatalf("dims = %dx%d", intent.Width, intent.Height)
	}
	if !intent.WantsCaptions || !intent.WantsBurnedCaptions {
		t.Fatalf("caption intent lost: %+v", intent)
	}
	if !intent.WantsAudio {
		t.Fatalf("audio intent lost")
	}
}

// ---- selector honours the treatment ----

func TestSelectClipsHonoursTreatmentSegmentsVerbatim(t *testing.T) {
	dir := t.TempDir()
	e := &Executor{Layout: Layout{Root: dir}, Obs: treatmentObservation()}
	stage := &Stage{
		ID: "stage_0003", Type: StageTypeSelectClips,
		Params: Params{
			TargetSeconds: 10,
			Segments: []SegmentCut{
				{ID: "seg_0001", AssetID: "asset_0001", SourcePath: "/tmp/assets/wide.mp4",
					SourceIn: 0, SourceOut: 4, Purpose: "cold open", DecisionID: "dec_0001"},
				{ID: "seg_0002", AssetID: "asset_0002", SourcePath: "/tmp/assets/talking.mp4",
					SourceIn: 1, SourceOut: 5, Purpose: "final line", DecisionID: "dec_0002"},
			},
		},
	}
	stageDir := filepath.Join(dir, "stages", stage.ID)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outputs, _, err := e.stageSelectClips(stage, stageDir)
	if err != nil {
		t.Fatalf("stageSelectClips: %v", err)
	}

	var edl EditDecisionList
	if err := ReadJSON(outputs[0], &edl); err != nil {
		t.Fatal(err)
	}
	if len(edl.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(edl.Decisions))
	}
	// Verbatim: the selector must not re-derive the edit.
	if edl.Decisions[1].SourceIn != 1 || edl.Decisions[1].SourceOut != 5 {
		t.Fatalf("selector altered the treatment's in/out: %+v", edl.Decisions[1])
	}
	if edl.Decisions[0].DecisionID != "dec_0001" {
		t.Fatalf("decision provenance lost on the cut")
	}
	if !strings.Contains(edl.Decisions[0].Reason, "cold open") {
		t.Fatalf("creative purpose lost on the cut: %q", edl.Decisions[0].Reason)
	}
}

func TestSelectClipsFallsBackToArithmeticWithoutATreatment(t *testing.T) {
	dir := t.TempDir()
	e := &Executor{Layout: Layout{Root: dir}, Obs: treatmentObservation()}
	stage := &Stage{ID: "stage_0003", Type: StageTypeSelectClips,
		Params: Params{TargetSeconds: 10, MaxClips: 8}}
	stageDir := filepath.Join(dir, "stages", stage.ID)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}

	outputs, _, err := e.stageSelectClips(stage, stageDir)
	if err != nil {
		t.Fatalf("stageSelectClips: %v", err)
	}
	var edl EditDecisionList
	if err := ReadJSON(outputs[0], &edl); err != nil {
		t.Fatal(err)
	}
	if len(edl.Decisions) == 0 || edl.Decisions[0].DecisionID != "" {
		t.Fatalf("intent-only path should produce cuts with no creative decision id")
	}
}

// ---- director bridge ----

func TestRunDirectorReportsUnavailableWithoutFailingTheRun(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	runner := func(python, workersDir string, args []string) ([]byte, error) {
		return nil, fmt.Errorf("No module named 'langgraph'")
	}
	res, unavailable, err := RunDirector(l, "prod_1", "brief",
		DirectorOptions{PythonInterpreter: "python3", WorkersDir: t.TempDir()}, runner)
	if err != nil {
		t.Fatalf("a missing sidecar must not fail the production: %v", err)
	}
	if unavailable == "" {
		t.Fatalf("expected an unavailability reason")
	}
	if res.EffectiveMode != ReasoningUnavailable {
		t.Fatalf("mode = %q", res.EffectiveMode)
	}
}

func TestRunDirectorWritesConfigAndBriefForTheSidecar(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	var seenArgs []string
	runner := func(python, workersDir string, args []string) ([]byte, error) {
		seenArgs = args
		return []byte(`{"production_id":"prod_1","effective_mode":"deterministic","llm_calls":0}`), nil
	}
	_, unavailable, err := RunDirector(l, "prod_1", "a rich brief with \"quotes\" and\nnewlines",
		DirectorOptions{Mode: ReasoningLLM, Model: "llama3", Provider: "ollama",
			PythonInterpreter: "python3", WorkersDir: t.TempDir()}, runner)
	if err != nil || unavailable != "" {
		t.Fatalf("err=%v unavailable=%q", err, unavailable)
	}

	var cfg directorConfig
	if err := ReadJSON(l.DirectorConfigPath(), &cfg); err != nil {
		t.Fatalf("director config not written: %v", err)
	}
	if cfg.Mode != ReasoningLLM || cfg.Model != "llama3" {
		t.Fatalf("config = %+v", cfg)
	}
	// The brief must travel by file so quoting and newlines survive.
	if !containsArg(seenArgs, "--brief-file") {
		t.Fatalf("brief should be passed as a file, got %v", seenArgs)
	}
	briefPath := filepath.Join(filepath.Dir(l.DirectorConfigPath()), "brief.txt")
	data, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("brief file not written: %v", err)
	}
	if !strings.Contains(string(data), "newlines") {
		t.Fatalf("brief was mangled: %q", data)
	}
}

func TestRunDirectorRejectsInvalidSidecarJSON(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	runner := func(python, workersDir string, args []string) ([]byte, error) {
		return []byte("not json"), nil
	}
	res, unavailable, err := RunDirector(l, "prod_1", "brief",
		DirectorOptions{PythonInterpreter: "python3", WorkersDir: t.TempDir()}, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unavailable == "" || res.EffectiveMode != ReasoningUnavailable {
		t.Fatalf("invalid sidecar output must degrade, got %q / %q", unavailable, res.EffectiveMode)
	}
}

func TestLoadValidatedTreatmentRejectsRatherThanPartiallyApplying(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	bad := richTreatment()
	bad.Segments[0].AssetID = "asset_9999"
	if err := WriteJSON(l.TreatmentPath(), bad); err != nil {
		t.Fatal(err)
	}
	_, rejection := LoadValidatedTreatment(l, treatmentObservation())
	if rejection == "" {
		t.Fatalf("a treatment referencing a nonexistent asset must be rejected outright")
	}
}

func TestDirectorSummaryLineIsHonest(t *testing.T) {
	llm := richTreatment()
	if !strings.Contains(DirectorSummaryLine(llm), "semantic reasoning") {
		t.Fatalf("llm summary = %q", DirectorSummaryLine(llm))
	}

	rules := richTreatment()
	rules.Reasoning = ReasoningProvenance{RequestedMode: ReasoningDeterministic, EffectiveMode: ReasoningDeterministic}
	line := DirectorSummaryLine(rules)
	if !strings.Contains(line, "no semantic reasoning") {
		t.Fatalf("rules summary must disclaim reasoning, got %q", line)
	}

	fell := richTreatment()
	fell.Reasoning = ReasoningProvenance{
		RequestedMode: ReasoningLLM, EffectiveMode: ReasoningDeterministic,
		FallbackUsed: true, FallbackReason: "connection refused",
	}
	if !strings.Contains(DirectorSummaryLine(fell), "connection refused") {
		t.Fatalf("fallback summary must carry the reason")
	}
}

func TestDirectorConfigFromModelsResolvesRoute(t *testing.T) {
	routes := map[string]string{"creative_director": "local_llm"}
	entries := map[string]ModelEntry{
		"local_llm": {Provider: "ollama", Model: "llama3", BaseURL: "http://localhost:11434"},
	}
	provider, model, baseURL, route := DirectorConfigFromModels(routes, entries, "")
	if provider != "ollama" || model != "llama3" || baseURL == "" || route != "creative_director" {
		t.Fatalf("resolution = %q %q %q %q", provider, model, baseURL, route)
	}

	_, model, _, _ = DirectorConfigFromModels(routes, entries, "missing_route")
	if model != "" {
		t.Fatalf("an unknown route must resolve to nothing, got %q", model)
	}
}

// ---- run record ----

func TestRunRecordCarriesReasoningProvenance(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	caps := CapabilitySet{}
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), richTreatment(), testNow)
	if err != nil {
		t.Fatal(err)
	}
	validation := ValidationResult{Status: "passed"}
	rec := BuildRunRecord(l, "prod_1", treatmentObservation(), []ProductionPlan{plan}, nil,
		&validation, caps, "output/final.mp4", nil, testNow).
		WithTreatment(l, richTreatment())

	if rec.Treatment == nil {
		t.Fatalf("run record lost the treatment summary")
	}
	if !rec.Treatment.Reasoning.SemanticReasoning {
		t.Fatalf("reasoning provenance lost")
	}
	if rec.Treatment.SegmentCount != 2 || rec.Treatment.GapCount != 1 {
		t.Fatalf("summary = %+v", rec.Treatment)
	}

	doc := Handoff(rec, []ProductionPlan{plan})
	for _, want := range []string{"Creative direction", "semantic reasoning", "creative_treatment.json"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("handoff missing %q", want)
		}
	}
}

func TestHandoffDisclosesFallbackReason(t *testing.T) {
	l := Layout{Root: t.TempDir()}
	tr := richTreatment()
	tr.Reasoning = ReasoningProvenance{
		RequestedMode: ReasoningLLM, EffectiveMode: ReasoningDeterministic,
		FallbackUsed: true, FallbackReason: "ollama request failed: connection refused",
	}
	plan, err := PlanFromTreatment("prod_1", "brief", treatmentObservation(), tr, testNow)
	if err != nil {
		t.Fatal(err)
	}
	rec := BuildRunRecord(l, "prod_1", treatmentObservation(), []ProductionPlan{plan}, nil,
		nil, CapabilitySet{}, "", nil, testNow).WithTreatment(l, tr)

	doc := Handoff(rec, []ProductionPlan{plan})
	if !strings.Contains(doc, "connection refused") {
		t.Fatalf("handoff must disclose why reasoning fell back")
	}
}

// ---- helpers ----

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

var _ = time.Now

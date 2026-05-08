package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseGoalMakeShortsExtractsMaxClips(t *testing.T) {
	preset, opts, err := ParseGoal("make 5 shorts", GoalOptions{})
	if err != nil {
		t.Fatalf("ParseGoal returned error: %v", err)
	}
	if preset != "shorts" || opts["roughcut_max_clips"] != 5 {
		t.Fatalf("preset=%q opts=%#v", preset, opts)
	}
	if opts["with_ffmpeg_script"] != true || opts["with_report"] != true {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestParseGoalMappings(t *testing.T) {
	cases := []struct {
		goal   string
		preset string
		key    string
	}{
		{"metadata only", "metadata", ""},
		{"transcribe this", "custom", "with_transcript"},
		{"make captions", "custom", "with_captions"},
		{"find highlights", "custom", "with_highlights"},
	}
	for _, tc := range cases {
		preset, opts, err := ParseGoal(tc.goal, GoalOptions{})
		if err != nil {
			t.Fatalf("ParseGoal(%q) returned error: %v", tc.goal, err)
		}
		if preset != tc.preset {
			t.Fatalf("ParseGoal(%q) preset=%q want %q", tc.goal, preset, tc.preset)
		}
		if tc.key != "" && opts[tc.key] != true {
			t.Fatalf("ParseGoal(%q) opts=%#v missing %s", tc.goal, opts, tc.key)
		}
	}
}

func TestParseGoalUnknown(t *testing.T) {
	_, _, err := ParseGoal("do magic", GoalOptions{})
	if err == nil {
		t.Fatal("ParseGoal returned nil error")
	}
}

func TestPlanArtifactAndActionLog(t *testing.T) {
	t.Chdir(t.TempDir())
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make 3 shorts", GoalOptions{}, time.Date(2026, 4, 29, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	log, err := OpenActionLog(plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Write("PLAN_CREATED", map[string]any{"plan_id": plan.PlanID}); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(PlansRoot, plan.PlanID, "agent_plan.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(PlansRoot, plan.PlanID, "actions.jsonl")); err != nil {
		t.Fatal(err)
	}
	read, err := ReadPlan(plan.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Goal != "make 3 shorts" {
		t.Fatalf("read goal = %q", read.Goal)
	}
	plans, err := ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].PlanID != plan.PlanID {
		t.Fatalf("plans = %#v", plans)
	}
}

func TestValidatePlanAcceptsValidPlan(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make 3 shorts", GoalOptions{}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if errs := ValidatePlan(plan); len(errs) != 0 {
		t.Fatalf("validation errors = %#v", errs)
	}
}

func TestValidatePlanRejectsUnknownActionType(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make 3 shorts", GoalOptions{}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].Type = "unknown"
	if errs := ValidatePlan(plan); len(errs) == 0 {
		t.Fatal("ValidatePlan returned no errors")
	}
}

func TestValidatePlanRejectsMissingSafety(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make 3 shorts", GoalOptions{}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plan.Safety.ExportsRequireExplicitExecution = false
	if errs := ValidatePlan(plan); len(errs) == 0 {
		t.Fatal("ValidatePlan returned no errors")
	}
}

func TestCommandPreviews(t *testing.T) {
	if got := CommandPreview("run_pipeline", "media/input.mov", "shorts", "", GoalOptions{}); got != "./byom-video pipeline \"media/input.mov\" --preset shorts" {
		t.Fatalf("preview = %q", got)
	}
	if got := CommandPreview("run_pipeline", "media/input.mov", "metadata", "", GoalOptions{}); got != "./byom-video pipeline \"media/input.mov\" --preset metadata" {
		t.Fatalf("preview = %q", got)
	}
	if got := CommandPreview("batch_pipeline", "media/folder", "shorts", "", GoalOptions{Recursive: true, Limit: 2}); got != "./byom-video batch \"media/folder\" --preset shorts --recursive --limit 2" {
		t.Fatalf("preview = %q", got)
	}
	if got := CommandPreview("watch_pipeline", "media/inbox", "shorts", "", GoalOptions{Once: true}); got != "./byom-video watch \"media/inbox\" --preset shorts --once" {
		t.Fatalf("preview = %q", got)
	}
}

func TestExactRunCommandPreviews(t *testing.T) {
	cases := []struct {
		name    string
		preset  string
		options map[string]any
		want    string
	}{
		{
			name:    "metadata",
			preset:  "metadata",
			options: map[string]any{},
			want:    "./byom-video run \"media/input.mov\"",
		},
		{
			name:    "transcript",
			preset:  "custom",
			options: map[string]any{"with_transcript": true},
			want:    "./byom-video run \"media/input.mov\" --with-transcript --transcript-model-size tiny",
		},
		{
			name:    "captions",
			preset:  "custom",
			options: map[string]any{"with_transcript": true, "with_captions": true},
			want:    "./byom-video run \"media/input.mov\" --with-transcript --with-captions --transcript-model-size tiny",
		},
		{
			name:    "highlights",
			preset:  "custom",
			options: map[string]any{"with_transcript": true, "with_chunks": true, "with_highlights": true},
			want:    "./byom-video run \"media/input.mov\" --with-transcript --with-chunks --with-highlights --transcript-model-size tiny",
		},
		{
			name:   "shorts",
			preset: "shorts",
			options: map[string]any{
				"with_transcript":    true,
				"with_captions":      true,
				"with_chunks":        true,
				"with_highlights":    true,
				"with_roughcut":      true,
				"with_ffmpeg_script": true,
				"with_report":        true,
				"roughcut_max_clips": 5,
			},
			want: "./byom-video run \"media/input.mov\" --with-transcript --with-captions --with-chunks --with-highlights --with-roughcut --with-ffmpeg-script --with-report --transcript-model-size tiny --roughcut-max-clips 5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CommandPreviewForOptions("run_pipeline", "media/input.mov", tc.preset, "", tc.options)
			if got != tc.want {
				t.Fatalf("preview = %q want %q", got, tc.want)
			}
		})
	}
}

func TestBatchAndWatchGoalMapping(t *testing.T) {
	dir := t.TempDir()
	plan, err := NewPlan(dir, "batch process shorts", GoalOptions{Mode: "batch"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetType != "batch" || plan.Actions[0].Type != "batch_pipeline" {
		t.Fatalf("plan = %#v", plan)
	}
	plan, err = NewPlan(dir, "watch this folder for shorts", GoalOptions{Mode: "watch", Once: true}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetType != "watch" || plan.Actions[0].Type != "watch_pipeline" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestGoalAwarePlanAddsActionsAndPreviews(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.mp4")
	if err := os.WriteFile(input, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := NewPlan(input, "make a short clip under 60 seconds", GoalOptions{
		GoalAware:                 true,
		GoalUseOllama:             true,
		GoalFallbackDeterministic: true,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) < 3 {
		t.Fatalf("expected goal-aware post actions, got %#v", plan.Actions)
	}
	if plan.Actions[1].Type != "goal_rerank" || plan.Actions[2].Type != "goal_roughcut" {
		t.Fatalf("unexpected actions: %#v", plan.Actions)
	}
	if !strings.Contains(plan.Actions[1].CommandPreview, "goal-rerank <run_id> --goal ") {
		t.Fatalf("unexpected rerank preview: %q", plan.Actions[1].CommandPreview)
	}
	if !strings.Contains(plan.Actions[1].CommandPreview, "--use-ollama") || !strings.Contains(plan.Actions[1].CommandPreview, "--fallback-deterministic") {
		t.Fatalf("missing goal-aware flags in preview: %q", plan.Actions[1].CommandPreview)
	}
	if plan.Actions[2].CommandPreview != "./byom-video goal-roughcut <run_id>" {
		t.Fatalf("unexpected roughcut preview: %q", plan.Actions[2].CommandPreview)
	}
}

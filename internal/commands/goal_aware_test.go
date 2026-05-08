package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirelahmd/byom-video/internal/highlights"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/modelrouter"
	"github.com/mirelahmd/byom-video/internal/report"
)

const testConfigWithGoalRerankRoute = `models:
  enabled: true

  entries:
    local_qwen:
      provider: ollama
      model: qwen2.5:7b
      role: reasoner
      base_url: http://localhost:11434

  routes:
    goal_reranking: local_qwen
`

func TestParseGoalConstraints(t *testing.T) {
	constraints := parseGoalConstraints("make 3 cinematic shorts under 60 seconds")
	if constraints.MaxTotalDurationSeconds != 60 {
		t.Fatalf("max_total_duration_seconds = %v", constraints.MaxTotalDurationSeconds)
	}
	if constraints.MaxClips != 3 {
		t.Fatalf("max_clips = %d", constraints.MaxClips)
	}
	if constraints.PreferredStyle != "cinematic" {
		t.Fatalf("preferred_style = %q", constraints.PreferredStyle)
	}

	technical := parseGoalConstraints("make a technical clip")
	if technical.PreferredStyle != "technical" {
		t.Fatalf("preferred_style = %q", technical.PreferredStyle)
	}
}

func TestDeterministicGoalRerankUsesGoalKeywordsAndDurationPenalty(t *testing.T) {
	items := []highlights.Highlight{
		{ID: "hl_0001", ChunkID: "chunk_0001", Start: 0, End: 20, DurationSeconds: 20, Score: 0.70, Text: "technical latency fix and system performance", Reason: "good"},
		{ID: "hl_0002", ChunkID: "chunk_0002", Start: 25, End: 100, DurationSeconds: 75, Score: 0.92, Text: "general commentary with no keyword match", Reason: "base"},
	}
	ranked := deterministicGoalRerank(items, "make a technical short under 60 seconds", parseGoalConstraints("make a technical short under 60 seconds"))
	if len(ranked) != 2 {
		t.Fatalf("ranked length = %d", len(ranked))
	}
	if ranked[0].HighlightID != "hl_0001" {
		t.Fatalf("top highlight = %q, want hl_0001", ranked[0].HighlightID)
	}
	if ranked[0].GoalScore <= ranked[1].GoalScore {
		t.Fatalf("expected top goal score to exceed penalized long clip: %#v", ranked)
	}
}

func TestGoalRoughcutRespectsMaxDurationAndClips(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	doc := map[string]any{
		"schema_version": "goal_rerank.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "make a short clip under 60 seconds",
		"mode":           "deterministic",
		"source": map[string]any{
			"highlights_artifact": "highlights.json",
		},
		"constraints": map[string]any{
			"max_total_duration_seconds": 60,
			"max_clips":                  2,
			"preferred_style":            "shorts",
		},
		"ranked_highlights": []map[string]any{
			{"highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 0, "end": 40, "duration_seconds": 40, "original_score": 0.9, "goal_score": 0.9, "rank": 1, "reason": "first", "text": "first"},
			{"highlight_id": "hl_0002", "chunk_id": "chunk_0002", "start": 50, "end": 85, "duration_seconds": 35, "original_score": 0.8, "goal_score": 0.8, "rank": 2, "reason": "second", "text": "second"},
			{"highlight_id": "hl_0003", "chunk_id": "chunk_0003", "start": 90, "end": 110, "duration_seconds": 20, "original_score": 0.7, "goal_score": 0.7, "rank": 3, "reason": "third", "text": "third"},
		},
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_rerank.json"), doc); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := GoalRoughcutCommand(runID, &out, GoalRoughcutOptions{}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	readJSON(t, filepath.Join(runDir, "goal_roughcut.json"), &got)
	clips := got["clips"].([]any)
	if len(clips) != 2 {
		t.Fatalf("clip count = %d, want 2", len(clips))
	}
	plan := got["plan"].(map[string]any)
	if plan["total_duration_seconds"].(float64) > 60.0 {
		t.Fatalf("total duration = %v", plan["total_duration_seconds"])
	}
}

func TestGoalRerankInvalidOllamaJSONFailsCleanly(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithGoalRerankRoute)
	runID, _ := setupGoalAwareRun(t)
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       ioNopCloser(`{"response":"not json"}`),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	err := GoalRerankCommand(runID, &bytes.Buffer{}, GoalRerankOptions{
		Goal:      "make a cinematic short",
		UseOllama: true,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error, got: %v", err)
	}
}

func TestGoalRerankFallbackDeterministicWorks(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, testConfigWithGoalRerankRoute)
	runID, runDir := setupGoalAwareRun(t)
	restore := modelrouter.SetHTTPClientFactoryForTests(func(timeout time.Duration) modelrouter.HTTPDoer {
		return httpDoerFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       ioNopCloser(`{"response":"not json"}`),
				Header:     make(http.Header),
			}, nil
		})
	})
	defer restore()

	if err := GoalRerankCommand(runID, &bytes.Buffer{}, GoalRerankOptions{
		Goal:                  "make a cinematic short",
		UseOllama:             true,
		FallbackDeterministic: true,
	}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	readJSON(t, filepath.Join(runDir, "goal_rerank.json"), &doc)
	if doc["mode"] != "deterministic" {
		t.Fatalf("mode = %v", doc["mode"])
	}
	warnings := fmt.Sprint(doc["warnings"])
	if !strings.Contains(warnings, "using deterministic fallback") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestValidateCatchesInvalidGoalRoughcutTiming(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := os.WriteFile(filepath.Join(runDir, "events.jsonl"), []byte(`{"type":"RUN_STARTED","time":"2026-05-05T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_roughcut.json"), map[string]any{
		"schema_version": "goal_roughcut.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "goal",
		"source": map[string]any{
			"goal_rerank_artifact": "goal_rerank.json",
		},
		"plan": map[string]any{
			"title":                  "Goal-Aware Roughcut Plan",
			"intent":                 "test",
			"total_duration_seconds": 5,
		},
		"clips": []map[string]any{
			{"id": "goal_clip_0001", "highlight_id": "hl_0001", "start": 10, "end": 5, "duration_seconds": 0, "order": 1, "goal_score": 0.9, "reason": "bad", "text": "bad"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Validate(runID, &out, ValidateOptions{})
	if err == nil || !strings.Contains(out.String(), "goal_roughcut.json") {
		t.Fatalf("expected goal_roughcut validation error, got: %v output=%s", err, out.String())
	}
}

func TestReportIncludesGoalAwareSections(t *testing.T) {
	runDir := t.TempDir()
	m := manifest.New("run-1", "/tmp/input.mp4", time.Now().UTC())
	m.Status = manifest.StatusCompleted
	if err := writeJSONFile(filepath.Join(runDir, "goal_rerank.json"), map[string]any{
		"schema_version": "goal_rerank.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         "run-1",
		"goal":           "make a cinematic short",
		"mode":           "deterministic",
		"source":         map[string]any{"highlights_artifact": "highlights.json"},
		"constraints":    map[string]any{"max_total_duration_seconds": 60, "max_clips": 3, "preferred_style": "cinematic"},
		"ranked_highlights": []map[string]any{
			{"highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 1, "end": 2, "duration_seconds": 1, "original_score": 0.7, "goal_score": 0.9, "rank": 1, "reason": "Strong match", "text": "visual opening"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_roughcut.json"), map[string]any{
		"schema_version": "goal_roughcut.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         "run-1",
		"goal":           "make a cinematic short",
		"source":         map[string]any{"goal_rerank_artifact": "goal_rerank.json"},
		"plan":           map[string]any{"title": "Goal-Aware Roughcut Plan", "intent": "Select clips matching the user goal.", "total_duration_seconds": 1},
		"clips": []map[string]any{
			{"id": "goal_clip_0001", "highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 1, "end": 2, "duration_seconds": 1, "order": 1, "goal_score": 0.9, "reason": "Strong match", "text": "visual opening"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := report.Write(runDir, m); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "report.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Goal Rerank", "Goal Roughcut", "make a cinematic short", "Strong match"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

func TestInspectShowsGoalAwareCounts(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := writeJSONFile(filepath.Join(runDir, "goal_rerank.json"), map[string]any{
		"schema_version": "goal_rerank.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "make a short clip",
		"mode":           "deterministic",
		"source":         map[string]any{"highlights_artifact": "highlights.json"},
		"constraints":    map[string]any{"max_total_duration_seconds": 60, "max_clips": 3, "preferred_style": "shorts"},
		"ranked_highlights": []map[string]any{
			{"highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 1, "end": 2, "duration_seconds": 1, "original_score": 0.7, "goal_score": 0.9, "rank": 1, "reason": "Strong match", "text": "short"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_roughcut.json"), map[string]any{
		"schema_version": "goal_roughcut.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "make a short clip",
		"source":         map[string]any{"goal_rerank_artifact": "goal_rerank.json"},
		"plan":           map[string]any{"title": "Goal-Aware Roughcut Plan", "intent": "Select clips matching the user goal.", "total_duration_seconds": 1},
		"clips": []map[string]any{
			{"id": "goal_clip_0001", "highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 1, "end": 2, "duration_seconds": 1, "order": 1, "goal_score": 0.9, "reason": "Strong match", "text": "short"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Inspect(runID, &out, InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "goal rerank:") || !strings.Contains(text, "goal roughcut:") || !strings.Contains(text, "goal rerank mode:") {
		t.Fatalf("inspect output missing goal-aware counts: %s", text)
	}
}

func TestGoalReviewBundleWritesArtifactAndShowsNextCommands(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := writeJSONFile(filepath.Join(runDir, "goal_rerank.json"), map[string]any{
		"schema_version": "goal_rerank.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "make a short clip under 60 seconds",
		"mode":           "deterministic",
		"source":         map[string]any{"highlights_artifact": "highlights.json"},
		"constraints":    map[string]any{"max_total_duration_seconds": 60, "max_clips": 3, "preferred_style": "shorts"},
		"ranked_highlights": []map[string]any{
			{"highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 0, "end": 28.4, "duration_seconds": 28.4, "original_score": 0.72, "goal_score": 0.91, "rank": 1, "reason": "Strong match", "text": "cinematic opening shot with strong visual moment"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "goal_roughcut.json"), map[string]any{
		"schema_version": "goal_roughcut.v1",
		"created_at":     time.Now().UTC(),
		"run_id":         runID,
		"goal":           "make a short clip under 60 seconds",
		"source":         map[string]any{"goal_rerank_artifact": "goal_rerank.json"},
		"plan":           map[string]any{"title": "Goal-Aware Roughcut Plan", "intent": "Select clips matching the user goal.", "total_duration_seconds": 28.4},
		"clips": []map[string]any{
			{"id": "goal_clip_0001", "highlight_id": "hl_0001", "chunk_id": "chunk_0001", "start": 0, "end": 28.4, "duration_seconds": 28.4, "order": 1, "goal_score": 0.91, "reason": "Strong match", "text": "cinematic opening shot with strong visual moment"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := GoalReviewBundleCommand(runID, &bytes.Buffer{}, GoalReviewBundleOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(runDir, "goal_review_bundle.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Goal Review Bundle", "Strong match", "byom-video goal-handoff " + runID + " --overwrite", "byom-video validate " + runID} {
		if !strings.Contains(text, want) {
			t.Fatalf("bundle missing %q: %s", want, text)
		}
	}
}

func TestInspectShowsGoalReviewBundlePath(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	if err := os.WriteFile(filepath.Join(runDir, "goal_review_bundle.md"), []byte("# Goal Review Bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("goal_review_bundle", "goal_review_bundle.md")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Inspect(runID, &out, InspectOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "goal bundle:") {
		t.Fatalf("inspect output missing goal bundle path: %s", out.String())
	}
}

func TestValidateFailsWhenListedGoalReviewBundleMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	runID, runDir := setupGoalAwareRun(t)
	m, err := manifest.Read(filepath.Join(runDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.AddArtifact("goal_review_bundle", "goal_review_bundle.md")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = Validate(runID, &out, ValidateOptions{})
	if err == nil || !strings.Contains(out.String(), "artifact missing: goal_review_bundle.md") {
		t.Fatalf("expected missing bundle validation failure, err=%v out=%s", err, out.String())
	}
}

func setupGoalAwareRun(t *testing.T) (string, string) {
	t.Helper()
	runID := "run-goal-aware"
	runDir := filepath.Join(".byom-video", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := manifest.New(runID, "media/input.mov", time.Now().UTC())
	m.Status = manifest.StatusCompleted
	m.AddArtifact("manifest", "manifest.json")
	m.AddArtifact("highlights", "highlights.json")
	if err := manifest.Write(filepath.Join(runDir, "manifest.json"), m); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(runDir, "highlights.json"), highlights.Document{
		SchemaVersion: "highlights.v1",
		Highlights: []highlights.Highlight{
			{ID: "hl_0001", ChunkID: "chunk_0001", Start: 0, End: 28.4, DurationSeconds: 28.4, Score: 0.72, Text: "cinematic opening shot with strong visual moment", Reason: "Strong opening"},
			{ID: "hl_0002", ChunkID: "chunk_0002", Start: 30, End: 65, DurationSeconds: 35, Score: 0.68, Text: "technical explanation of system performance and latency", Reason: "Technical detail"},
			{ID: "hl_0003", ChunkID: "chunk_0003", Start: 70, End: 145, DurationSeconds: 75, Score: 0.90, Text: "general commentary with no obvious keyword match", Reason: "Long candidate"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return runID, runDir
}

func ioNopCloser(text string) *readCloser {
	return &readCloser{Reader: strings.NewReader(text)}
}

type readCloser struct {
	*strings.Reader
}

func (r *readCloser) Close() error { return nil }

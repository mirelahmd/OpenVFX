package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/goalartifacts"
	"github.com/mirelahmd/byom-video/internal/highlights"
	"github.com/mirelahmd/byom-video/internal/modelrouter"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type GoalRerankOptions struct {
	Goal                  string
	UseOllama             bool
	FallbackDeterministic bool
}

type GoalRoughcutOptions struct {
	Overwrite bool
	JSON      bool
}

func GoalRerankCommand(runID string, stdout io.Writer, opts GoalRerankOptions) error {
	if strings.TrimSpace(opts.Goal) == "" {
		return fmt.Errorf("--goal is required")
	}
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("GOAL_RERANK_STARTED", map[string]any{"run_id": runID, "goal": opts.Goal, "use_ollama": opts.UseOllama})
	}

	doc, err := buildGoalRerank(runDir, runID, opts)
	if err != nil {
		writeMaskFailure(log, "GOAL_RERANK_FAILED", err.Error())
		return err
	}
	outPath := filepath.Join(runDir, "goal_rerank.json")
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "GOAL_RERANK_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "goal_rerank", "goal_rerank.json"); err != nil {
		writeMaskFailure(log, "GOAL_RERANK_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "GOAL_RERANK_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("GOAL_RERANK_COMPLETED", map[string]any{
			"path":       "goal_rerank.json",
			"mode":       doc.Mode,
			"highlights": len(doc.RankedHighlights),
		})
	}

	fmt.Fprintln(stdout, "Goal rerank created")
	fmt.Fprintf(stdout, "  run id:      %s\n", runID)
	fmt.Fprintf(stdout, "  goal:        %s\n", doc.Goal)
	fmt.Fprintf(stdout, "  mode:        %s\n", doc.Mode)
	fmt.Fprintf(stdout, "  path:        %s\n", outPath)
	fmt.Fprintf(stdout, "  highlights:  %d\n", len(doc.RankedHighlights))
	fmt.Fprintf(stdout, "  max total:   %.0f seconds\n", doc.Constraints.MaxTotalDurationSeconds)
	fmt.Fprintf(stdout, "  max clips:   %d\n", doc.Constraints.MaxClips)
	fmt.Fprintf(stdout, "  style:       %s\n", doc.Constraints.PreferredStyle)
	for _, warning := range doc.Warnings {
		fmt.Fprintf(stdout, "  warning:     %s\n", warning)
	}
	return nil
}

func GoalRoughcutCommand(runID string, stdout io.Writer, opts GoalRoughcutOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("GOAL_ROUGHCUT_STARTED", map[string]any{"run_id": runID})
	}
	outPath := filepath.Join(runDir, "goal_roughcut.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "GOAL_ROUGHCUT_FAILED", "goal_roughcut.json already exists; pass --overwrite")
			return fmt.Errorf("goal_roughcut.json already exists; pass --overwrite")
		}
	}
	doc, err := buildGoalRoughcut(runDir, runID)
	if err != nil {
		writeMaskFailure(log, "GOAL_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "GOAL_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "goal_roughcut", "goal_roughcut.json"); err != nil {
		writeMaskFailure(log, "GOAL_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "GOAL_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("GOAL_ROUGHCUT_COMPLETED", map[string]any{"path": "goal_roughcut.json", "clips": len(doc.Clips)})
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Goal roughcut created")
	fmt.Fprintf(stdout, "  run id:  %s\n", runID)
	fmt.Fprintf(stdout, "  path:    %s\n", outPath)
	fmt.Fprintf(stdout, "  clips:   %d\n", len(doc.Clips))
	fmt.Fprintf(stdout, "  total:   %.3f seconds\n", doc.Plan.TotalDurationSeconds)
	return nil
}

func buildGoalRerank(runDir string, runID string, opts GoalRerankOptions) (goalartifacts.GoalRerank, error) {
	highlightsDoc, err := readHighlightsDocument(filepath.Join(runDir, "highlights.json"))
	if err != nil {
		return goalartifacts.GoalRerank{}, fmt.Errorf("read highlights: %w", err)
	}
	constraints := parseGoalConstraints(opts.Goal)
	warnings := []string{}
	mode := "deterministic"
	ranked := deterministicGoalRerank(highlightsDoc.Highlights, opts.Goal, constraints)
	var rerankErr error

	if opts.UseOllama {
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			if opts.FallbackDeterministic {
				warnings = append(warnings, "ollama goal rerank failed to load config; using deterministic fallback: "+err.Error())
			} else {
				return goalartifacts.GoalRerank{}, err
			}
		} else {
			ranked, warnings, rerankErr = ollamaGoalRerank(runDir, cfg, highlightsDoc.Highlights, opts.Goal, constraints, opts.FallbackDeterministic)
			if rerankErr == nil {
				if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, " "), "using deterministic fallback") {
					mode = "ollama"
				}
			}
		}
		if rerankErr != nil {
			return goalartifacts.GoalRerank{}, rerankErr
		}
		if len(ranked) > 0 && !hasFallbackWarning(warnings) {
			mode = "ollama"
		}
	}

	doc := goalartifacts.GoalRerank{
		SchemaVersion: "goal_rerank.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Goal:          strings.TrimSpace(opts.Goal),
		Mode:          mode,
		Source: goalartifacts.GoalRerankSource{
			HighlightsArtifact: "highlights.json",
		},
		Constraints:      constraints,
		RankedHighlights: ranked,
		Warnings:         dedupeStrings(warnings),
	}
	if _, err := os.Stat(filepath.Join(runDir, "chunks.json")); err == nil {
		doc.Source.ChunksArtifact = "chunks.json"
	}
	return doc, nil
}

func buildGoalRoughcut(runDir string, runID string) (goalartifacts.GoalRoughcut, error) {
	rerank, err := goalartifacts.ReadGoalRerank(filepath.Join(runDir, "goal_rerank.json"))
	if err != nil {
		return goalartifacts.GoalRoughcut{}, err
	}
	selected := []goalartifacts.GoalRoughcutClip{}
	total := 0.0
	ordered := append([]goalartifacts.RankedHighlight(nil), rerank.RankedHighlights...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Rank == ordered[j].Rank {
			return ordered[i].GoalScore > ordered[j].GoalScore
		}
		return ordered[i].Rank < ordered[j].Rank
	})
	for _, item := range ordered {
		if len(selected) >= rerank.Constraints.MaxClips {
			break
		}
		if item.DurationSeconds <= 0 {
			continue
		}
		if total+item.DurationSeconds > rerank.Constraints.MaxTotalDurationSeconds {
			continue
		}
		selected = append(selected, goalartifacts.GoalRoughcutClip{
			ID:              fmt.Sprintf("goal_clip_%04d", len(selected)+1),
			HighlightID:     item.HighlightID,
			ChunkID:         item.ChunkID,
			Start:           item.Start,
			End:             item.End,
			DurationSeconds: item.DurationSeconds,
			Order:           len(selected) + 1,
			GoalScore:       item.GoalScore,
			Reason:          item.Reason,
			Text:            item.Text,
		})
		total += item.DurationSeconds
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Start < selected[j].Start
	})
	for i := range selected {
		selected[i].ID = fmt.Sprintf("goal_clip_%04d", i+1)
		selected[i].Order = i + 1
	}
	doc := goalartifacts.GoalRoughcut{
		SchemaVersion: "goal_roughcut.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Goal:          rerank.Goal,
		Source: goalartifacts.GoalRoughcutSource{
			GoalRerankArtifact: "goal_rerank.json",
		},
		Plan: goalartifacts.GoalRoughcutPlan{
			Title:                "Goal-Aware Roughcut Plan",
			Intent:               "Select clips matching the user goal.",
			TotalDurationSeconds: total,
		},
		Clips: selected,
	}
	if _, err := os.Stat(filepath.Join(runDir, "roughcut.json")); err == nil {
		doc.Source.RoughcutArtifact = "roughcut.json"
	}
	return doc, nil
}

func readHighlightsDocument(path string) (highlights.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return highlights.Document{}, err
	}
	var doc highlights.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return highlights.Document{}, fmt.Errorf("decode highlights: %w", err)
	}
	return doc, nil
}

func parseGoalConstraints(goal string) goalartifacts.GoalConstraints {
	constraints := goalartifacts.GoalConstraints{
		MaxTotalDurationSeconds: 60,
		MaxClips:                5,
		PreferredStyle:          "general",
	}
	lower := strings.ToLower(goal)
	if matches := regexp.MustCompile(`(?:under|less than)\s+(\d+)\s+seconds`).FindStringSubmatch(lower); len(matches) == 2 {
		if value, err := strconv.Atoi(matches[1]); err == nil && value > 0 {
			constraints.MaxTotalDurationSeconds = float64(value)
		}
	}
	if matches := regexp.MustCompile(`(?:make\s+)?(\d+)\b`).FindStringSubmatch(lower); len(matches) == 2 && goalContainsAny(lower, []string{"clips", "shorts"}) {
		if value, err := strconv.Atoi(matches[1]); err == nil && value > 0 {
			constraints.MaxClips = value
		}
	}
	switch {
	case strings.Contains(lower, "cinematic"):
		constraints.PreferredStyle = "cinematic"
	case strings.Contains(lower, "technical"):
		constraints.PreferredStyle = "technical"
	case strings.Contains(lower, "funny"):
		constraints.PreferredStyle = "funny"
	case strings.Contains(lower, "emotional"):
		constraints.PreferredStyle = "emotional"
	case goalContainsAny(lower, []string{"short", "shorts", "reel", "tiktok", "instagram"}):
		constraints.PreferredStyle = "shorts"
	}
	return constraints
}

func deterministicGoalRerank(items []highlights.Highlight, goal string, constraints goalartifacts.GoalConstraints) []goalartifacts.RankedHighlight {
	keywords := extractGoalKeywords(goal)
	ranked := make([]goalartifacts.RankedHighlight, 0, len(items))
	for _, item := range items {
		textLower := strings.ToLower(item.Text)
		matches := 0
		for _, keyword := range keywords {
			if strings.Contains(textLower, keyword) {
				matches++
			}
		}
		score := item.Score
		reasonBits := []string{}
		if matches > 0 {
			score += math.Min(0.18, float64(matches)*0.06)
			reasonBits = append(reasonBits, fmt.Sprintf("matched %d goal keyword(s)", matches))
		}
		if item.DurationSeconds > constraints.MaxTotalDurationSeconds {
			score -= 0.25
			reasonBits = append(reasonBits, "penalized for exceeding goal duration")
		} else if constraints.PreferredStyle == "shorts" && item.DurationSeconds <= constraints.MaxTotalDurationSeconds {
			score += 0.08
			reasonBits = append(reasonBits, "fits shorts-style duration")
		}
		if item.DurationSeconds <= constraints.MaxTotalDurationSeconds/2 {
			score += 0.04
			reasonBits = append(reasonBits, "concise candidate")
		}
		if constraints.PreferredStyle == "technical" && goalContainsAny(textLower, []string{"system", "technical", "architecture", "performance", "latency", "debug"}) {
			score += 0.05
			reasonBits = append(reasonBits, "matches technical style")
		}
		if constraints.PreferredStyle == "cinematic" && goalContainsAny(textLower, []string{"scene", "visual", "moment", "shot", "cinematic"}) {
			score += 0.05
			reasonBits = append(reasonBits, "matches cinematic style")
		}
		if constraints.PreferredStyle == "funny" && goalContainsAny(textLower, []string{"funny", "joke", "laugh", "hilarious"}) {
			score += 0.05
			reasonBits = append(reasonBits, "matches funny style")
		}
		if constraints.PreferredStyle == "emotional" && goalContainsAny(textLower, []string{"emotional", "feeling", "heart", "powerful"}) {
			score += 0.05
			reasonBits = append(reasonBits, "matches emotional style")
		}
		score = math.Max(0, math.Min(1, score))
		reason := "Deterministic rerank using original highlight score and simple goal matching."
		if len(reasonBits) > 0 {
			reason = strings.Join(reasonBits, "; ") + "."
		}
		ranked = append(ranked, goalartifacts.RankedHighlight{
			HighlightID:     item.ID,
			ChunkID:         item.ChunkID,
			Start:           item.Start,
			End:             item.End,
			DurationSeconds: item.DurationSeconds,
			OriginalScore:   item.Score,
			GoalScore:       score,
			Reason:          reason,
			Text:            item.Text,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].GoalScore == ranked[j].GoalScore {
			if ranked[i].OriginalScore == ranked[j].OriginalScore {
				return ranked[i].Start < ranked[j].Start
			}
			return ranked[i].OriginalScore > ranked[j].OriginalScore
		}
		return ranked[i].GoalScore > ranked[j].GoalScore
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return ranked
}

func extractGoalKeywords(goal string) []string {
	stop := map[string]bool{
		"make": true, "clip": true, "clips": true, "short": true, "shorts": true, "under": true, "less": true,
		"than": true, "seconds": true, "second": true, "with": true, "this": true, "that": true, "into": true,
		"video": true, "videos": true, "for": true, "the": true, "and": true, "reel": true, "tiktok": true, "instagram": true,
	}
	fields := strings.FieldsFunc(strings.ToLower(goal), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	keywords := []string{}
	seen := map[string]bool{}
	for _, field := range fields {
		if len(field) < 4 || stop[field] || seen[field] {
			continue
		}
		seen[field] = true
		keywords = append(keywords, field)
	}
	return keywords
}

func ollamaGoalRerank(runDir string, cfg config.Config, items []highlights.Highlight, goal string, constraints goalartifacts.GoalConstraints, fallback bool) ([]goalartifacts.RankedHighlight, []string, error) {
	if !cfg.Models.Enabled {
		err := fmt.Errorf("models.enabled is false; rerun without --use-ollama or enable models in byom-video.yaml")
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank unavailable; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}
	entryName, ok := cfg.Models.Routes["goal_reranking"]
	if !ok || strings.TrimSpace(entryName) == "" {
		err := fmt.Errorf("models.routes.goal_reranking is not configured")
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank unavailable; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}
	entry, ok := cfg.Models.Entries[entryName]
	if !ok {
		err := fmt.Errorf("models entry %q is not configured", entryName)
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank unavailable; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}
	adapter, ok := modelrouter.DefaultRegistry().ForProvider(entry.Provider)
	if !ok {
		err := fmt.Errorf("no adapter registered for provider %q", entry.Provider)
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank unavailable; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}

	decisions := make([]modelrouter.DecisionInput, 0, len(items))
	for _, item := range items {
		decisions = append(decisions, modelrouter.DecisionInput{
			ID:          item.ID,
			Start:       item.Start,
			End:         item.End,
			TextPreview: trimPreview(item.Text),
			Reason:      fmt.Sprintf("original_score=%.3f duration=%.3f", item.Score, item.DurationSeconds),
		})
	}
	req := modelrouter.Request{
		TaskID:         "task_goal_rerank_0001",
		TaskType:       "goal_reranking",
		RouteName:      "goal_reranking",
		ModelEntryName: entryName,
		Provider:       entry.Provider,
		Model:          entry.Model,
		Role:           entry.Role,
		BaseURL:        entry.BaseURL,
		Options:        entry.Options,
		Input: modelrouter.RequestInput{
			Decisions: decisions,
			Constraints: map[string]any{
				"goal":                       goal,
				"max_total_duration_seconds": constraints.MaxTotalDurationSeconds,
				"max_clips":                  constraints.MaxClips,
				"preferred_style":            constraints.PreferredStyle,
			},
			OutputContract: map[string]any{
				"response_shape": "ranked_highlights",
			},
		},
	}
	resp, err := adapter.Execute(req)
	if err != nil {
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank failed; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}
	raw, _ := resp.Details["raw_response"].(string)
	if strings.TrimSpace(raw) == "" && len(resp.Texts) > 0 {
		raw = strings.Join(resp.Texts, "\n")
	}
	ranked, warnings, err := parseGoalRerankResponse(raw, items)
	if err != nil {
		if fallback {
			return deterministicGoalRerank(items, goal, constraints), []string{"ollama goal rerank failed; using deterministic fallback: " + err.Error()}, nil
		}
		return nil, nil, err
	}
	return ranked, warnings, nil
}

func parseGoalRerankResponse(raw string, items []highlights.Highlight) ([]goalartifacts.RankedHighlight, []string, error) {
	type rankedItem struct {
		HighlightID string  `json:"highlight_id"`
		GoalScore   float64 `json:"goal_score"`
		Reason      string  `json:"reason"`
	}
	var payload struct {
		RankedHighlights []rankedItem `json:"ranked_highlights"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return nil, nil, fmt.Errorf("ollama goal rerank returned invalid JSON")
	}
	if len(payload.RankedHighlights) == 0 {
		return nil, nil, fmt.Errorf("ollama goal rerank returned no ranked highlights")
	}
	byID := map[string]highlights.Highlight{}
	for _, item := range items {
		byID[item.ID] = item
	}
	seen := map[string]bool{}
	ranked := []goalartifacts.RankedHighlight{}
	warnings := []string{}
	for _, item := range payload.RankedHighlights {
		source, ok := byID[item.HighlightID]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("provider returned unknown highlight id %q", item.HighlightID))
			continue
		}
		if seen[item.HighlightID] {
			continue
		}
		seen[item.HighlightID] = true
		score := math.Max(0, math.Min(1, item.GoalScore))
		reason := strings.TrimSpace(item.Reason)
		if reason == "" {
			reason = "Model-ranked highlight for the requested goal."
		}
		ranked = append(ranked, goalartifacts.RankedHighlight{
			HighlightID:     source.ID,
			ChunkID:         source.ChunkID,
			Start:           source.Start,
			End:             source.End,
			DurationSeconds: source.DurationSeconds,
			OriginalScore:   source.Score,
			GoalScore:       score,
			Reason:          reason,
			Text:            source.Text,
		})
	}
	if len(ranked) == 0 {
		return nil, warnings, fmt.Errorf("ollama goal rerank returned no usable ranked highlights")
	}
	for _, item := range items {
		if seen[item.ID] {
			continue
		}
		ranked = append(ranked, goalartifacts.RankedHighlight{
			HighlightID:     item.ID,
			ChunkID:         item.ChunkID,
			Start:           item.Start,
			End:             item.End,
			DurationSeconds: item.DurationSeconds,
			OriginalScore:   item.Score,
			GoalScore:       item.Score,
			Reason:          "Model response omitted this highlight; kept original score ordering fallback.",
			Text:            item.Text,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].GoalScore == ranked[j].GoalScore {
			if ranked[i].OriginalScore == ranked[j].OriginalScore {
				return ranked[i].Start < ranked[j].Start
			}
			return ranked[i].OriginalScore > ranked[j].OriginalScore
		}
		return ranked[i].GoalScore > ranked[j].GoalScore
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return ranked, dedupeStrings(warnings), nil
}

func hasFallbackWarning(warnings []string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, "using deterministic fallback") {
			return true
		}
	}
	return false
}

func goalContainsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

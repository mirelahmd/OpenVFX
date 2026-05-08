package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

// ---- timeline schema types ----

type CreativeTimelineArtifact struct {
	SchemaVersion  string                  `json:"schema_version"`
	CreatedAt      time.Time               `json:"created_at"`
	CreativePlanID string                  `json:"creative_plan_id"`
	RunID          string                  `json:"run_id,omitempty"`
	Goal           string                  `json:"goal"`
	InputPath      string                  `json:"input_path"`
	Mode           string                  `json:"mode"`
	Source         CreativeTimelineSource  `json:"source"`
	Tracks         []CreativeTimelineTrack `json:"tracks"`
	TotalDuration  float64                 `json:"total_duration_seconds"`
	Warnings       []string                `json:"warnings,omitempty"`
}

type CreativeTimelineSource struct {
	ClipArtifact string `json:"clip_artifact,omitempty"`
	ClipCount    int    `json:"clip_count"`
	StubOutputs  bool   `json:"stub_outputs"`
}

type CreativeTimelineTrack struct {
	ID    string                  `json:"id"`
	Kind  string                  `json:"kind"`
	Items []CreativeTimelineItem  `json:"items"`
}

type CreativeTimelineItem struct {
	ID            string  `json:"id"`
	Kind          string  `json:"kind"`
	TimelineStart float64 `json:"timeline_start"`
	TimelineEnd   float64 `json:"timeline_end"`
	SourceStart   float64 `json:"source_start,omitempty"`
	SourceEnd     float64 `json:"source_end,omitempty"`
	Text          string  `json:"text,omitempty"`
	Label         string  `json:"label,omitempty"`
	Notes         string  `json:"notes,omitempty"`
}

// ---- render plan schema types ----

type CreativeRenderPlanArtifact struct {
	SchemaVersion  string                  `json:"schema_version"`
	CreatedAt      time.Time               `json:"created_at"`
	CreativePlanID string                  `json:"creative_plan_id"`
	RunID          string                  `json:"run_id,omitempty"`
	Goal           string                  `json:"goal"`
	Mode           string                  `json:"mode"`
	Source         CreativeRenderPlanSource `json:"source"`
	PlannedOutput  CreativeRenderOutput    `json:"planned_output"`
	Steps          []CreativeRenderStep    `json:"steps"`
	Warnings       []string                `json:"warnings,omitempty"`
}

type CreativeRenderPlanSource struct {
	TimelineArtifact string `json:"timeline_artifact"`
	TrackCount       int    `json:"track_count"`
	TotalDuration    float64 `json:"total_duration_seconds"`
}

type CreativeRenderOutput struct {
	PlannedFile    string  `json:"planned_file"`
	DurationSeconds float64 `json:"duration_seconds"`
	Format         string  `json:"format"`
	Mode           string  `json:"mode"`
}

type CreativeRenderStep struct {
	StepIndex   int     `json:"step_index"`
	Operation   string  `json:"operation"`
	ItemID      string  `json:"item_id"`
	TrackID     string  `json:"track_id"`
	TimelineStart float64 `json:"timeline_start"`
	TimelineEnd  float64 `json:"timeline_end"`
	Notes       string  `json:"notes,omitempty"`
}

// ---- options ----

type CreativeTimelineOptions struct {
	RunID      string
	Overwrite  bool
	JSON       bool
	PreferGoal bool
}

type CreativeRenderPlanOptions struct {
	Overwrite bool
	JSON      bool
}

type ReviewCreativeTimelineOptions struct {
	JSON          bool
	WriteArtifact bool
}

// ---- internal clip helper ----

type timelineClip struct {
	ID    string
	Start float64
	End   float64
	Text  string
}

// readClipsFromArtifact reads clips from a run artifact JSON file.
// It tries "clips", "items", and "segments" keys in that order.
func readClipsFromArtifact(path string) ([]timelineClip, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("malformed clip artifact %q: %w", path, err)
	}
	for _, key := range []string{"clips", "items", "segments"} {
		list, ok := raw[key].([]any)
		if !ok || len(list) == 0 {
			continue
		}
		out := make([]timelineClip, 0, len(list))
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			start, _ := jsonFloat(m, "start")
			end, _ := jsonFloat(m, "end")
			dur, _ := jsonFloat(m, "duration_seconds")
			if end == 0 && dur > 0 {
				end = start + dur
			}
			text, _ := m["text"].(string)
			if text == "" {
				text, _ = m["source_text"].(string)
			}
			id, _ := m["id"].(string)
			out = append(out, timelineClip{ID: id, Start: start, End: end, Text: text})
		}
		return out, nil
	}
	return nil, fmt.Errorf("no clips/items/segments key found in %q", path)
}

func jsonFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// updateCreativeOutputsIndex adds or updates an artifact entry in creative_outputs.json.
// Creates the index if it doesn't exist.
func updateCreativeOutputsIndex(planID, artifactType, relPath, stepID string) error {
	indexPath := filepath.Join(creativePlansRoot, planID, "outputs", "creative_outputs.json")
	var idx CreativeOutputsIndex
	if data, err := os.ReadFile(indexPath); err == nil {
		_ = json.Unmarshal(data, &idx)
	}
	if idx.SchemaVersion == "" {
		idx.SchemaVersion = "creative_outputs.v1"
		idx.CreatedAt = time.Now().UTC()
		idx.CreativePlanID = planID
		idx.Mode = "stub"
	}
	updated := false
	for i, a := range idx.Artifacts {
		if a.Type == artifactType {
			idx.Artifacts[i].Path = relPath
			idx.Artifacts[i].Status = "created"
			if stepID != "" {
				idx.Artifacts[i].StepID = stepID
			}
			updated = true
			break
		}
	}
	if !updated {
		idx.Artifacts = append(idx.Artifacts, CreativeOutputArtifact{
			Type:   artifactType,
			Path:   relPath,
			StepID: stepID,
			Status: "created",
		})
	}
	return writeJSONFile(indexPath, idx)
}

// ---- creative-timeline ----

func CreativeTimeline(planID string, stdout io.Writer, opts CreativeTimelineOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")
	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}

	goal, _ := m["goal"].(string)
	inputPath, _ := m["input_path"].(string)

	outputsDir := filepath.Join(planDir, "outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		return fmt.Errorf("creating outputs dir: %w", err)
	}

	outPath := filepath.Join(outputsDir, "creative_timeline.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("creative_timeline.json already exists; use --overwrite to replace")
		}
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_TIMELINE_STARTED", map[string]any{"plan_id": planID})
	}

	var clips []timelineClip
	var clipArtifact string
	var warnings []string

	if opts.RunID != "" {
		runDir, err := runstore.RequireRunDir(opts.RunID)
		if err != nil {
			if log != nil {
				_ = log.Write("CREATIVE_TIMELINE_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
				_ = log.Close()
			}
			return fmt.Errorf("run %q: %w", opts.RunID, err)
		}

		// preference order for clip source
		candidates := []struct {
			name string
			path string
		}{
			{"selected_clips.json", filepath.Join(runDir, "selected_clips.json")},
		}
		if opts.PreferGoal {
			candidates = []struct {
				name string
				path string
			}{
				{"goal_roughcut.json", filepath.Join(runDir, "goal_roughcut.json")},
				{"enhanced_roughcut.json", filepath.Join(runDir, "enhanced_roughcut.json")},
				{"roughcut.json", filepath.Join(runDir, "roughcut.json")},
				{"selected_clips.json", filepath.Join(runDir, "selected_clips.json")},
			}
		}

		for _, c := range candidates {
			if cs, err2 := readClipsFromArtifact(c.path); err2 == nil && len(cs) > 0 {
				clips = cs
				clipArtifact = filepath.Join(runDir, c.name)
				break
			}
		}
		if len(clips) == 0 {
			warnings = append(warnings, fmt.Sprintf("run %q: no usable clip artifact found; timeline will have empty video track", opts.RunID))
		}
	}

	// build tracks
	var videoItems []CreativeTimelineItem
	var captionItems []CreativeTimelineItem
	cursor := 0.0

	for i, clip := range clips {
		dur := clip.End - clip.Start
		if dur <= 0 {
			dur = 5.0
		}
		itemID := fmt.Sprintf("video_%04d", i+1)
		if clip.ID != "" {
			itemID = "video_" + clip.ID
		}
		videoItems = append(videoItems, CreativeTimelineItem{
			ID:            itemID,
			Kind:          "source_clip",
			TimelineStart: cursor,
			TimelineEnd:   cursor + dur,
			SourceStart:   clip.Start,
			SourceEnd:     clip.End,
			Text:          clip.Text,
		})
		if clip.Text != "" {
			captionItems = append(captionItems, CreativeTimelineItem{
				ID:            "caption_" + itemID,
				Kind:          "caption",
				TimelineStart: cursor,
				TimelineEnd:   cursor + dur,
				Text:          clip.Text,
			})
		}
		cursor += dur
	}

	totalDuration := cursor

	// voiceover: single placeholder spanning full duration
	var voiceoverItems []CreativeTimelineItem
	voiceoverPlanPath := filepath.Join(outputsDir, "voiceover_plan.json")
	voiceoverNotes := "placeholder — no audio generated"
	if _, err2 := os.Stat(voiceoverPlanPath); err2 == nil {
		voiceoverNotes = "source: outputs/voiceover_plan.json (stub)"
	}
	voiceoverItems = append(voiceoverItems, CreativeTimelineItem{
		ID:            "voiceover_main",
		Kind:          "voiceover_placeholder",
		TimelineStart: 0,
		TimelineEnd:   totalDuration,
		Notes:         voiceoverNotes,
	})

	// visual overlays: one placeholder per visual prompt if visual_asset_prompts.json exists
	var visualItems []CreativeTimelineItem
	visualPromptsPath := filepath.Join(outputsDir, "visual_asset_prompts.json")
	if data, err2 := os.ReadFile(visualPromptsPath); err2 == nil {
		var vap VisualAssetPromptsOutput
		if json.Unmarshal(data, &vap) == nil {
			segDur := totalDuration
			if len(vap.Prompts) > 0 {
				segDur = totalDuration / float64(len(vap.Prompts))
			}
			for i, p := range vap.Prompts {
				start := float64(i) * segDur
				end := start + segDur
				visualItems = append(visualItems, CreativeTimelineItem{
					ID:            fmt.Sprintf("visual_%04d", i+1),
					Kind:          "visual_overlay_placeholder",
					TimelineStart: start,
					TimelineEnd:   end,
					Label:         p.IntendedUse,
					Notes:         p.Prompt,
				})
			}
		}
	}
	if len(visualItems) == 0 {
		visualItems = append(visualItems, CreativeTimelineItem{
			ID:            "visual_main",
			Kind:          "visual_overlay_placeholder",
			TimelineStart: 0,
			TimelineEnd:   totalDuration,
			Notes:         "placeholder — no visual asset prompts generated",
		})
	}

	tracks := []CreativeTimelineTrack{
		{ID: "track_video_main", Kind: "video", Items: videoItems},
		{ID: "track_voiceover", Kind: "audio", Items: voiceoverItems},
		{ID: "track_captions", Kind: "text", Items: captionItems},
		{ID: "track_visual_overlays", Kind: "visual", Items: visualItems},
	}

	timeline := CreativeTimelineArtifact{
		SchemaVersion:  "creative_timeline.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		RunID:          opts.RunID,
		Goal:           goal,
		InputPath:      inputPath,
		Mode:           "stub",
		Source: CreativeTimelineSource{
			ClipArtifact: clipArtifact,
			ClipCount:    len(clips),
			StubOutputs:  true,
		},
		Tracks:        tracks,
		TotalDuration: totalDuration,
		Warnings:      dedupeStrings(warnings),
	}

	if err := writeJSONFile(outPath, timeline); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_TIMELINE_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	if err := updateCreativeOutputsIndex(planID, "creative_timeline", "outputs/creative_timeline.json", ""); err != nil {
		warnings = append(warnings, fmt.Sprintf("warning: could not update creative_outputs.json: %v", err))
	}

	if log != nil {
		_ = log.Write("CREATIVE_TIMELINE_COMPLETED", map[string]any{
			"plan_id":          planID,
			"total_duration":   totalDuration,
			"track_count":      len(tracks),
			"clip_count":       len(clips),
		})
		_ = log.Close()
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(timeline)
	}

	fmt.Fprintf(stdout, "creative-timeline: %s\n", planID)
	fmt.Fprintf(stdout, "  goal:            %s\n", goal)
	fmt.Fprintf(stdout, "  clips:           %d\n", len(clips))
	fmt.Fprintf(stdout, "  total duration:  %.2fs\n", totalDuration)
	fmt.Fprintf(stdout, "  tracks:          %d\n", len(tracks))
	fmt.Fprintf(stdout, "  artifact:        %s\n", outPath)
	for _, w := range dedupeStrings(warnings) {
		fmt.Fprintf(stdout, "  warning: %s\n", w)
	}
	fmt.Fprintf(stdout, "\nnext: byom-video creative-render-plan %s\n", planID)
	return nil
}

// ---- creative-render-plan ----

func CreativeRenderPlan(planID string, stdout io.Writer, opts CreativeRenderPlanOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)
	planPath := filepath.Join(planDir, "creative_plan.json")
	raw, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("creative plan %q not found: %w", planID, err)
	}
	var planMap map[string]any
	if err := json.Unmarshal(raw, &planMap); err != nil {
		return fmt.Errorf("creative plan is malformed: %w", err)
	}
	goal, _ := planMap["goal"].(string)
	runID, _ := planMap["run_id"].(string)

	timelinePath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	timelineData, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("creative_timeline.json not found — run creative-timeline first: %w", err)
	}
	var timeline CreativeTimelineArtifact
	if err := json.Unmarshal(timelineData, &timeline); err != nil {
		return fmt.Errorf("creative_timeline.json is malformed: %w", err)
	}

	outPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("creative_render_plan.json already exists; use --overwrite to replace")
		}
	}

	log, _ := events.Open(filepath.Join(planDir, "events.jsonl"))
	if log != nil {
		_ = log.Write("CREATIVE_RENDER_PLAN_STARTED", map[string]any{"plan_id": planID})
	}

	var steps []CreativeRenderStep
	stepIndex := 0

	for _, track := range timeline.Tracks {
		for _, item := range track.Items {
			op := renderOperationFor(track.Kind, item.Kind)
			steps = append(steps, CreativeRenderStep{
				StepIndex:     stepIndex,
				Operation:     op,
				ItemID:        item.ID,
				TrackID:       track.ID,
				TimelineStart: item.TimelineStart,
				TimelineEnd:   item.TimelineEnd,
				Notes:         renderStepNotes(track.Kind, item),
			})
			stepIndex++
		}
	}

	renderPlan := CreativeRenderPlanArtifact{
		SchemaVersion:  "creative_render_plan.v1",
		CreatedAt:      time.Now().UTC(),
		CreativePlanID: planID,
		RunID:          runID,
		Goal:           goal,
		Mode:           "stub",
		Source: CreativeRenderPlanSource{
			TimelineArtifact: "outputs/creative_timeline.json",
			TrackCount:       len(timeline.Tracks),
			TotalDuration:    timeline.TotalDuration,
		},
		PlannedOutput: CreativeRenderOutput{
			PlannedFile:     "outputs/draft.mp4",
			DurationSeconds: timeline.TotalDuration,
			Format:          "mp4",
			Mode:            "stub",
		},
		Steps: steps,
	}

	if err := writeJSONFile(outPath, renderPlan); err != nil {
		if log != nil {
			_ = log.Write("CREATIVE_RENDER_PLAN_FAILED", map[string]any{"plan_id": planID, "reason": err.Error()})
			_ = log.Close()
		}
		return err
	}

	if err := updateCreativeOutputsIndex(planID, "creative_render_plan", "outputs/creative_render_plan.json", ""); err != nil {
		fmt.Fprintf(stdout, "warning: could not update creative_outputs.json: %v\n", err)
	}

	if log != nil {
		_ = log.Write("CREATIVE_RENDER_PLAN_COMPLETED", map[string]any{
			"plan_id":       planID,
			"step_count":    len(steps),
			"planned_output": renderPlan.PlannedOutput.PlannedFile,
		})
		_ = log.Close()
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(renderPlan)
	}

	fmt.Fprintf(stdout, "creative-render-plan: %s\n", planID)
	fmt.Fprintf(stdout, "  goal:          %s\n", goal)
	fmt.Fprintf(stdout, "  steps:         %d\n", len(steps))
	fmt.Fprintf(stdout, "  planned file:  %s\n", renderPlan.PlannedOutput.PlannedFile)
	fmt.Fprintf(stdout, "  duration:      %.2fs\n", renderPlan.PlannedOutput.DurationSeconds)
	fmt.Fprintf(stdout, "  artifact:      %s\n", outPath)
	fmt.Fprintf(stdout, "\nnext: byom-video review-creative-timeline %s\n", planID)
	return nil
}

func renderOperationFor(trackKind, itemKind string) string {
	switch trackKind {
	case "video":
		return "cut_source_clip"
	case "audio":
		return "attach_voiceover_placeholder"
	case "text":
		return "add_caption_placeholder"
	case "visual":
		return "add_visual_overlay_placeholder"
	}
	return "unknown_operation"
}

func renderStepNotes(trackKind string, item CreativeTimelineItem) string {
	switch trackKind {
	case "video":
		if item.Text != "" {
			return fmt.Sprintf("source %.2f–%.2fs | %s", item.SourceStart, item.SourceEnd, timelineTruncate(item.Text, 60))
		}
		return fmt.Sprintf("source %.2f–%.2fs", item.SourceStart, item.SourceEnd)
	case "audio":
		return item.Notes
	case "text":
		return timelineTruncate(item.Text, 80)
	case "visual":
		return timelineTruncate(item.Notes, 80)
	}
	return ""
}

func timelineTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---- review-creative-timeline ----

func ReviewCreativeTimeline(planID string, stdout io.Writer, opts ReviewCreativeTimelineOptions) error {
	planDir := filepath.Join(creativePlansRoot, planID)

	timelinePath := filepath.Join(planDir, "outputs", "creative_timeline.json")
	timelineData, err := os.ReadFile(timelinePath)
	if err != nil {
		return fmt.Errorf("creative_timeline.json not found — run creative-timeline first: %w", err)
	}
	var timeline CreativeTimelineArtifact
	if err := json.Unmarshal(timelineData, &timeline); err != nil {
		return fmt.Errorf("creative_timeline.json is malformed: %w", err)
	}

	var renderPlan *CreativeRenderPlanArtifact
	renderPath := filepath.Join(planDir, "outputs", "creative_render_plan.json")
	if data, err2 := os.ReadFile(renderPath); err2 == nil {
		var rp CreativeRenderPlanArtifact
		if json.Unmarshal(data, &rp) == nil {
			renderPlan = &rp
		}
	}

	if opts.JSON {
		out := map[string]any{
			"creative_plan_id": planID,
			"goal":             timeline.Goal,
			"total_duration":   timeline.TotalDuration,
			"clip_count":       timeline.Source.ClipCount,
			"track_count":      len(timeline.Tracks),
			"has_render_plan":  renderPlan != nil,
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	var b strings.Builder
	b.WriteString("# Creative Timeline Review\n\n")
	fmt.Fprintf(&b, "- Plan ID: `%s`\n", planID)
	fmt.Fprintf(&b, "- Goal: %s\n", timeline.Goal)
	fmt.Fprintf(&b, "- Total duration: %.2fs\n", timeline.TotalDuration)
	fmt.Fprintf(&b, "- Clips: %d\n", timeline.Source.ClipCount)
	if timeline.Source.ClipArtifact != "" {
		fmt.Fprintf(&b, "- Clip source: `%s`\n", timeline.Source.ClipArtifact)
	}
	fmt.Fprintf(&b, "- Tracks: %d\n\n", len(timeline.Tracks))

	b.WriteString("## Tracks\n\n")
	for _, track := range timeline.Tracks {
		fmt.Fprintf(&b, "### %s (%s)\n\n", track.ID, track.Kind)
		fmt.Fprintf(&b, "Items: %d\n\n", len(track.Items))
		for _, item := range track.Items {
			fmt.Fprintf(&b, "- `%s` [%.2f–%.2fs]", item.ID, item.TimelineStart, item.TimelineEnd)
			if item.Text != "" {
				fmt.Fprintf(&b, " — %s", timelineTruncate(item.Text, 80))
			} else if item.Label != "" {
				fmt.Fprintf(&b, " — %s", item.Label)
			} else if item.Notes != "" {
				fmt.Fprintf(&b, " — %s", timelineTruncate(item.Notes, 80))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if renderPlan != nil {
		b.WriteString("## Render Plan\n\n")
		fmt.Fprintf(&b, "- Planned output: `%s`\n", renderPlan.PlannedOutput.PlannedFile)
		fmt.Fprintf(&b, "- Steps: %d\n\n", len(renderPlan.Steps))
		for _, step := range renderPlan.Steps {
			fmt.Fprintf(&b, "- [%d] `%s` on `%s` [%.2f–%.2fs]", step.StepIndex, step.Operation, step.ItemID, step.TimelineStart, step.TimelineEnd)
			if step.Notes != "" {
				fmt.Fprintf(&b, " — %s", timelineTruncate(step.Notes, 60))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(timeline.Warnings) > 0 {
		b.WriteString("## Warnings\n\n")
		for _, w := range timeline.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
		b.WriteString("\n")
	}

	// assemble result if present
	assembleResultPath := filepath.Join(planDir, "outputs", "creative_assemble_result.json")
	if arData, err2 := os.ReadFile(assembleResultPath); err2 == nil {
		var ar CreativeAssembleResult
		if json.Unmarshal(arData, &ar) == nil {
			b.WriteString("## Assemble\n\n")
			fmt.Fprintf(&b, "- Status: `%s`\n", ar.Status)
			fmt.Fprintf(&b, "- Mode: `%s`\n", ar.Mode)
			fmt.Fprintf(&b, "- Output: `%s`\n\n", ar.OutputFile)
		}
	}

	b.WriteString("## Next Commands\n\n")
	if renderPlan == nil {
		fmt.Fprintf(&b, "```sh\nbyom-video creative-render-plan %s\n```\n\n", planID)
	}
	if _, err2 := os.Stat(assembleResultPath); err2 != nil {
		fmt.Fprintf(&b, "```sh\nbyom-video creative-assemble %s\n```\n\n", planID)
	}
	fmt.Fprintf(&b, "```sh\nbyom-video validate-creative-plan %s\n```\n", planID)

	review := b.String()
	fmt.Fprint(stdout, review)

	if opts.WriteArtifact {
		reviewPath := filepath.Join(planDir, "outputs", "creative_timeline_review.md")
		if err := os.WriteFile(reviewPath, []byte(review), 0o644); err != nil {
			return fmt.Errorf("writing creative_timeline_review.md: %w", err)
		}
		if err := updateCreativeOutputsIndex(planID, "creative_timeline_review", "outputs/creative_timeline_review.md", ""); err != nil {
			fmt.Fprintf(stdout, "warning: could not update creative_outputs.json: %v\n", err)
		}
		fmt.Fprintf(stdout, "\nartifact written: %s\n", reviewPath)
	}

	return nil
}

package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/editorartifacts"
	"github.com/mirelahmd/byom-video/internal/events"
	"github.com/mirelahmd/byom-video/internal/goalartifacts"
	"github.com/mirelahmd/byom-video/internal/manifest"
	"github.com/mirelahmd/byom-video/internal/report"
	"github.com/mirelahmd/byom-video/internal/roughcut"
	"github.com/mirelahmd/byom-video/internal/runstore"
)

type ClipCardsOptions struct {
	Overwrite          bool
	JSON               bool
	PreferGoalRoughcut bool
}

type ReviewClipsOptions struct {
	JSON          bool
	WriteArtifact bool
}

type EnhanceRoughcutOptions struct {
	Overwrite bool
	JSON      bool
}

type ClipCardsSummary struct {
	RunID    string   `json:"run_id"`
	Artifact string   `json:"artifact"`
	Count    int      `json:"count"`
	Warnings []string `json:"warnings,omitempty"`
}

type ClipCardsReview struct {
	RunID    string                     `json:"run_id"`
	Artifact string                     `json:"artifact"`
	Cards    []editorartifacts.ClipCard `json:"cards"`
}

type EnhancedRoughcutSummary struct {
	RunID    string `json:"run_id"`
	Artifact string `json:"artifact"`
	Count    int    `json:"count"`
}

func ClipCardsCommand(runID string, stdout io.Writer, opts ClipCardsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("CLIP_CARDS_STARTED", map[string]any{"run_id": runID})
	}

	outPath := filepath.Join(runDir, "clip_cards.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "CLIP_CARDS_FAILED", "clip_cards.json already exists; pass --overwrite")
			return fmt.Errorf("clip_cards.json already exists; pass --overwrite")
		}
	}

	doc, warnings, err := buildClipCards(runDir, runID, opts)
	if err != nil {
		writeMaskFailure(log, "CLIP_CARDS_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "CLIP_CARDS_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "clip_cards", "clip_cards.json"); err != nil {
		writeMaskFailure(log, "CLIP_CARDS_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "CLIP_CARDS_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("CLIP_CARDS_COMPLETED", map[string]any{"path": "clip_cards.json", "cards": len(doc.Cards)})
	}

	summary := ClipCardsSummary{
		RunID:    runID,
		Artifact: filepath.Join(runDir, "clip_cards.json"),
		Count:    len(doc.Cards),
		Warnings: warnings,
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, "Clip cards created")
	fmt.Fprintf(stdout, "  run id:   %s\n", runID)
	fmt.Fprintf(stdout, "  path:     %s\n", outPath)
	fmt.Fprintf(stdout, "  cards:    %d\n", len(doc.Cards))
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "  warning:  %s\n", warning)
	}
	return nil
}

func ReviewClips(runID string, stdout io.Writer, opts ReviewClipsOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	doc, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
	if err != nil {
		return err
	}
	review := ClipCardsReview{
		RunID:    runID,
		Artifact: "clip_cards.json",
		Cards:    doc.Cards,
	}
	if opts.WriteArtifact {
		path := filepath.Join(runDir, "clip_cards_review.md")
		if err := writeClipCardsReview(path, doc); err != nil {
			return err
		}
		if err := addManifestArtifact(runDir, "clip_cards_review", "clip_cards_review.md"); err != nil {
			return err
		}
		if err := refreshReportIfPresent(runDir); err != nil {
			return err
		}
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(review, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Clip cards review")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  cards:  %d\n", len(doc.Cards))
	for _, card := range doc.Cards {
		fmt.Fprintf(stdout, "  - %s | %s | %.3f-%.3f | %.3fs | %s | %s\n",
			card.ID,
			card.Title,
			card.Start,
			card.End,
			card.DurationSeconds,
			emptyDash(firstCaption(card.Captions)),
			card.VerificationStatus,
		)
		for _, warning := range card.Warnings {
			fmt.Fprintf(stdout, "    warning: %s\n", warning)
		}
	}
	if opts.WriteArtifact {
		fmt.Fprintf(stdout, "  artifact: %s\n", filepath.Join(runDir, "clip_cards_review.md"))
	}
	return nil
}

func EnhanceRoughcut(runID string, stdout io.Writer, opts EnhanceRoughcutOptions) error {
	runDir, err := runstore.RequireRunDir(runID)
	if err != nil {
		return err
	}
	log, _ := events.Open(filepath.Join(runDir, "events.jsonl"))
	if log != nil {
		defer log.Close()
		_ = log.Write("ENHANCED_ROUGHCUT_STARTED", map[string]any{"run_id": runID})
	}

	outPath := filepath.Join(runDir, "enhanced_roughcut.json")
	if !opts.Overwrite {
		if _, err := os.Stat(outPath); err == nil {
			writeMaskFailure(log, "ENHANCED_ROUGHCUT_FAILED", "enhanced_roughcut.json already exists; pass --overwrite")
			return fmt.Errorf("enhanced_roughcut.json already exists; pass --overwrite")
		}
	}

	doc, err := buildEnhancedRoughcut(runDir, runID)
	if err != nil {
		writeMaskFailure(log, "ENHANCED_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := writeJSONFile(outPath, doc); err != nil {
		writeMaskFailure(log, "ENHANCED_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := addManifestArtifact(runDir, "enhanced_roughcut", "enhanced_roughcut.json"); err != nil {
		writeMaskFailure(log, "ENHANCED_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if err := refreshReportIfPresent(runDir); err != nil {
		writeMaskFailure(log, "ENHANCED_ROUGHCUT_FAILED", err.Error())
		return err
	}
	if log != nil {
		_ = log.Write("ENHANCED_ROUGHCUT_COMPLETED", map[string]any{"path": "enhanced_roughcut.json", "clips": len(doc.Clips)})
	}

	summary := EnhancedRoughcutSummary{
		RunID:    runID,
		Artifact: filepath.Join(runDir, "enhanced_roughcut.json"),
		Count:    len(doc.Clips),
	}
	if opts.JSON {
		data, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}
	fmt.Fprintln(stdout, "Enhanced roughcut created")
	fmt.Fprintf(stdout, "  run id: %s\n", runID)
	fmt.Fprintf(stdout, "  path:   %s\n", outPath)
	fmt.Fprintf(stdout, "  clips:  %d\n", len(doc.Clips))
	return nil
}

func buildClipCards(runDir string, runID string, opts ClipCardsOptions) (editorartifacts.ClipCards, []string, error) {
	mask, _ := readInferenceMask(filepath.Join(runDir, "inference_mask.json"))
	maskPresent := len(mask.Decisions) > 0

	captionByDecision := readExpansionTextsByDecision(filepath.Join(runDir, "expansions", "caption_variants.json"))
	labelByDecision := readExpansionTextsByDecision(filepath.Join(runDir, "expansions", "timeline_labels.json"))
	descriptionByDecision := readExpansionTextsByDecision(filepath.Join(runDir, "expansions", "short_descriptions.json"))

	verificationWarnings, verificationStatusByDecision, overallVerificationStatus := loadVerificationWarnings(filepath.Join(runDir, "verification_results.json"))
	if overallVerificationStatus == "" {
		overallVerificationStatus = "unknown"
	}

	warnings := []string{}
	if len(verificationWarnings) > 0 {
		warnings = append(warnings, verificationWarnings...)
	}
	if !maskPresent {
		warnings = append(warnings, "inference_mask.json not present; using roughcut-only card mapping")
	}

	cards := []editorartifacts.ClipCard{}
	source := editorartifacts.ClipCardsSource{
		ExpansionsDir: "expansions",
	}
	if opts.PreferGoalRoughcut {
		goalDoc, err := goalartifacts.ReadGoalRoughcut(filepath.Join(runDir, "goal_roughcut.json"))
		if err != nil {
			return editorartifacts.ClipCards{}, nil, fmt.Errorf("read goal roughcut: %w", err)
		}
		source.GoalRoughcutArtifact = "goal_roughcut.json"
		source.RoughcutArtifact = "roughcut.json"
		for index, clip := range goalDoc.Clips {
			decision, found := matchDecisionForGoalClip(mask.Decisions, clip)
			decisionID := ""
			sourceText := strings.TrimSpace(clip.Text)
			editIntent := strings.TrimSpace(clip.Reason)
			if found {
				decisionID = decision.ID
				if sourceText == "" {
					sourceText = decision.TextPreview
				}
				if editIntent == "" {
					editIntent = decision.Reason
				}
			}
			title := fallbackClipTitle(clip.Text)
			if labels := labelByDecision[decisionID]; len(labels) > 0 {
				title = labels[0]
			}
			description := fallbackClipDescription(clip.Text, editIntent)
			if descriptions := descriptionByDecision[decisionID]; len(descriptions) > 0 {
				description = descriptions[0]
			}
			cardWarnings := []string{}
			verificationStatus := overallVerificationStatus
			if verificationStatus == "" {
				verificationStatus = "unknown"
			}
			if found {
				if status, ok := verificationStatusByDecision[decision.ID]; ok {
					verificationStatus = status.Status
					cardWarnings = append(cardWarnings, status.Warnings...)
				} else if overallVerificationStatus != "passed" && overallVerificationStatus != "unknown" {
					cardWarnings = append(cardWarnings, "verification reported warnings or failures; review verification_results.json")
				}
			}
			duration := clip.DurationSeconds
			if duration <= 0 && clip.End >= clip.Start {
				duration = clip.End - clip.Start
			}
			cards = append(cards, editorartifacts.ClipCard{
				ID:                 fmt.Sprintf("card_%04d", index+1),
				ClipID:             clip.ID,
				HighlightID:        clip.HighlightID,
				DecisionID:         decisionID,
				Start:              clip.Start,
				End:                clip.End,
				DurationSeconds:    duration,
				Score:              clip.GoalScore,
				Title:              title,
				Description:        description,
				Captions:           uniqueNonEmpty(captionByDecision[decisionID]),
				SourceText:         sourceText,
				EditIntent:         editIntent,
				VerificationStatus: nonEmptyString(verificationStatus, "unknown"),
				Warnings:           cardWarnings,
			})
		}
	} else {
		roughcutPath := filepath.Join(runDir, "roughcut.json")
		roughcutDoc, err := readRoughcutDocument(roughcutPath)
		if err != nil {
			return editorartifacts.ClipCards{}, nil, fmt.Errorf("read roughcut: %w", err)
		}
		source.RoughcutArtifact = "roughcut.json"
		cards = make([]editorartifacts.ClipCard, 0, len(roughcutDoc.Clips))
		for index, clip := range roughcutDoc.Clips {
			decision, found := matchDecisionForClip(mask.Decisions, clip)
			decisionID := ""
			sourceText := strings.TrimSpace(clip.Text)
			editIntent := strings.TrimSpace(clip.EditIntent)
			if found {
				decisionID = decision.ID
				if sourceText == "" {
					sourceText = decision.TextPreview
				}
				if editIntent == "" {
					editIntent = decision.Reason
				}
			}
			title := fallbackClipTitle(clip.Text)
			if labels := labelByDecision[decisionID]; len(labels) > 0 {
				title = labels[0]
			}
			description := fallbackClipDescription(clip.Text, editIntent)
			if descriptions := descriptionByDecision[decisionID]; len(descriptions) > 0 {
				description = descriptions[0]
			}
			cardWarnings := []string{}
			verificationStatus := overallVerificationStatus
			if verificationStatus == "" {
				verificationStatus = "unknown"
			}
			if found {
				if status, ok := verificationStatusByDecision[decision.ID]; ok {
					verificationStatus = status.Status
					cardWarnings = append(cardWarnings, status.Warnings...)
				} else if overallVerificationStatus != "passed" && overallVerificationStatus != "unknown" {
					cardWarnings = append(cardWarnings, "verification reported warnings or failures; review verification_results.json")
				}
			} else if overallVerificationStatus != "passed" && overallVerificationStatus != "unknown" {
				cardWarnings = append(cardWarnings, "verification reported warnings or failures; review verification_results.json")
			}

			duration := clip.DurationSeconds
			if duration <= 0 && clip.End >= clip.Start {
				duration = clip.End - clip.Start
			}
			card := editorartifacts.ClipCard{
				ID:                 fmt.Sprintf("card_%04d", index+1),
				ClipID:             clip.ID,
				HighlightID:        clip.HighlightID,
				DecisionID:         decisionID,
				Start:              clip.Start,
				End:                clip.End,
				DurationSeconds:    duration,
				Score:              clip.Score,
				Title:              title,
				Description:        description,
				Captions:           uniqueNonEmpty(captionByDecision[decisionID]),
				SourceText:         sourceText,
				EditIntent:         editIntent,
				VerificationStatus: nonEmptyString(verificationStatus, "unknown"),
				Warnings:           cardWarnings,
			}
			cards = append(cards, card)
		}
	}

	doc := editorartifacts.ClipCards{
		SchemaVersion: "clip_cards.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Source:        source,
		Cards:         cards,
	}
	if maskPresent {
		doc.Source.InferenceMaskArtifact = "inference_mask.json"
	}
	return doc, warnings, nil
}

func buildEnhancedRoughcut(runDir string, runID string) (editorartifacts.EnhancedRoughcut, error) {
	roughcutDoc, err := readRoughcutDocument(filepath.Join(runDir, "roughcut.json"))
	if err != nil {
		return editorartifacts.EnhancedRoughcut{}, fmt.Errorf("read roughcut: %w", err)
	}
	cardMap := map[string]editorartifacts.ClipCard{}
	source := editorartifacts.EnhancedRoughcutSource{RoughcutArtifact: "roughcut.json"}
	if _, err := os.Stat(filepath.Join(runDir, "clip_cards.json")); err == nil {
		cards, err := editorartifacts.ReadClipCards(filepath.Join(runDir, "clip_cards.json"))
		if err != nil {
			return editorartifacts.EnhancedRoughcut{}, err
		}
		source.ClipCardsArtifact = "clip_cards.json"
		for _, card := range cards.Cards {
			cardMap[card.ClipID] = card
		}
	}

	clips := make([]editorartifacts.EnhancedRoughcutClip, 0, len(roughcutDoc.Clips))
	total := 0.0
	planTitle := "Enhanced Rough Cut Plan"
	planIntent := ""
	if strings.TrimSpace(roughcutDoc.Plan.Title) != "" {
		planTitle = roughcutDoc.Plan.Title
	}
	planIntent = roughcutDoc.Plan.Intent
	total = roughcutDoc.Plan.TotalDurationSeconds
	for _, clip := range roughcutDoc.Clips {
		card, ok := cardMap[clip.ID]
		if !ok {
			card = editorartifacts.ClipCard{
				ClipID:             clip.ID,
				Start:              clip.Start,
				End:                clip.End,
				DurationSeconds:    clip.DurationSeconds,
				Title:              fallbackClipTitle(clip.Text),
				Description:        fallbackClipDescription(clip.Text, clip.EditIntent),
				SourceText:         clip.Text,
				EditIntent:         clip.EditIntent,
				VerificationStatus: "unknown",
			}
		}
		clips = append(clips, editorartifacts.EnhancedRoughcutClip{
			ID:                 clip.ID,
			Start:              clip.Start,
			End:                clip.End,
			Order:              clip.Order,
			Title:              card.Title,
			Description:        card.Description,
			CaptionSuggestions: card.Captions,
			EditIntent:         nonEmptyString(card.EditIntent, clip.EditIntent),
			VerificationStatus: nonEmptyString(card.VerificationStatus, "unknown"),
			SourceText:         nonEmptyString(card.SourceText, clip.Text),
		})
	}
	return editorartifacts.EnhancedRoughcut{
		SchemaVersion: "enhanced_roughcut.v1",
		CreatedAt:     time.Now().UTC(),
		RunID:         runID,
		Source:        source,
		Plan: editorartifacts.EnhancedRoughcutPlan{
			Title:                planTitle,
			Intent:               planIntent,
			TotalDurationSeconds: total,
		},
		Clips: clips,
	}, nil
}

type decisionVerificationStatus struct {
	Status   string
	Warnings []string
}

func loadVerificationWarnings(path string) ([]string, map[string]decisionVerificationStatus, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, map[string]decisionVerificationStatus{}, ""
	}
	var results VerificationResults
	if err := json.Unmarshal(data, &results); err != nil {
		return []string{"verification_results.json could not be decoded"}, map[string]decisionVerificationStatus{}, "warning"
	}
	perDecision := map[string]decisionVerificationStatus{}
	globalWarnings := []string{}
	for _, check := range results.Checks {
		if check.Type != "missing_required_decisions" {
			continue
		}
		raw, ok := check.Details["missing_decision_ids"]
		if !ok {
			continue
		}
		values, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, value := range values {
			decisionID, ok := value.(string)
			if !ok || strings.TrimSpace(decisionID) == "" {
				continue
			}
			perDecision[decisionID] = decisionVerificationStatus{
				Status:   check.Status,
				Warnings: []string{nonEmptyString(check.Message, "verification reported a missing required decision")},
			}
		}
	}
	if results.Status == "failed" {
		globalWarnings = append(globalWarnings, "verification_results.json reported failures")
	} else if results.Status == "warning" {
		globalWarnings = append(globalWarnings, "verification_results.json reported warnings")
	}
	return globalWarnings, perDecision, results.Status
}

func matchDecisionForClip(decisions []MaskDecision, clip roughcut.Clip) (MaskDecision, bool) {
	for _, decision := range decisions {
		if decision.ClipID != "" && decision.ClipID == clip.ID {
			return decision, true
		}
	}
	for _, decision := range decisions {
		if clip.HighlightID != "" && decision.HighlightID == clip.HighlightID {
			return decision, true
		}
	}
	for _, decision := range decisions {
		if clip.SourceChunkID != "" && (decision.SourceChunkID == clip.SourceChunkID || decision.ChunkID == clip.SourceChunkID) {
			return decision, true
		}
	}
	for _, decision := range decisions {
		if sameTiming(decision.Start, clip.Start) && sameTiming(decision.End, clip.End) {
			return decision, true
		}
	}
	return MaskDecision{}, false
}

func matchDecisionForGoalClip(decisions []MaskDecision, clip goalartifacts.GoalRoughcutClip) (MaskDecision, bool) {
	for _, decision := range decisions {
		if clip.HighlightID != "" && decision.HighlightID == clip.HighlightID {
			return decision, true
		}
	}
	for _, decision := range decisions {
		if clip.ChunkID != "" && (decision.SourceChunkID == clip.ChunkID || decision.ChunkID == clip.ChunkID) {
			return decision, true
		}
	}
	for _, decision := range decisions {
		if sameTiming(decision.Start, clip.Start) && sameTiming(decision.End, clip.End) {
			return decision, true
		}
	}
	return MaskDecision{}, false
}

func sameTiming(a float64, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 0.01
}

func readExpansionTextsByDecision(path string) map[string][]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string][]string{}
	}
	var output ExpansionOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return map[string][]string{}
	}
	values := map[string][]string{}
	for _, item := range output.Items {
		if strings.TrimSpace(item.DecisionID) == "" || strings.TrimSpace(item.Text) == "" {
			continue
		}
		values[item.DecisionID] = append(values[item.DecisionID], strings.TrimSpace(item.Text))
	}
	return values
}

func fallbackClipTitle(text string) string {
	clean := strings.Join(strings.Fields(text), " ")
	if clean == "" {
		return "Untitled clip"
	}
	title := firstNWords(clean, 6)
	return strings.TrimSpace(strings.TrimSuffix(title, "..."))
}

func fallbackClipDescription(text string, editIntent string) string {
	if strings.TrimSpace(editIntent) != "" {
		return editIntent
	}
	clean := strings.Join(strings.Fields(text), " ")
	if clean == "" {
		return "Clip derived from roughcut selection."
	}
	return firstNWords(clean, 18)
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstCaption(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func refreshReportIfPresent(runDir string) error {
	manifestPath := filepath.Join(runDir, "manifest.json")
	m, err := manifest.Read(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	hasReport := false
	for _, artifact := range m.Artifacts {
		if artifact.Path == "report.html" || artifact.Name == "report" {
			hasReport = true
			break
		}
	}
	if !hasReport {
		if _, err := os.Stat(filepath.Join(runDir, "report.html")); err == nil {
			hasReport = true
		}
	}
	if !hasReport {
		return nil
	}
	if !slices.ContainsFunc(m.Artifacts, func(artifact manifest.Artifact) bool {
		return artifact.Path == "report.html"
	}) {
		m.AddArtifact("report", "report.html")
		if err := manifest.Write(manifestPath, m); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
	}
	_, err = report.Write(runDir, m)
	return err
}

func readRoughcutDocument(path string) (roughcut.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return roughcut.Document{}, err
	}
	var doc roughcut.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return roughcut.Document{}, fmt.Errorf("decode roughcut: %w", err)
	}
	return doc, nil
}

func writeClipCardsReview(path string, doc editorartifacts.ClipCards) error {
	var builder strings.Builder
	builder.WriteString("# Clip Cards Review\n\n")
	builder.WriteString(fmt.Sprintf("- generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- run_id: %s\n", doc.RunID))
	builder.WriteString(fmt.Sprintf("- cards: %d\n\n", len(doc.Cards)))
	for _, card := range doc.Cards {
		builder.WriteString(fmt.Sprintf("## %s\n\n", card.Title))
		builder.WriteString(fmt.Sprintf("- card_id: %s\n", card.ID))
		builder.WriteString(fmt.Sprintf("- clip_id: %s\n", card.ClipID))
		builder.WriteString(fmt.Sprintf("- range: %.3f-%.3f\n", card.Start, card.End))
		builder.WriteString(fmt.Sprintf("- duration_seconds: %.3f\n", card.DurationSeconds))
		builder.WriteString(fmt.Sprintf("- verification_status: %s\n", card.VerificationStatus))
		if len(card.Captions) > 0 {
			builder.WriteString("- captions:\n")
			for _, caption := range card.Captions {
				builder.WriteString(fmt.Sprintf("  - %s\n", caption))
			}
		}
		builder.WriteString(fmt.Sprintf("- description: %s\n", card.Description))
		if len(card.Warnings) > 0 {
			builder.WriteString("- warnings:\n")
			for _, warning := range card.Warnings {
				builder.WriteString(fmt.Sprintf("  - %s\n", warning))
			}
		}
		builder.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write clip cards review: %w", err)
	}
	return nil
}

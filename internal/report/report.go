package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirelahmd/byom-video/internal/editorartifacts"
	"github.com/mirelahmd/byom-video/internal/exportartifacts"
	"github.com/mirelahmd/byom-video/internal/exporter"
	"github.com/mirelahmd/byom-video/internal/goalartifacts"
	"github.com/mirelahmd/byom-video/internal/manifest"
)

type Summary struct {
	ArtifactPath string
}

type metadataDoc struct {
	Format *struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
	} `json:"streams"`
}

type transcriptDoc struct {
	Language        string   `json:"language"`
	DurationSeconds *float64 `json:"duration_seconds"`
	Segments        []struct {
		ID    string  `json:"id"`
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
}

type chunksDoc struct {
	Chunks []struct {
		ID string `json:"id"`
	} `json:"chunks"`
}

type highlightsDoc struct {
	Highlights []struct {
		ID     string  `json:"id"`
		Start  float64 `json:"start"`
		End    float64 `json:"end"`
		Score  float64 `json:"score"`
		Text   string  `json:"text"`
		Reason string  `json:"reason"`
	} `json:"highlights"`
}

type roughcutDoc struct {
	Plan *struct {
		TotalDurationSeconds float64 `json:"total_duration_seconds"`
	} `json:"plan"`
	Clips []struct {
		ID              string  `json:"id"`
		HighlightID     string  `json:"highlight_id"`
		SourceChunkID   string  `json:"source_chunk_id"`
		Start           float64 `json:"start"`
		End             float64 `json:"end"`
		DurationSeconds float64 `json:"duration_seconds"`
		Order           int     `json:"order"`
		Score           float64 `json:"score"`
		Text            string  `json:"text"`
	} `json:"clips"`
}

type expansionOutputDoc struct {
	TaskType string `json:"task_type"`
	Items    []struct {
		Text       string `json:"text"`
		DecisionID string `json:"decision_id"`
	} `json:"items"`
}

type verificationResultsDoc struct {
	Status  string `json:"status"`
	Summary struct {
		ChecksTotal  int `json:"checks_total"`
		ChecksPassed int `json:"checks_passed"`
		ChecksFailed int `json:"checks_failed"`
		Warnings     int `json:"warnings"`
	} `json:"summary"`
}

func Write(runDir string, m manifest.Manifest) (Summary, error) {
	var b bytes.Buffer
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>BYOM Video Run Report</title>\n")
	b.WriteString("<style>body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;line-height:1.45;margin:32px;max-width:1100px;color:#151515}table{border-collapse:collapse;width:100%;margin:12px 0}th,td{border:1px solid #ddd;padding:8px;text-align:left;vertical-align:top}th{background:#f5f5f5}code{background:#f5f5f5;padding:2px 4px}.muted{color:#666}.preview{max-width:520px}</style>\n")
	b.WriteString("</head>\n<body>\n")
	fmt.Fprintf(&b, "<h1>BYOM Video Run Report</h1>\n<p class=\"muted\">All paths are local to this machine.</p>\n")
	writeKVTable(&b, "Run", [][2]string{
		{"run id", m.RunID},
		{"input path", m.InputPath},
		{"created at", m.CreatedAt.String()},
		{"status", m.Status},
	})
	writeMetadata(&b, filepath.Join(runDir, "metadata.json"))
	writeTranscript(&b, filepath.Join(runDir, "transcript.json"))
	writeCaptions(&b, filepath.Join(runDir, "captions.srt"))
	writeChunks(&b, filepath.Join(runDir, "chunks.json"))
	writeHighlights(&b, filepath.Join(runDir, "highlights.json"))
	writeRoughcut(&b, filepath.Join(runDir, "roughcut.json"))
	writeGoalAwareEditingIntro(&b, runDir)
	writeGoalRerank(&b, filepath.Join(runDir, "goal_rerank.json"))
	writeClipCards(&b, filepath.Join(runDir, "clip_cards.json"))
	writeEnhancedRoughcut(&b, filepath.Join(runDir, "enhanced_roughcut.json"))
	writeGoalRoughcut(&b, filepath.Join(runDir, "goal_roughcut.json"))
	writeSelectedClips(&b, filepath.Join(runDir, "selected_clips.json"))
	writeExportManifest(&b, filepath.Join(runDir, "export_manifest.json"))
	writeExpansionOutputs(&b, runDir)
	writeVerificationSummary(&b, filepath.Join(runDir, "verification_results.json"))
	writeFFmpegScript(&b, filepath.Join(runDir, "ffmpeg_commands.sh"))
	writeConcatPlan(&b, runDir)
	writeExports(&b, runDir, m)
	writeArtifacts(&b, m)
	b.WriteString("</body>\n</html>\n")

	if err := os.WriteFile(filepath.Join(runDir, "report.html"), b.Bytes(), 0o644); err != nil {
		return Summary{}, fmt.Errorf("write report: %w", err)
	}
	return Summary{ArtifactPath: "report.html"}, nil
}

func Escape(value string) string {
	return html.EscapeString(value)
}

func writeKVTable(b *bytes.Buffer, title string, rows [][2]string) {
	fmt.Fprintf(b, "<h2>%s</h2>\n<table><tbody>\n", Escape(title))
	for _, row := range rows {
		fmt.Fprintf(b, "<tr><th>%s</th><td>%s</td></tr>\n", Escape(row[0]), Escape(row[1]))
	}
	b.WriteString("</tbody></table>\n")
}

func writeMetadata(b *bytes.Buffer, path string) {
	var doc metadataDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	video := 0
	audio := 0
	for _, stream := range doc.Streams {
		switch stream.CodecType {
		case "video":
			video++
		case "audio":
			audio++
		}
	}
	duration := "unknown"
	if doc.Format != nil && doc.Format.Duration != "" {
		duration = doc.Format.Duration + " seconds"
	}
	writeKVTable(b, "Media Metadata", [][2]string{
		{"duration", duration},
		{"video streams", fmt.Sprintf("%d", video)},
		{"audio streams", fmt.Sprintf("%d", audio)},
		{"total streams", fmt.Sprintf("%d", len(doc.Streams))},
	})
}

func writeTranscript(b *bytes.Buffer, path string) {
	var doc transcriptDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	duration := "unknown"
	if doc.DurationSeconds != nil {
		duration = fmt.Sprintf("%.3f seconds", *doc.DurationSeconds)
	}
	writeKVTable(b, "Transcript", [][2]string{
		{"language", doc.Language},
		{"segments", fmt.Sprintf("%d", len(doc.Segments))},
		{"duration", duration},
	})
}

func writeCaptions(b *bytes.Buffer, path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	b.WriteString("<h2>Captions</h2>\n<p><code>captions.srt</code> exists.</p>\n")
}

func writeChunks(b *bytes.Buffer, path string) {
	var doc chunksDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	writeKVTable(b, "Chunks", [][2]string{{"chunk count", fmt.Sprintf("%d", len(doc.Chunks))}})
}

func writeHighlights(b *bytes.Buffer, path string) {
	var doc highlightsDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	fmt.Fprintf(b, "<h2>Highlights</h2>\n<p>Highlight count: %d</p>\n", len(doc.Highlights))
	b.WriteString("<table><thead><tr><th>score</th><th>range</th><th>text</th><th>reason</th></tr></thead><tbody>\n")
	limit := len(doc.Highlights)
	if limit > 5 {
		limit = 5
	}
	for _, highlight := range doc.Highlights[:limit] {
		fmt.Fprintf(b, "<tr><td>%.3f</td><td>%.3f-%.3f</td><td class=\"preview\">%s</td><td>%s</td></tr>\n",
			highlight.Score, highlight.Start, highlight.End, Escape(preview(highlight.Text, 220)), Escape(highlight.Reason))
	}
	b.WriteString("</tbody></table>\n")
}

func writeRoughcut(b *bytes.Buffer, path string) {
	var doc roughcutDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	total := 0.0
	if doc.Plan != nil {
		total = doc.Plan.TotalDurationSeconds
	}
	fmt.Fprintf(b, "<h2>Rough Cut</h2>\n<p>Clip count: %d<br>Total duration: %.3f seconds</p>\n", len(doc.Clips), total)
	b.WriteString("<table><thead><tr><th>order</th><th>clip</th><th>highlight</th><th>range</th><th>score</th><th>text</th></tr></thead><tbody>\n")
	for _, clip := range doc.Clips {
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%.3f-%.3f</td><td>%.3f</td><td class=\"preview\">%s</td></tr>\n",
			clip.Order, Escape(clip.ID), Escape(clip.HighlightID), clip.Start, clip.End, clip.Score, Escape(preview(clip.Text, 220)))
	}
	b.WriteString("</tbody></table>\n")
}

func writeClipCards(b *bytes.Buffer, path string) {
	doc, err := editorartifacts.ReadClipCards(path)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "<h2>Clip Cards</h2>\n<p>Card count: %d</p>\n", len(doc.Cards))
	if doc.Source.GoalRoughcutArtifact != "" {
		fmt.Fprintf(b, "<p>Source: <code>%s</code></p>\n", Escape(doc.Source.GoalRoughcutArtifact))
	}
	b.WriteString("<table><thead><tr><th>title</th><th>range</th><th>captions</th><th>description</th><th>verification</th></tr></thead><tbody>\n")
	for _, card := range doc.Cards {
		fmt.Fprintf(b, "<tr><td>%s</td><td>%.3f-%.3f</td><td class=\"preview\">%s</td><td class=\"preview\">%s</td><td>%s</td></tr>\n",
			Escape(card.Title),
			card.Start,
			card.End,
			Escape(strings.Join(card.Captions, " | ")),
			Escape(preview(card.Description, 220)),
			Escape(card.VerificationStatus),
		)
	}
	b.WriteString("</tbody></table>\n")
}

func writeEnhancedRoughcut(b *bytes.Buffer, path string) {
	doc, err := editorartifacts.ReadEnhancedRoughcut(path)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "<h2>Enhanced Roughcut</h2>\n<p>Clip count: %d<br>Total duration: %.3f seconds</p>\n", len(doc.Clips), doc.Plan.TotalDurationSeconds)
	b.WriteString("<table><thead><tr><th>order</th><th>title</th><th>range</th><th>description</th><th>captions</th><th>verification</th></tr></thead><tbody>\n")
	for _, clip := range doc.Clips {
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%.3f-%.3f</td><td class=\"preview\">%s</td><td class=\"preview\">%s</td><td>%s</td></tr>\n",
			clip.Order,
			Escape(clip.Title),
			clip.Start,
			clip.End,
			Escape(preview(clip.Description, 220)),
			Escape(strings.Join(clip.CaptionSuggestions, " | ")),
			Escape(clip.VerificationStatus),
		)
	}
	b.WriteString("</tbody></table>\n")
}

func writeSelectedClips(b *bytes.Buffer, path string) {
	doc, err := exportartifacts.ReadSelectedClips(path)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "<h2>Selected Clips</h2>\n<p>Clip count: %d</p>\n", len(doc.Clips))
	if source := selectedClipSourceLabel(doc.Source); source != "" {
		fmt.Fprintf(b, "<p>Source: <code>%s</code></p>\n", Escape(source))
	}
	b.WriteString("<table><thead><tr><th>order</th><th>title</th><th>range</th><th>output</th><th>description</th></tr></thead><tbody>\n")
	for _, clip := range doc.Clips {
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%.3f-%.3f</td><td><code>%s</code></td><td class=\"preview\">%s</td></tr>\n",
			clip.Order, Escape(clip.Title), clip.Start, clip.End, Escape(clip.OutputFilename), Escape(preview(clip.Description, 220)))
	}
	b.WriteString("</tbody></table>\n")
}

func writeExportManifest(b *bytes.Buffer, path string) {
	doc, err := exportartifacts.ReadExportManifest(path)
	if err != nil {
		return
	}
	writeKVTable(b, "Export Manifest", [][2]string{
		{"planned", fmt.Sprintf("%d", doc.Summary.Planned)},
		{"exported", fmt.Sprintf("%d", doc.Summary.Exported)},
		{"validated", fmt.Sprintf("%d", doc.Summary.Validated)},
		{"missing", fmt.Sprintf("%d", doc.Summary.Missing)},
	})
}

func writeGoalRerank(b *bytes.Buffer, path string) {
	doc, err := goalartifacts.ReadGoalRerank(path)
	if err != nil {
		return
	}
	writeKVTable(b, "Goal Rerank", [][2]string{
		{"goal", doc.Goal},
		{"mode", doc.Mode},
		{"preferred style", doc.Constraints.PreferredStyle},
		{"max total duration", fmt.Sprintf("%.0f seconds", doc.Constraints.MaxTotalDurationSeconds)},
		{"max clips", fmt.Sprintf("%d", doc.Constraints.MaxClips)},
		{"ranked highlights", fmt.Sprintf("%d", len(doc.RankedHighlights))},
	})
	if len(doc.RankedHighlights) == 0 {
		return
	}
	b.WriteString("<table><thead><tr><th>rank</th><th>highlight</th><th>range</th><th>goal score</th><th>reason</th><th>text</th></tr></thead><tbody>\n")
	limit := len(doc.RankedHighlights)
	if limit > 5 {
		limit = 5
	}
	for _, item := range doc.RankedHighlights[:limit] {
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%.3f-%.3f</td><td>%.3f</td><td>%s</td><td class=\"preview\">%s</td></tr>\n",
			item.Rank, Escape(item.HighlightID), item.Start, item.End, item.GoalScore, Escape(item.Reason), Escape(preview(item.Text, 220)))
	}
	b.WriteString("</tbody></table>\n")
}

func writeGoalRoughcut(b *bytes.Buffer, path string) {
	doc, err := goalartifacts.ReadGoalRoughcut(path)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "<h2>Goal Roughcut</h2>\n<p>Goal: %s<br>Clip count: %d<br>Total duration: %.3f seconds</p>\n",
		Escape(doc.Goal), len(doc.Clips), doc.Plan.TotalDurationSeconds)
	b.WriteString("<table><thead><tr><th>order</th><th>highlight</th><th>range</th><th>goal score</th><th>reason</th><th>text</th></tr></thead><tbody>\n")
	for _, clip := range doc.Clips {
		fmt.Fprintf(b, "<tr><td>%d</td><td>%s</td><td>%.3f-%.3f</td><td>%.3f</td><td>%s</td><td class=\"preview\">%s</td></tr>\n",
			clip.Order, Escape(clip.HighlightID), clip.Start, clip.End, clip.GoalScore, Escape(clip.Reason), Escape(preview(clip.Text, 220)))
	}
	b.WriteString("</tbody></table>\n")
}

func writeGoalAwareEditingIntro(b *bytes.Buffer, runDir string) {
	if _, err := os.Stat(filepath.Join(runDir, "goal_roughcut.json")); err != nil {
		return
	}
	b.WriteString("<h2>Goal-Aware Editing</h2>\n<p>These artifacts preserve the original deterministic roughcut and add a separate goal-aware selection path for editor review and export handoff.</p>\n")
	if _, err := os.Stat(filepath.Join(runDir, "goal_review_bundle.md")); err == nil {
		fmt.Fprintf(b, "<p>Review bundle: <code>%s</code></p>\n", Escape(filepath.Join(runDir, "goal_review_bundle.md")))
	}
}

func selectedClipSourceLabel(source exportartifacts.SelectedClipsSource) string {
	switch {
	case source.GoalRoughcutArtifact != "":
		return source.GoalRoughcutArtifact
	case source.EnhancedRoughcutArtifact != "":
		return source.EnhancedRoughcutArtifact
	case source.ClipCardsArtifact != "":
		return source.ClipCardsArtifact
	case source.RoughcutArtifact != "":
		return source.RoughcutArtifact
	default:
		return ""
	}
}

func writeExpansionOutputs(b *bytes.Buffer, runDir string) {
	total := 0
	rows := [][2]string{}
	for _, name := range []string{"caption_variants", "timeline_labels", "short_descriptions"} {
		var doc expansionOutputDoc
		if !readJSONIfExists(filepath.Join(runDir, "expansions", name+".json"), &doc) {
			continue
		}
		total += len(doc.Items)
		rows = append(rows, [2]string{name, fmt.Sprintf("%d items", len(doc.Items))})
	}
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "<h2>Expansion Outputs</h2>\n<p>Total items: %d</p>\n<table><tbody>\n", total)
	for _, row := range rows {
		fmt.Fprintf(b, "<tr><th>%s</th><td>%s</td></tr>\n", Escape(row[0]), Escape(row[1]))
	}
	b.WriteString("</tbody></table>\n")
}

func writeVerificationSummary(b *bytes.Buffer, path string) {
	var doc verificationResultsDoc
	if !readJSONIfExists(path, &doc) {
		return
	}
	writeKVTable(b, "Verification Summary", [][2]string{
		{"status", doc.Status},
		{"checks total", fmt.Sprintf("%d", doc.Summary.ChecksTotal)},
		{"checks passed", fmt.Sprintf("%d", doc.Summary.ChecksPassed)},
		{"checks failed", fmt.Sprintf("%d", doc.Summary.ChecksFailed)},
		{"warnings", fmt.Sprintf("%d", doc.Summary.Warnings)},
	})
}

func writeFFmpegScript(b *bytes.Buffer, path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	mode := detectFFmpegMode(path)
	b.WriteString("<h2>FFmpeg Script</h2>\n<p><code>ffmpeg_commands.sh</code> exists. It is not executed during <code>run</code>; use <code>byom-video export &lt;run_id&gt;</code>.</p>\n")
	if mode != "" {
		fmt.Fprintf(b, "<p>Mode: <strong>%s</strong></p>\n", Escape(mode))
	}
}

func writeConcatPlan(b *bytes.Buffer, runDir string) {
	if _, err := os.Stat(filepath.Join(runDir, "concat_list.txt")); err != nil {
		return
	}
	if _, err := os.Stat(filepath.Join(runDir, "ffmpeg_concat.sh")); err != nil {
		return
	}
	b.WriteString("<h2>Concat Plan</h2>\n<p><code>concat_list.txt</code> and <code>ffmpeg_concat.sh</code> exist. They are planning artifacts only and are not executed automatically.</p>\n")
}

func writeExports(b *bytes.Buffer, runDir string, m manifest.Manifest) {
	files, err := exporter.DiscoverExportedFiles(runDir)
	if err != nil || len(files) == 0 {
		return
	}
	b.WriteString("<h2>Exports</h2>\n")
	if m.ExportValidationStatus != "" {
		fmt.Fprintf(b, "<p>Export validation: <strong>%s</strong></p>\n", Escape(m.ExportValidationStatus))
	}
	validationByPath := map[string]exporter.ExportValidationFile{}
	if validation, err := exporter.ReadExportValidation(runDir); err == nil {
		for _, file := range validation.Files {
			validationByPath[file.Path] = file
		}
	}
	b.WriteString("<table><thead><tr><th>file</th><th>duration</th><th>video</th><th>audio</th><th>validation</th></tr></thead><tbody>\n")
	for _, filePath := range files {
		duration := "unknown"
		video := "-"
		audio := "-"
		status := "-"
		if validationFile, ok := validationByPath[filePath]; ok {
			if validationFile.DurationSeconds != nil {
				duration = fmt.Sprintf("%.3f seconds", *validationFile.DurationSeconds)
			}
			video = fmt.Sprintf("%d", validationFile.VideoStreams)
			audio = fmt.Sprintf("%d", validationFile.AudioStreams)
			status = validationFile.Status
		}
		fmt.Fprintf(b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			Escape(filePath), Escape(duration), Escape(video), Escape(audio), Escape(status))
	}
	b.WriteString("</tbody></table>\n")
}

func writeArtifacts(b *bytes.Buffer, m manifest.Manifest) {
	b.WriteString("<h2>Artifacts</h2>\n<ul>\n")
	for _, artifact := range m.Artifacts {
		fmt.Fprintf(b, "<li><code>%s</code> - %s</li>\n", Escape(artifact.Path), Escape(artifact.Name))
	}
	b.WriteString("</ul>\n")
}

func readJSONIfExists(path string, out any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, out) == nil
}

func preview(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return value[:limit-1] + "..."
}

func detectFFmpegMode(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# mode:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# mode:"))
		}
	}
	return ""
}

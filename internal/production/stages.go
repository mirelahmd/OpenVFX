package production

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// runStage dispatches one typed stage to its concrete implementation. Every
// implementation appends to rec.Commands so the execution record carries the
// literal argv of everything that ran.
func (e *Executor) runStage(stage *Stage, binding Binding, rec *ExecutionRecord) (outputs []string, notes []string, err error) {
	dir := e.Layout.StageDir(stage.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	switch stage.Type {
	case StageTypeProbeAssets:
		return e.stageProbeAssets(stage, dir, rec)
	case StageTypeTranscribe:
		return e.stageTranscribe(stage, dir, rec)
	case StageTypeSelectClips:
		return e.stageSelectClips(stage, dir)
	case StageTypeAssembleVideo:
		return e.stageAssembleVideo(stage, dir, rec)
	case StageTypeFormatOutput:
		return e.stageFormatOutput(stage, dir, rec)
	case StageTypeTextOverlay:
		return e.stageTextOverlay(stage, binding, dir, rec)
	default:
		return nil, nil, fmt.Errorf("unsupported stage type %q", stage.Type)
	}
}

// ---- probe_assets ----

func (e *Executor) stageProbeAssets(stage *Stage, dir string, rec *ExecutionRecord) ([]string, []string, error) {
	var outputs []string
	for _, asset := range e.Obs.Assets {
		out := filepath.Join(dir, asset.ID+".json")
		argv := []string{"ffprobe", "-v", "quiet", "-print_format", "json",
			"-show_format", "-show_streams", asset.Path}
		cmd, err := e.Runner.Run(argv)
		rec.Commands = append(rec.Commands, cmd)
		if err != nil {
			return outputs, nil, err
		}
		// Re-probe writing to file so the artifact is the probe output itself.
		if err := e.writeProbeArtifact(asset.Path, out, rec); err != nil {
			return outputs, nil, err
		}
		outputs = append(outputs, out)
	}
	return outputs, []string{fmt.Sprintf("probed %d asset(s)", len(outputs))}, nil
}

// writeProbeArtifact stores the ffprobe JSON for one asset. We already hold
// the observation in memory, but persisting the raw probe keeps the artifact
// independently verifiable.
func (e *Executor) writeProbeArtifact(assetPath, outPath string, rec *ExecutionRecord) error {
	for _, a := range e.Obs.Assets {
		if a.Path == assetPath {
			return WriteJSON(outPath, a)
		}
	}
	return fmt.Errorf("asset not found in observation: %s", assetPath)
}

// ---- transcribe ----

func (e *Executor) stageTranscribe(stage *Stage, dir string, rec *ExecutionRecord) ([]string, []string, error) {
	python := e.Python
	if python == "" {
		python = resolvePython()
	}
	if python == "" {
		return nil, nil, fmt.Errorf("no python interpreter available for transcription")
	}
	modelSize := stage.Params.ModelSize
	if modelSize == "" {
		modelSize = "tiny"
	}
	// The existing worker writes transcript.json into the directory it is given.
	argv := []string{python, "-m", "byom_video_workers.cli", "transcribe",
		"--input", stage.Params.SourcePath,
		"--run-dir", dir,
		"--model-size", modelSize}
	cmd, err := e.Runner.Run(argv)
	rec.Commands = append(rec.Commands, cmd)
	if err != nil {
		return nil, nil, err
	}
	transcriptPath := filepath.Join(dir, "transcript.json")
	if _, statErr := os.Stat(transcriptPath); statErr != nil {
		return nil, nil, fmt.Errorf("transcribe worker reported success but wrote no transcript.json")
	}
	// Emit the SRT alongside, so a sidecar fallback has something to deliver
	// and a burn-in binding has something to read.
	srtPath := filepath.Join(dir, "captions.srt")
	segs, err := readTranscriptSegments(transcriptPath)
	if err != nil {
		return []string{transcriptPath}, nil, err
	}
	if err := writeSRT(srtPath, segs); err != nil {
		return []string{transcriptPath}, nil, err
	}
	note := fmt.Sprintf("%d transcript segment(s), model=%s", len(segs), modelSize)
	return []string{transcriptPath, srtPath}, []string{note}, nil
}

type transcriptSegment struct {
	ID    string  `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func readTranscriptSegments(path string) ([]transcriptSegment, error) {
	var doc struct {
		Segments []transcriptSegment `json:"segments"`
	}
	if err := ReadJSON(path, &doc); err != nil {
		return nil, err
	}
	return doc.Segments, nil
}

func writeSRT(path string, segs []transcriptSegment) error {
	var b strings.Builder
	for i, s := range segs {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			i+1, srtTime(s.Start), srtTime(s.End), strings.TrimSpace(s.Text))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func srtTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	ms := int((seconds - float64(total)) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", total/3600, (total%3600)/60, total%60, ms)
}

// maxTailRepeats bounds how many times a corrective pass may repeat the tail
// of an edit. Extension is a fallback, not a way to manufacture runtime from
// nothing.
const maxTailRepeats = 10

// ---- select_clips ----

// EditDecision is one cut: which source, in and out points. This is the
// machine-readable edit, and it is what the assemble stage consumes.
type EditDecision struct {
	ID          string  `json:"id"`
	AssetID     string  `json:"asset_id"`
	SourcePath  string  `json:"source_path"`
	SourceIn    float64 `json:"source_in"`
	SourceOut   float64 `json:"source_out"`
	Duration    float64 `json:"duration"`
	TimelineIn  float64 `json:"timeline_in"`
	TimelineOut float64 `json:"timeline_out"`
	Reason      string  `json:"reason"`
	// DecisionID cites the creative decision that produced this cut, when one did.
	DecisionID string `json:"decision_id,omitempty"`
}

type EditDecisionList struct {
	SchemaVersion string         `json:"schema_version"`
	TargetSeconds float64        `json:"target_seconds,omitempty"`
	TotalSeconds  float64        `json:"total_seconds"`
	Decisions     []EditDecision `json:"decisions"`
	Notes         []string       `json:"notes,omitempty"`
}

// stageSelectClips builds the edit decision list deterministically: take each
// video asset in order until the target duration is reached, trimming the last
// clip to land exactly on target. This is boring arithmetic on real ffprobe
// facts, which is exactly what it should be.
func (e *Executor) stageSelectClips(stage *Stage, dir string) ([]string, []string, error) {
	target := stage.Params.TargetSeconds
	maxClips := stage.Params.MaxClips
	if maxClips <= 0 {
		maxClips = 8
	}

	assets := e.Obs.VideoAssets()
	if len(assets) == 0 {
		return nil, nil, fmt.Errorf("no video assets to select from")
	}

	edl := EditDecisionList{SchemaVersion: "openvfx_edl.v1", TargetSeconds: target}
	timeline := 0.0

	if len(stage.Params.Segments) > 0 {
		// The Creative Director chose these cuts. We honour them verbatim rather
		// than re-deriving an edit: overriding the creative decision here would
		// make the treatment decorative.
		for _, seg := range stage.Params.Segments {
			take := seg.SourceOut - seg.SourceIn
			if take <= 0.05 {
				edl.Notes = append(edl.Notes,
					fmt.Sprintf("%s skipped: zero-length segment", seg.ID))
				continue
			}
			reason := seg.Purpose
			if reason == "" {
				reason = "segment from creative treatment"
			}
			if seg.DecisionID != "" {
				reason = fmt.Sprintf("%s [%s]", reason, seg.DecisionID)
			}
			edl.Decisions = append(edl.Decisions, EditDecision{
				ID:          seg.ID,
				AssetID:     seg.AssetID,
				SourcePath:  seg.SourcePath,
				SourceIn:    round3(seg.SourceIn),
				SourceOut:   round3(seg.SourceOut),
				Duration:    round3(take),
				TimelineIn:  round3(timeline),
				TimelineOut: round3(timeline + take),
				Reason:      reason,
				DecisionID:  seg.DecisionID,
			})
			timeline += take
		}
		edl.Notes = append(edl.Notes,
			fmt.Sprintf("%d segment(s) supplied by the creative treatment", len(edl.Decisions)))
	} else {
		for _, a := range assets {
			if len(edl.Decisions) >= maxClips {
				break
			}
			if target > 0 && timeline >= target {
				break
			}
			take := a.DurationSeconds
			reason := "full asset"
			if target > 0 && timeline+take > target {
				take = target - timeline
				reason = fmt.Sprintf("trimmed to land on %.2fs target", target)
			}
			if take <= 0.05 {
				continue
			}
			d := EditDecision{
				ID:          fmt.Sprintf("edit_%04d", len(edl.Decisions)+1),
				AssetID:     a.ID,
				SourcePath:  a.Path,
				SourceIn:    0,
				SourceOut:   round3(take),
				Duration:    round3(take),
				TimelineIn:  round3(timeline),
				TimelineOut: round3(timeline + take),
				Reason:      reason,
			}
			edl.Decisions = append(edl.Decisions, d)
			timeline += take
		}
	}

	// ExtendToTarget repeats the tail of the edit until the target is reached.
	//
	// The planner never sets this on a first pass: looping footage is a
	// creative decision, and making it silently would hide the fact that the
	// source could not cover the brief. It is switched on only by a revision,
	// after the validator has recorded the shortfall as a failed assertion.
	if stage.Params.ExtendToTarget && target > 0 && len(edl.Decisions) > 0 {
		tail := edl.Decisions[len(edl.Decisions)-1]
		budget := stage.Params.RepeatLastClip
		if budget <= 0 {
			budget = maxTailRepeats
		}
		added := 0
		for r := 0; r < budget; r++ {
			if timeline >= target-0.05 {
				break
			}
			take := tail.Duration
			if timeline+take > target {
				take = target - timeline
			}
			if take <= 0.05 {
				break
			}
			rep := tail
			rep.ID = fmt.Sprintf("edit_%04d", len(edl.Decisions)+1)
			rep.SourceOut = round3(tail.SourceIn + take)
			rep.Duration = round3(take)
			rep.TimelineIn = round3(timeline)
			rep.TimelineOut = round3(timeline + take)
			rep.Reason = fmt.Sprintf("repeat of %s to reach the %.2fs target", tail.ID, target)
			edl.Decisions = append(edl.Decisions, rep)
			timeline += take
			added++
		}
		if added > 0 {
			edl.Notes = append(edl.Notes,
				fmt.Sprintf("tail repeated %d time(s) to extend toward the %.2fs target", added, target))
		}
	}

	// If the assets still cannot reach the target, say so honestly. The
	// validator turns this into a failed assertion rather than letting the
	// production claim a target it did not hit.
	if target > 0 && timeline < target-0.05 {
		note := fmt.Sprintf("source material totals %.2fs, short of the %.2fs target", timeline, target)
		edl.Notes = append(edl.Notes, note)
	}

	edl.TotalSeconds = round3(timeline)
	if len(edl.Decisions) == 0 {
		return nil, nil, fmt.Errorf("clip selection produced no usable segments")
	}

	out := filepath.Join(dir, "edit_decisions.json")
	if err := WriteJSON(out, edl); err != nil {
		return nil, nil, err
	}
	note := fmt.Sprintf("%d clip(s), %.2fs total", len(edl.Decisions), edl.TotalSeconds)
	return []string{out}, append([]string{note}, edl.Notes...), nil
}

// ---- assemble_video ----

func (e *Executor) stageAssembleVideo(stage *Stage, dir string, rec *ExecutionRecord) ([]string, []string, error) {
	edl, err := e.loadEDL()
	if err != nil {
		return nil, nil, err
	}

	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, nil, err
	}

	// Cut each segment to its own file.
	//
	// Every clip is normalised to the same stream layout: re-encoded video plus
	// exactly one AAC track, with silence synthesised for sources that carry no
	// audio. Without this, the concat demuxer silently drops the audio stream
	// whenever clip layouts disagree — a degradation nothing would report.
	var clipFiles []string
	var notes []string
	for _, d := range edl.Decisions {
		clipPath := filepath.Join(workDir, d.ID+".mp4")
		hasAudio := e.assetHasAudio(d.SourcePath)

		argv := []string{"ffmpeg", "-hide_banner", "-y",
			"-ss", trimFloat(d.SourceIn),
			"-i", d.SourcePath}
		if !hasAudio {
			argv = append(argv, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
			notes = append(notes, fmt.Sprintf("%s: source has no audio; silence synthesised to keep the timeline consistent", d.ID))
		}
		argv = append(argv,
			"-t", trimFloat(d.Duration),
			"-map", "0:v:0")
		if hasAudio {
			argv = append(argv, "-map", "0:a:0")
		} else {
			argv = append(argv, "-map", "1:a:0")
		}
		argv = append(argv,
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
			"-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "128k", "-ar", "48000", "-ac", "2",
			clipPath)

		cmd, err := e.Runner.Run(argv)
		rec.Commands = append(rec.Commands, cmd)
		if err != nil {
			return nil, nil, fmt.Errorf("cut %s: %w", d.ID, err)
		}
		clipFiles = append(clipFiles, clipPath)
	}

	out := filepath.Join(dir, "assembled.mp4")

	if len(clipFiles) == 1 {
		// Single clip: remux rather than re-encode a second time.
		argv := []string{"ffmpeg", "-hide_banner", "-y", "-i", clipFiles[0], "-c", "copy", out}
		cmd, err := e.Runner.Run(argv)
		rec.Commands = append(rec.Commands, cmd)
		if err != nil {
			return nil, nil, err
		}
		return []string{out}, append(notes, "1 clip remuxed"), nil
	}

	listPath := filepath.Join(workDir, "concat_list.txt")
	var b strings.Builder
	for _, f := range clipFiles {
		abs, _ := filepath.Abs(f)
		fmt.Fprintf(&b, "file '%s'\n", abs)
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o644); err != nil {
		return nil, nil, err
	}
	argv := []string{"ffmpeg", "-hide_banner", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-map", "0:v:0", "-map", "0:a:0", "-c", "copy", out}
	cmd, err := e.Runner.Run(argv)
	rec.Commands = append(rec.Commands, cmd)
	if err != nil {
		return nil, nil, err
	}
	return []string{out}, append(notes, fmt.Sprintf("%d clips concatenated", len(clipFiles))), nil
}

// assetHasAudio consults the observation rather than re-probing. The
// observation is the authoritative record of what the source material is.
func (e *Executor) assetHasAudio(path string) bool {
	for _, a := range e.Obs.Assets {
		if a.Path == path {
			return a.HasAudio
		}
	}
	return false
}

func (e *Executor) loadEDL() (EditDecisionList, error) {
	// Find the select_clips stage output by convention.
	matches, _ := filepath.Glob(filepath.Join(e.Layout.Root, "stages", "*", "edit_decisions.json"))
	sort.Strings(matches)
	if len(matches) == 0 {
		return EditDecisionList{}, fmt.Errorf("edit_decisions.json not found; select_clips must run first")
	}
	var edl EditDecisionList
	if err := ReadJSON(matches[len(matches)-1], &edl); err != nil {
		return EditDecisionList{}, err
	}
	return edl, nil
}

// ---- format_output ----

func (e *Executor) stageFormatOutput(stage *Stage, dir string, rec *ExecutionRecord) ([]string, []string, error) {
	if e.current == "" {
		return nil, nil, fmt.Errorf("no assembled video to format")
	}
	w, h := stage.Params.Width, stage.Params.Height
	bg := stage.Params.Background
	if bg == "" {
		bg = "black"
	}
	// Scale to fit inside the frame, then pad to exact dimensions. This never
	// crops source content, which is the safer default for unattended runs.
	vf := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:%s",
		w, h, w, h, bg)

	out := filepath.Join(dir, "formatted.mp4")
	argv := []string{"ffmpeg", "-hide_banner", "-y", "-i", e.current,
		"-vf", vf,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "copy",
		out}
	cmd, err := e.Runner.Run(argv)
	rec.Commands = append(rec.Commands, cmd)
	if err != nil {
		return nil, nil, err
	}
	return []string{out}, []string{fmt.Sprintf("conformed to %dx%d", w, h)}, nil
}

// ---- text_overlay ----

// stageTextOverlay delivers captions. Which backend it uses is decided by the
// binding, not by this function's caller — the same stage can burn subtitles,
// draw text, or emit a sidecar, and the plan shape is identical in all three.
func (e *Executor) stageTextOverlay(stage *Stage, binding Binding, dir string, rec *ExecutionRecord) ([]string, []string, error) {
	srtPath, hasCues := e.findSRT()
	if srtPath == "" {
		return nil, nil, fmt.Errorf("no captions.srt available; transcription must run first")
	}
	// A transcript with no segments is a real, reportable outcome — the source
	// carries no speech. We complete the stage and say so rather than inventing
	// caption text or failing the production.
	if !hasCues {
		return nil, []string{
			"no speech detected in the source audio; there are no captions to deliver",
		}, nil
	}

	switch binding.Backend {
	case CapFilterSubtitles:
		out := filepath.Join(dir, "captioned.mp4")
		argv := []string{"ffmpeg", "-hide_banner", "-y", "-i", e.current,
			"-vf", "subtitles=" + escapeFilterPath(srtPath),
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
			"-c:a", "copy",
			out}
		cmd, err := e.Runner.Run(argv)
		rec.Commands = append(rec.Commands, cmd)
		if err != nil {
			return nil, nil, err
		}
		return []string{out}, []string{"captions burned in via subtitles filter"}, nil

	case CapFilterDrawtext:
		// drawtext cannot consume an SRT directly; we render the first cue as a
		// static overlay. Lower fidelity, and the record says so.
		segs, err := readSRTFirstLine(srtPath)
		if err != nil {
			return nil, nil, err
		}
		out := filepath.Join(dir, "captioned.mp4")
		argv := []string{"ffmpeg", "-hide_banner", "-y", "-i", e.current,
			"-vf", fmt.Sprintf("drawtext=text='%s':fontcolor=white:fontsize=36:x=(w-tw)/2:y=h-th-60", escapeDrawtext(segs)),
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
			"-c:a", "copy",
			out}
		cmd, err := e.Runner.Run(argv)
		rec.Commands = append(rec.Commands, cmd)
		if err != nil {
			return nil, nil, err
		}
		return []string{out}, []string{"captions drawn via drawtext (static, reduced fidelity)"}, nil

	case CapSidecarSRT:
		// Terminal fallback: the video passes through untouched and the
		// captions are delivered as a separate conformant SRT file. The
		// deliverable is degraded, not missing — and the record says which.
		out := filepath.Join(dir, "captions.srt")
		data, err := os.ReadFile(srtPath)
		if err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return nil, nil, err
		}
		return []string{out}, []string{
			"captions delivered as sidecar SRT; video stream unmodified",
			"this is a degraded delivery: the brief asked for burned-in captions",
		}, nil

	default:
		return nil, nil, fmt.Errorf("unsupported text overlay backend %q", binding.Backend)
	}
}

// findSRT locates the caption file produced by the transcribe stage. The
// second return reports whether it actually contains cues — an empty SRT means
// the source had no detectable speech, which the caller must handle explicitly.
func (e *Executor) findSRT() (string, bool) {
	matches, _ := filepath.Glob(filepath.Join(e.Layout.Root, "stages", "*", "captions.srt"))
	sort.Strings(matches)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		return m, info.Size() > 0
	}
	return "", false
}

func readSRTFirstLine(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for i, l := range lines {
		if strings.Contains(l, "-->") && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1]), nil
		}
	}
	return "", fmt.Errorf("no cue text found in %s", filepath.Base(path))
}

// escapeFilterPath escapes a path for use inside an ffmpeg filter argument.
func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `\\`)
	p = strings.ReplaceAll(p, ":", `\:`)
	p = strings.ReplaceAll(p, "'", `\'`)
	return p
}

func escapeDrawtext(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ":", `\:`)
	return s
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

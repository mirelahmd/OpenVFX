package production

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BuildAssetObservations turns the raw ffprobe observation into the compact
// contract the creative director reasons over.
//
// Everything here is cheap and local: no computer vision, no model calls. A
// per-asset failure becomes a warning rather than aborting the session, because
// losing one asset should not cost the creator their whole production.
func BuildAssetObservations(productionID string, obs Observation, productionRoot string, now time.Time) AssetObservations {
	out := AssetObservations{
		SchemaVersion: AssetObservationsSchemaVersion,
		CreatedAt:     now,
		ProductionID:  productionID,
		InputPath:     obs.InputPath,
		Totals:        obs.Totals,
	}

	for _, a := range obs.Assets {
		observed := ObservedAsset{
			ID:              a.ID,
			Path:            a.Path,
			MediaType:       mediaTypeFor(a),
			DurationSeconds: a.DurationSeconds,
			Width:           a.Width,
			Height:          a.Height,
			AspectRatio:     aspectRatioLabel(a.Width, a.Height),
			FrameRate:       frameRateLabel(a.FrameRate),
			HasVideo:        a.HasVideo,
			HasAudio:        a.HasAudio,
			SizeBytes:       a.SizeBytes,
			Origin:          originFor(a),
		}

		// Transcripts are referenced, never embedded: this artifact has to stay
		// small enough to put in a prompt.
		if productionRoot != "" {
			ref, chars, err := findTranscriptFor(productionRoot, a)
			switch {
			case err != nil:
				out.Warnings = append(out.Warnings,
					fmt.Sprintf("%s: could not inspect transcript: %v", a.ID, err))
			case ref != "":
				observed.HasTranscript = true
				observed.TranscriptRef = ref
				observed.TranscriptChars = chars
			}
		}

		out.Assets = append(out.Assets, observed)
	}

	for _, s := range obs.Skipped {
		out.Warnings = append(out.Warnings,
			fmt.Sprintf("%s: not observed (%s)", filepath.Base(s.Path), s.Reason))
	}
	return out
}

func mediaTypeFor(a Asset) string {
	switch {
	case a.HasVideo && a.HasAudio:
		return "audiovisual"
	case a.HasVideo:
		return "video_only"
	case a.HasAudio:
		return "audio_only"
	default:
		return "unknown"
	}
}

// originFor reports where an asset came from when that is cheaply knowable.
// It returns "unknown" rather than guessing, because a fabricated provenance
// claim is worse than an absent one.
func originFor(a Asset) string {
	lower := strings.ToLower(filepath.Base(a.Path))
	dir := strings.ToLower(a.Path)
	switch {
	case strings.Contains(dir, "/visual_assets/"), strings.Contains(dir, "/generated/"):
		return OriginGenerated
	case strings.HasPrefix(lower, "generated_"):
		return OriginGenerated
	default:
		return OriginSource
	}
}

func aspectRatioLabel(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	g := gcd(w, h)
	rw, rh := w/g, h/g
	// Collapse near-standard ratios onto their common names so a model sees
	// "16:9" rather than "1918:1080".
	switch {
	case closeRatio(w, h, 16, 9):
		return "16:9"
	case closeRatio(w, h, 9, 16):
		return "9:16"
	case closeRatio(w, h, 1, 1):
		return "1:1"
	case closeRatio(w, h, 4, 5):
		return "4:5"
	case closeRatio(w, h, 4, 3):
		return "4:3"
	case closeRatio(w, h, 21, 9):
		return "21:9"
	}
	if rw > 999 || rh > 999 {
		return fmt.Sprintf("%.3f:1", float64(w)/float64(h))
	}
	return fmt.Sprintf("%d:%d", rw, rh)
}

func closeRatio(w, h, tw, th int) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	actual := float64(w) / float64(h)
	target := float64(tw) / float64(th)
	diff := actual - target
	if diff < 0 {
		diff = -diff
	}
	return diff <= target*0.02
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// frameRateLabel turns ffprobe's rational frame rate into a readable number.
func frameRateLabel(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return raw
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return raw
	}
	fps := num / den
	if fps == float64(int64(fps)) {
		return strconv.FormatInt(int64(fps), 10)
	}
	return strconv.FormatFloat(fps, 'f', 3, 64)
}

// findTranscriptFor looks for a transcript already produced for an asset. It
// returns a reference and a character count, never the transcript body.
func findTranscriptFor(productionRoot string, a Asset) (string, int, error) {
	// Transcripts are written by the transcribe stage, so they live under
	// stages/<stage_id>/transcript.json within the production.
	matches, err := filepath.Glob(filepath.Join(productionRoot, "stages", "*", "transcript.json"))
	if err != nil {
		return "", 0, err
	}
	for _, m := range matches {
		var doc struct {
			Source struct {
				InputPath string `json:"input_path"`
			} `json:"source"`
			Segments []struct {
				Text string `json:"text"`
			} `json:"segments"`
		}
		if err := ReadJSON(m, &doc); err != nil {
			continue
		}
		if doc.Source.InputPath != a.Path {
			continue
		}
		chars := 0
		for _, s := range doc.Segments {
			chars += len(s.Text)
		}
		return m, chars, nil
	}
	return "", 0, nil
}

func (l Layout) AssetObservationsPath() string {
	return filepath.Join(l.Root, "asset_observations.json")
}

func (l Layout) TreatmentPath() string {
	return filepath.Join(l.Root, "creative_treatment.json")
}

func (l Layout) DirectorConfigPath() string {
	return filepath.Join(l.Root, "director", "director_config.json")
}

func (l Layout) DirectorTracePath() string {
	return filepath.Join(l.Root, "director", "graph_trace.json")
}

// ReadTreatment loads a persisted treatment.
func ReadTreatment(l Layout) (CreativeTreatment, error) {
	var t CreativeTreatment
	if err := ReadJSON(l.TreatmentPath(), &t); err != nil {
		return CreativeTreatment{}, err
	}
	return t, nil
}

// HasTreatment reports whether a production recorded a creative treatment.
func HasTreatment(l Layout) bool {
	_, err := os.Stat(l.TreatmentPath())
	return err == nil
}

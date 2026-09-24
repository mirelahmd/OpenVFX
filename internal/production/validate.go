package production

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	AssertPass = "PASS"
	AssertFail = "FAIL"
	AssertSkip = "SKIP"
)

// Assertion is one checkable claim derived from the brief. Expected and Actual
// are both recorded so a reader can see the gap without rerunning anything.
type Assertion struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Evidence string `json:"evidence,omitempty"`
	// Degraded marks an assertion that passed only in a reduced form.
	Degraded bool `json:"degraded,omitempty"`
}

type ValidationResult struct {
	SchemaVersion string      `json:"schema_version"`
	ProductionID  string      `json:"production_id"`
	PlanVersion   int         `json:"plan_version"`
	ValidatedAt   time.Time   `json:"validated_at"`
	OutputPath    string      `json:"output_path"`
	Assertions    []Assertion `json:"assertions"`
	Passed        int         `json:"passed"`
	Failed        int         `json:"failed"`
	Status        string      `json:"status"`
}

func (v ValidationResult) OK() bool { return v.Failed == 0 }

// FirstFailure returns the first failed assertion, which is what the reviser
// reacts to.
func (v ValidationResult) FirstFailure() (Assertion, bool) {
	for _, a := range v.Assertions {
		if a.Status == AssertFail {
			return a, true
		}
	}
	return Assertion{}, false
}

// Validate checks the real output file against the intent that was parsed from
// the brief. It probes the artifact rather than trusting the execution records,
// because the question is whether the delivered file is correct, not whether
// the commands exited zero.
func Validate(plan ProductionPlan, outputPath string, probe ProbeFunc) ValidationResult {
	res := ValidationResult{
		SchemaVersion: ValidationSchemaVersion,
		ProductionID:  plan.ProductionID,
		PlanVersion:   plan.PlanVersion,
		ValidatedAt:   time.Now().UTC(),
		OutputPath:    outputPath,
	}
	add := func(a Assertion) { res.Assertions = append(res.Assertions, a) }

	// Existence is the precondition for every other assertion.
	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		add(Assertion{
			ID: "output_exists", Status: AssertFail,
			Expected: "a readable output file",
			Actual:   fmt.Sprintf("not found: %s", outputPath),
		})
		res.finish()
		return res
	}
	add(Assertion{
		ID: "output_exists", Status: AssertPass,
		Expected: "a readable output file",
		Actual:   fmt.Sprintf("%s (%d bytes)", filepath.Base(outputPath), info.Size()),
	})

	raw, probeErr := probe(outputPath)
	if probeErr != nil {
		add(Assertion{
			ID: "output_is_probeable", Status: AssertFail,
			Expected: "ffprobe can read the output",
			Actual:   probeErr.Error(),
		})
		res.finish()
		return res
	}
	asset, parseErr := parseProbe(outputPath, raw)
	if parseErr != nil {
		add(Assertion{
			ID: "output_is_probeable", Status: AssertFail,
			Expected: "ffprobe reports at least one stream",
			Actual:   parseErr.Error(),
		})
		res.finish()
		return res
	}
	evidence := fmt.Sprintf("ffprobe: %dx%d %s %.3fs",
		asset.Width, asset.Height, asset.VideoCodec, asset.DurationSeconds)
	add(Assertion{
		ID: "output_is_probeable", Status: AssertPass,
		Expected: "ffprobe can read the output",
		Actual:   "probed successfully", Evidence: evidence,
	})

	add(assertVideoStream(asset, evidence))

	intent := plan.Intent

	if intent.TargetDurationSeconds > 0 {
		add(assertDuration(asset, intent, evidence))
	}

	if intent.Width > 0 && intent.Height > 0 {
		add(assertDimensions(asset, intent, evidence))
	}

	if intent.WantsAudio {
		add(assertAudio(asset, evidence))
	}

	if intent.WantsCaptions {
		add(assertCaptions(plan, intent))
	}

	res.finish()
	return res
}

func (v *ValidationResult) finish() {
	for _, a := range v.Assertions {
		switch a.Status {
		case AssertPass:
			v.Passed++
		case AssertFail:
			v.Failed++
		}
	}
	switch {
	case v.Failed > 0:
		v.Status = "failed"
	case v.hasDegraded():
		v.Status = "passed_degraded"
	default:
		v.Status = "passed"
	}
}

func (v ValidationResult) hasDegraded() bool {
	for _, a := range v.Assertions {
		if a.Degraded {
			return true
		}
	}
	return false
}

func assertVideoStream(a Asset, evidence string) Assertion {
	if a.HasVideo {
		return Assertion{ID: "has_video_stream", Status: AssertPass,
			Expected: "at least one video stream",
			Actual:   fmt.Sprintf("video stream present (%s)", a.VideoCodec), Evidence: evidence}
	}
	return Assertion{ID: "has_video_stream", Status: AssertFail,
		Expected: "at least one video stream", Actual: "no video stream", Evidence: evidence}
}

func assertDuration(a Asset, intent Intent, evidence string) Assertion {
	tol := intent.DurationTolerance
	if tol <= 0 {
		tol = 0.10
	}
	allowed := intent.TargetDurationSeconds * tol
	delta := math.Abs(a.DurationSeconds - intent.TargetDurationSeconds)
	expected := fmt.Sprintf("%.2fs ±%.0f%%", intent.TargetDurationSeconds, tol*100)
	actual := fmt.Sprintf("%.3fs (off by %.3fs)", a.DurationSeconds, delta)
	if delta <= allowed {
		return Assertion{ID: "duration_within_tolerance", Status: AssertPass,
			Expected: expected, Actual: actual, Evidence: evidence}
	}
	return Assertion{ID: "duration_within_tolerance", Status: AssertFail,
		Expected: expected, Actual: actual, Evidence: evidence}
}

func assertDimensions(a Asset, intent Intent, evidence string) Assertion {
	expected := fmt.Sprintf("%dx%d (%s)", intent.Width, intent.Height, intent.AspectRatio)
	actual := fmt.Sprintf("%dx%d", a.Width, a.Height)
	id := "aspect_ratio_matches"
	if a.Width == intent.Width && a.Height == intent.Height {
		return Assertion{ID: id, Status: AssertPass, Expected: expected, Actual: actual, Evidence: evidence}
	}
	return Assertion{ID: id, Status: AssertFail, Expected: expected, Actual: actual, Evidence: evidence}
}

func assertAudio(a Asset, evidence string) Assertion {
	if a.HasAudio {
		return Assertion{ID: "has_audio_stream", Status: AssertPass,
			Expected: "at least one audio stream",
			Actual:   fmt.Sprintf("audio stream present (%s)", a.AudioCodec), Evidence: evidence}
	}
	return Assertion{ID: "has_audio_stream", Status: AssertFail,
		Expected: "at least one audio stream", Actual: "no audio stream", Evidence: evidence}
}

// assertCaptions is the assertion that carries the degraded-delivery concept.
// Captions delivered as a sidecar still satisfy "captions were delivered", but
// the result is explicitly marked degraded so no reader can mistake it for the
// burn-in that was asked for.
func assertCaptions(plan ProductionPlan, intent Intent) Assertion {
	var overlay *Stage
	for i := range plan.Stages {
		if plan.Stages[i].Type == StageTypeTextOverlay {
			overlay = &plan.Stages[i]
			break
		}
	}
	expected := "captions delivered"
	if intent.WantsBurnedCaptions {
		expected = "captions burned into the video stream"
	}
	if overlay == nil {
		return Assertion{ID: "captions_delivered", Status: AssertFail,
			Expected: expected, Actual: "no text overlay stage in the plan"}
	}
	if overlay.Status != StageStatusCompleted {
		return Assertion{ID: "captions_delivered", Status: AssertFail,
			Expected: expected,
			Actual:   fmt.Sprintf("text overlay stage status is %q", overlay.Status)}
	}
	// A completed stage that produced nothing delivered nothing. The most
	// common cause is a source with no detectable speech, which the stage
	// records as a note. Report it rather than claiming success.
	if len(overlay.Produced) == 0 {
		actual := "no captions produced"
		if len(overlay.Notes) > 0 {
			actual = overlay.Notes[0]
		}
		return Assertion{ID: "captions_delivered", Status: AssertFail,
			Expected: expected, Actual: actual, Evidence: "stage " + overlay.ID}
	}
	backend := ""
	if overlay.Binding != nil {
		backend = overlay.Binding.Backend
	}
	switch backend {
	case CapFilterSubtitles:
		return Assertion{ID: "captions_delivered", Status: AssertPass,
			Expected: expected, Actual: "burned in via subtitles filter",
			Evidence: "stage " + overlay.ID}
	case CapFilterDrawtext:
		return Assertion{ID: "captions_delivered", Status: AssertPass, Degraded: true,
			Expected: expected, Actual: "drawn via drawtext (static, reduced fidelity)",
			Evidence: "stage " + overlay.ID}
	case CapSidecarSRT:
		actual := "delivered as sidecar SRT, not burned in"
		return Assertion{ID: "captions_delivered", Status: AssertPass, Degraded: intent.WantsBurnedCaptions,
			Expected: expected, Actual: actual, Evidence: "stage " + overlay.ID}
	default:
		return Assertion{ID: "captions_delivered", Status: AssertFail,
			Expected: expected, Actual: "no binding recorded for the text overlay stage"}
	}
}

// Summary renders assertions for terminal output.
func (v ValidationResult) Summary() string {
	var b strings.Builder
	for _, a := range v.Assertions {
		mark := a.Status
		suffix := ""
		if a.Degraded {
			suffix = "  (degraded)"
		}
		fmt.Fprintf(&b, "    %-28s %-4s  %s%s\n", a.ID, mark, a.Actual, suffix)
	}
	return b.String()
}

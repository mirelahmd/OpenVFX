package production

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/events"
)

// ErrCapabilityUnavailable is returned when a stage cannot be bound to any
// backend. It is not a run failure: it is the signal the reviser reacts to.
var ErrCapabilityUnavailable = errors.New("required capability unavailable")

// ExecutionRecord is the provenance artifact. One per stage attempt.
//
// It exists so that the question "what actually ran?" has a file-on-disk
// answer, including the literal argv. Nothing in this struct is reconstructed
// after the fact — it is written by the code that made the call.
type ExecutionRecord struct {
	SchemaVersion string          `json:"schema_version"`
	ProductionID  string          `json:"production_id"`
	PlanVersion   int             `json:"plan_version"`
	StageID       string          `json:"stage_id"`
	StageType     string          `json:"stage_type"`
	Binding       Binding         `json:"binding"`
	Requirements  Requirements    `json:"requirements"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   time.Time       `json:"completed_at"`
	DurationMS    int64           `json:"duration_ms"`
	Status        string          `json:"status"`
	Commands      []CommandRecord `json:"commands"`
	Outputs       []string        `json:"outputs,omitempty"`
	Error         string          `json:"error,omitempty"`
	Notes         []string        `json:"notes,omitempty"`
}

// CommandRecord is one external process invocation, captured verbatim.
type CommandRecord struct {
	Argv        []string `json:"argv"`
	ExitCode    int      `json:"exit_code"`
	DurationMS  int64    `json:"duration_ms"`
	ToolVersion string   `json:"tool_version,omitempty"`
	StderrTail  string   `json:"stderr_tail,omitempty"`
}

// Runner executes external commands and returns what happened. The seam keeps
// the executor unit-testable without ffmpeg installed.
type Runner interface {
	Run(argv []string) (CommandRecord, error)
}

type execRunner struct {
	versions map[string]string
}

func NewExecRunner() Runner { return &execRunner{versions: map[string]string{}} }

func (r *execRunner) Run(argv []string) (CommandRecord, error) {
	if len(argv) == 0 {
		return CommandRecord{}, fmt.Errorf("empty argv")
	}
	rec := CommandRecord{Argv: argv, ToolVersion: r.toolVersion(argv[0])}
	start := time.Now()
	cmd := exec.Command(argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	rec.DurationMS = time.Since(start).Milliseconds()
	rec.ExitCode = cmd.ProcessState.ExitCode()
	if err != nil {
		rec.StderrTail = tailLines(string(out), 8, 800)
		return rec, fmt.Errorf("%s failed (exit %d): %s", filepath.Base(argv[0]), rec.ExitCode, rec.StderrTail)
	}
	return rec, nil
}

// toolVersion caches `<tool> -version` so provenance records carry the exact
// build that produced each artifact.
func (r *execRunner) toolVersion(tool string) string {
	if v, ok := r.versions[tool]; ok {
		return v
	}
	out, err := exec.Command(tool, "-version").Output()
	v := ""
	if err == nil {
		v = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	}
	r.versions[tool] = v
	return v
}

func tailLines(s string, maxLines, maxBytes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	out := strings.Join(lines, "\n")
	if len(out) > maxBytes {
		out = "..." + out[len(out)-maxBytes:]
	}
	return out
}

// ---- executor ----

// Executor walks a plan and runs it. It owns the binding decision, the
// execution records, and the event log. It does not decide WHAT to do — that
// came from the plan — and it does not decide what to do about failure, which
// is the reviser's job.
type Executor struct {
	Layout Layout
	Caps   CapabilitySet
	Obs    Observation
	Runner Runner
	Python string
	Log    *events.Log
	// Result carries the output of the most recently completed stage, so the
	// next stage knows which file to consume.
	current string
}

// ExecuteResult summarises one pass over a plan.
type ExecuteResult struct {
	Records      []ExecutionRecord
	FinalOutput  string
	BlockedStage string
	BlockedCap   string
	Err          error
}

// Execute runs every planned stage in order. Completed stages from a previous
// plan version are reused rather than re-run, which is what makes a revision
// cheap: only the substituted stage and anything downstream of it re-executes.
func (e *Executor) Execute(plan *ProductionPlan) ExecuteResult {
	res := ExecuteResult{}
	if err := plan.Validate(); err != nil {
		res.Err = err
		return res
	}
	if err := os.MkdirAll(e.Layout.OutputDir(), 0o755); err != nil {
		res.Err = err
		return res
	}

	for i := range plan.Stages {
		stage := &plan.Stages[i]

		if stage.Status == StageStatusCompleted {
			// Carried over from a previous plan version.
			e.event("STAGE_REUSED", map[string]any{"stage_id": stage.ID, "type": stage.Type})
			if out := e.stageOutput(stage); out != "" {
				e.current = out
			}
			continue
		}

		binding, err := e.bind(stage)
		if err != nil {
			stage.Status = StageStatusBlocked
			e.event("STAGE_BLOCKED", map[string]any{
				"stage_id": stage.ID, "type": stage.Type,
				"requires": stage.Requires, "reason": err.Error(),
			})
			res.BlockedStage = stage.ID
			res.BlockedCap = stage.Requires
			res.Err = fmt.Errorf("stage %s: %w: %s", stage.ID, ErrCapabilityUnavailable, stage.Requires)
			return res
		}
		stage.Binding = &binding

		e.event("STAGE_STARTED", map[string]any{
			"stage_id": stage.ID, "type": stage.Type, "backend": binding.Backend,
		})

		rec := ExecutionRecord{
			SchemaVersion: ExecutionSchemaVersion,
			ProductionID:  plan.ProductionID,
			PlanVersion:   plan.PlanVersion,
			StageID:       stage.ID,
			StageType:     stage.Type,
			Binding:       binding,
			Requirements:  stage.Requirements,
			StartedAt:     time.Now().UTC(),
		}

		outputs, notes, runErr := e.runStage(stage, binding, &rec)

		rec.CompletedAt = time.Now().UTC()
		rec.DurationMS = rec.CompletedAt.Sub(rec.StartedAt).Milliseconds()
		rec.Outputs = outputs
		rec.Notes = notes

		if runErr != nil {
			rec.Status = StageStatusFailed
			rec.Error = runErr.Error()
			stage.Status = StageStatusFailed
			_ = WriteJSON(e.Layout.ExecutionPath(stage.ID), rec)
			res.Records = append(res.Records, rec)
			e.event("STAGE_FAILED", map[string]any{
				"stage_id": stage.ID, "error": runErr.Error(),
			})
			res.Err = fmt.Errorf("stage %s (%s): %w", stage.ID, stage.Type, runErr)
			return res
		}

		rec.Status = StageStatusCompleted
		stage.Status = StageStatusCompleted
		stage.Degraded = binding.Fallback
		stage.Produced = outputs
		stage.Notes = notes
		if err := WriteJSON(e.Layout.ExecutionPath(stage.ID), rec); err != nil {
			res.Err = err
			return res
		}
		res.Records = append(res.Records, rec)
		e.event("STAGE_COMPLETED", map[string]any{
			"stage_id": stage.ID, "type": stage.Type,
			"backend": binding.Backend, "duration_ms": rec.DurationMS,
			"degraded": stage.Degraded,
		})

		if len(outputs) > 0 && isVideoFile(outputs[0]) {
			e.current = outputs[0]
		}
	}

	res.FinalOutput = e.current
	return res
}

// stageOutput recovers a reused stage's output path from its persisted record.
func (e *Executor) stageOutput(stage *Stage) string {
	var rec ExecutionRecord
	if err := ReadJSON(e.Layout.ExecutionPath(stage.ID), &rec); err != nil {
		return ""
	}
	for _, o := range rec.Outputs {
		if isVideoFile(o) {
			return o
		}
	}
	return ""
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".mkv", ".m4v":
		return true
	}
	return false
}

// ---- binding ----

// bind resolves a stage's required capability to a concrete backend.
//
// It binds to the declared capability or to nothing at all. The executor
// deliberately does NOT substitute a fallback on its own: choosing a different
// means to the same end is a planning decision, and planning decisions must be
// recorded as a revision with a reason. An executor that quietly degraded work
// would produce exactly the kind of unexplained output this architecture
// exists to prevent.
func (e *Executor) bind(stage *Stage) (Binding, error) {
	if e.Caps.IsAvailable(stage.Requires) {
		b := Binding{Backend: stage.Requires, BoundAt: time.Now().UTC()}
		// A stage whose Requires was rewritten by the reviser is still a
		// fallback binding; the plan records where it came from.
		if orig, ok := originalCapability(stage); ok && orig != stage.Requires {
			b.Fallback = true
			b.Reason = fmt.Sprintf("substituted for %s by revision", orig)
		}
		return b, nil
	}
	detail := "not probed"
	if c, ok := e.Caps.Get(stage.Requires); ok {
		detail = c.Detail
	}
	return Binding{}, fmt.Errorf("%s (%s)", stage.Requires, detail)
}

// originalCapability reports the capability a stage type was originally
// planned against, so a substituted binding can be labelled as a fallback.
func originalCapability(stage *Stage) (string, bool) {
	if stage.Type == StageTypeTextOverlay {
		return CapFilterSubtitles, true
	}
	return "", false
}

// bindingCandidates is the ordered preference list for a capability. The first
// entry is the capability itself; later entries are acceptable substitutes in
// descending order of fidelity.
//
// Note that caption burn-in degrades to a sidecar SRT: a different deliverable
// shape, not a silent drop. The stage still delivers captions.
func bindingCandidates(capability string) []string {
	switch capability {
	case CapFilterSubtitles:
		return []string{CapFilterSubtitles, CapFilterDrawtext, CapSidecarSRT}
	case CapFilterScale:
		return []string{CapFilterScale}
	default:
		return []string{capability}
	}
}

// ---- events ----

func (e *Executor) event(kind string, details map[string]any) {
	if e.Log == nil {
		return
	}
	_ = e.Log.Write(kind, details)
}

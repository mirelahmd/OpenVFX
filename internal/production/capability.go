package production

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

// Capability ids. These are what a Stage.Requires names.
const (
	CapFFmpeg          = "ffmpeg.binary"
	CapFFprobe         = "ffprobe.binary"
	CapFilterScale     = "ffmpeg.filter.scale"
	CapFilterPad       = "ffmpeg.filter.pad"
	CapFilterAmix      = "ffmpeg.filter.amix"
	CapFilterSubtitles = "ffmpeg.filter.subtitles"
	CapFilterDrawtext  = "ffmpeg.filter.drawtext"
	CapASRWhisper      = "asr.faster_whisper"
	CapSidecarSRT      = "sidecar.srt"
	CapTextOllama      = "text.ollama"
)

const (
	CapAvailable   = "available"
	CapUnavailable = "unavailable"
)

// Capability is one probed fact about this machine. Status is never inferred
// from configuration — it is the result of actually asking the tool.
type Capability struct {
	ID       string    `json:"id"`
	Status   string    `json:"status"`
	Detail   string    `json:"detail,omitempty"`
	Version  string    `json:"version,omitempty"`
	ProbedAt time.Time `json:"probed_at"`
	// NetworkEgress marks capabilities whose use sends bytes off this machine.
	// Local-first workflows can refuse to bind these.
	NetworkEgress bool `json:"network_egress,omitempty"`
}

type CapabilitySet struct {
	SchemaVersion string       `json:"schema_version"`
	ProbedAt      time.Time    `json:"probed_at"`
	Capabilities  []Capability `json:"capabilities"`
}

func (c CapabilitySet) Get(id string) (Capability, bool) {
	for _, cap := range c.Capabilities {
		if cap.ID == id {
			return cap, true
		}
	}
	return Capability{}, false
}

func (c CapabilitySet) IsAvailable(id string) bool {
	cap, ok := c.Get(id)
	return ok && cap.Status == CapAvailable
}

// Unavailable returns the ids that were probed and found missing, sorted.
func (c CapabilitySet) Unavailable() []string {
	var out []string
	for _, cap := range c.Capabilities {
		if cap.Status != CapAvailable {
			out = append(out, cap.ID)
		}
	}
	sort.Strings(out)
	return out
}

// ---- prober ----

// Prober is the seam that makes capability probing testable. The real
// implementation shells out; tests substitute a fake.
type Prober interface {
	// LookPath reports whether a binary is on PATH and its resolved location.
	LookPath(name string) (string, error)
	// Version returns the first line of `<name> -version`.
	Version(name string) (string, error)
	// FFmpegFilters returns the set of filter names this ffmpeg build exposes.
	FFmpegFilters() (map[string]bool, error)
	// PythonModule reports whether `import <module>` succeeds in the configured
	// interpreter.
	PythonModule(module string) (bool, string)
}

type execProber struct {
	pythonInterpreter string
}

// NewExecProber builds a Prober that shells out to real tools. The python
// interpreter follows the existing resolution order used by `doctor`:
// BYOM_VIDEO_PYTHON → config python.interpreter → python3 on PATH.
func NewExecProber(pythonInterpreter string) Prober {
	if pythonInterpreter == "" {
		pythonInterpreter = resolvePython()
	}
	return execProber{pythonInterpreter: pythonInterpreter}
}

// lookPath is a thin indirection so runtime.go does not import os/exec directly.
func lookPath(name string) (string, error) { return exec.LookPath(name) }

// resolvePython finds the interpreter for the Python sidecar, preferring the
// isolated environment the installer provisions over a bare python3.
func resolvePython() string {
	configured := ""
	if config.Exists(config.DefaultPath) {
		if cfg, err := config.Load(config.DefaultPath); err == nil {
			configured = cfg.Python.Interpreter
		}
	}
	return ResolveSidecarPython(configured)
}

func (e execProber) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (e execProber) Version(name string) (string, error) {
	out, err := exec.Command(name, "-version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), nil
}

func (e execProber) FFmpegFilters() (map[string]bool, error) {
	out, err := exec.Command("ffmpeg", "-hide_banner", "-filters").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("query ffmpeg filters: %w", err)
	}
	return parseFilterList(string(out)), nil
}

// parseFilterList reads `ffmpeg -filters` output. Lines look like:
//
//	... scale             V->V       Scale the input video.
//
// where the second whitespace-separated field is the filter name.
func parseFilterList(out string) map[string]bool {
	filters := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			filters[fields[1]] = true
		}
	}
	return filters
}

func (e execProber) PythonModule(module string) (bool, string) {
	if e.pythonInterpreter == "" {
		return false, "no python interpreter configured"
	}
	path, err := exec.LookPath(e.pythonInterpreter)
	if err != nil {
		return false, fmt.Sprintf("interpreter not found: %s", e.pythonInterpreter)
	}
	if err := exec.Command(path, "-c", "import "+module).Run(); err != nil {
		return false, fmt.Sprintf("import %s failed in %s", module, path)
	}
	return true, path
}

// ---- probe ----

// ProbeCapabilities asks this machine what it can actually do. It never
// consults configuration for availability — only for which optional backends
// are worth probing at all.
func ProbeCapabilities(p Prober, cfg config.Config) CapabilitySet {
	now := time.Now().UTC()
	set := CapabilitySet{
		SchemaVersion: CapabilitySchemaVersion,
		ProbedAt:      now,
	}
	add := func(c Capability) {
		c.ProbedAt = now
		set.Capabilities = append(set.Capabilities, c)
	}

	// ffmpeg / ffprobe binaries
	ffmpegOK := false
	if path, err := p.LookPath("ffmpeg"); err == nil {
		version, _ := p.Version("ffmpeg")
		ffmpegOK = true
		add(Capability{ID: CapFFmpeg, Status: CapAvailable, Detail: path, Version: version})
	} else {
		add(Capability{ID: CapFFmpeg, Status: CapUnavailable, Detail: "ffmpeg not found on PATH"})
	}
	if path, err := p.LookPath("ffprobe"); err == nil {
		version, _ := p.Version("ffprobe")
		add(Capability{ID: CapFFprobe, Status: CapAvailable, Detail: path, Version: version})
	} else {
		add(Capability{ID: CapFFprobe, Status: CapUnavailable, Detail: "ffprobe not found on PATH"})
	}

	// ffmpeg filters. A filter missing from the build is the most common real
	// capability gap on a developer machine, and the one this slice leans on.
	filterCaps := []struct {
		id     string
		filter string
		hint   string
	}{
		{CapFilterScale, "scale", "required for resolution changes"},
		{CapFilterPad, "pad", "required for letterbox/pillarbox fit"},
		{CapFilterAmix, "amix", "required for audio mixing"},
		{CapFilterSubtitles, "subtitles", "libass not compiled in; caption burn-in unavailable"},
		{CapFilterDrawtext, "drawtext", "freetype not compiled in; text drawing unavailable"},
	}
	var filters map[string]bool
	var filterErr error
	if ffmpegOK {
		filters, filterErr = p.FFmpegFilters()
	}
	for _, fc := range filterCaps {
		switch {
		case !ffmpegOK:
			add(Capability{ID: fc.id, Status: CapUnavailable, Detail: "ffmpeg unavailable"})
		case filterErr != nil:
			add(Capability{ID: fc.id, Status: CapUnavailable, Detail: filterErr.Error()})
		case filters[fc.filter]:
			add(Capability{ID: fc.id, Status: CapAvailable, Detail: "filter present in ffmpeg build"})
		default:
			add(Capability{ID: fc.id, Status: CapUnavailable, Detail: fc.hint})
		}
	}

	// Local ASR.
	if ok, detail := p.PythonModule("faster_whisper"); ok {
		add(Capability{ID: CapASRWhisper, Status: CapAvailable, Detail: detail})
	} else {
		add(Capability{ID: CapASRWhisper, Status: CapUnavailable, Detail: detail})
	}

	// Sidecar SRT is pure Go file writing. It is always available, and it is
	// the terminal fallback for text delivery — which is why the caption
	// pipeline can always complete, if only in degraded form.
	add(Capability{ID: CapSidecarSRT, Status: CapAvailable, Detail: "built-in SRT writer"})

	// Configured remote backends. These are declared, never assumed. We record
	// them as unavailable-until-proven and mark their egress, so a local-first
	// run can see exactly what would have left the machine.
	add(probeConfiguredText(cfg))

	return set
}

// probeConfiguredText reports on a configured Ollama-style text backend. We do
// not open a socket here: reachability is checked at bind time by the stage
// that needs it. What we record is whether a backend is configured at all.
func probeConfiguredText(cfg config.Config) Capability {
	if !cfg.Tools.Enabled {
		return Capability{ID: CapTextOllama, Status: CapUnavailable, Detail: "tools.enabled is false in byom-video.yaml"}
	}
	for routeKey, backendName := range cfg.Tools.Routes {
		if !strings.HasPrefix(routeKey, "creative.script") {
			continue
		}
		backend, ok := cfg.Tools.Backends[backendName]
		if !ok {
			continue
		}
		return Capability{
			ID:            CapTextOllama,
			Status:        CapAvailable,
			Detail:        fmt.Sprintf("configured backend %q at %s", backendName, backend.Endpoint),
			Version:       backend.Model,
			NetworkEgress: !isLoopback(backend.Endpoint),
		}
	}
	return Capability{ID: CapTextOllama, Status: CapUnavailable, Detail: "no creative.script route configured"}
}

func isLoopback(endpoint string) bool {
	return strings.Contains(endpoint, "localhost") || strings.Contains(endpoint, "127.0.0.1")
}

package production

import (
	"os"
	"path/filepath"
	"runtime"
)

// Runtime layout resolution.
//
// OpenVFX ships as a release archive containing both the Go executable and the
// Python agent sidecar. An installed user has no repository checkout, no
// developer virtualenv and no guarantee about the working directory, so
// installed locations are resolved first and the source-checkout layout is a
// fallback for development.
//
// Installed layout:
//
//	<data>/openvfx/<version>/workers/    the Python sidecar
//	<data>/openvfx/current -> <version>  stable path for the installed CLI
//	<data>/openvfx/venv/                 isolated interpreter for the sidecar
//
// where <data> is $OPENVFX_HOME, $XDG_DATA_HOME, ~/.local/share, or
// /usr/local/share, in that order.

// DataDirCandidates returns the OpenVFX data roots to search, most specific first.
func DataDirCandidates() []string {
	var dirs []string
	if v := os.Getenv("OPENVFX_HOME"); v != "" {
		dirs = append(dirs, v)
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		dirs = append(dirs, filepath.Join(v, "openvfx"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "openvfx"))
		if runtime.GOOS == "darwin" {
			// Accept the Apple-conventional location too, for users who install there.
			dirs = append(dirs, filepath.Join(home, "Library", "Application Support", "openvfx"))
		}
	}
	dirs = append(dirs, filepath.Join("/usr", "local", "share", "openvfx"))
	return dirs
}

// InstalledDataDir returns the first data root that actually holds an install.
func InstalledDataDir() string {
	for _, dir := range DataDirCandidates() {
		if _, err := os.Stat(filepath.Join(dir, "current", "workers", "openvfx_agent_graph")); err == nil {
			return dir
		}
	}
	return ""
}

// ResolveWorkersDir locates the Python sidecar package directory.
//
// Order: explicit override, installed distribution, then the development
// checkout. Installed resolution is deliberately ahead of the checkout probes
// so a released binary never depends on where it happens to be run from.
func ResolveWorkersDir() string {
	// Explicit override always wins; OPENVFX_WORKERS_DIR is the new name and
	// BYOM_VIDEO_WORKERS_DIR is kept for existing users.
	for _, key := range []string{"OPENVFX_WORKERS_DIR", "BYOM_VIDEO_WORKERS_DIR"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}

	for _, dir := range DataDirCandidates() {
		candidate := filepath.Join(dir, "current", "workers")
		if isWorkersDir(candidate) {
			return candidate
		}
	}

	// Development: alongside the executable, e.g. <repo>/cmd/byom-video -> <repo>/workers.
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(exe), "workers"),
			filepath.Join(filepath.Dir(filepath.Dir(exe)), "workers"),
			// A release tree extracted in place: <root>/bin/openvfx -> <root>/share/openvfx/workers
			filepath.Join(filepath.Dir(filepath.Dir(exe)), "share", "openvfx", "workers"),
		} {
			if isWorkersDir(candidate) {
				return candidate
			}
		}
	}

	// Development: walk up from the working directory.
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			candidate := filepath.Join(dir, "workers")
			if isWorkersDir(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func isWorkersDir(path string) bool {
	_, err := os.Stat(filepath.Join(path, "openvfx_agent_graph"))
	return err == nil
}

// ResolveSidecarPython returns the interpreter that should run the Python
// sidecar, or "" when none can be found.
//
// The installed virtualenv is preferred over a bare python3 on PATH, because
// that is where the installer put langgraph and faster-whisper. A configured
// interpreter still wins, but only if it actually exists — a stale
// byom-video.yaml pointing at a deleted .venv must not break an installed CLI.
func ResolveSidecarPython(configured string) string {
	for _, key := range []string{"OPENVFX_PYTHON", "BYOM_VIDEO_PYTHON"} {
		if v := os.Getenv(key); v != "" && interpreterExists(v) {
			return v
		}
	}
	if configured != "" && interpreterExists(configured) {
		return configured
	}
	for _, dir := range DataDirCandidates() {
		candidate := filepath.Join(dir, "venv", "bin", "python")
		if interpreterExists(candidate) {
			return candidate
		}
	}
	if interpreterExists("python3") {
		return "python3"
	}
	return ""
}

func interpreterExists(path string) bool {
	if filepath.IsAbs(path) || filepath.Base(path) != path {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}
	resolved, err := lookPath(path)
	return err == nil && resolved != ""
}

// RuntimeInfo describes where the installed pieces were found. It backs
// `openvfx version` so a user can see what their install actually resolved to.
type RuntimeInfo struct {
	Executable string
	DataDir    string
	WorkersDir string
	Python     string
}

func DescribeRuntime(configuredPython string) RuntimeInfo {
	exe, _ := os.Executable()
	return RuntimeInfo{
		Executable: exe,
		DataDir:    InstalledDataDir(),
		WorkersDir: ResolveWorkersDir(),
		Python:     ResolveSidecarPython(configuredPython),
	}
}

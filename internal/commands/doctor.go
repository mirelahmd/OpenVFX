package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mirelahmd/byom-video/internal/config"
	"github.com/mirelahmd/byom-video/internal/media"
)

type DoctorOptions struct {
	Transcription bool
}

func Doctor(stdout io.Writer, opts DoctorOptions) error {
	fmt.Fprintln(stdout, "byom-video doctor")
	fmt.Fprintf(stdout, "OK      go runtime: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	ffprobePath, ffprobeErr := media.FindExecutable("ffprobe")
	printToolStatus(stdout, "ffprobe", ffprobePath, ffprobeErr)

	ffmpegPath, ffmpegErr := media.FindExecutable("ffmpeg")
	printToolStatus(stdout, "ffmpeg", ffmpegPath, ffmpegErr)

	pythonPath, pythonErr := exec.LookPath("python3")
	printPythonStatus(stdout, pythonPath, pythonErr)

	if config.Exists(config.DefaultPath) {
		fmt.Fprintf(stdout, "OK      config: %s\n", config.DefaultPath)
	} else {
		fmt.Fprintf(stdout, "MISSING config: %s not found; run `byom-video init` to create one\n", config.DefaultPath)
	}

	// resolution order: BYOM_VIDEO_PYTHON → config → python3
	effectivePython, source := resolvePythonWithSource()
	if effectivePython != "" {
		path, err := exec.LookPath(effectivePython)
		printConfiguredPythonStatus(stdout, effectivePython, source, path, err)

		if err == nil {
			workersOK := checkPythonImport(path, "byom_video_workers")
			if workersOK {
				fmt.Fprintln(stdout, "OK      byom_video_workers: importable")
			} else if opts.Transcription {
				fmt.Fprintln(stdout, "MISSING byom_video_workers: not importable — run install.sh or: pip install -e workers/")
			} else {
				fmt.Fprintln(stdout, "OPTIONAL byom_video_workers: not installed (needed for transcription)")
			}

			whisperOK := checkPythonImport(path, "faster_whisper")
			if whisperOK {
				fmt.Fprintln(stdout, "OK      faster_whisper: importable")
			} else if opts.Transcription {
				fmt.Fprintln(stdout, "MISSING faster_whisper: not importable — pip install faster-whisper")
			} else {
				fmt.Fprintln(stdout, "OPTIONAL faster_whisper: not installed (needed for transcription)")
			}
		}
	} else {
		fmt.Fprintln(stdout, "MISSING configured python: set BYOM_VIDEO_PYTHON or configure python.interpreter in byom-video.yaml")
	}

	fmt.Fprintln(stdout)
	if opts.Transcription {
		fmt.Fprintln(stdout, "Transcription check active: MISSING items above require attention.")
	}
	fmt.Fprintln(stdout, "Install hint:")
	fmt.Fprintln(stdout, "  macOS:      brew install ffmpeg")
	fmt.Fprintln(stdout, "  Ubuntu:     sudo apt-get install ffmpeg")
	fmt.Fprintln(stdout, "  Windows:    winget install Gyan.FFmpeg")
	fmt.Fprintln(stdout, "  Python env: python3 -m venv ~/.byom-venv && ~/.byom-venv/bin/pip install -e workers[transcribe]")
	fmt.Fprintln(stdout, "  Set python: export BYOM_VIDEO_PYTHON=~/.byom-venv/bin/python")
	fmt.Fprintln(stdout, "  Note: transcription dependencies are optional for metadata-only runs.")

	return nil
}

// resolvePythonWithSource returns the effective python interpreter and its source label.
// Resolution order: BYOM_VIDEO_PYTHON → config python.interpreter → python3 on PATH.
func resolvePythonWithSource() (string, string) {
	if v := os.Getenv("BYOM_VIDEO_PYTHON"); v != "" {
		return v, "BYOM_VIDEO_PYTHON"
	}
	if config.Exists(config.DefaultPath) {
		if cfg, err := config.Load(config.DefaultPath); err == nil && cfg.Python.Interpreter != "" {
			return cfg.Python.Interpreter, "config"
		}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p, "PATH"
	}
	return "", ""
}

func checkPythonImport(pythonPath, module string) bool {
	return exec.Command(pythonPath, "-c", "import "+module).Run() == nil
}

func printConfiguredPythonStatus(stdout io.Writer, interpreter, source, path string, err error) {
	if err != nil {
		fmt.Fprintf(stdout, "MISSING configured python [%s]: %s not found\n", source, interpreter)
		return
	}
	versionOutput, versionErr := exec.Command(path, "--version").CombinedOutput()
	if versionErr != nil {
		fmt.Fprintf(stdout, "OK      configured python [%s]: %s\n", source, path)
		return
	}
	fmt.Fprintf(stdout, "OK      configured python [%s]: %s\n", source, path)
	fmt.Fprintf(stdout, "        version: %s\n", strings.TrimSpace(string(versionOutput)))
}

func printToolStatus(stdout io.Writer, name string, path string, err error) {
	if err != nil {
		fmt.Fprintf(stdout, "MISSING %s: not found on PATH\n", name)
		return
	}
	version, versionErr := media.ToolVersion(name)
	if versionErr != nil {
		fmt.Fprintf(stdout, "OK      %s: %s\n", name, path)
		return
	}
	fmt.Fprintf(stdout, "OK      %s: %s\n", name, path)
	fmt.Fprintf(stdout, "        %s version: %s\n", name, version)
}

func printPythonStatus(stdout io.Writer, path string, err error) {
	if err != nil {
		fmt.Fprintln(stdout, "MISSING python3: not found on PATH")
		return
	}
	versionOutput, versionErr := exec.Command(path, "--version").CombinedOutput()
	if versionErr != nil {
		fmt.Fprintf(stdout, "OK      python3: %s\n", path)
		return
	}
	fmt.Fprintf(stdout, "OK      python3: %s\n", path)
	fmt.Fprintf(stdout, "        python3 version: %s\n", strings.TrimSpace(string(versionOutput)))
}

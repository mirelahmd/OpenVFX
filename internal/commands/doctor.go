package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/media"
)

func Doctor(stdout io.Writer) error {
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
		interpreter := os.Getenv("BYOM_VIDEO_PYTHON")
		if interpreter == "" {
			if cfg, err := config.Load(config.DefaultPath); err == nil {
				interpreter = cfg.Python.Interpreter
			}
		}
		if interpreter != "" {
			path, err := exec.LookPath(interpreter)
			printConfiguredPythonStatus(stdout, interpreter, path, err)
		}
	} else {
		fmt.Fprintf(stdout, "MISSING config: %s not found; run `byom-video init` to create one\n", config.DefaultPath)
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Install hint:")
	fmt.Fprintln(stdout, "  macOS:   brew install ffmpeg")
	fmt.Fprintln(stdout, "  Ubuntu:  sudo apt-get install ffmpeg")
	fmt.Fprintln(stdout, "  Windows: winget install Gyan.FFmpeg")
	fmt.Fprintln(stdout, "  Python:  install python3 or set BYOM_VIDEO_PYTHON")
	fmt.Fprintln(stdout, "  Transcription: python3 -m pip install -e \"workers[transcribe]\"")
	fmt.Fprintln(stdout, "  Note: real transcription dependencies are optional and not required for metadata-only runs.")

	return nil
}

func printConfiguredPythonStatus(stdout io.Writer, configured string, path string, err error) {
	if err != nil {
		fmt.Fprintf(stdout, "MISSING configured python: %s not found\n", configured)
		return
	}
	versionOutput, versionErr := exec.Command(path, "--version").CombinedOutput()
	if versionErr != nil {
		fmt.Fprintf(stdout, "OK      configured python: %s\n", path)
		return
	}
	fmt.Fprintf(stdout, "OK      configured python: %s\n", path)
	fmt.Fprintf(stdout, "        configured python version: %s\n", strings.TrimSpace(string(versionOutput)))
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

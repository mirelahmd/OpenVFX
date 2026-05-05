package workers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RunTranscribeStub(inputPath string, runDir string) error {
	return RunTranscribeStubWithPython("", inputPath, runDir)
}

func RunTranscribeStubWithPython(python string, inputPath string, runDir string) error {
	return runWorker(python, "transcribe stub", []string{
		"-m",
		"byom_video_workers.cli",
		"transcribe-stub",
		"--input",
		inputPath,
		"--run-dir",
		runDir,
	})
}

func RunTranscribe(inputPath string, runDir string, modelSize string) error {
	return RunTranscribeWithPython("", inputPath, runDir, modelSize)
}

func RunTranscribeWithPython(python string, inputPath string, runDir string, modelSize string) error {
	return runWorker(python, "transcribe", []string{
		"-m",
		"byom_video_workers.cli",
		"transcribe",
		"--input",
		inputPath,
		"--run-dir",
		runDir,
		"--model-size",
		modelSize,
	})
}

func runWorker(python string, label string, args []string) error {
	if envPython := os.Getenv("BYOM_VIDEO_PYTHON"); envPython != "" {
		python = envPython
	}
	if python == "" {
		python = "python3"
	}
	if _, err := exec.LookPath(python); err != nil {
		return fmt.Errorf("python worker runtime missing: %s not found; set BYOM_VIDEO_PYTHON or install python3", python)
	}

	cmd := exec.Command(python, args...)
	cmd.Env = append(os.Environ(), "PYTHONPATH=workers")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s worker failed: %s", label, message)
	}
	return nil
}

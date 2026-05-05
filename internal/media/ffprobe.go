package media

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrFFprobeMissing = errors.New("ffprobe is missing")

func FindExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return path, nil
}

func Probe(inputPath string) ([]byte, error) {
	if _, err := FindExecutable("ffprobe"); err != nil {
		return nil, ErrFFprobeMissing
	}

	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", inputPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run ffprobe: %w", err)
	}
	return out, nil
}

func ToolVersion(name string) (string, error) {
	if _, err := FindExecutable(name); err != nil {
		return "", err
	}
	out, err := exec.Command(name, "-version").Output()
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(line), nil
}

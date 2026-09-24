package commands

import (
	"fmt"
	"io"

	"github.com/mirelahmd/OpenVFX/internal/config"
	"github.com/mirelahmd/OpenVFX/internal/production"
)

// Build metadata, injected at link time by scripts/build-release.sh and the
// Makefile. Git is never consulted at runtime.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionCommand(stdout io.Writer) error {
	fmt.Fprintln(stdout, "OpenVFX")
	fmt.Fprintf(stdout, "  version:    %s\n", Version)
	fmt.Fprintf(stdout, "  commit:     %s\n", Commit)
	fmt.Fprintf(stdout, "  build date: %s\n", BuildDate)

	// Show where this executable actually resolved its runtime, so an installed
	// user can tell a working install from a half-finished one without guessing.
	configured := ""
	if config.Exists(config.DefaultPath) {
		if cfg, err := config.Load(config.DefaultPath); err == nil {
			configured = cfg.Python.Interpreter
		}
	}
	info := production.DescribeRuntime(configured)

	fmt.Fprintln(stdout, "  runtime:")
	fmt.Fprintf(stdout, "    executable:       %s\n", orNone(info.Executable))
	fmt.Fprintf(stdout, "    data dir:         %s\n", orNone(info.DataDir))
	fmt.Fprintf(stdout, "    agent sidecar:    %s\n", orNone(info.WorkersDir))
	fmt.Fprintf(stdout, "    python:           %s\n", orNone(info.Python))
	return nil
}

func orNone(value string) string {
	if value == "" {
		return "not found"
	}
	return value
}

package commands

import (
	"fmt"
	"io"
)

var (
	Version   = "v0.1.0-alpha"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func VersionCommand(stdout io.Writer) error {
	fmt.Fprintln(stdout, "BYOM Video")
	fmt.Fprintf(stdout, "  version:    %s\n", Version)
	fmt.Fprintf(stdout, "  commit:     %s\n", Commit)
	fmt.Fprintf(stdout, "  build date: %s\n", BuildDate)
	return nil
}

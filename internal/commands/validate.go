package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mirelahmd/byom-video/internal/runvalidate"
)

type ValidateOptions struct {
	JSON bool
}

type ValidationFailedError struct {
	Count int
}

func (e ValidationFailedError) Error() string {
	return fmt.Sprintf("validation failed with %d error(s)", e.Count)
}

func Validate(runID string, stdout io.Writer, opts ValidateOptions) error {
	result, err := runvalidate.Validate(runID)
	if err != nil {
		return err
	}
	if opts.JSON {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("encode validation result: %w", err)
		}
		fmt.Fprintln(stdout, string(encoded))
	} else {
		fmt.Fprintln(stdout, "Run validation")
		fmt.Fprintf(stdout, "  run id:          %s\n", result.RunID)
		fmt.Fprintf(stdout, "  manifest status: %s\n", statusText(result.ManifestOK))
		fmt.Fprintf(stdout, "  events status:   %s\n", statusText(result.EventsOK))
		fmt.Fprintf(stdout, "  checks passed:   %d\n", len(result.ChecksPassed))
		for _, check := range result.ChecksPassed {
			fmt.Fprintf(stdout, "    - %s\n", check)
		}
		if len(result.Warnings) > 0 {
			fmt.Fprintln(stdout, "  warnings:")
			for _, warning := range result.Warnings {
				fmt.Fprintf(stdout, "    - %s\n", warning)
			}
		}
		if len(result.Errors) > 0 {
			fmt.Fprintln(stdout, "  errors:")
			for _, validationError := range result.Errors {
				fmt.Fprintf(stdout, "    - %s\n", validationError)
			}
		}
	}
	if result.HasErrors() {
		return ValidationFailedError{Count: len(result.Errors)}
	}
	return nil
}

func statusText(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}

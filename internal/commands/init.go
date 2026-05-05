package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/mirelahmd/OpenVFX/internal/config"
)

func Init(stdout io.Writer, force bool) error {
	created := []string{}
	skipped := []string{}

	if _, err := os.Stat(config.DefaultPath); err == nil && !force {
		skipped = append(skipped, config.DefaultPath)
	} else {
		if err := os.WriteFile(config.DefaultPath, []byte(config.DefaultContent()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", config.DefaultPath, err)
		}
		created = append(created, config.DefaultPath)
	}

	for _, dir := range []string{"media", "exports", ".byom-video"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			skipped = append(skipped, dir+"/")
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		created = append(created, dir+"/")
	}

	fmt.Fprintln(stdout, "Project initialized")
	for _, path := range created {
		fmt.Fprintf(stdout, "  created: %s\n", path)
	}
	for _, path := range skipped {
		fmt.Fprintf(stdout, "  skipped: %s\n", path)
	}
	return nil
}

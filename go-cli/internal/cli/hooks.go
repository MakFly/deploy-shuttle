package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/output"
)

func runLocalHooks(phase string, hooks []string, dryRun bool) error {
	if len(hooks) == 0 {
		return nil
	}
	fmt.Println()
	output.Step("Running %s hooks (%d)...", phase, len(hooks))
	for _, h := range hooks {
		if dryRun {
			output.Detail("[dry-run] %s", h)
			continue
		}
		output.Cmd("%s", h)
		cmd := exec.Command("sh", "-c", h)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s hook failed: %s: %w", phase, h, err)
		}
	}
	return nil
}

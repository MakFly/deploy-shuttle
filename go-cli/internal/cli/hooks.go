package cli

import (
	"fmt"
	"os"
	"os/exec"
)

func runLocalHooks(phase string, hooks []string, dryRun bool) error {
	if len(hooks) == 0 {
		return nil
	}
	fmt.Printf("-> Running %s hooks (%d)...\n", phase, len(hooks))
	for _, h := range hooks {
		if dryRun {
			fmt.Printf("  [dry-run] %s\n", h)
			continue
		}
		fmt.Printf("  $ %s\n", h)
		cmd := exec.Command("sh", "-c", h)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s hook failed: %s: %w", phase, h, err)
		}
	}
	return nil
}

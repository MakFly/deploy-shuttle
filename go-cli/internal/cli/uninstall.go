package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newUninstallCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove deploy-shuttle from this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("cannot locate binary: %w", err)
			}
			exe, _ = filepath.EvalSymlinks(exe)

			fmt.Println("This will remove:")
			fmt.Printf("  • %s\n", exe)

			home, _ := os.UserHomeDir()
			var extras []string
			if home != "" {
				candidates := []string{
					filepath.Join(home, ".shuttle"),
					filepath.Join(home, ".deployshuttle"),
				}
				for _, c := range candidates {
					if info, err := os.Stat(c); err == nil && info.IsDir() {
						extras = append(extras, c)
						fmt.Printf("  • %s/\n", c)
					}
				}
			}

			if !yes {
				fmt.Print("\nContinue? [y/N] ")
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			for _, dir := range extras {
				if err := os.RemoveAll(dir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", dir, err)
				} else {
					fmt.Printf("✓ Removed %s\n", dir)
				}
			}

			if err := os.Remove(exe); err != nil {
				return fmt.Errorf("failed to remove binary: %w", err)
			}
			fmt.Printf("✓ Removed %s\n", exe)
			fmt.Println("\ndeploy-shuttle has been uninstalled.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

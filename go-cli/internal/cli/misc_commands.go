package cli

import (
	"os"
	"path/filepath"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/templates"
	"github.com/spf13/cobra"
)

func newCICommand() *cobra.Command {
	root := &cobra.Command{Use: "ci", Short: "Generate CI/CD pipeline configuration"}
	root.AddCommand(&cobra.Command{
		Use:   "generate",
		Short: "Generate a GitHub Actions workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ".github/workflows"
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "shuttle.yml"), []byte(templates.CIWorkflow("")), 0o644)
		},
	})
	return root
}

package cli

import (
	"os"
	"path/filepath"

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
			content := `name: DeployShuttle
on:
  workflow_dispatch:
jobs:
  doctor:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: deploy-shuttle doctor --format json --fail-below 75
`
			return os.WriteFile(filepath.Join(dir, "deploy-shuttle.yml"), []byte(content), 0o644)
		},
	})
	return root
}

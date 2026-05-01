package cli

import (
	"fmt"
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

func newLicenseCommand() *cobra.Command {
	root := &cobra.Command{Use: "license", Short: "Manage DeployShuttle license"}
	root.AddCommand(&cobra.Command{Use: "status", Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("DeployShuttle Pro licensing is not active in the Go port.")
	}})
	root.AddCommand(&cobra.Command{Use: "activate <key>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".shuttle"), 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(os.Getenv("HOME"), ".shuttle", "license"), []byte(args[0]), 0o600)
	}})
	root.AddCommand(&cobra.Command{Use: "deactivate", RunE: func(cmd *cobra.Command, args []string) error {
		return os.Remove(filepath.Join(os.Getenv("HOME"), ".shuttle", "license"))
	}})
	return root
}

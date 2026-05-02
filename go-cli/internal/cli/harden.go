package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/harden"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/spf13/cobra"
)

func newHardenCommand() *cobra.Command {
	var input string
	var format string
	var target string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "harden",
		Short: "Plan hardening actions from a doctor readiness report (dry-run only)",
		Long: "harden reads a doctor JSON report and prints concrete proposed actions for each finding. " +
			"This release is dry-run only: no commands are executed and no remote changes are made.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dryRun {
				return errors.New("harden currently supports --dry-run only; pass --dry-run to acknowledge")
			}
			if input == "" {
				input = ".deployshuttle/latest-report.json"
			}
			raw, err := os.ReadFile(input)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("doctor report %q not found; run `deploy-shuttle doctor --output %s` first or pass --input", input, input)
				}
				return err
			}
			var report readiness.Report
			if err := json.Unmarshal(raw, &report); err != nil {
				return fmt.Errorf("invalid doctor JSON report: %w", err)
			}
			if target != "" {
				if _, err := parseSSHTarget(target); err != nil {
					return err
				}
				report.Target = target
			}
			plan := harden.BuildPlan(report)
			switch format {
			case "", "console":
				fmt.Print(harden.Console(plan))
			case "json":
				encoded, err := json.MarshalIndent(plan, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(encoded))
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "doctor JSON report path (default .deployshuttle/latest-report.json)")
	cmd.Flags().StringVar(&format, "format", "console", "output format: console or json")
	cmd.Flags().StringVar(&target, "target", "", "remote SSH target user@host annotated in the plan output (no SSH actions yet)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "explicit acknowledgement that harden is dry-run only in this release")
	return cmd
}

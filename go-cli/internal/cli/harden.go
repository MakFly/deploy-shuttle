package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/harden"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/license"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newHardenCommand() *cobra.Command {
	var input string
	var format string
	var target string
	var dryRun bool
	var apply bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "harden",
		Short: "Plan or apply hardening actions from a doctor readiness report",
		Long: "harden reads a doctor JSON report and prints concrete proposed actions for each finding.\n" +
			"  --dry-run only prints the plan and never touches the system.\n" +
			"  --apply runs the subset of actions flagged as safe-for-apply (currently only chmod 600 on a project-local .env).\n" +
			"  --apply --target user@host runs the same safe subset over SSH on the remote home directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun == apply {
				return errors.New("pass exactly one of --dry-run or --apply")
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

			if dryRun {
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
			}

			// Apply path.
			if err := license.Require("harden --apply"); err != nil {
				return err
			}
			safeCount := 0
			for _, action := range plan.Actions {
				if action.SafeAutoApply {
					safeCount++
				}
			}
			if safeCount == 0 {
				fmt.Println("No safe-for-local-apply actions in this plan. Nothing to do.")
				return nil
			}
			if !yes {
				fmt.Printf("About to apply %d safe local action(s) on this machine. Re-run with --yes to confirm.\n", safeCount)
				return nil
			}
			adapter := execx.Adapter(execx.Local{})
			if target != "" {
				parsed, err := parseSSHTarget(target)
				if err != nil {
					return err
				}
				client, err := ssh.NewClient(parsed.Host, parsed.User, parsed.Port)
				if err != nil {
					return err
				}
				adapter = execx.SSH{Client: client}
			}
			results := harden.Apply(adapter, plan)
			switch format {
			case "", "console":
				fmt.Print(harden.RenderApplyResults(results))
			case "json":
				encoded, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(encoded))
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
			for _, r := range results {
				if r.Status == "failed" {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "doctor JSON report path (default .deployshuttle/latest-report.json)")
	cmd.Flags().StringVar(&format, "format", "console", "output format: console or json")
	cmd.Flags().StringVar(&target, "target", "", "remote SSH target user@host annotated in the plan output (no SSH actions yet)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print proposed actions without touching the system")
	cmd.Flags().BoolVar(&apply, "apply", false, "execute safe local actions (currently chmod 600 .env only)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm execution when --apply is set")
	return cmd
}

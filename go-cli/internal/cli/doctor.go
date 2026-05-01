package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var target string
	var format string
	var profile string
	var failBelow string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run a VPS production readiness audit",
		RunE: func(cmd *cobra.Command, args []string) error {
			if target != "" {
				return errors.New("remote doctor targets are planned but not implemented in this Go slice")
			}
			threshold := -1
			if failBelow != "" {
				value, err := strconv.Atoi(failBelow)
				if err != nil || value < 0 || value > 100 {
					return errors.New("--fail-below must be a number between 0 and 100")
				}
				threshold = value
			}
			report := readiness.Run(execx.Local{}, "local", splitCSV(profile, []string{"docker", "caddy"}))
			switch format {
			case "", "console":
				fmt.Print(readiness.Console(report))
			case "json":
				fmt.Println(readiness.JSON(report))
			default:
				return fmt.Errorf("unsupported format %q", format)
			}
			critical := false
			for _, check := range report.Checks {
				if check.Severity == readiness.Critical && check.Status == readiness.Failed {
					critical = true
				}
			}
			if critical || (threshold >= 0 && report.Score < threshold) {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "remote SSH target user@host")
	cmd.Flags().StringVar(&format, "format", "console", "output format: console or json")
	cmd.Flags().StringVar(&profile, "profile", "", "comma-separated profile labels")
	cmd.Flags().StringVar(&failBelow, "fail-below", "", "exit with code 1 when score is below threshold")
	return cmd
}

func splitCSV(value string, fallback []string) []string {
	if value == "" {
		return fallback
	}
	parts := []string{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

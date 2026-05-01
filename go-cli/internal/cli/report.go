package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/spf13/cobra"
)

func newReportCommand() *cobra.Command {
	var input string
	var output string
	var format string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a readiness report from doctor JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				input = ".deployshuttle/latest-report.json"
			}
			if format == "" {
				format = "markdown"
			}
			raw, err := os.ReadFile(input)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("doctor report %q not found; run doctor --output %s first or pass --input", input, input)
				}
				return err
			}
			var report readiness.Report
			if err := json.Unmarshal(raw, &report); err != nil {
				return fmt.Errorf("invalid doctor JSON report: %w", err)
			}
			switch format {
			case "markdown", "md":
				if output == "" {
					output = "deployshuttle-report.md"
				}
				return os.WriteFile(output, []byte(markdownReport(report)), 0o644)
			case "pdf":
				if output == "" {
					output = "deployshuttle-report.pdf"
				}
				return renderPDF(input, output)
			default:
				return fmt.Errorf("unsupported report format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "doctor JSON report input path (default .deployshuttle/latest-report.json)")
	cmd.Flags().StringVar(&output, "output", "", "report output path")
	cmd.Flags().StringVar(&format, "format", "markdown", "report format: markdown or pdf")
	return cmd
}

func markdownReport(report readiness.Report) string {
	var b strings.Builder
	failed := openFindings(report.Checks)
	ignored := ignoredChecks(report.Checks)
	fmt.Fprintf(&b, "# DeployShuttle Production Readiness Report\n\n")
	fmt.Fprintf(&b, "- Target: `%s`\n", report.Target)
	fmt.Fprintf(&b, "- Score: `%d/100`\n", report.Score)
	fmt.Fprintf(&b, "- Level: `%s`\n", readiness.LevelLabel(report.Level))
	fmt.Fprintf(&b, "- Open findings: `%d`\n", len(failed))
	fmt.Fprintf(&b, "- Accepted risks: `%d`\n", len(ignored))
	if report.ConfigPath != "" {
		fmt.Fprintf(&b, "- Config: `%s`\n", report.ConfigPath)
	}
	fmt.Fprintf(&b, "- Generated: `%s`\n\n", report.GeneratedAt)
	fmt.Fprintf(&b, "## Executive Summary\n\n")
	fmt.Fprintf(&b, "%s\n\n", executiveSummary(report))
	next := nextActions(failed)
	if len(next) > 0 {
		fmt.Fprintf(&b, "## Next Actions\n\n")
		for i, action := range next {
			fmt.Fprintf(&b, "%d. %s\n", i+1, action)
		}
		b.WriteString("\n")
	}

	for _, severity := range []readiness.Severity{readiness.Critical, readiness.High, readiness.Medium, readiness.Low, readiness.Info} {
		checks := filterChecks(report.Checks, severity, false)
		if len(checks) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", strings.Title(string(severity)))
		for _, check := range checks {
			fmt.Fprintf(&b, "- **%s** (`%s`): %s\n", check.Title, check.ID, check.Summary)
			if check.Remediation != "" {
				fmt.Fprintf(&b, "  - Fix: %s\n", check.Remediation)
			}
			if evidence := evidenceSummary(check); evidence != "" {
				fmt.Fprintf(&b, "  - Evidence: %s\n", evidence)
			}
		}
		b.WriteString("\n")
	}

	if len(ignored) > 0 {
		b.WriteString("## Accepted Risks\n\n")
		for _, check := range ignored {
			fmt.Fprintf(&b, "- **%s** (`%s`): %s\n", check.Title, check.ID, check.IgnoreReason)
			if evidence := evidenceSummary(check); evidence != "" {
				fmt.Fprintf(&b, "  - Evidence: %s\n", evidence)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func openFindings(checks []readiness.CheckResult) []readiness.CheckResult {
	out := []readiness.CheckResult{}
	for _, check := range checks {
		if check.Status == readiness.Failed && !check.Ignored {
			out = append(out, check)
		}
	}
	return out
}

func filterChecks(checks []readiness.CheckResult, severity readiness.Severity, includeIgnored bool) []readiness.CheckResult {
	out := []readiness.CheckResult{}
	for _, check := range checks {
		if check.Severity != severity {
			continue
		}
		if check.Ignored != includeIgnored {
			continue
		}
		if check.Status == readiness.Passed || check.Status == readiness.Skipped {
			continue
		}
		out = append(out, check)
	}
	return out
}

func ignoredChecks(checks []readiness.CheckResult) []readiness.CheckResult {
	out := []readiness.CheckResult{}
	for _, check := range checks {
		if check.Ignored {
			out = append(out, check)
		}
	}
	return out
}

func executiveSummary(report readiness.Report) string {
	switch report.Level {
	case "production-ready":
		return "This VPS is production-ready for the selected profile. Remaining findings should still be reviewed before client handoff."
	case "almost-ready":
		return "This VPS is close to production-ready. Fix or explicitly accept the high-priority findings before relying on it for critical workloads."
	case "risky":
		return "This VPS has meaningful production risks. Address the high and medium findings before treating the setup as client-ready."
	default:
		return "This VPS is not production-ready. Critical issues should be fixed before deploying production or client workloads."
	}
}

func nextActions(checks []readiness.CheckResult) []string {
	actions := []string{}
	for _, check := range checks {
		if check.Remediation == "" {
			continue
		}
		actions = append(actions, fmt.Sprintf("%s: %s", check.Title, check.Remediation))
		if len(actions) == 5 {
			break
		}
	}
	return actions
}

func evidenceSummary(check readiness.CheckResult) string {
	if len(check.Evidence) == 0 {
		return ""
	}
	parts := []string{}
	for _, key := range []string{"runtimeMode", "publicPorts", "workloads", "ignoredWorkloads", "readWriteWorkloads", "firewallRestricted", "ipRestriction", "denyRule", "basicAuth"} {
		value, ok := check.Evidence[key]
		if !ok {
			continue
		}
		compact := compactEvidenceValue(value)
		if compact == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, compact))
	}
	return strings.Join(parts, "; ")
}

func compactEvidenceValue(value any) string {
	switch typed := value.(type) {
	case []string:
		return strings.Join(typed, ", ")
	case []any:
		if len(typed) == 0 {
			return ""
		}
		values := []string{}
		for _, item := range typed {
			values = append(values, fmt.Sprint(item))
		}
		return strings.Join(values, ", ")
	default:
		return fmt.Sprint(typed)
	}
}

func renderPDF(input string, output string) error {
	rendererDir, err := findPDFRendererDir()
	if err != nil {
		return err
	}
	absInput, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	cmd := exec.Command("bun", "run", "render", "--input", absInput, "--output", absOutput)
	cmd.Dir = rendererDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findPDFRendererDir() (string, error) {
	if dir := os.Getenv("DEPLOY_SHUTTLE_PDF_RENDERER_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "report-pdf")
		if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("report-pdf renderer not found; set DEPLOY_SHUTTLE_PDF_RENDERER_DIR")
		}
		dir = parent
	}
}

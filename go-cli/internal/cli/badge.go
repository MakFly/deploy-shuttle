package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/readiness"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newBadgeCommand() *cobra.Command {
	var target string
	var format string
	var configPath string
	var fromReport string

	cmd := &cobra.Command{
		Use:   "badge",
		Short: "Generate a production readiness score badge",
		Long: `Generate a shields.io compatible badge URL or raw SVG for the production readiness score.

The score is obtained by running the doctor checks (or reading an existing report).
Badge colors reflect the score:
  90-100  brightgreen
  70-89   green
  50-69   yellow
  30-49   orange
  0-29    red`,
		RunE: func(cmd *cobra.Command, args []string) error {
			score, err := resolveScore(target, configPath, fromReport)
			if err != nil {
				return err
			}

			switch format {
			case "url":
				fmt.Println(badgeURL(score))
			case "svg":
				fmt.Println(badgeSVG(score))
			default:
				return fmt.Errorf("unsupported format %q; use url or svg", format)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "remote SSH target user@host (runs doctor live)")
	cmd.Flags().StringVar(&format, "format", "url", "output format: url or svg")
	cmd.Flags().StringVar(&configPath, "config", "", "path to .deployshuttle.yml readiness config")
	cmd.Flags().StringVar(&fromReport, "from-report", "", "read score from existing JSON report instead of running doctor")

	return cmd
}

func resolveScore(target, configPath, fromReport string) (int, error) {
	if fromReport != "" {
		return scoreFromReport(fromReport)
	}

	readinessConfig, resolvedConfigPath, err := readiness.LoadConfig(configPath)
	if err != nil {
		return 0, err
	}

	adapter := execx.Adapter(execx.Local{})
	reportTarget := "local"
	if target != "" {
		sshTarget, err := parseSSHTarget(target)
		if err != nil {
			return 0, err
		}
		client, err := ssh.NewClient(sshTarget.Host, sshTarget.User, sshTarget.Port)
		if err != nil {
			return 0, err
		}
		adapter = execx.SSH{Client: client}
		reportTarget = sshTarget.String()
	}

	report := readiness.RunWithConfig(adapter, reportTarget, nil, readinessConfig, resolvedConfigPath)
	return report.Score, nil
}

func scoreFromReport(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read report: %w", err)
	}
	var report struct {
		Score int `json:"score"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return 0, fmt.Errorf("cannot parse report JSON: %w", err)
	}
	if report.Score < 0 || report.Score > 100 {
		return 0, errors.New("report score out of range 0-100")
	}
	return report.Score, nil
}

func badgeColor(score int) string {
	switch {
	case score >= 90:
		return "brightgreen"
	case score >= 70:
		return "green"
	case score >= 50:
		return "yellow"
	case score >= 30:
		return "orange"
	default:
		return "red"
	}
}

func badgeURL(score int) string {
	label := "production_readiness"
	value := strconv.Itoa(score) + "%25"
	color := badgeColor(score)
	return fmt.Sprintf("https://img.shields.io/badge/%s-%s-%s", label, value, color)
}

func badgeSVG(score int) string {
	color := badgeColor(score)
	hexColor := badgeColorHex(color)
	label := "production readiness"
	value := strconv.Itoa(score) + "%"

	labelWidth := len(label)*6 + 10
	valueWidth := len(value)*6 + 10
	totalWidth := labelWidth + valueWidth

	labelX := labelWidth / 2
	valueX := labelWidth + valueWidth/2

	escapedLabel := svgEscape(label)
	escapedValue := svgEscape(value)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">
  <title>%s: %s</title>
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r">
    <rect width="%d" height="20" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="20" fill="#555"/>
    <rect x="%d" width="%d" height="20" fill="%s"/>
    <rect width="%d" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14" fill="#fff">%s</text>
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14" fill="#fff">%s</text>
  </g>
</svg>`,
		totalWidth,
		escapedLabel, escapedValue,
		escapedLabel, escapedValue,
		totalWidth,
		labelWidth,
		labelWidth, valueWidth, hexColor,
		totalWidth,
		labelX, escapedLabel,
		labelX, escapedLabel,
		valueX, escapedValue,
		valueX, escapedValue,
	)
}

func badgeColorHex(color string) string {
	switch color {
	case "brightgreen":
		return "#4c1"
	case "green":
		return "#97ca00"
	case "yellow":
		return "#dfb317"
	case "orange":
		return "#fe7d37"
	case "red":
		return "#e05d44"
	default:
		return "#9f9f9f"
	}
}

func svgEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

package harden

import (
	"fmt"
	"strings"
)

func Console(plan Plan) string {
	var b strings.Builder
	target := plan.Target
	if target == "" {
		target = "local"
	}
	fmt.Fprintf(&b, "DeployShuttle harden plan (dry-run)\n")
	fmt.Fprintf(&b, "Target: %s\n", target)
	if plan.Score > 0 || plan.Level != "" {
		fmt.Fprintf(&b, "Score: %d/100  Level: %s\n", plan.Score, plan.Level)
	}
	if plan.GeneratedAt != "" {
		fmt.Fprintf(&b, "Source report generated: %s\n", plan.GeneratedAt)
	}
	if len(plan.Actions) == 0 {
		b.WriteString("\nNo failed findings to act on. Nothing to harden.\n")
		return b.String()
	}
	b.WriteString("\nThis is a dry-run. No commands have been executed and no remote changes were made.\n\n")

	current := ""
	for i, action := range plan.Actions {
		if action.Category != current {
			if current != "" {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "== %s ==\n", strings.ToUpper(action.Category))
			current = action.Category
		}
		fmt.Fprintf(&b, "[%d] %s\n", i+1, action.Title)
		fmt.Fprintf(&b, "    From: %s (%s)\n", action.SourceCheck, action.Severity)
		if action.Rationale != "" {
			fmt.Fprintf(&b, "    Why : %s\n", action.Rationale)
		}
		if len(action.Commands) > 0 {
			b.WriteString("    Proposed commands:\n")
			for _, cmd := range action.Commands {
				fmt.Fprintf(&b, "      $ %s\n", cmd)
			}
		}
		if len(action.Notes) > 0 {
			b.WriteString("    Notes:\n")
			for _, note := range action.Notes {
				fmt.Fprintf(&b, "      - %s\n", note)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("Run each command manually after reviewing it. Re-run `shuttle doctor` to verify the score.\n")
	return b.String()
}

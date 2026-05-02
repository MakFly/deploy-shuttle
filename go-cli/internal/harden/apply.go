package harden

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

// ApplyResult records the outcome of a single safe action.
type ApplyResult struct {
	ActionID string `json:"actionId"`
	Title    string `json:"title"`
	Status   string `json:"status"` // applied | skipped | failed
	Detail   string `json:"detail,omitempty"`
}

// Apply runs only actions flagged SafeAutoApply through the supplied
// execution adapter (local shell or SSH). Each command is mapped to a small
// allow-listed operation; anything outside that list is rejected before any
// shell call is issued.
func Apply(adapter execx.Adapter, plan Plan) []ApplyResult {
	results := []ApplyResult{}
	for _, action := range plan.Actions {
		if !action.SafeAutoApply {
			results = append(results, ApplyResult{
				ActionID: action.ID,
				Title:    action.Title,
				Status:   "skipped",
				Detail:   "action is not flagged as safe for automatic apply; run the proposed commands manually",
			})
			continue
		}
		applied := []string{}
		var failure error
		for _, raw := range action.Commands {
			if err := runSafeCommand(adapter, raw); err != nil {
				failure = fmt.Errorf("%s: %w", raw, err)
				break
			}
			applied = append(applied, raw)
		}
		if failure != nil {
			results = append(results, ApplyResult{
				ActionID: action.ID,
				Title:    action.Title,
				Status:   "failed",
				Detail:   failure.Error(),
			})
			continue
		}
		results = append(results, ApplyResult{
			ActionID: action.ID,
			Title:    action.Title,
			Status:   "applied",
			Detail:   strings.Join(applied, "; "),
		})
	}
	return results
}

func runSafeCommand(adapter execx.Adapter, raw string) error {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	switch parts[0] {
	case "chmod":
		return runChmod(adapter, parts[1:])
	case "ufw":
		return runUFWDeny(adapter, parts[1:])
	}
	return fmt.Errorf("command %q is not in the safe allow list", parts[0])
}

func runUFWDeny(adapter execx.Adapter, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("ufw safe form requires `deny <port>/tcp`, got %v", args)
	}
	if args[0] != "deny" {
		return fmt.Errorf("only `ufw deny` is allow-listed, got %q", args[0])
	}
	spec := args[1]
	port, proto, ok := strings.Cut(spec, "/")
	if !ok || proto != "tcp" {
		return fmt.Errorf("ufw target %q must be of the form <port>/tcp", spec)
	}
	if port == "" || len(port) > 5 {
		return fmt.Errorf("ufw port %q is invalid", port)
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return fmt.Errorf("ufw port %q must be numeric", port)
		}
	}
	res := adapter.Run("ufw deny "+shellQuote(spec), 10*time.Second)
	if res.ExitCode != 0 {
		stderr := strings.TrimSpace(res.Stderr)
		if stderr == "" {
			stderr = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return fmt.Errorf("ufw deny failed: %s", stderr)
	}
	return nil
}

func runChmod(adapter execx.Adapter, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("chmod requires <mode> <path>, got %v", args)
	}
	mode := args[0]
	path := args[1]
	if mode != "600" {
		return fmt.Errorf("chmod mode %q is not allowed in safe apply (only 600)", mode)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("chmod target %q must be a relative project-local path", path)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("chmod target %q must not traverse parent directories", path)
	}
	if filepath.Base(path) != ".env" {
		return fmt.Errorf("chmod target %q must be a .env file", path)
	}
	probe := adapter.Run("test -f "+shellQuote(path), 5*time.Second)
	if probe.ExitCode != 0 {
		return fmt.Errorf("chmod target %q does not exist on the target", path)
	}
	res := adapter.Run("chmod 600 "+shellQuote(path), 5*time.Second)
	if res.ExitCode != 0 {
		stderr := strings.TrimSpace(res.Stderr)
		if stderr == "" {
			stderr = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return fmt.Errorf("chmod failed: %s", stderr)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func RenderApplyResults(results []ApplyResult) string {
	var b strings.Builder
	if len(results) == 0 {
		b.WriteString("No actions to apply.\n")
		return b.String()
	}
	applied, skipped, failed := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "applied":
			applied++
		case "skipped":
			skipped++
		case "failed":
			failed++
		}
	}
	fmt.Fprintf(&b, "Applied: %d  Skipped: %d  Failed: %d\n\n", applied, skipped, failed)
	for _, r := range results {
		fmt.Fprintf(&b, "[%s] %s (%s)\n", strings.ToUpper(r.Status), r.Title, r.ActionID)
		if r.Detail != "" {
			fmt.Fprintf(&b, "    %s\n", r.Detail)
		}
	}
	return b.String()
}

package harden

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyResult records the outcome of a single safe local action.
type ApplyResult struct {
	ActionID string `json:"actionId"`
	Title    string `json:"title"`
	Status   string `json:"status"` // applied | skipped | failed
	Detail   string `json:"detail,omitempty"`
}

// ApplySafeLocal runs only actions flagged SafeLocalApply on the local
// machine. It never opens an SSH session and never executes commands from
// non-safe actions. Each command is mapped to a small set of allow-listed
// operations; anything outside that list is rejected.
func ApplySafeLocal(plan Plan) []ApplyResult {
	results := []ApplyResult{}
	for _, action := range plan.Actions {
		if !action.SafeLocalApply {
			results = append(results, ApplyResult{
				ActionID: action.ID,
				Title:    action.Title,
				Status:   "skipped",
				Detail:   "action is not flagged as safe for local apply; run the proposed commands manually",
			})
			continue
		}
		applied := []string{}
		var failure error
		for _, raw := range action.Commands {
			if err := runSafeCommand(raw); err != nil {
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

func runSafeCommand(raw string) error {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return errors.New("empty command")
	}
	switch parts[0] {
	case "chmod":
		return runChmod(parts[1:])
	}
	return fmt.Errorf("command %q is not in the safe local allow list", parts[0])
}

func runChmod(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("chmod requires <mode> <path>, got %v", args)
	}
	mode := args[0]
	path := args[1]
	if mode != "600" {
		return fmt.Errorf("chmod mode %q is not allowed in safe local apply (only 600)", mode)
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
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("chmod target %q does not exist", path)
		}
		return err
	}
	return os.Chmod(path, 0o600)
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

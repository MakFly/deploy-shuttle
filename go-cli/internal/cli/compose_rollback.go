package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
)

type composeState struct {
	Version    string        `json:"version"`
	DeployedAt string        `json:"deployed_at"`
	Strategy   string        `json:"strategy"`
	Previous   *composeState `json:"previous,omitempty"`
}

func rollbackCompose(cfg *config.Config, dryRun bool) error {
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			client, err := connectSSH(group, host)
			if err != nil {
				return fmt.Errorf("connect to %s: %w", host, err)
			}
			if err := rollbackComposeHost(cfg, client, host, dryRun); err != nil {
				return fmt.Errorf("rollback on %s: %w", host, err)
			}
		}
	}
	return nil
}

func rollbackComposeHost(cfg *config.Config, client *ssh.Client, host string, dryRun bool) error {
	statePath := runtime.StatePath(cfg.App)
	res := client.Run(fmt.Sprintf("cat %s 2>/dev/null", shell.Escape(statePath)))
	if res.Code != 0 || strings.TrimSpace(res.Stdout) == "" {
		return fmt.Errorf("no state.json found — nothing to rollback to")
	}

	var state composeState
	if err := json.Unmarshal([]byte(res.Stdout), &state); err != nil {
		return fmt.Errorf("parse state.json: %w", err)
	}

	if state.Previous == nil {
		return fmt.Errorf("no previous deployment in state.json — nothing to rollback to")
	}

	fmt.Printf("Current: %s (deployed %s)\n", state.Version, state.DeployedAt)
	fmt.Printf("Rolling back to: %s (deployed %s)\n", state.Previous.Version, state.Previous.DeployedAt)

	remoteDir := runtime.AppDir(cfg.App)

	if dryRun {
		fmt.Printf("[dry-run] Would run: cd %s && docker compose down && docker compose up -d\n", remoteDir)
		return nil
	}

	fmt.Println("-> Restarting services with previous image...")
	restartCmd := fmt.Sprintf("cd %s && docker compose down && docker compose up -d --remove-orphans",
		shell.Escape(remoteDir))
	res = client.Run(restartCmd)
	if res.Code != 0 {
		return fmt.Errorf("restart failed: %s", res.Stderr)
	}

	newState := composeState{
		Version:    state.Previous.Version,
		DeployedAt: state.Previous.DeployedAt,
		Strategy:   "compose",
		Previous: &composeState{
			Version:    state.Version,
			DeployedAt: state.DeployedAt,
		},
	}
	stateJSON, _ := json.MarshalIndent(newState, "", "  ")
	client.UploadContent(string(stateJSON)+"\n", statePath, 0o644)

	fmt.Printf("✓ Rolled back to %s\n", state.Previous.Version)
	return nil
}

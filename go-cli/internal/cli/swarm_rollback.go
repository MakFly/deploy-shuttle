package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
)

func rollbackSwarm(cfg *config.Config, dryRun bool) error {
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			client, err := connectSSH(group, host)
			if err != nil {
				return fmt.Errorf("connect to %s: %w", host, err)
			}

			if err := rollbackSwarmHost(cfg, client, host, dryRun); err != nil {
				return fmt.Errorf("rollback on %s: %w", host, err)
			}
		}
	}

	return nil
}

func rollbackSwarmHost(cfg *config.Config, client *ssh.Client, host string, dryRun bool) error {
	statePath := runtime.StatePath(cfg.App)

	// Step 1: Read state.json
	fmt.Printf("-> Reading deployment state from %s...\n", host)
	res := client.Run(fmt.Sprintf("cat %s", shell.Escape(statePath)))
	if res.Code != 0 {
		return fmt.Errorf("cannot read state.json: %s", strings.TrimSpace(res.Stderr))
	}

	var state swarmState
	if err := json.Unmarshal([]byte(res.Stdout), &state); err != nil {
		return fmt.Errorf("parse state.json: %w", err)
	}

	// Step 2: Verify previous exists
	if state.Previous == nil {
		return fmt.Errorf("no previous deployment to rollback to")
	}

	fmt.Printf("  Current: version=%s deployed=%s\n", state.Version, state.DeployedAt)
	fmt.Printf("  Rolling back to: version=%s deployed=%s\n", state.Previous.Version, state.Previous.DeployedAt)

	if dryRun {
		fmt.Println("\n[dry-run] Would execute:")
		fmt.Printf("  1. List services in stack %s\n", cfg.App)
		fmt.Printf("  2. For each service: docker service update --rollback <service>\n")
		fmt.Printf("  3. Wait for convergence\n")
		fmt.Printf("  4. Update state.json\n")
		return nil
	}

	// Step 3: List services in the stack and rollback each one
	fmt.Printf("-> Rolling back services in stack %s...\n", cfg.App)
	listCmd := fmt.Sprintf(
		"docker service ls --filter name=%s --format '{{.Name}}'",
		shell.Escape(cfg.App),
	)
	res = client.Run(listCmd)
	if res.Code != 0 {
		return fmt.Errorf("list services: %s", strings.TrimSpace(res.Stderr))
	}

	services := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	if len(services) == 0 || (len(services) == 1 && services[0] == "") {
		return fmt.Errorf("no services found in stack %s", cfg.App)
	}

	for _, svcName := range services {
		svcName = strings.TrimSpace(svcName)
		if svcName == "" {
			continue
		}
		fmt.Printf("-> Rolling back service %s...\n", svcName)
		rollbackCmd := fmt.Sprintf("docker service update --rollback %s", shell.Escape(svcName))
		res = client.Run(rollbackCmd)
		if res.Code != 0 {
			return fmt.Errorf("rollback service %s: %s", svcName, strings.TrimSpace(res.Stderr))
		}
	}

	// Step 4: Wait for convergence
	fmt.Println("-> Waiting for service convergence...")
	if err := waitForSwarmConvergence(client, cfg.App, cfg.Deploy.Timeout); err != nil {
		return fmt.Errorf("convergence after rollback failed: %w", err)
	}

	// Step 5: Update state.json (swap version/previous)
	fmt.Println("-> Updating state.json...")
	newState := swarmState{
		Version:    state.Previous.Version,
		DeployedAt: time.Now().UTC().Format(time.RFC3339),
		Strategy:   "swarm",
		Stack:      cfg.App,
		Previous: &swarmPrev{
			Version:    state.Version,
			DeployedAt: state.DeployedAt,
		},
	}

	stateJSON, err := json.MarshalIndent(newState, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	res = client.UploadContent(string(stateJSON)+"\n", statePath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("write state.json: %s", strings.TrimSpace(res.Stderr))
	}

	fmt.Printf("-> Rollback complete on %s (now version: %s)\n", host, state.Previous.Version)
	return nil
}

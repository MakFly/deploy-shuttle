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

type deployState struct {
	Active     string       `json:"active"`
	Version    string       `json:"version"`
	DeployedAt string       `json:"deployed_at"`
	Previous   *deployState `json:"previous,omitempty"`
}

func rollbackDeploy(cfg *config.Config, dryRun bool) error {
	if cfg.Deploy.Strategy != "blue-green" {
		return fmt.Errorf("rollback requires blue-green deploy strategy; current strategy is %q", cfg.Deploy.Strategy)
	}

	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			client, err := connectSSH(group, host)
			if err != nil {
				return fmt.Errorf("connect to %s: %w", host, err)
			}

			if err := rollbackHost(cfg, client, host, dryRun); err != nil {
				return fmt.Errorf("rollback on %s: %w", host, err)
			}
		}
	}

	return nil
}

func rollbackHost(cfg *config.Config, client *ssh.Client, host string, dryRun bool) error {
	appDir := runtime.AppDir(cfg.App, cfg.Deploy.Path)
	statePath := runtime.StatePath(cfg.App, cfg.Deploy.Path)

	// Step 1: Read state.json
	fmt.Printf("→ Reading deployment state from %s...\n", host)
	res := client.Run(fmt.Sprintf("cat %s", shell.Escape(statePath)))
	if res.Code != 0 {
		return fmt.Errorf("cannot read state.json: %s", strings.TrimSpace(res.Stderr))
	}

	var state deployState
	if err := json.Unmarshal([]byte(res.Stdout), &state); err != nil {
		return fmt.Errorf("parse state.json: %w", err)
	}

	// Step 2: Verify previous exists
	if state.Previous == nil {
		return fmt.Errorf("no previous deployment to rollback to")
	}

	currentSlot := state.Active
	prevSlot := state.Previous.Active
	prevVersion := state.Previous.Version

	fmt.Printf("  Current: slot=%s version=%s\n", currentSlot, state.Version)
	fmt.Printf("  Rolling back to: slot=%s version=%s\n", prevSlot, prevVersion)

	if dryRun {
		fmt.Println("\n[dry-run] Would execute:")
		fmt.Printf("  1. Start containers: docker compose -p %s-%s -f %s/docker-compose.yml start\n", cfg.App, prevSlot, appDir)
		fmt.Printf("  2. Wait for healthcheck on %s-%s containers\n", cfg.App, prevSlot)
		fmt.Printf("  3. Switch Caddy upstream to %s slot\n", prevSlot)
		fmt.Printf("  4. Stop containers: docker compose -p %s-%s -f %s/docker-compose.yml stop\n", cfg.App, currentSlot, appDir)
		fmt.Printf("  5. Update state.json (active=%s)\n", prevSlot)
		return nil
	}

	// Step 3: Start previous slot containers
	fmt.Printf("→ Starting previous slot (%s)...\n", prevSlot)
	composeCmd := fmt.Sprintf("docker compose -p %s -f %s/docker-compose.yml start",
		shell.Escape(cfg.App+"-"+prevSlot),
		shell.Escape(appDir))
	res = client.Run(composeCmd)
	if res.Code != 0 {
		return fmt.Errorf("start previous slot: %s", strings.TrimSpace(res.Stderr))
	}

	// Step 4: Wait for healthcheck
	fmt.Printf("→ Waiting for healthcheck on %s-%s...\n", cfg.App, prevSlot)
	if err := waitForSlotHealthy(client, cfg.App, prevSlot, appDir, cfg.Deploy.Timeout); err != nil {
		return fmt.Errorf("healthcheck failed: %w", err)
	}

	// Step 5: Switch Caddy upstream
	if len(cfg.Caddy.Routes) > 0 {
		fmt.Println("→ Switching Caddy upstream...")
		caddyConf := generateCaddyConfForSlot(cfg, prevSlot)
		caddyPath := fmt.Sprintf("%s/50-%s.caddy", cfg.Caddy.ConfDir, cfg.App)
		res = client.UploadContent(caddyConf, caddyPath, 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload caddy config: %s", strings.TrimSpace(res.Stderr))
		}
		res = client.Run(cfg.Caddy.ReloadCommand)
		if res.Code != 0 {
			fmt.Printf("⚠ Caddy reload warning: %s\n", strings.TrimSpace(res.Stderr))
		}
	}

	// Step 6: Stop current active slot
	fmt.Printf("→ Stopping current slot (%s)...\n", currentSlot)
	composeCmd = fmt.Sprintf("docker compose -p %s -f %s/docker-compose.yml stop",
		shell.Escape(cfg.App+"-"+currentSlot),
		shell.Escape(appDir))
	res = client.Run(composeCmd)
	if res.Code != 0 {
		fmt.Printf("⚠ Stop current slot warning: %s\n", strings.TrimSpace(res.Stderr))
	}

	// Step 7: Update state.json (swap active/previous)
	fmt.Println("→ Updating state.json...")
	newState := deployState{
		Active:     prevSlot,
		Version:    prevVersion,
		DeployedAt: time.Now().UTC().Format(time.RFC3339),
		Previous: &deployState{
			Active:     currentSlot,
			Version:    state.Version,
			DeployedAt: state.DeployedAt,
		},
	}
	stateJSON, err := json.MarshalIndent(newState, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	res = client.UploadContent(string(stateJSON), statePath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("write state.json: %s", strings.TrimSpace(res.Stderr))
	}

	fmt.Printf("✓ Rollback complete on %s (now active: %s, version: %s)\n", host, prevSlot, prevVersion)
	return nil
}

func waitForSlotHealthy(client *ssh.Client, app string, slot string, appDir string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	projectName := app + "-" + slot
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		cmd := fmt.Sprintf("docker compose -p %s -f %s/docker-compose.yml ps --format json",
			shell.Escape(projectName),
			shell.Escape(appDir))
		res := client.Run(cmd)
		if res.Code == 0 && res.Stdout != "" {
			if allContainersRunning(res.Stdout) {
				return nil
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("containers did not become healthy within %ds", timeoutSec)
}

func allContainersRunning(jsonOutput string) bool {
	// docker compose ps --format json outputs one JSON object per line
	lines := strings.Split(strings.TrimSpace(jsonOutput), "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var container struct {
			State  string `json:"State"`
			Health string `json:"Health"`
		}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			return false
		}
		if container.State != "running" {
			return false
		}
		// If a health check is defined but not healthy yet, keep waiting
		if container.Health != "" && container.Health != "healthy" {
			return false
		}
	}
	return true
}

func generateCaddyConfForSlot(cfg *config.Config, slot string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Generated by deploy-shuttle for %s (slot: %s)\n", cfg.App, slot))
	b.WriteString("# Do not edit manually — regenerated on each deploy/rollback\n\n")

	for domain, upstream := range cfg.Caddy.Routes {
		slotUpstream := switchUpstreamSlot(upstream, cfg.App, slot)

		fmt.Fprintf(&b, "%s {\n", domain)
		tlsSnippet := cfg.Caddy.TLSSnippet
		if tlsSnippet == "" {
			tlsSnippet = "standard_tls"
		}
		fmt.Fprintf(&b, "    import %s\n\n", tlsSnippet)
		writeCaddySecurityHeaders(&b)
		writeCaddyBasicAuth(&b, cfg)
		fmt.Fprintf(&b, "    reverse_proxy %s {\n", slotUpstream)
		b.WriteString("        header_up Host {host}\n")
		b.WriteString("        header_up X-Real-IP {remote}\n")
		b.WriteString("    }\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}

// switchUpstreamSlot replaces the opposite slot reference in the upstream with the target slot.
// E.g. "app-blue-web-1:3000" -> "app-green-web-1:3000"
func switchUpstreamSlot(upstream string, app string, targetSlot string) string {
	oppositeSlot := "green"
	if targetSlot == "green" {
		oppositeSlot = "blue"
	}

	old := app + "-" + oppositeSlot
	newVal := app + "-" + targetSlot
	if strings.Contains(upstream, old) {
		return strings.ReplaceAll(upstream, old, newVal)
	}

	return upstream
}

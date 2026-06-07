package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/ssh"
	"gopkg.in/yaml.v3"
)

// blueGreenState represents the deployment state persisted on the remote host.
type blueGreenState struct {
	Active     string         `json:"active"`
	Version    string         `json:"version"`
	DeployedAt string         `json:"deployed_at"`
	Previous   *blueGreenPrev `json:"previous,omitempty"`
}

type blueGreenPrev struct {
	Active     string `json:"active"`
	Version    string `json:"version"`
	DeployedAt string `json:"deployed_at"`
}

func deployBlueGreen(cfg *config.Config, skipBuild bool, dryRun bool) error {
	// Load .env for build-arg expansion
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	loadDotenv(envFile)

	composeFiles := cfg.Deploy.ComposeFiles
	if len(composeFiles) == 0 {
		composeFiles = []string{"docker-compose.yml"}
	}

	parsed, err := parseComposeFile(composeFiles[0])
	if err != nil {
		return fmt.Errorf("parse %s: %w", composeFiles[0], err)
	}

	buildServices := findBuildServices(parsed)
	if len(buildServices) == 0 {
		return fmt.Errorf("no services with build: found in %s", composeFiles[0])
	}

	fmt.Printf("Found %d services to build: %s\n", len(buildServices), strings.Join(buildServices, ", "))
	fmt.Println("Strategy: blue-green (zero-downtime)")

	registryAddr := fmt.Sprintf("127.0.0.1:%d", registryPort)

	// Step 1: Build images locally
	if !skipBuild {
		for _, svc := range buildServices {
			imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, svc)
			fmt.Printf("\n-> Building %s...\n", svc)
			if err := buildServiceImage(parsed, svc, imageTag, dryRun); err != nil {
				return fmt.Errorf("build %s: %w", svc, err)
			}
		}
	}

	version := detectVersion()

	if dryRun {
		fmt.Println("\n[dry-run] Would start ephemeral registry, SSH tunnel, and deploy via blue-green")
		fmt.Printf("[dry-run] Version: %s\n", version)
		prodCompose := generateBlueGreenCompose(parsed, cfg, buildServices, registryAddr, "blue")
		fmt.Printf("\n[dry-run] Generated blue-green compose (slot=blue):\n%s\n", prodCompose)
		return nil
	}

	// Step 2: Start ephemeral registry
	fmt.Printf("\n-> Starting ephemeral registry on :%d...\n", registryPort)
	if err := startRegistry(); err != nil {
		return fmt.Errorf("start registry: %w", err)
	}
	defer stopRegistry()

	// Step 3: Push images to local registry
	for _, svc := range buildServices {
		imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, svc)
		fmt.Printf("-> Pushing %s to local registry...\n", svc)
		if err := dockerPush(imageTag); err != nil {
			return fmt.Errorf("push %s: %w", svc, err)
		}
	}

	// Step 4: Deploy to each server
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			if err := deployBlueGreenToHost(cfg, group, host, parsed, buildServices, registryAddr, version); err != nil {
				return err
			}
		}
	}

	fmt.Println("\n-> Blue-green deploy complete (zero-downtime)")
	return nil
}

func deployBlueGreenToHost(cfg *config.Config, group config.ServerGroup, host string, parsed *composeFile, buildServices []string, registryAddr string, version string) error {
	client, err := connectSSH(group, host)
	if err != nil {
		return err
	}

	appDir := runtime.AppDir(cfg.App)
	fmt.Printf("\n-> Deploying (blue-green) to %s@%s:%d (%s)...\n", group.User, host, group.Port, appDir)

	// SSH reverse tunnel
	fmt.Println("-> Opening SSH tunnel...")
	tunnel, err := startSSHTunnel(host, group.User, group.Port, registryPort)
	if err != nil {
		return fmt.Errorf("SSH tunnel: %w", err)
	}
	defer func() {
		fmt.Println("-> Closing SSH tunnel...")
		tunnel.Process.Kill()
		tunnel.Wait()
	}()

	time.Sleep(2 * time.Second)

	// Ensure app directory exists
	res := client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(appDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir on %s: %s", host, res.Stderr)
	}

	// Read current state to determine active slot
	currentState := readRemoteState(client, cfg.App)
	activeSlot := currentState.Active
	if activeSlot == "" {
		activeSlot = "green" // no previous deploy, so "green" is current => deploy to "blue"
	}
	targetSlot := oppositeSlot(activeSlot)

	fmt.Printf("-> Current active slot: %s, deploying to: %s\n", activeSlot, targetSlot)

	// Create slot directory
	slotDir := runtime.BlueGreenDir(cfg.App, targetSlot)
	res = client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(slotDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir slot dir on %s: %s", host, res.Stderr)
	}

	// Upload .env to slot directory
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil {
		fmt.Printf("-> Uploading %s to %s...\n", envFile, targetSlot)
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", envFile, err)
		}
		res = client.UploadContent(string(envContent), slotDir+".env", 0o600)
		if res.Code != 0 {
			return fmt.Errorf("upload .env to %s: %s", host, res.Stderr)
		}
	}

	// Generate and upload blue-green compose file
	prodCompose := generateBlueGreenCompose(parsed, cfg, buildServices, registryAddr, targetSlot)
	fmt.Printf("-> Uploading docker-compose.yml to %s slot...\n", targetSlot)
	res = client.UploadContent(prodCompose, slotDir+"docker-compose.yml", 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload compose to %s: %s", host, res.Stderr)
	}

	// Pull images on the target slot
	projectName := fmt.Sprintf("%s-%s", cfg.App, targetSlot)
	fmt.Printf("-> Pulling images for %s...\n", projectName)
	pullCmd := fmt.Sprintf("cd %s && docker compose -p %s pull",
		shell.Escape(slotDir), shell.Escape(projectName))
	res = client.Run(pullCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose pull on %s: %s", host, res.Stderr)
	}

	// Start the new slot
	fmt.Printf("-> Starting %s slot...\n", targetSlot)
	upCmd := fmt.Sprintf("cd %s && docker compose -p %s up -d --remove-orphans",
		shell.Escape(slotDir), shell.Escape(projectName))
	res = client.Run(upCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose up on %s: %s", host, res.Stderr)
	}

	// Wait for healthcheck
	fmt.Printf("-> Waiting for healthcheck on %s slot...\n", targetSlot)
	readinessDelay := cfg.Deploy.BlueGreen.ReadinessDelay
	if readinessDelay <= 0 {
		readinessDelay = 5
	}
	timeout := cfg.Deploy.Timeout
	if timeout <= 0 {
		timeout = 120
	}

	if err := waitForHealthy(client, projectName, buildServices, cfg.App, targetSlot, timeout, readinessDelay); err != nil {
		// Rollback: stop the failed slot
		fmt.Printf("-> Healthcheck failed, stopping %s slot...\n", targetSlot)
		stopCmd := fmt.Sprintf("cd %s && docker compose -p %s down",
			shell.Escape(slotDir), shell.Escape(projectName))
		client.Run(stopCmd)
		return fmt.Errorf("healthcheck failed for %s slot: %w", targetSlot, err)
	}

	// Switch Caddy upstream to new slot
	if len(cfg.Caddy.Routes) > 0 {
		fmt.Printf("-> Switching Caddy upstream to %s slot...\n", targetSlot)
		caddyConf := generateBlueGreenCaddyConf(cfg, targetSlot)
		caddyPath := fmt.Sprintf("%s/50-%s.caddy", cfg.Caddy.ConfDir, cfg.App)
		res = client.UploadContent(caddyConf, caddyPath, 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload caddy config to %s: %s", host, res.Stderr)
		}
		fmt.Println("-> Reloading Caddy...")
		res = client.Run(cfg.Caddy.ReloadCommand)
		if res.Code != 0 {
			fmt.Printf("WARNING: Caddy reload failed: %s\n", res.Stderr)
		}
	}

	// Drain old slot
	drainTimeout := cfg.Deploy.BlueGreen.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = 10
	}

	if currentState.Active != "" {
		fmt.Printf("-> Draining old slot (%s) for %ds...\n", activeSlot, drainTimeout)
		time.Sleep(time.Duration(drainTimeout) * time.Second)

		oldSlotDir := runtime.BlueGreenDir(cfg.App, activeSlot)
		oldProjectName := fmt.Sprintf("%s-%s", cfg.App, activeSlot)
		fmt.Printf("-> Stopping old slot (%s)...\n", activeSlot)
		stopCmd := fmt.Sprintf("cd %s && docker compose -p %s down",
			shell.Escape(oldSlotDir), shell.Escape(oldProjectName))
		res = client.Run(stopCmd)
		if res.Code != 0 {
			fmt.Printf("WARNING: failed to stop old slot: %s\n", res.Stderr)
		}
	}

	// Save new state
	newState := blueGreenState{
		Active:     targetSlot,
		Version:    version,
		DeployedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if currentState.Active != "" {
		newState.Previous = &blueGreenPrev{
			Active:     currentState.Active,
			Version:    currentState.Version,
			DeployedAt: currentState.DeployedAt,
		}
	}

	stateJSON, err := json.MarshalIndent(newState, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	statePath := runtime.StatePath(cfg.App)
	fmt.Printf("-> Saving state to %s...\n", statePath)
	res = client.UploadContent(string(stateJSON)+"\n", statePath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload state to %s: %s", host, res.Stderr)
	}

	// Prune old images
	fmt.Println("-> Pruning old images...")
	client.Run("docker image prune -f")

	return nil
}

func oppositeSlot(slot string) string {
	if slot == "blue" {
		return "green"
	}
	return "blue"
}

func readRemoteState(client *ssh.Client, app string) blueGreenState {
	statePath := runtime.StatePath(app)
	res := client.Run(fmt.Sprintf("cat %s 2>/dev/null", shell.Escape(statePath)))
	if res.Code != 0 || strings.TrimSpace(res.Stdout) == "" {
		return blueGreenState{}
	}
	var state blueGreenState
	if err := json.Unmarshal([]byte(res.Stdout), &state); err != nil {
		return blueGreenState{}
	}
	return state
}

func waitForHealthy(client *ssh.Client, projectName string, services []string, app string, slot string, timeoutSecs int, readinessDelay int) error {
	// Initial delay before polling
	time.Sleep(time.Duration(readinessDelay) * time.Second)

	deadline := time.Now().Add(time.Duration(timeoutSecs) * time.Second)

	for _, svc := range services {
		containerName := fmt.Sprintf("%s-%s-%s", app, slot, svc)
		fmt.Printf("   Checking container %s...\n", containerName)

		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for container %s to become healthy", containerName)
			}

			inspectCmd := fmt.Sprintf(
				"docker inspect --format '{{.State.Health.Status}}' %s 2>/dev/null || echo none",
				shell.Escape(containerName),
			)
			res := client.Run(inspectCmd)
			status := strings.TrimSpace(res.Stdout)

			switch status {
			case "healthy":
				fmt.Printf("   %s is healthy\n", containerName)
				goto nextService
			case "unhealthy":
				return fmt.Errorf("container %s is unhealthy", containerName)
			case "none", "":
				// No healthcheck defined — consider the container ready if running
				runningCmd := fmt.Sprintf(
					"docker inspect --format '{{.State.Running}}' %s 2>/dev/null || echo false",
					shell.Escape(containerName),
				)
				runRes := client.Run(runningCmd)
				if strings.TrimSpace(runRes.Stdout) == "true" {
					fmt.Printf("   %s is running (no healthcheck defined)\n", containerName)
					goto nextService
				}
			}

			time.Sleep(2 * time.Second)
		}
	nextService:
	}
	return nil
}

func generateBlueGreenCompose(cf *composeFile, cfg *config.Config, buildServices []string, registryAddr string, slot string) string {
	buildSet := map[string]bool{}
	for _, s := range buildServices {
		buildSet[s] = true
	}

	prod := &composeFile{
		Services: map[string]composeService{},
		Volumes:  map[string]any{},
		Networks: map[string]any{
			"default": map[string]any{
				"name": fmt.Sprintf("%s-%s-net", cfg.App, slot),
			},
			"caddy_network": map[string]any{
				"external": true,
				"name":     "caddy_network",
			},
		},
	}

	for name, svc := range cf.Services {
		// Skip caddy services
		if svc.Image != "" && strings.Contains(svc.Image, "caddy") {
			continue
		}
		if name == "caddy" {
			continue
		}

		containerName := fmt.Sprintf("%s-%s-%s", cfg.App, slot, name)
		prodSvc := composeService{
			ContainerName: containerName,
			Environment:   svc.Environment,
			Volumes:       svc.Volumes,
			Expose:        svc.Expose,
			Restart:       svc.Restart,
			Command:       svc.Command,
			Healthcheck:   svc.Healthcheck,
			Networks:      []string{"default", "caddy_network"},
			EnvFile:       ".env",
		}

		if buildSet[name] {
			prodSvc.Image = fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, name)
		} else {
			prodSvc.Image = svc.Image
		}

		// Remove depends_on references to caddy
		if deps, ok := svc.DependsOn.([]any); ok {
			var filtered []string
			for _, d := range deps {
				if s, ok := d.(string); ok && s != "caddy" {
					filtered = append(filtered, s)
				}
			}
			if len(filtered) > 0 {
				prodSvc.DependsOn = filtered
			}
		}

		prodSvc.Labels = map[string]string{
			"shuttle.app":  cfg.App,
			"shuttle.slot": slot,
		}

		prod.Services[name] = prodSvc
	}

	// Carry over volumes (except caddy ones)
	if cf.Volumes != nil {
		for name, v := range cf.Volumes {
			if strings.Contains(name, "caddy") {
				continue
			}
			prod.Volumes[name] = v
		}
	}

	out, _ := yaml.Marshal(prod)

	header := fmt.Sprintf("# Generated by deploy-shuttle (blue-green, slot=%s)\n# App: %s\n# Generated: %s\n\n",
		slot, cfg.App, time.Now().UTC().Format(time.RFC3339))

	return header + string(out)
}

func generateBlueGreenCaddyConf(cfg *config.Config, activeSlot string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Generated by deploy-shuttle for %s (blue-green, active=%s)\n", cfg.App, activeSlot))
	b.WriteString("# Do not edit manually — regenerated on each deploy\n\n")

	for domain, upstream := range cfg.Caddy.Routes {
		// Replace upstream with slot-specific container reference
		slotUpstream := rewriteUpstreamForSlot(upstream, cfg.App, activeSlot)

		fmt.Fprintf(&b, "%s {\n", domain)
		tlsSnippet := cfg.Caddy.TLSSnippet
		if tlsSnippet == "" {
			tlsSnippet = "standard_tls"
		}
		fmt.Fprintf(&b, "    import %s\n\n", tlsSnippet)
		b.WriteString("    header {\n")
		b.WriteString("        X-Content-Type-Options \"nosniff\"\n")
		b.WriteString("        X-Frame-Options \"DENY\"\n")
		b.WriteString("        Referrer-Policy \"strict-origin-when-cross-origin\"\n")
		b.WriteString("        Strict-Transport-Security \"max-age=31536000; includeSubDomains\"\n")
		b.WriteString("        -Server\n")
		b.WriteString("    }\n\n")
		fmt.Fprintf(&b, "    reverse_proxy %s {\n", slotUpstream)
		b.WriteString("        header_up Host {host}\n")
		b.WriteString("        header_up X-Real-IP {remote}\n")
		b.WriteString("    }\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}

// rewriteUpstreamForSlot replaces the upstream target with the slot-specific container.
// If the upstream contains a container reference like "app-web:3000", it rewrites to
// "app-<slot>-web:3000". Otherwise it prefixes with the slot container naming pattern.
func rewriteUpstreamForSlot(upstream string, app string, slot string) string {
	// upstream format examples: "myapp-web:3000", "web:3000", "http://web:3000"
	// We want to rewrite the host part to the blue-green container name pattern.

	// Strip protocol prefix if present
	proto := ""
	host := upstream
	if strings.HasPrefix(upstream, "http://") {
		proto = "http://"
		host = strings.TrimPrefix(upstream, "http://")
	} else if strings.HasPrefix(upstream, "https://") {
		proto = "https://"
		host = strings.TrimPrefix(upstream, "https://")
	}

	// Split host:port
	hostPart := host
	portPart := ""
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		hostPart = host[:idx]
		portPart = host[idx:]
	}

	// If hostPart already starts with app name, extract the service name
	serviceName := hostPart
	if strings.HasPrefix(hostPart, app+"-") {
		serviceName = strings.TrimPrefix(hostPart, app+"-")
	}

	// Build the slot-specific container name
	newHost := fmt.Sprintf("%s-%s-%s", app, slot, serviceName)
	return proto + newHost + portPart
}

func detectVersion() string {
	// Try git short SHA first
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v
		}
	}
	// Fallback to timestamp
	return time.Now().UTC().Format("20060102T150405")
}

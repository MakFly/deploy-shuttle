package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/output"
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

	output.Header("Found %d services to build: %s", len(buildServices), strings.Join(buildServices, ", "))
	output.Detail("Strategy: blue-green (zero-downtime)")

	// Run pre-deploy hooks
	if err := runLocalHooks("pre-deploy", cfg.Deploy.Hooks.PreDeploy, dryRun); err != nil {
		return err
	}

	registryAddr := fmt.Sprintf("127.0.0.1:%d", registryPort)

	// Step 1: Build images locally (parallel)
	if !skipBuild {
		if len(buildServices) == 1 {
			imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, buildServices[0])
			fmt.Println()
			output.Step("Building %s...", buildServices[0])
			if err := buildServiceImage(parsed, buildServices[0], imageTag, dryRun); err != nil {
				return fmt.Errorf("build %s: %w", buildServices[0], err)
			}
		} else {
			fmt.Println()
			output.Step("Building %d services in parallel...", len(buildServices))
			var wg sync.WaitGroup
			errs := make(chan error, len(buildServices))
			for _, svc := range buildServices {
				wg.Add(1)
				go func(s string) {
					defer wg.Done()
					imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, s)
					output.Step("Building %s...", s)
					if err := buildServiceImage(parsed, s, imageTag, dryRun); err != nil {
						errs <- fmt.Errorf("build %s: %w", s, err)
					}
				}(svc)
			}
			wg.Wait()
			close(errs)
			if err, ok := <-errs; ok {
				return err
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
	fmt.Println()
	output.Step("Starting ephemeral registry on :%d...", registryPort)
	if err := startRegistry(); err != nil {
		return fmt.Errorf("start registry: %w", err)
	}
	defer stopRegistry(cfg.Deploy.PruneBuildCache != "off")

	// Step 3: Push images to local registry (parallel)
	if len(buildServices) == 1 {
		imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, buildServices[0])
		output.Step("Pushing %s to local registry...", buildServices[0])
		if err := dockerPush(imageTag); err != nil {
			return fmt.Errorf("push %s: %w", buildServices[0], err)
		}
	} else {
		output.Step("Pushing %d images to local registry in parallel...", len(buildServices))
		var wg sync.WaitGroup
		errs := make(chan error, len(buildServices))
		for _, svc := range buildServices {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, s)
				output.Step("Pushing %s to local registry...", s)
				if err := dockerPush(imageTag); err != nil {
					errs <- fmt.Errorf("push %s: %w", s, err)
				}
			}(svc)
		}
		wg.Wait()
		close(errs)
		if err, ok := <-errs; ok {
			return err
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

	// Run post-deploy hooks
	if err := runLocalHooks("post-deploy", cfg.Deploy.Hooks.PostDeploy, dryRun); err != nil {
		return err
	}

	// Reclaim local disk: prune the Docker build cache used by this deploy.
	pruneLocalBuildCache(cfg)

	fmt.Println()
	output.OK("Blue-green deploy complete (zero-downtime)")
	return nil
}

func deployBlueGreenToHost(cfg *config.Config, group config.ServerGroup, host string, parsed *composeFile, buildServices []string, registryAddr string, version string) error {
	client, err := connectSSH(group, host)
	if err != nil {
		return err
	}

	appDir := runtime.AppDir(cfg.App, cfg.Deploy.Path)
	fmt.Println()
	output.Step("Deploying (blue-green) to %s@%s:%d (%s)...", group.User, host, group.Port, appDir)

	// SSH reverse tunnel
	output.Step("Opening SSH tunnel...")
	tunnel, err := startSSHTunnel(host, group.User, group.Port, registryPort)
	if err != nil {
		return fmt.Errorf("SSH tunnel: %w", err)
	}
	defer func() {
		output.Step("Closing SSH tunnel...")
		tunnel.Process.Kill()
		tunnel.Wait()
	}()

	// Wait for SSH tunnel to be ready (TCP dial probe instead of blind sleep)
	tunnelAddr := fmt.Sprintf("127.0.0.1:%d", registryPort)
	tunnelReady := false
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", tunnelAddr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			tunnelReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !tunnelReady {
		output.Attn("SSH tunnel may not be ready, proceeding anyway")
	}

	// Ensure app directory exists
	res := client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(appDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir on %s: %s", host, res.Stderr)
	}

	// Read current state to determine active slot
	currentState := readRemoteState(client, cfg)
	activeSlot := currentState.Active
	if activeSlot == "" {
		activeSlot = "green" // no previous deploy, so "green" is current => deploy to "blue"
	}
	targetSlot := oppositeSlot(activeSlot)

	output.Step("Current active slot: %s, deploying to: %s", activeSlot, targetSlot)

	// Create slot directory
	slotDir := runtime.BlueGreenDir(cfg.App, targetSlot, cfg.Deploy.Path)
	res = client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(slotDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir slot dir on %s: %s", host, res.Stderr)
	}

	// Upload .env to app root (shared between blue/green slots)
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil {
		output.Step("Uploading .env (shared)...")
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", envFile, err)
		}
		res = client.UploadContent(string(envContent), appDir+"/.env", 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload .env to %s: %s", host, res.Stderr)
		}
	}
	// Upload .env.secrets if it exists locally (non-committed secrets)
	if _, err := os.Stat(".env.secrets"); err == nil {
		output.Step("Uploading .env.secrets (shared, chmod 600)...")
		secretsContent, err := os.ReadFile(".env.secrets")
		if err != nil {
			return fmt.Errorf("read .env.secrets: %w", err)
		}
		res = client.UploadContent(string(secretsContent), appDir+"/.env.secrets", 0o600)
		if res.Code != 0 {
			return fmt.Errorf("upload .env.secrets to %s: %s", host, res.Stderr)
		}
	}

	// Generate and upload blue-green compose file
	prodCompose := generateBlueGreenCompose(parsed, cfg, buildServices, registryAddr, targetSlot)
	output.Step("Uploading docker-compose.yml to %s slot...", targetSlot)
	res = client.UploadContent(prodCompose, slotDir+"docker-compose.yml", 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload compose to %s: %s", host, res.Stderr)
	}

	// Pull images on the target slot
	projectName := fmt.Sprintf("%s-%s", cfg.App, targetSlot)
	output.Step("Pulling images for %s...", projectName)
	pullCmd := fmt.Sprintf("cd %s && docker compose -p %s pull",
		shell.Escape(slotDir), shell.Escape(projectName))
	res = client.Run(pullCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose pull on %s: %s", host, res.Stderr)
	}

	// Start the new slot
	output.Step("Starting %s slot...", targetSlot)
	upCmd := fmt.Sprintf("cd %s && docker compose -p %s up -d --remove-orphans",
		shell.Escape(slotDir), shell.Escape(projectName))
	res = client.Run(upCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose up on %s: %s", host, res.Stderr)
	}

	// Wait for healthcheck
	output.Step("Waiting for healthcheck on %s slot...", targetSlot)
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
		output.Attn("Healthcheck failed, stopping %s slot...", targetSlot)
		stopCmd := fmt.Sprintf("cd %s && docker compose -p %s down",
			shell.Escape(slotDir), shell.Escape(projectName))
		client.Run(stopCmd)
		return fmt.Errorf("healthcheck failed for %s slot: %w", targetSlot, err)
	}

	// Switch Caddy upstream to new slot
	if len(cfg.Caddy.Routes) > 0 {
		output.Step("Switching Caddy upstream to %s slot...", targetSlot)

		// Remove any conflicting caddy files for this app before writing ours
		canonicalName := fmt.Sprintf("50-%s.caddy", cfg.App)
		cleanCmd := fmt.Sprintf(
			`for f in %s/*%s*.caddy; do [ -f "$f" ] && [ "$(basename "$f")" != %s ] && echo "Removing conflicting: $f" && rm "$f"; done`,
			shell.Escape(cfg.Caddy.ConfDir), cfg.App, shell.Escape(canonicalName))
		cleanRes := client.Run(cleanCmd)
		if cleanRes.Stdout != "" {
			fmt.Print(cleanRes.Stdout)
		}

		caddyConf := generateBlueGreenCaddyConf(cfg, targetSlot)
		caddyPath := fmt.Sprintf("%s/%s", cfg.Caddy.ConfDir, canonicalName)
		res = client.UploadContent(caddyConf, caddyPath, 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload caddy config to %s: %s", host, res.Stderr)
		}
		output.Step("Reloading Caddy...")
		res = client.Run(cfg.Caddy.ReloadCommand)
		if res.Code != 0 {
			output.Attn("Caddy reload failed: %s", res.Stderr)
		}
	}

	// Drain old slot
	drainTimeout := cfg.Deploy.BlueGreen.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = 10
	}

	if currentState.Active != "" {
		output.Step("Draining old slot (%s) for %ds...", activeSlot, drainTimeout)
		time.Sleep(time.Duration(drainTimeout) * time.Second)

		oldSlotDir := runtime.BlueGreenDir(cfg.App, activeSlot, cfg.Deploy.Path)
		oldProjectName := fmt.Sprintf("%s-%s", cfg.App, activeSlot)
		output.Step("Stopping old slot (%s)...", activeSlot)
		stopCmd := fmt.Sprintf("cd %s && docker compose -p %s down",
			shell.Escape(oldSlotDir), shell.Escape(oldProjectName))
		res = client.Run(stopCmd)
		if res.Code != 0 {
			output.Attn("failed to stop old slot: %s", res.Stderr)
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

	statePath := runtime.StatePath(cfg.App, cfg.Deploy.Path)
	output.Step("Saving state to %s...", statePath)
	res = client.UploadContent(string(stateJSON)+"\n", statePath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload state to %s: %s", host, res.Stderr)
	}

	// Prune old images
	output.Step("Pruning old images...")
	client.Run("docker image prune -f")

	return nil
}

func oppositeSlot(slot string) string {
	if slot == "blue" {
		return "green"
	}
	return "blue"
}

func readRemoteState(client *ssh.Client, cfg *config.Config) blueGreenState {
	statePath := runtime.StatePath(cfg.App, cfg.Deploy.Path)
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
		output.Detail("Checking container %s...", containerName)

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
				output.OK("%s is healthy", containerName)
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
					output.OK("%s is running (no healthcheck defined)", containerName)
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

	caddyNetwork := cfg.Caddy.Network
	prod := &composeFile{
		Services: map[string]composeService{},
		Volumes:  map[string]any{},
		Networks: map[string]any{
			"default": map[string]any{
				"name": fmt.Sprintf("%s-%s-net", cfg.App, slot),
			},
			caddyNetwork: map[string]any{
				"external": true,
				"name":     caddyNetwork,
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
			Networks:      []string{"default", caddyNetwork},
			EnvFile:       []string{"../.env", "../.env.secrets"},
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
		if cfg.Caddy.TLSSnippet != "" {
			fmt.Fprintf(&b, "    import %s\n\n", cfg.Caddy.TLSSnippet)
		}
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

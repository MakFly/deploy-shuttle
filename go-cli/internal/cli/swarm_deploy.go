package cli

import (
	"encoding/json"
	"fmt"
	"os"
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

// swarmState represents the deployment state for Swarm deployments.
type swarmState struct {
	Version    string     `json:"version"`
	DeployedAt string     `json:"deployed_at"`
	Strategy   string     `json:"strategy"`
	Stack      string     `json:"stack"`
	Previous   *swarmPrev `json:"previous,omitempty"`
}

type swarmPrev struct {
	Version    string `json:"version"`
	DeployedAt string `json:"deployed_at"`
}

// swarmStackFile mirrors the Docker stack YAML structure for Swarm deploys.
type swarmStackFile struct {
	Version  string                   `yaml:"version"`
	Services map[string]swarmStackSvc `yaml:"services"`
	Networks map[string]any           `yaml:"networks,omitempty"`
	Volumes  map[string]any           `yaml:"volumes,omitempty"`
	Secrets  map[string]any           `yaml:"secrets,omitempty"`
}

type swarmStackSvc struct {
	Image       string            `yaml:"image"`
	Environment any               `yaml:"environment,omitempty"`
	EnvFile     any               `yaml:"env_file,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Expose      []string          `yaml:"expose,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	Command     any               `yaml:"command,omitempty"`
	DependsOn   any               `yaml:"depends_on,omitempty"`
	Deploy      *swarmDeploy      `yaml:"deploy,omitempty"`
	Healthcheck any               `yaml:"healthcheck,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Secrets     []any             `yaml:"secrets,omitempty"`
}

type swarmDeploy struct {
	Replicas       int                `yaml:"replicas"`
	UpdateConfig   *swarmUpdateConfig `yaml:"update_config,omitempty"`
	RollbackConfig *swarmUpdateConfig `yaml:"rollback_config,omitempty"`
	RestartPolicy  *swarmRestart      `yaml:"restart_policy,omitempty"`
	Labels         map[string]string  `yaml:"labels,omitempty"`
}

type swarmUpdateConfig struct {
	Parallelism   int    `yaml:"parallelism"`
	Delay         string `yaml:"delay"`
	Order         string `yaml:"order"`
	FailureAction string `yaml:"failure_action,omitempty"`
}

type swarmRestart struct {
	Condition   string `yaml:"condition"`
	Delay       string `yaml:"delay"`
	MaxAttempts int    `yaml:"max_attempts"`
}

func deploySwarm(cfg *config.Config, skipBuild bool, dryRun bool) error {
	totalStartedAt := time.Now()
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
	output.Detail("Strategy: swarm (docker stack deploy with rolling updates)")

	if err := runLocalHooks("pre-deploy", cfg.Deploy.Hooks.PreDeploy, dryRun); err != nil {
		return err
	}

	registryAddr := fmt.Sprintf("127.0.0.1:%d", registryPort)

	// Step 1: Build images locally
	if !skipBuild {
		if err := buildSwarmServiceImages(parsed, buildServices, registryAddr, cfg.App, dryRun); err != nil {
			return err
		}
	}

	version := detectVersion()
	webPort := detectWebPort(parsed, cfg)

	if dryRun {
		fmt.Println("\n[dry-run] Would start ephemeral registry, SSH tunnel, and deploy via Swarm")
		fmt.Printf("[dry-run] Version: %s\n", version)
		stackYAML := generateSwarmStackYAML(parsed, cfg, buildServices, registryAddr, webPort)
		fmt.Printf("\n[dry-run] Generated Swarm stack YAML:\n%s\n", stackYAML)
		if len(cfg.Caddy.Routes) > 0 {
			caddyConf := generateSwarmCaddyConf(cfg)
			fmt.Printf("[dry-run] Generated Caddy config:\n%s\n", caddyConf)
		}
		return nil
	}

	// Step 2: Start ephemeral registry
	fmt.Println()
	output.Step("Starting ephemeral registry on :%d...", registryPort)
	if err := startRegistry(); err != nil {
		return fmt.Errorf("start registry: %w", err)
	}
	registryRunning := true
	defer func() {
		if registryRunning {
			stopRegistry(cfg.Deploy.PruneBuildCache != "off")
		}
	}()

	// Step 3: Push independent service images concurrently. The registry
	// deduplicates shared blobs, while slow compression or disk writes no longer
	// serialize the whole deployment.
	pushStartedAt := time.Now()
	if err := pushServiceImages(buildServices, registryAddr, cfg.App); err != nil {
		return err
	}
	output.Detail("Timing push: %s", formatDeployDuration(time.Since(pushStartedAt)))

	// Step 4: Deploy to each server
	promotionStartedAt := time.Now()
	availability := startAvailabilityProbe(cfg.Deploy.AvailabilityURL, cfg.Deploy.AvailabilityIntervalMS)
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			if err := deploySwarmToHost(cfg, group, host, parsed, buildServices, registryAddr, version, webPort); err != nil {
				availability.Stop()
				return err
			}
		}
	}
	promotionDuration := time.Since(promotionStartedAt)
	promotionSLOMissed := false
	availabilityResult := availability.Stop()
	if cfg.Deploy.AvailabilityURL != "" {
		output.Detail("Availability: %d samples, %d failures", availabilityResult.Samples, availabilityResult.Failures)
		if availabilityResult.Failures > 0 {
			return fmt.Errorf("zero-downtime contract missed: %d/%d availability probes failed (first: %s)", availabilityResult.Failures, availabilityResult.Samples, availabilityResult.FirstFailure)
		}
		output.OK("Zero-downtime verified: %d/%d probes succeeded", availabilityResult.Samples, availabilityResult.Samples)
	}
	output.Detail("Timing promotion: %s", formatDeployDuration(promotionDuration))
	if cfg.Deploy.PromotionSLOSeconds > 0 {
		budget := time.Duration(cfg.Deploy.PromotionSLOSeconds) * time.Second
		if promotionDuration > budget {
			promotionSLOMissed = true
			output.Attn("Promotion SLO missed: %s > %s", formatDeployDuration(promotionDuration), formatDeployDuration(budget))
		} else {
			output.OK("Promotion SLO met: %s <= %s", formatDeployDuration(promotionDuration), formatDeployDuration(budget))
		}
	}

	if err := runLocalHooks("post-deploy", cfg.Deploy.Hooks.PostDeploy, dryRun); err != nil {
		return err
	}

	// Reclaim local disk: prune the Docker build cache used by this deploy.
	pruneLocalBuildCache(cfg)
	stopRegistry(cfg.Deploy.PruneBuildCache != "off")
	registryRunning = false

	fmt.Println()
	totalDuration := time.Since(totalStartedAt)
	output.Detail("Timing total: %s", formatDeployDuration(totalDuration))
	if promotionSLOMissed {
		return fmt.Errorf("promotion SLO missed: %s > %ds", formatDeployDuration(promotionDuration), cfg.Deploy.PromotionSLOSeconds)
	}
	if cfg.Deploy.TotalSLOSeconds > 0 {
		budget := time.Duration(cfg.Deploy.TotalSLOSeconds) * time.Second
		if totalDuration > budget {
			return fmt.Errorf("total deploy SLO missed: %s > %s", formatDeployDuration(totalDuration), formatDeployDuration(budget))
		}
		output.OK("Total deploy SLO met: %s <= %s", formatDeployDuration(totalDuration), formatDeployDuration(budget))
	}
	output.OK("Swarm deploy complete (rolling updates)")
	return nil
}

func buildSwarmServiceImages(parsed *composeFile, services []string, registryAddr string, app string, dryRun bool) error {
	if len(services) == 1 {
		service := services[0]
		phaseStartedAt := time.Now()
		imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, app, service)
		fmt.Println()
		output.Step("Building %s...", service)
		if err := buildServiceImage(parsed, service, imageTag, dryRun); err != nil {
			return fmt.Errorf("build %s: %w", service, err)
		}
		output.Detail("Timing build.%s: %s", service, formatDeployDuration(time.Since(phaseStartedAt)))
		return nil
	}

	fmt.Println()
	output.Step("Building %d services in parallel...", len(services))
	var wg sync.WaitGroup
	errs := make(chan error, len(services))

	for _, service := range services {
		service := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			phaseStartedAt := time.Now()
			imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, app, service)
			output.Step("Building %s...", service)
			if err := buildServiceImage(parsed, service, imageTag, dryRun); err != nil {
				errs <- fmt.Errorf("build %s: %w", service, err)
				return
			}
			output.Detail("Timing build.%s: %s", service, formatDeployDuration(time.Since(phaseStartedAt)))
		}()
	}

	wg.Wait()
	close(errs)
	if err, ok := <-errs; ok {
		return err
	}
	return nil
}

func pushServiceImages(services []string, registryAddr string, app string) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(services))

	for _, service := range services {
		service := service
		wg.Add(1)
		go func() {
			defer wg.Done()
			imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, app, service)
			output.Step("Pushing %s to local registry...", service)
			if err := dockerPush(imageTag); err != nil {
				errs <- fmt.Errorf("push %s: %w", service, err)
			}
		}()
	}

	wg.Wait()
	close(errs)
	if err, ok := <-errs; ok {
		return err
	}
	return nil
}

func formatDeployDuration(duration time.Duration) string {
	return duration.Round(10 * time.Millisecond).String()
}

func deploySwarmToHost(cfg *config.Config, group config.ServerGroup, host string, parsed *composeFile, buildServices []string, registryAddr string, version string, webPort string) error {
	client, err := connectSSH(group, host)
	if err != nil {
		return err
	}

	appDir := runtime.AppDir(cfg.App, cfg.Deploy.Path)
	fmt.Println()
	output.Step("Deploying (swarm) to %s@%s:%d (%s)...", group.User, host, group.Port, appDir)

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

	time.Sleep(2 * time.Second)

	// Create remote dir
	res := client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(appDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir on %s: %s", host, res.Stderr)
	}

	// Upload .env (config) + .env.secrets (secrets)
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil {
		output.Step("Uploading .env...")
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", envFile, err)
		}
		res = client.UploadContent(string(envContent), appDir+"/.env", 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload .env to %s: %s", host, res.Stderr)
		}
	}
	if _, err := os.Stat(".env.secrets"); err == nil {
		output.Step("Uploading .env.secrets (chmod 600)...")
		secretsContent, err := os.ReadFile(".env.secrets")
		if err != nil {
			return fmt.Errorf("read .env.secrets: %w", err)
		}
		res = client.UploadContent(string(secretsContent), appDir+"/.env.secrets", 0o600)
		if res.Code != 0 {
			return fmt.Errorf("upload .env.secrets to %s: %s", host, res.Stderr)
		}
	}
	for _, ref := range collectComposeEnvFiles(parsed) {
		if ref == ".env" || ref == ".env.secrets" {
			continue
		}
		if _, err := os.Stat(ref); err != nil {
			continue
		}
		mode := os.FileMode(0o644)
		if strings.Contains(ref, "secret") {
			mode = 0o600
		}
		output.Step("Uploading %s...", ref)
		content, err := os.ReadFile(ref)
		if err != nil {
			return fmt.Errorf("read %s: %w", ref, err)
		}
		res = client.UploadContent(string(content), appDir+"/"+ref, mode)
		if res.Code != 0 {
			return fmt.Errorf("upload %s to %s: %s", ref, host, res.Stderr)
		}
	}

	// Discover Docker Swarm secrets for this app
	var swarmSecrets []string
	secretPrefix := cfg.App + "_"
	lsCmd := fmt.Sprintf("docker secret ls --filter name=%s --format '{{.Name}}'", shell.Escape(secretPrefix))
	secretsRes := client.Run(lsCmd)
	if secretsRes.Code == 0 && strings.TrimSpace(secretsRes.Stdout) != "" {
		for _, line := range strings.Split(strings.TrimSpace(secretsRes.Stdout), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				swarmSecrets = append(swarmSecrets, line)
			}
		}
		if len(swarmSecrets) > 0 {
			output.Step("Found %d Docker Swarm secrets for %s", len(swarmSecrets), cfg.App)
		}
	}

	// Generate and upload Swarm stack YAML
	stackYAML := generateSwarmStackYAML(parsed, cfg, buildServices, registryAddr, webPort, swarmSecrets)
	stackPath := appDir + "/docker-stack.yml"
	output.Step("Uploading docker-stack.yml...")
	res = client.UploadContent(stackYAML, stackPath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload stack YAML to %s: %s", host, res.Stderr)
	}

	// Pull images via compose (uses the tunnel)
	output.Step("Pulling images on VPS...")
	pullCmd := fmt.Sprintf("cd %s && docker compose -f docker-stack.yml pull", shell.Escape(appDir))
	res = client.Run(pullCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose pull on %s: %s", host, res.Stderr)
	}

	// Deploy stack
	output.Step("Deploying stack %s...", cfg.App)
	deployCmd := fmt.Sprintf("docker stack deploy -c %s --with-registry-auth --detach %s",
		shell.Escape(stackPath), shell.Escape(cfg.App))
	res = client.Run(deployCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker stack deploy on %s: %s", host, res.Stderr)
	}
	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}

	// Wait for service convergence
	output.Step("Waiting for service convergence...")
	if err := waitForSwarmConvergence(client, cfg.App, cfg.Deploy.Timeout); err != nil {
		return fmt.Errorf("service convergence failed: %w", err)
	}

	// Caddy conf.d
	if len(cfg.Caddy.Routes) > 0 {
		output.Step("Generating Caddy config...")
		caddyConf := generateSwarmCaddyConf(cfg)
		caddyPath := fmt.Sprintf("%s/50-%s.caddy", cfg.Caddy.ConfDir, cfg.App)
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

	// Read current state to preserve previous info
	currentState := readRemoteSwarmState(client, cfg)

	// Save state.json
	newState := swarmState{
		Version:    version,
		DeployedAt: time.Now().UTC().Format(time.RFC3339),
		Strategy:   "swarm",
		Stack:      cfg.App,
	}
	if currentState.Version != "" {
		newState.Previous = &swarmPrev{
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

// waitForSwarmConvergence polls docker service ls until all services in the
// stack show their desired replica count (e.g. "1/1").
func waitForSwarmConvergence(client *ssh.Client, stack string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	// Initial delay to let Swarm start scheduling
	time.Sleep(5 * time.Second)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for stack %s to converge within %ds", stack, timeoutSec)
		}

		cmd := fmt.Sprintf(
			"docker service ls --filter name=%s --format '{{.Name}} {{.Replicas}}'",
			shell.Escape(stack),
		)
		res := client.Run(cmd)
		if res.Code != 0 {
			time.Sleep(3 * time.Second)
			continue
		}

		lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			time.Sleep(3 * time.Second)
			continue
		}

		allConverged := true
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				allConverged = false
				break
			}
			replicas := parts[1]
			// Replicas format is "1/1" or "0/1" etc.
			replicaParts := strings.Split(replicas, "/")
			if len(replicaParts) != 2 || replicaParts[0] != replicaParts[1] {
				allConverged = false
				fmt.Printf("   %s: %s\n", parts[0], replicas)
				break
			}
		}

		if allConverged {
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					fmt.Printf("   %s: %s (converged)\n", parts[0], parts[1])
				}
			}
			return nil
		}

		time.Sleep(3 * time.Second)
	}
}

func readRemoteSwarmState(client *ssh.Client, cfg *config.Config) swarmState {
	statePath := runtime.StatePath(cfg.App, cfg.Deploy.Path)
	res := client.Run(fmt.Sprintf("cat %s 2>/dev/null", shell.Escape(statePath)))
	if res.Code != 0 || strings.TrimSpace(res.Stdout) == "" {
		return swarmState{}
	}
	var state swarmState
	if err := json.Unmarshal([]byte(res.Stdout), &state); err != nil {
		return swarmState{}
	}
	return state
}

// generateSwarmStackYAML creates a Swarm-compatible stack YAML from the parsed
// compose file, rewriting build services to use the ephemeral registry images
// and adding Swarm deploy configuration for rolling updates.
func generateSwarmStackYAML(cf *composeFile, cfg *config.Config, buildServices []string, registryAddr string, webPort string, swarmSecrets ...[]string) string {
	buildSet := map[string]bool{}
	for _, s := range buildServices {
		buildSet[s] = true
	}

	replicas := cfg.Deploy.Swarm.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	caddyNetwork := cfg.Caddy.Network
	stack := &swarmStackFile{
		Version:  "3.8",
		Services: map[string]swarmStackSvc{},
		Networks: map[string]any{
			caddyNetwork: map[string]any{
				"external": true,
			},
			"default": map[string]any{
				"name": fmt.Sprintf("%s_network", cfg.App),
			},
		},
		Volumes: map[string]any{},
	}

	for name, svc := range cf.Services {
		// Skip caddy services — VPS has centralized Caddy
		if svc.Image != "" && strings.Contains(svc.Image, "caddy") {
			continue
		}
		if name == "caddy" {
			continue
		}

		stackSvc := swarmStackSvc{
			Environment: svc.Environment,
			Volumes:     svc.Volumes,
			Expose:      svc.Expose,
			Command:     svc.Command,
			Healthcheck: svc.Healthcheck,
			Networks:    []string{caddyNetwork, "default"},
			EnvFile:     normalizeEnvFiles(svc.EnvFile),
		}

		if buildSet[name] {
			stackSvc.Image = fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, name)

			// Add Swarm deploy config for build services
			stackSvc.Deploy = &swarmDeploy{
				Replicas: replicas,
				UpdateConfig: &swarmUpdateConfig{
					Parallelism:   1,
					Delay:         "5s",
					Order:         "start-first",
					FailureAction: "rollback",
				},
				RollbackConfig: &swarmUpdateConfig{
					Parallelism: 1,
					Delay:       "5s",
					Order:       "stop-first",
				},
				RestartPolicy: &swarmRestart{
					Condition:   "on-failure",
					Delay:       "5s",
					MaxAttempts: 3,
				},
				Labels: map[string]string{
					"shuttle.app": cfg.App,
				},
			}

			// Healthcheck precedence: an explicit compose healthcheck wins;
			// otherwise a shuttle.yml services.<name>.healthcheck; otherwise a
			// generic default for the web service only. The default probes the
			// exposed port over HTTP via wget/curl (no PHP assumption), so
			// non-PHP stacks and custom-named services are no longer forced onto
			// a broken check.
			if stackSvc.Healthcheck == nil {
				if hc := healthcheckFromConfig(cfg.Services[name], webPort); hc != nil {
					stackSvc.Healthcheck = hc
				} else if name == "web" && webPort != "" {
					stackSvc.Healthcheck = defaultWebHealthcheck(webPort)
				}
			}
		} else {
			// Non-build services (databases, redis, etc.) keep their original image
			stackSvc.Image = svc.Image
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
				stackSvc.DependsOn = filtered
			}
		}

		stackSvc.Labels = map[string]string{
			"shuttle.app": cfg.App,
		}

		stack.Services[name] = stackSvc
	}

	// Carry over volumes (except caddy ones)
	if cf.Volumes != nil {
		for name, v := range cf.Volumes {
			if strings.Contains(name, "caddy") {
				continue
			}
			stack.Volumes[name] = v
		}
	}

	// Add Docker Swarm native secrets if available
	var secretNames []string
	if len(swarmSecrets) > 0 {
		secretNames = swarmSecrets[0]
	}
	if len(secretNames) > 0 {
		stack.Secrets = map[string]any{}
		secretPrefix := cfg.App + "_"
		for _, secretName := range secretNames {
			// Register as external at top level
			stack.Secrets[secretName] = map[string]any{"external": true}
		}
		// Add secrets to build services (web service and other app services)
		for name, svc := range stack.Services {
			if !buildSet[name] {
				continue
			}
			var svcSecrets []any
			for _, secretName := range secretNames {
				// Derive the target filename from the secret name:
				// remove "<app>_" prefix and uppercase it
				envKey := strings.ToUpper(strings.TrimPrefix(secretName, secretPrefix))
				svcSecrets = append(svcSecrets, map[string]any{
					"source": secretName,
					"target": "/run/secrets/" + envKey,
					"mode":   0o444,
				})
			}
			svc.Secrets = svcSecrets
			stack.Services[name] = svc
		}
	}

	out, _ := yaml.Marshal(stack)

	header := fmt.Sprintf("# Generated by shuttle (swarm strategy)\n# App: %s\n# Generated: %s\n\n",
		cfg.App, time.Now().UTC().Format(time.RFC3339))

	return header + string(out)
}

func collectComposeEnvFiles(cf *composeFile) []string {
	seen := map[string]bool{}
	var refs []string
	for _, svc := range cf.Services {
		for _, ref := range normalizeEnvFiles(svc.EnvFile) {
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs
}

func normalizeEnvFiles(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		files := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				files = append(files, s)
			}
		}
		return files
	default:
		return nil
	}
}

// generateSwarmCaddyConf generates a Caddy config for Swarm deployments.
// In Swarm, the service DNS name is <stack>_<service>, so for stack "myapp"
// and service "web", the DNS name is "myapp_web".
func generateSwarmCaddyConf(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Generated by shuttle for %s (swarm)\n", cfg.App))
	b.WriteString("# Do not edit manually — regenerated on each deploy\n\n")

	for domain, upstream := range cfg.Caddy.Routes {
		swarmUpstream := rewriteUpstreamForSwarm(upstream, cfg.App)

		fmt.Fprintf(&b, "%s {\n", domain)
		if cfg.Caddy.TLSSnippet != "" {
			fmt.Fprintf(&b, "    import %s\n\n", cfg.Caddy.TLSSnippet)
		}
		writeCaddySecurityHeaders(&b)
		writeCaddyAccess(&b, cfg, swarmUpstream)
		b.WriteString("}\n\n")
	}

	return b.String()
}

// rewriteUpstreamForSwarm rewrites the upstream target to use Swarm service DNS.
// In Swarm, services are addressed as <stack>_<service>:<port>.
// Input examples: "web:3000", "myapp-web:3000", "http://web:3000"
// Output: "myapp_web:3000"
func rewriteUpstreamForSwarm(upstream string, app string) string {
	proto := ""
	host := upstream
	if strings.HasPrefix(upstream, "http://") {
		proto = "http://"
		host = strings.TrimPrefix(upstream, "http://")
	} else if strings.HasPrefix(upstream, "https://") {
		proto = "https://"
		host = strings.TrimPrefix(upstream, "https://")
	}

	hostPart := host
	portPart := ""
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		hostPart = host[:idx]
		portPart = host[idx:]
	}

	serviceName := hostPart
	if strings.HasPrefix(hostPart, app+"-") {
		serviceName = strings.TrimPrefix(hostPart, app+"-")
	}

	newHost := fmt.Sprintf("%s_%s", app, serviceName)
	return proto + newHost + portPart
}

// detectWebPort extracts the port for the web service from the parsed compose
// file, falling back to config Services["web"].Port, then 3000.
func detectWebPort(parsed *composeFile, cfg *config.Config) string {
	for name, svc := range parsed.Services {
		if name != "web" {
			continue
		}
		if len(svc.Expose) > 0 {
			return svc.Expose[0]
		}
		if len(svc.Ports) > 0 {
			portStr := svc.Ports[0]
			if idx := strings.LastIndex(portStr, ":"); idx >= 0 {
				return portStr[idx+1:]
			}
			return portStr
		}
	}

	if webSvc, ok := cfg.Services["web"]; ok && webSvc.Port != 0 {
		return fmt.Sprintf("%d", webSvc.Port)
	}

	return "3000"
}

// healthcheckFromConfig builds a Swarm healthcheck from a shuttle.yml
// services.<name>.healthcheck block. Returns nil when nothing is configured, so
// callers can fall back to a default. Supported: an explicit shell `command`, an
// `http` probe on `path` (using the service port or the detected web port), or
// `type: none` to disable the check.
func healthcheckFromConfig(svc config.Service, defaultPort string) map[string]any {
	hc := svc.Healthcheck
	if hc.Type == "" && hc.Command == "" && hc.Path == "" {
		return nil
	}

	var test []string
	switch {
	case strings.EqualFold(hc.Type, "none"):
		return map[string]any{"test": []string{"NONE"}}
	case hc.Command != "":
		test = []string{"CMD-SHELL", hc.Command}
	case hc.Path != "" || strings.EqualFold(hc.Type, "http"):
		port := defaultPort
		if svc.Port != 0 {
			port = fmt.Sprintf("%d", svc.Port)
		}
		path := hc.Path
		if path == "" {
			path = "/"
		}
		test = []string{"CMD-SHELL", httpProbe(port, path)}
	default:
		return nil
	}

	out := map[string]any{"test": test, "start_period": "30s"}
	if hc.Interval > 0 {
		out["interval"] = fmt.Sprintf("%ds", hc.Interval)
	}
	if hc.Timeout > 0 {
		out["timeout"] = fmt.Sprintf("%ds", hc.Timeout)
	}
	if hc.Retries > 0 {
		out["retries"] = hc.Retries
	}
	return out
}

// defaultWebHealthcheck is the generic default for the `web` service: a plain
// HTTP GET on the exposed port, trying wget then curl so it works on any base
// image (no PHP assumption). Overridden by an explicit compose or shuttle.yml
// healthcheck.
func defaultWebHealthcheck(port string) map[string]any {
	return map[string]any{
		"test":         []string{"CMD-SHELL", httpProbe(port, "/")},
		"interval":     "10s",
		"timeout":      "5s",
		"start_period": "30s",
		"retries":      3,
	}
}

// httpProbe returns a shell command that succeeds when an HTTP GET on the given
// port/path returns, trying wget (busybox or GNU) then curl — the two clients
// present on the vast majority of images.
func httpProbe(port, path string) string {
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, path)
	return fmt.Sprintf("wget -q -O /dev/null %s 2>/dev/null || curl -fsS %s >/dev/null 2>&1", url, url)
}

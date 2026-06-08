package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
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
	fmt.Println("Strategy: swarm (docker stack deploy with rolling updates)")

	if err := runLocalHooks("pre-deploy", cfg.Deploy.Hooks.PreDeploy, dryRun); err != nil {
		return err
	}

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
			if err := deploySwarmToHost(cfg, group, host, parsed, buildServices, registryAddr, version, webPort); err != nil {
				return err
			}
		}
	}

	if err := runLocalHooks("post-deploy", cfg.Deploy.Hooks.PostDeploy, dryRun); err != nil {
		return err
	}

	fmt.Println("\n-> Swarm deploy complete (rolling updates)")
	return nil
}

func deploySwarmToHost(cfg *config.Config, group config.ServerGroup, host string, parsed *composeFile, buildServices []string, registryAddr string, version string, webPort string) error {
	client, err := connectSSH(group, host)
	if err != nil {
		return err
	}

	appDir := runtime.AppDir(cfg.App)
	fmt.Printf("\n-> Deploying (swarm) to %s@%s:%d (%s)...\n", group.User, host, group.Port, appDir)

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
		fmt.Println("-> Uploading .env...")
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
		fmt.Println("-> Uploading .env.secrets (chmod 600)...")
		secretsContent, err := os.ReadFile(".env.secrets")
		if err != nil {
			return fmt.Errorf("read .env.secrets: %w", err)
		}
		res = client.UploadContent(string(secretsContent), appDir+"/.env.secrets", 0o600)
		if res.Code != 0 {
			return fmt.Errorf("upload .env.secrets to %s: %s", host, res.Stderr)
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
			fmt.Printf("-> Found %d Docker Swarm secrets for %s\n", len(swarmSecrets), cfg.App)
		}
	}

	// Generate and upload Swarm stack YAML
	stackYAML := generateSwarmStackYAML(parsed, cfg, buildServices, registryAddr, webPort, swarmSecrets)
	stackPath := appDir + "/docker-stack.yml"
	fmt.Println("-> Uploading docker-stack.yml...")
	res = client.UploadContent(stackYAML, stackPath, 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload stack YAML to %s: %s", host, res.Stderr)
	}

	// Pull images via compose (uses the tunnel)
	fmt.Println("-> Pulling images on VPS...")
	pullCmd := fmt.Sprintf("cd %s && docker compose -f docker-stack.yml pull", shell.Escape(appDir))
	res = client.Run(pullCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose pull on %s: %s", host, res.Stderr)
	}

	// Deploy stack
	fmt.Printf("-> Deploying stack %s...\n", cfg.App)
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
	fmt.Println("-> Waiting for service convergence...")
	if err := waitForSwarmConvergence(client, cfg.App, cfg.Deploy.Timeout); err != nil {
		return fmt.Errorf("service convergence failed: %w", err)
	}

	// Caddy conf.d
	if len(cfg.Caddy.Routes) > 0 {
		fmt.Println("-> Generating Caddy config...")
		caddyConf := generateSwarmCaddyConf(cfg)
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

	// Read current state to preserve previous info
	currentState := readRemoteSwarmState(client, cfg.App)

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

func readRemoteSwarmState(client *ssh.Client, app string) swarmState {
	statePath := runtime.StatePath(app)
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

	stack := &swarmStackFile{
		Version:  "3.8",
		Services: map[string]swarmStackSvc{},
		Networks: map[string]any{
			"caddy_network": map[string]any{
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
			Networks:    []string{"caddy_network", "default"},
			EnvFile:     []string{".env", ".env.secrets"},
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

			// Add default healthcheck if none present and this is the web service
			if svc.Healthcheck == nil && webPort != "" {
				stackSvc.Healthcheck = map[string]any{
					"test":         []string{"CMD", "php", "-r", fmt.Sprintf(`exit(false === @file_get_contents("http://127.0.0.1:%s/", context: stream_context_create(["http" => ["timeout" => 3]])) ? 1 : 0);`, webPort)},
					"interval":     "10s",
					"timeout":      "5s",
					"start_period": "30s",
					"retries":      3,
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
		b.WriteString("    header {\n")
		b.WriteString("        X-Content-Type-Options \"nosniff\"\n")
		b.WriteString("        X-Frame-Options \"DENY\"\n")
		b.WriteString("        Referrer-Policy \"strict-origin-when-cross-origin\"\n")
		b.WriteString("        Strict-Transport-Security \"max-age=31536000; includeSubDomains\"\n")
		b.WriteString("        -Server\n")
		b.WriteString("    }\n\n")
		fmt.Fprintf(&b, "    reverse_proxy %s {\n", swarmUpstream)
		b.WriteString("        header_up Host {host}\n")
		b.WriteString("        header_up X-Real-IP {remote}\n")
		b.WriteString("    }\n")
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

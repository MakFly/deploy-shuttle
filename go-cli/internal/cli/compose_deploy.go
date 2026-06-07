package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/config"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/runtime"
	"github.com/MakFly/deploy-shuttle/go-cli/internal/shell"
	"gopkg.in/yaml.v3"
)

const registryPort = 5080
const registryContainer = "shuttle-registry"

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]any            `yaml:"volumes,omitempty"`
	Networks map[string]any            `yaml:"networks,omitempty"`
}

type composeService struct {
	Build         any               `yaml:"build,omitempty"`
	Image         string            `yaml:"image,omitempty"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Environment   any               `yaml:"environment,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Expose        []string          `yaml:"expose,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	DependsOn     any               `yaml:"depends_on,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	EnvFile       any               `yaml:"env_file,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	Command       any               `yaml:"command,omitempty"`
	Healthcheck   any               `yaml:"healthcheck,omitempty"`
}

func deployCompose(cfg *config.Config, skipBuild bool, dryRun bool) error {
	// Load .env file into process environment for build-arg expansion
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	loadDotenv(envFile)

	composeFiles := cfg.Deploy.ComposeFiles
	if len(composeFiles) == 0 {
		composeFiles = []string{"docker-compose.yml"}
	}

	composePath := composeFiles[0]
	parsed, err := parseComposeFile(composePath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", composeFiles[0], err)
	}

	buildServices := findBuildServices(parsed)
	if len(buildServices) == 0 {
		return fmt.Errorf("no services with build: found in %s", composeFiles[0])
	}

	fmt.Printf("Found %d services to build: %s\n", len(buildServices), strings.Join(buildServices, ", "))

	registryAddr := fmt.Sprintf("127.0.0.1:%d", registryPort)

	// Step 1: Build images locally
	if !skipBuild {
		for _, svc := range buildServices {
			imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, svc)
			fmt.Printf("\n→ Building %s...\n", svc)
			if err := buildServiceImage(parsed, svc, imageTag, dryRun); err != nil {
				return fmt.Errorf("build %s: %w", svc, err)
			}
		}
	}

	if dryRun {
		fmt.Println("\n[dry-run] Would start ephemeral registry, SSH tunnel, and deploy")
		prodCompose := generateProdCompose(parsed, cfg, buildServices, registryAddr)
		fmt.Printf("\n[dry-run] Generated production compose:\n%s\n", prodCompose)
		return nil
	}

	// Step 2: Start ephemeral registry
	fmt.Printf("\n→ Starting ephemeral registry on :%d...\n", registryPort)
	if err := startRegistry(); err != nil {
		return fmt.Errorf("start registry: %w", err)
	}
	defer stopRegistry()

	// Step 3: Push images to local registry
	for _, svc := range buildServices {
		imageTag := fmt.Sprintf("%s/%s-%s:latest", registryAddr, cfg.App, svc)
		fmt.Printf("→ Pushing %s to local registry...\n", svc)
		if err := dockerPush(imageTag); err != nil {
			return fmt.Errorf("push %s: %w", svc, err)
		}
	}

	// Step 4-8: Deploy to each server
	for _, group := range cfg.Servers {
		for _, host := range group.Hosts {
			if err := deployComposeToHost(cfg, group, host, parsed, buildServices, registryAddr, composeFiles[0]); err != nil {
				return err
			}
		}
	}

	fmt.Println("\n✓ Deploy complete")
	return nil
}

func deployComposeToHost(cfg *config.Config, group config.ServerGroup, host string, parsed *composeFile, buildServices []string, registryAddr string, composeFilePath string) error {
	client, err := connectSSH(group, host)
	if err != nil {
		return err
	}

	remoteDir := runtime.AppDir(cfg.App)
	fmt.Printf("\n→ Deploying to %s@%s:%d (%s)...\n", group.User, host, group.Port, remoteDir)

	// Step 4: SSH reverse tunnel
	fmt.Println("→ Opening SSH tunnel...")
	tunnel, err := startSSHTunnel(host, group.User, group.Port, registryPort)
	if err != nil {
		return fmt.Errorf("SSH tunnel: %w", err)
	}
	defer func() {
		fmt.Println("→ Closing SSH tunnel...")
		tunnel.Process.Kill()
		tunnel.Wait()
	}()

	time.Sleep(2 * time.Second)

	// Step 5: Create remote dir
	res := client.Run(fmt.Sprintf("mkdir -p %s", shell.Escape(remoteDir)))
	if res.Code != 0 {
		return fmt.Errorf("mkdir on %s: %s", host, res.Stderr)
	}

	// Step 6: Upload .env
	envFile := cfg.Deploy.EnvFile
	if envFile == "" {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil {
		fmt.Printf("→ Uploading %s...\n", envFile)
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", envFile, err)
		}
		res := client.UploadContent(string(envContent), remoteDir+"/.env", 0o600)
		if res.Code != 0 {
			return fmt.Errorf("upload .env to %s: %s", host, res.Stderr)
		}
	}

	// Step 7: Generate + upload production compose
	prodCompose := generateProdCompose(parsed, cfg, buildServices, registryAddr)
	fmt.Println("→ Uploading docker-compose.yml...")
	res = client.UploadContent(prodCompose, remoteDir+"/docker-compose.yml", 0o644)
	if res.Code != 0 {
		return fmt.Errorf("upload compose to %s: %s", host, res.Stderr)
	}

	// Step 8: Pull + up
	fmt.Println("→ Pulling images on VPS...")
	composeCmd := fmt.Sprintf("cd %s && docker compose pull", shell.Escape(remoteDir))
	res = client.Run(composeCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose pull on %s: %s", host, res.Stderr)
	}

	fmt.Println("→ Starting services...")
	composeCmd = fmt.Sprintf("cd %s && docker compose up -d --remove-orphans", shell.Escape(remoteDir))
	res = client.Run(composeCmd)
	if res.Code != 0 {
		return fmt.Errorf("docker compose up on %s: %s", host, res.Stderr)
	}
	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}

	// Step 9: Caddy conf.d
	if len(cfg.Caddy.Routes) > 0 {
		fmt.Println("→ Generating Caddy config...")
		caddyConf := generateCaddyConf(cfg)
		caddyPath := fmt.Sprintf("%s/50-%s.caddy", cfg.Caddy.ConfDir, cfg.App)
		res = client.UploadContent(caddyConf, caddyPath, 0o644)
		if res.Code != 0 {
			return fmt.Errorf("upload caddy config to %s: %s", host, res.Stderr)
		}
		fmt.Println("→ Reloading Caddy...")
		res = client.Run(cfg.Caddy.ReloadCommand)
		if res.Code != 0 {
			fmt.Printf("⚠ Caddy reload failed: %s\n", res.Stderr)
		}
	}

	// Prune old images
	fmt.Println("→ Pruning old images...")
	client.Run("docker image prune -f")

	return nil
}

func parseComposeFile(path string) (*composeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

func findBuildServices(cf *composeFile) []string {
	var services []string
	for name, svc := range cf.Services {
		if svc.Build != nil {
			services = append(services, name)
		}
	}
	return services
}

func buildServiceImage(cf *composeFile, serviceName string, imageTag string, dryRun bool) error {
	svc := cf.Services[serviceName]

	var dockerfile, context string
	var buildArgs []string

	switch b := svc.Build.(type) {
	case string:
		context = b
		dockerfile = "Dockerfile"
	case map[string]any:
		if v, ok := b["context"].(string); ok {
			context = v
		} else {
			context = "."
		}
		if v, ok := b["dockerfile"].(string); ok {
			dockerfile = v
		} else {
			dockerfile = "Dockerfile"
		}
		if args, ok := b["args"]; ok {
			switch a := args.(type) {
			case map[string]any:
				for k, v := range a {
					val := fmt.Sprintf("%v", v)
					expanded := expandShellVar(val)
					buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, expanded))
				}
			}
		}
	default:
		return fmt.Errorf("unsupported build format for service %s", serviceName)
	}

	cmdArgs := []string{"build", "-t", imageTag, "-f", dockerfile}
	cmdArgs = append(cmdArgs, buildArgs...)
	cmdArgs = append(cmdArgs, context)

	if dryRun {
		fmt.Printf("[dry-run] docker %s\n", strings.Join(cmdArgs, " "))
		return nil
	}

	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startRegistry() error {
	exec.Command("docker", "rm", "-f", registryContainer).Run()
	cmd := exec.Command("docker", "run", "-d",
		"--name", registryContainer,
		"-p", fmt.Sprintf("127.0.0.1:%d:5000", registryPort),
		"registry:2",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return waitForRegistry()
}

func waitForRegistry() error {
	url := fmt.Sprintf("http://127.0.0.1:%d/v2/", registryPort)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var lastErr error
	for range 40 {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
			lastErr = fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("registry did not become ready: %w", lastErr)
}

func stopRegistry() {
	fmt.Println("→ Stopping ephemeral registry...")
	exec.Command("docker", "rm", "-f", registryContainer).Run()
}

func dockerPush(imageTag string) error {
	cmd := exec.Command("docker", "push", imageTag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startSSHTunnel(host string, user string, sshPort int, tunnelPort int) (*exec.Cmd, error) {
	target := host
	if user != "" {
		target = fmt.Sprintf("%s@%s", user, host)
	}
	cmd := exec.Command("ssh",
		"-p", strconv.Itoa(sshPort),
		"-N",
		"-R", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", tunnelPort, tunnelPort),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ExitOnForwardFailure=yes",
		target,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func generateProdCompose(cf *composeFile, cfg *config.Config, buildServices []string, registryAddr string) string {
	buildSet := map[string]bool{}
	for _, s := range buildServices {
		buildSet[s] = true
	}

	prod := &composeFile{
		Services: map[string]composeService{},
		Volumes:  map[string]any{},
		Networks: map[string]any{
			"caddy_network": map[string]any{
				"external": true,
				"name":     "caddy_network",
			},
		},
	}

	for name, svc := range cf.Services {
		// Skip the project's own caddy service — VPS has centralized Caddy
		if svc.Image != "" && strings.Contains(svc.Image, "caddy") {
			continue
		}
		if name == "caddy" {
			continue
		}

		prodSvc := composeService{
			Environment: svc.Environment,
			Volumes:     svc.Volumes,
			Expose:      svc.Expose,
			Restart:     svc.Restart,
			Command:     svc.Command,
			Healthcheck: svc.Healthcheck,
			Networks:    []string{"caddy_network"},
			EnvFile:     ".env",
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

		// Container name for Caddy routing
		prodSvc.Labels = map[string]string{
			"shuttle.app": cfg.App,
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

	header := fmt.Sprintf("# Generated by shuttle — do not edit manually\n# App: %s\n# Generated: %s\n\n", cfg.App, time.Now().UTC().Format(time.RFC3339))

	return header + string(out)
}

// expandShellVar handles ${VAR:-default}, ${VAR:?error}, ${VAR} and $VAR patterns.
func expandShellVar(val string) string {
	if !strings.Contains(val, "$") {
		return val
	}
	// Extract variable name from patterns like ${VAR:-default} or ${VAR:?msg}
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		inner := val[2 : len(val)-1]
		var varName, defaultVal string
		if idx := strings.Index(inner, ":-"); idx >= 0 {
			varName = inner[:idx]
			defaultVal = inner[idx+2:]
		} else if idx := strings.Index(inner, ":?"); idx >= 0 {
			varName = inner[:idx]
		} else {
			varName = inner
		}
		if v := os.Getenv(varName); v != "" {
			return v
		}
		return defaultVal
	}
	return os.ExpandEnv(val)
}

func loadDotenv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func generateCaddyConf(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Generated by shuttle for %s\n", cfg.App))
	b.WriteString(fmt.Sprintf("# Do not edit manually — regenerated on each deploy\n\n"))

	for domain, upstream := range cfg.Caddy.Routes {
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
		fmt.Fprintf(&b, "    reverse_proxy %s {\n", upstream)
		b.WriteString("        header_up Host {host}\n")
		b.WriteString("        header_up X-Real-IP {remote}\n")
		b.WriteString("    }\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}

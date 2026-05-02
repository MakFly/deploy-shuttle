package readiness

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

type Check func(execx.Adapter) CheckResult

func Run(adapter execx.Adapter, target string, profile []string) Report {
	return RunWithConfig(adapter, target, profile, EmptyConfig(), "")
}

func RunWithConfig(adapter execx.Adapter, target string, profile []string, cfg Config, configPath string) Report {
	if target == "" {
		target = "local"
	}
	if len(profile) == 0 {
		profile = cfg.Checks.Profile
	}
	if len(profile) == 0 {
		profile = []string{"docker", "caddy"}
	}

	checks := []Check{
		checkOSSupported,
		checkDiskSpace,
		checkUpdatesPending,
		checkMemoryLow,
		checkDockerInstalled,
		checkDockerServiceEnabled,
		checkDockerRestartPolicies,
		checkDockerHealthchecks,
		checkDockerRunningAsRoot,
		checkDockerSockExposed,
		checkDockerPublishedSensitivePorts,
		checkUFWActive,
		checkDatabasePorts,
		checkEnvWorldReadable,
		checkEnvTracked,
		checkSecretsWeakPermissions,
		checkCaddyInstalled,
		checkCaddyAdminExposed,
		checkCaddySecurityHeaders,
		checkCaddyConfigValid,
		checkAdminerRestricted,
		checkSSHRootLogin,
		checkSSHPasswordAuth,
		checkSSHPortDefault,
		checkUnattendedUpgrades,
		checkFail2ban,
		checkSwap,
		checkTimeSync,
		checkTLSCertificate(cfg.App.Domain),
		checkHSTSHeader(cfg.App.Domain),
		checkDNSPointsToServer(cfg.App.Domain),
		checkHealthEndpoint(cfg.App.Domain, cfg.App.HealthcheckPath),
		checkDatabaseBackup,
	}
	results := make([]CheckResult, 0, len(checks))
	for _, check := range checks {
		results = append(results, applyConfig(check(adapter), cfg))
	}
	composeSnap := loadCompose(adapter)
	results = append(results,
		applyConfig(checkComposeMissingProdFile(composeSnap), cfg),
		applyConfig(checkComposeEnvFileMissing(adapter, composeSnap), cfg),
		applyConfig(checkComposeLatestTag(composeSnap), cfg),
		applyConfig(checkComposeNoResourceLimits(composeSnap), cfg),
		applyConfig(checkComposeBindMountSensitivePaths(composeSnap), cfg),
	)
	var cfClient *CloudflareClient
	if cfg.Cloudflare.Enabled {
		if token := ResolveCloudflareToken(cfg.Cloudflare); token != "" {
			cfClient = newCloudflareClient(token)
		}
	}
	for _, check := range cloudflareChecks(cfg.Cloudflare, cfg.App.Domain, cfClient) {
		results = append(results, applyConfig(check(adapter), cfg))
	}
	score := Score(results)
	return Report{
		Target:      target,
		Profile:     profile,
		ConfigPath:  configPath,
		Score:       score,
		Level:       ReadinessLevel(score),
		Checks:      results,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func Console(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DeployShuttle Doctor Report\n\n")
	fmt.Fprintf(&b, "Target: %s\n", report.Target)
	fmt.Fprintf(&b, "Profile: %s\n", strings.Join(report.Profile, ", "))
	fmt.Fprintf(&b, "Score: %d/100 - %s\n", report.Score, LevelLabel(report.Level))
	fmt.Fprintf(&b, "Generated: %s\n\n", report.GeneratedAt)
	for _, severity := range []Severity{Critical, High, Medium, Low, Info} {
		var group []CheckResult
		for _, check := range report.Checks {
			if check.Severity == severity {
				group = append(group, check)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s:\n", strings.Title(string(severity)))
		for _, check := range group {
			marker := "[?]"
			if check.Status == Passed {
				marker = "[ok]"
			} else if check.Status == Failed {
				marker = "[x]"
			}
			fmt.Fprintf(&b, "  %s %s - %s\n", marker, check.Title, check.Summary)
			if check.Ignored {
				fmt.Fprintf(&b, "      Ignored: %s\n", check.IgnoreReason)
			}
			if check.Status == Failed && check.Remediation != "" {
				fmt.Fprintf(&b, "      Fix: %s\n", check.Remediation)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func applyConfig(result CheckResult, cfg Config) CheckResult {
	if listContains(cfg.Checks.Ignore, result.ID) {
		return markIgnored(result, "ignored by checks.ignore")
	}
	switch result.ID {
	case "docker.sock_exposed":
		return ignoreAllowedWorkloads(result, cfg.Docker.AllowDockerSocket, "allowed by docker.allowDockerSocket")
	case "docker.containers_running_as_root":
		return ignoreAllowedWorkloads(result, cfg.Docker.AllowRoot, "allowed by docker.allowRoot")
	case "docker.containers_without_healthcheck":
		return ignoreAllowedWorkloads(result, cfg.Docker.WorkerServices, "allowed by docker.workerServices")
	default:
		return result
	}
}

func ignoreAllowedWorkloads(result CheckResult, allow []string, reason string) CheckResult {
	workloads, ok := evidenceStringSlice(result.Evidence["workloads"])
	if !ok || len(workloads) == 0 {
		return result
	}
	remaining := []string{}
	ignored := []string{}
	for _, workload := range workloads {
		if matchesAnyWorkload(workload, allow) {
			ignored = append(ignored, workload)
			continue
		}
		remaining = append(remaining, workload)
	}
	if len(ignored) == 0 {
		return result
	}
	if result.Evidence == nil {
		result.Evidence = map[string]any{}
	}
	result.Evidence["workloads"] = remaining
	result.Evidence["ignoredWorkloads"] = ignored
	if len(remaining) == 0 {
		return markIgnored(result, reason)
	}
	result.Summary = strings.Replace(result.Summary, fmt.Sprintf("%d Docker workload(s)", len(workloads)), fmt.Sprintf("%d Docker workload(s)", len(remaining)), 1)
	return result
}

func markIgnored(result CheckResult, reason string) CheckResult {
	result.Ignored = true
	result.IgnoreReason = reason
	result.Status = Skipped
	if result.Evidence == nil {
		result.Evidence = map[string]any{}
	}
	result.Evidence["ignored"] = true
	return result
}

func JSON(report Report) string {
	raw, _ := json.MarshalIndent(report, "", "  ")
	return string(raw)
}

func checkOSSupported(adapter execx.Adapter) CheckResult {
	res := adapter.Run("cat /etc/os-release", 3*time.Second)
	out := res.Stdout
	id := capture(out, `(?m)^ID="?([^"\n]+)"?`)
	version := capture(out, `(?m)^VERSION_ID="?([^"\n]+)"?`)
	supported := (id == "ubuntu" && (version == "22.04" || version == "24.04")) || (id == "debian" && version == "12")
	status := Passed
	if !supported {
		status = Failed
	}
	if res.ExitCode != 0 {
		status = Unknown
	}
	return CheckResult{ID: "system.os_supported", Title: "Operating system is supported", Category: "system", Severity: High, Status: status, Summary: fmt.Sprintf("%s %s", fallback(id, "unknown"), fallback(version, "unknown")), Remediation: "Use Ubuntu 22.04, Ubuntu 24.04, or Debian 12 for MVP support.", Evidence: map[string]any{"id": id, "version": version}}
}

func checkDiskSpace(adapter execx.Adapter) CheckResult {
	res := adapter.Run("df -Pk / | awk 'NR==2 {print $5}'", 3*time.Second)
	usage, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(res.Stdout), "%"))
	status := Passed
	severity := Medium
	if usage >= 90 {
		status = Failed
		severity = Critical
	} else if usage >= 80 {
		status = Failed
	}
	return CheckResult{ID: "system.disk_space_low", Title: "Disk space has enough free capacity", Category: "system", Severity: severity, Status: status, Summary: fmt.Sprintf("Root filesystem usage is %d%%.", usage), Remediation: "Free disk space, prune unused Docker resources, or increase VPS disk size.", Evidence: map[string]any{"usagePercent": usage}}
}

func checkDockerInstalled(adapter execx.Adapter) CheckResult {
	res := adapter.Run("command -v docker && docker --version", 3*time.Second)
	status := Passed
	summary := strings.TrimSpace(res.Stdout)
	if res.ExitCode != 0 {
		status = Failed
		summary = "Docker is not installed or not available in PATH."
	}
	return CheckResult{ID: "docker.not_installed", Title: "Docker is installed", Category: "docker", Severity: High, Status: status, Summary: summary, Remediation: "Install Docker Engine before running production Docker workloads."}
}

func checkDockerServiceEnabled(adapter execx.Adapter) CheckResult {
	res := adapter.Run("systemctl is-enabled docker 2>/dev/null; systemctl is-active docker 2>/dev/null", 3*time.Second)
	lines := nonEmptyLines(res.Stdout)
	enabled := len(lines) > 0 && lines[0] == "enabled"
	active := len(lines) > 1 && lines[1] == "active"
	status := Passed
	if !enabled || !active {
		status = Failed
	}
	return CheckResult{
		ID:          "docker.service_not_enabled",
		Title:       "Docker service is enabled and active",
		Category:    "docker",
		Severity:    High,
		Status:      status,
		Summary:     ternary(status == Passed, "Docker service is enabled on boot and active.", "Docker service is not enabled on boot or not active."),
		Remediation: "Run systemctl enable --now docker on systemd hosts.",
		Evidence: map[string]any{
			"enabled": enabled,
			"active":  active,
		},
	}
}

func checkDockerRestartPolicies(adapter execx.Adapter) CheckResult {
	res := adapter.Run(dockerRuntimeCommand(
		"for id in $(docker service ls -q); do docker service inspect --format '{{.Spec.Name}}\t{{.Spec.TaskTemplate.RestartPolicy.Condition}}' \"$id\"; done",
		"docker ps -q --filter label!=com.docker.swarm.service.name | xargs -r docker inspect --format '{{.Name}}\t{{.HostConfig.RestartPolicy.Name}}'",
	), 5*time.Second)
	mode, body := splitRuntimeOutput(res.Stdout)
	missing := []string{}
	for _, line := range nonEmptyLines(body) {
		name, policy := splitTab2(line)
		policy = strings.TrimSpace(policy)
		if policy == "" || policy == "no" || policy == "none" {
			missing = append(missing, strings.TrimPrefix(name, "/"))
		}
	}
	status := Passed
	if len(missing) > 0 {
		status = Failed
	}
	return CheckResult{
		ID:          "docker.containers_without_restart_policy",
		Title:       "Docker workloads have restart policies",
		Category:    "docker",
		Severity:    High,
		Status:      status,
		Summary:     ternary(len(missing) == 0, "All Docker workloads have restart policies.", fmt.Sprintf("%d Docker workload(s) lack restart policy.", len(missing))),
		Remediation: "For Docker classic, set restart: unless-stopped/always. For Swarm, set deploy.restart_policy.condition to on-failure or any.",
		Evidence: map[string]any{
			"runtimeMode": mode,
			"workloads":   missing,
		},
	}
}

func checkDockerHealthchecks(adapter execx.Adapter) CheckResult {
	res := adapter.Run(dockerRuntimeCommand(
		"for id in $(docker service ls -q); do docker service inspect --format '{{.Spec.Name}}\t{{json .Spec.TaskTemplate.ContainerSpec.Healthcheck}}' \"$id\"; done",
		"docker ps -q --filter label!=com.docker.swarm.service.name | xargs -r docker inspect --format '{{.Name}}\t{{json .Config.Healthcheck}}'",
	), 5*time.Second)
	mode, body := splitRuntimeOutput(res.Stdout)
	missing := []string{}
	for _, line := range nonEmptyLines(body) {
		name, health := splitTab2(line)
		if healthMissing(health) {
			missing = append(missing, strings.TrimPrefix(name, "/"))
		}
	}
	status := Passed
	if len(missing) > 0 {
		status = Failed
	}
	return CheckResult{
		ID:          "docker.containers_without_healthcheck",
		Title:       "Docker workloads have healthchecks",
		Category:    "docker",
		Severity:    Medium,
		Status:      status,
		Summary:     ternary(len(missing) == 0, "All Docker workloads have healthchecks.", fmt.Sprintf("%d Docker workload(s) lack healthchecks.", len(missing))),
		Remediation: "Add Docker HEALTHCHECK instructions or compose/swarm healthcheck definitions for web apps and stateful services. Workers can use command-level health probes or be explicitly documented.",
		Evidence: map[string]any{
			"runtimeMode": mode,
			"workloads":   missing,
		},
	}
}

func checkDockerRunningAsRoot(adapter execx.Adapter) CheckResult {
	res := adapter.Run(dockerRuntimeCommand(
		"for id in $(docker service ls -q); do docker service inspect --format '{{.Spec.Name}}\t{{.Spec.TaskTemplate.ContainerSpec.User}}' \"$id\"; done",
		"docker ps -q --filter label!=com.docker.swarm.service.name | xargs -r docker inspect --format '{{.Name}}\t{{.Config.User}}'",
	), 5*time.Second)
	mode, body := splitRuntimeOutput(res.Stdout)
	rootWorkloads := []string{}
	for _, line := range nonEmptyLines(body) {
		name, user := splitTab2(line)
		user = strings.TrimSpace(user)
		if user == "" || user == "0" || user == "root" {
			rootWorkloads = append(rootWorkloads, strings.TrimPrefix(name, "/"))
		}
	}
	status := Passed
	if len(rootWorkloads) > 0 {
		status = Failed
	}
	return CheckResult{
		ID:          "docker.containers_running_as_root",
		Title:       "Docker workloads do not run as root",
		Category:    "docker",
		Severity:    Medium,
		Status:      status,
		Summary:     ternary(len(rootWorkloads) == 0, "No Docker workload appears to run as root.", fmt.Sprintf("%d Docker workload(s) appear to run as root.", len(rootWorkloads))),
		Remediation: "Set USER in Dockerfiles or service/container user fields for application workloads. Some trusted infrastructure images may need an explicit ignore later.",
		Evidence: map[string]any{
			"runtimeMode": mode,
			"workloads":   rootWorkloads,
		},
	}
}

func checkDockerSockExposed(adapter execx.Adapter) CheckResult {
	res := adapter.Run(dockerRuntimeCommand(
		"for id in $(docker service ls -q); do docker service inspect --format '{{.Spec.Name}}\t{{json .Spec.TaskTemplate.ContainerSpec.Mounts}}' \"$id\"; done",
		"docker ps -q --filter label!=com.docker.swarm.service.name | xargs -r docker inspect --format '{{.Name}}\t{{json .Mounts}}'",
	), 5*time.Second)
	mode, body := splitRuntimeOutput(res.Stdout)
	exposed := []string{}
	readWrite := []string{}
	for _, line := range nonEmptyLines(body) {
		name, mounts := splitTab2(line)
		if !strings.Contains(mounts, "/var/run/docker.sock") {
			continue
		}
		cleanName := strings.TrimPrefix(name, "/")
		exposed = append(exposed, cleanName)
		if !regexp.MustCompile(`(?i)"ReadOnly":true|"RW":false`).MatchString(mounts) {
			readWrite = append(readWrite, cleanName)
		}
	}
	status := Passed
	severity := High
	if len(exposed) > 0 {
		status = Failed
	}
	if len(readWrite) > 0 {
		severity = Critical
	}
	return CheckResult{
		ID:          "docker.sock_exposed",
		Title:       "Docker socket is not exposed to workloads",
		Category:    "docker",
		Severity:    severity,
		Status:      status,
		Summary:     ternary(len(exposed) == 0, "No Docker workload mounts /var/run/docker.sock.", fmt.Sprintf("%d Docker workload(s) mount /var/run/docker.sock.", len(exposed))),
		Remediation: "Avoid mounting the Docker socket. If needed for tools like Dozzle/Uptime Kuma, keep it read-only, IP-restricted, and explicitly documented.",
		Evidence: map[string]any{
			"runtimeMode":        mode,
			"workloads":          exposed,
			"readWriteWorkloads": readWrite,
		},
	}
}

func checkUFWActive(adapter execx.Adapter) CheckResult {
	res := adapter.Run("command -v ufw >/dev/null 2>&1 && ufw status", 3*time.Second)
	active := res.ExitCode == 0 && regexp.MustCompile(`(?mi)^Status:\s+active$`).MatchString(res.Stdout)
	status := Failed
	if active {
		status = Passed
	}
	return CheckResult{ID: "firewall.ufw_inactive", Title: "UFW firewall is active", Category: "firewall", Severity: High, Status: status, Summary: ternary(active, "UFW is active.", "UFW is missing or inactive."), Remediation: "Enable a firewall with SSH, HTTP and HTTPS explicitly allowed.", AutoFixAvailable: true}
}

func checkDatabasePorts(adapter execx.Adapter) CheckResult {
	res := adapter.Run("if command -v ss >/dev/null 2>&1; then ss -ltnp; elif command -v netstat >/dev/null 2>&1; then netstat -ltnp; else exit 127; fi; printf '\\n---UFW---\\n'; command -v ufw >/dev/null 2>&1 && ufw status verbose || true", 3*time.Second)
	listenerText, firewallText, _ := strings.Cut(res.Stdout, "\n---UFW---\n")
	listeners := publicDatabaseListeners(listenerText)
	ports := uniqueListenerPorts(listeners)
	status := Passed
	severity := Critical
	summary := "No public sensitive database listeners detected."
	remediation := "Bind databases to localhost/private Docker networks and close public database ports."
	firewallRestricted := databasePortsFirewallRestricted(firewallText, ports)
	if len(ports) > 0 {
		status = Failed
		if firewallRestricted {
			severity = High
			summary = "Sensitive database listeners bind public interfaces, but UFW appears to restrict public access: " + strings.Join(ports, ", ") + "."
			remediation = "Prefer binding databases to localhost or a private Docker bridge. Keep UFW allowing DB access only from the API/Adminer network or explicit admin IPs."
		} else {
			summary = "Public sensitive database listeners detected: " + strings.Join(ports, ", ") + "."
			remediation = "Do not expose PostgreSQL/MySQL/Redis/Admin databases to the Internet. Bind to localhost/private Docker networks or restrict the port to trusted API/Adminer sources only."
		}
	}
	return CheckResult{
		ID:          "firewall.database_port_public",
		Title:       "Sensitive database ports are not public",
		Category:    "firewall",
		Severity:    severity,
		Status:      status,
		Summary:     summary,
		Remediation: remediation,
		Evidence: map[string]any{
			"publicPorts":          ports,
			"listeners":            listeners,
			"firewallRestricted":   firewallRestricted,
			"expectedAccessModel":  "API/Adminer may access DB through localhost/private network or explicit allowlist; the DB port should not be reachable from the Internet.",
			"firewallStatusSample": strings.TrimSpace(firewallText),
		},
	}
}

func checkEnvWorldReadable(adapter execx.Adapter) CheckResult {
	if adapter.Run("test -f .env", time.Second).ExitCode != 0 {
		return CheckResult{ID: "secrets.env_world_readable", Title: ".env is not world-readable", Category: "secrets", Severity: Critical, Status: Skipped, Summary: "No .env file found in the current project."}
	}
	res := adapter.Run("stat -c '%a' .env 2>/dev/null || stat -f '%Lp' .env", time.Second)
	mode := strings.TrimSpace(res.Stdout)
	worldReadable := strings.HasSuffix(mode, "4") || strings.HasSuffix(mode, "5") || strings.HasSuffix(mode, "6") || strings.HasSuffix(mode, "7")
	status := Passed
	if worldReadable {
		status = Failed
	}
	return CheckResult{ID: "secrets.env_world_readable", Title: ".env is not world-readable", Category: "secrets", Severity: Critical, Status: status, Summary: "Current .env permissions: " + mode + ".", Remediation: "Run chmod 600 .env and keep secrets out of shared-readable files.", AutoFixAvailable: true}
}

func checkEnvTracked(adapter execx.Adapter) CheckResult {
	tracked := adapter.Run("git ls-files --error-unmatch .env", time.Second).ExitCode == 0
	status := Passed
	if tracked {
		status = Failed
	}
	return CheckResult{ID: "secrets.env_in_git", Title: ".env is not tracked by Git", Category: "secrets", Severity: Critical, Status: status, Summary: ternary(tracked, ".env is tracked by Git.", ".env is not tracked by Git."), Remediation: "Remove .env from Git history/tracking and keep .env in .gitignore."}
}

func checkCaddyInstalled(adapter execx.Adapter) CheckResult {
	res := adapter.Run("command -v caddy >/dev/null 2>&1 || docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^caddy$|caddy'", 3*time.Second)
	installed := res.ExitCode == 0
	status := Failed
	if installed {
		status = Passed
	}
	return CheckResult{ID: "caddy.not_installed", Title: "Caddy or Caddy container is present", Category: "reverse-proxy", Severity: Medium, Status: status, Summary: ternary(installed, "Caddy was detected.", "No Caddy binary or running Caddy container detected."), Remediation: "Install/configure Caddy or another reverse proxy before exposing the app."}
}

func checkCaddyAdminExposed(adapter execx.Adapter) CheckResult {
	res := adapter.Run("if command -v ss >/dev/null 2>&1; then ss -ltnp; elif command -v netstat >/dev/null 2>&1; then netstat -ltnp; else exit 127; fi", 3*time.Second)
	exposed := []string{}
	for _, line := range nonEmptyLines(res.Stdout) {
		if !regexp.MustCompile(`(?::|\]:)2019\b`).MatchString(line) {
			continue
		}
		if regexp.MustCompile(`(?:0\.0\.0\.0|\[::\]|\*:|::):2019\b|\*:2019\b`).MatchString(line) {
			exposed = append(exposed, line)
		}
	}
	status := Passed
	if len(exposed) > 0 {
		status = Failed
	}
	return CheckResult{
		ID:          "caddy.admin_exposed",
		Title:       "Caddy admin API is not publicly exposed",
		Category:    "reverse-proxy",
		Severity:    Critical,
		Status:      status,
		Summary:     ternary(len(exposed) == 0, "No public Caddy admin listener detected.", "Caddy admin API appears to listen on a public interface."),
		Remediation: "Bind Caddy admin API to localhost, disable it when unused, or keep it on an internal-only network.",
		Evidence: map[string]any{
			"listeners": exposed,
		},
	}
}

func checkAdminerRestricted(adapter execx.Adapter) CheckResult {
	detected := adapter.Run("docker ps --format '{{.Names}}\t{{.Image}}' 2>/dev/null | grep -i adminer", 3*time.Second)
	if detected.ExitCode != 0 {
		return CheckResult{ID: "adminer.ip_restriction_missing", Title: "Adminer is IP restricted", Category: "database", Severity: High, Status: Skipped, Summary: "No running Adminer container detected."}
	}
	config := adapter.Run("grep -Rih --include='*.caddy' --include='Caddyfile' 'adminer\\|Cf-Connecting-Ip\\|remote_ip\\|client_ip\\|basic_auth\\|respond .*403' /etc/caddy /opt/caddy 2>/dev/null | head -n 200", 3*time.Second)
	text := config.Stdout
	hasIPRestriction := regexp.MustCompile(`(?i)(Cf-Connecting-Ip|remote_ip|client_ip)`).MatchString(text)
	hasDeny := regexp.MustCompile(`(?i)(respond\s+.*403|abort|deny)`).MatchString(text)
	hasAuth := regexp.MustCompile(`(?i)basic_auth`).MatchString(text)
	status := Passed
	summary := "Adminer appears to be protected by an IP restriction."
	if !hasIPRestriction || !hasDeny {
		status = Failed
		summary = "Adminer is running, but no clear IP restriction/deny rule was detected in Caddy config."
	}
	return CheckResult{
		ID:          "adminer.ip_restriction_missing",
		Title:       "Adminer is IP restricted",
		Category:    "database",
		Severity:    High,
		Status:      status,
		Summary:     summary,
		Remediation: "Expose Adminer only behind the reverse proxy with an allowlist for your home IP, deny-by-default behavior, and basic auth. Do not publish Adminer directly with Docker ports.",
		Evidence: map[string]any{
			"adminerContainer": strings.TrimSpace(detected.Stdout),
			"ipRestriction":    hasIPRestriction,
			"denyRule":         hasDeny,
			"basicAuth":        hasAuth,
		},
	}
}

func publicDatabaseListeners(output string) []map[string]string {
	listeners := []map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, port := range []string{"5432", "3306", "6379", "7700", "9200", "27017"} {
			if !regexp.MustCompile(`(?::|\]:)` + port + `\b`).MatchString(line) {
				continue
			}
			if !regexp.MustCompile(`(?:0\.0\.0\.0|\[::\]|\*:|::):` + port + `\b|\*:` + port + `\b`).MatchString(line) {
				continue
			}
			process := capture(line, `users:\(\("([^"]+)`)
			if process == "" {
				process = capture(line, `\b([a-zA-Z0-9._-]+)\s*$`)
			}
			listeners = append(listeners, map[string]string{
				"port":    port,
				"process": fallback(process, "unknown"),
				"line":    line,
			})
		}
	}
	return listeners
}

func uniqueListenerPorts(listeners []map[string]string) []string {
	seen := map[string]bool{}
	ports := []string{}
	for _, listener := range listeners {
		port := listener["port"]
		if port == "" || seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}
	return ports
}

func databasePortsFirewallRestricted(firewallText string, ports []string) bool {
	if len(ports) == 0 {
		return false
	}
	if !regexp.MustCompile(`(?mi)^Status:\s+active$`).MatchString(firewallText) {
		return false
	}
	if !regexp.MustCompile(`(?mi)^Default:\s+deny\s+\(incoming\)`).MatchString(firewallText) {
		return false
	}
	for _, port := range ports {
		publicAllow := regexp.MustCompile(`(?mi)^` + regexp.QuoteMeta(port) + `(?:/tcp)?\s+ALLOW IN\s+Anywhere(?:\s|$)|^` + regexp.QuoteMeta(port) + `(?:/tcp)?\s+ALLOW IN\s+0\.0\.0\.0/0(?:\s|$)|^` + regexp.QuoteMeta(port) + `(?:/tcp)?\s+\(v6\)\s+ALLOW IN\s+Anywhere`)
		if publicAllow.MatchString(firewallText) {
			return false
		}
	}
	return true
}

func dockerRuntimeCommand(swarmCommand string, classicCommand string) string {
	return "if docker info --format '{{.Swarm.ControlAvailable}}' 2>/dev/null | grep -q '^true$'; then echo __deployshuttle_runtime=swarm; " + swarmCommand + "; fi; echo __deployshuttle_runtime=classic; " + classicCommand
}

func splitRuntimeOutput(output string) (string, string) {
	seen := map[string]bool{}
	bodyLines := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "__deployshuttle_runtime=") {
			seen[strings.TrimPrefix(line, "__deployshuttle_runtime=")] = true
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	mode := "classic"
	if seen["swarm"] && seen["classic"] {
		mode = "mixed"
	} else if seen["swarm"] {
		mode = "swarm"
	}
	return mode, strings.Join(bodyLines, "\n")
}

func splitTab2(line string) (string, string) {
	left, right, ok := strings.Cut(line, "\t")
	if !ok {
		return strings.TrimSpace(line), ""
	}
	return strings.TrimSpace(left), strings.TrimSpace(right)
}

func healthMissing(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return true
	}
	return regexp.MustCompile(`(?i)"Test":\s*\[\s*"NONE"\s*\]`).MatchString(value)
}

func nonEmptyLines(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func listContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func evidenceStringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		values := []string{}
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values = append(values, text)
		}
		return values, true
	default:
		return nil, false
	}
}

func matchesAnyWorkload(workload string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchesWorkload(workload, pattern) {
			return true
		}
	}
	return false
}

func matchesWorkload(workload string, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == workload {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(workload, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(workload, strings.TrimPrefix(pattern, "*")) {
		return true
	}
	return false
}

func capture(input string, pattern string) string {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func fallback(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func ternary(condition bool, yes string, no string) string {
	if condition {
		return yes
	}
	return no
}

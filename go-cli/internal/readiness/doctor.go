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
	if target == "" {
		target = "local"
	}
	if len(profile) == 0 {
		profile = []string{"docker", "caddy"}
	}

	checks := []Check{
		checkOSSupported,
		checkDiskSpace,
		checkDockerInstalled,
		checkUFWActive,
		checkDatabasePorts,
		checkEnvWorldReadable,
		checkEnvTracked,
		checkCaddyInstalled,
		checkAdminerRestricted,
	}
	results := make([]CheckResult, 0, len(checks))
	for _, check := range checks {
		results = append(results, check(adapter))
	}
	score := Score(results)
	return Report{
		Target:      target,
		Profile:     profile,
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
			if check.Status == Failed && check.Remediation != "" {
				fmt.Fprintf(&b, "      Fix: %s\n", check.Remediation)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
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

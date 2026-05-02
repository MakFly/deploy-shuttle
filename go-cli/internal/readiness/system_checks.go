package readiness

import (
	"strings"
	"time"

	"github.com/MakFly/deploy-shuttle/go-cli/internal/execx"
)

// checkSSHRootLogin fails if sshd_config explicitly enables PermitRootLogin yes.
// Linux-only; skipped if the config is not readable (e.g. macOS dev machine).
func checkSSHRootLogin(adapter execx.Adapter) CheckResult {
	res := adapter.Run("test -r /etc/ssh/sshd_config && grep -E '^[[:space:]]*PermitRootLogin[[:space:]]+' /etc/ssh/sshd_config | tail -n 1", 2*time.Second)
	if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
		return CheckResult{
			ID: "ssh.root_login_enabled", Title: "SSH root login is restricted",
			Category: "ssh", Severity: High, Status: Skipped,
			Summary: "Could not read /etc/ssh/sshd_config (no SSH config or insufficient permissions).",
		}
	}
	line := strings.ToLower(strings.Fields(strings.TrimSpace(res.Stdout))[1])
	worldOpen := line == "yes"
	status := Passed
	severity := High
	if worldOpen {
		status = Failed
	}
	return CheckResult{
		ID: "ssh.root_login_enabled", Title: "SSH root login is restricted",
		Category: "ssh", Severity: severity, Status: status,
		Summary:     "PermitRootLogin = " + line + ".",
		Remediation: "Set 'PermitRootLogin prohibit-password' (or 'no') in /etc/ssh/sshd_config and reload sshd.",
		Evidence:    map[string]any{"permitRootLogin": line},
	}
}

// checkSSHPasswordAuth fails when password authentication is enabled.
func checkSSHPasswordAuth(adapter execx.Adapter) CheckResult {
	res := adapter.Run("test -r /etc/ssh/sshd_config && grep -E '^[[:space:]]*PasswordAuthentication[[:space:]]+' /etc/ssh/sshd_config | tail -n 1", 2*time.Second)
	if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) == "" {
		return CheckResult{
			ID: "ssh.password_auth_enabled", Title: "SSH password authentication is disabled",
			Category: "ssh", Severity: High, Status: Skipped,
			Summary: "Could not read /etc/ssh/sshd_config (no SSH config or insufficient permissions).",
		}
	}
	line := strings.ToLower(strings.Fields(strings.TrimSpace(res.Stdout))[1])
	enabled := line == "yes"
	status := Passed
	if enabled {
		status = Failed
	}
	return CheckResult{
		ID: "ssh.password_auth_enabled", Title: "SSH password authentication is disabled",
		Category: "ssh", Severity: High, Status: status,
		Summary:     "PasswordAuthentication = " + line + ".",
		Remediation: "Set 'PasswordAuthentication no' in /etc/ssh/sshd_config, ensure key-based access works, then reload sshd.",
		Evidence:    map[string]any{"passwordAuthentication": line},
	}
}

// checkUnattendedUpgrades verifies the unattended-upgrades package and timer are active (Debian/Ubuntu).
func checkUnattendedUpgrades(adapter execx.Adapter) CheckResult {
	installed := adapter.Run("dpkg -s unattended-upgrades 2>/dev/null | grep -q '^Status: install ok installed'", 2*time.Second).ExitCode == 0
	active := adapter.Run("systemctl is-enabled unattended-upgrades 2>/dev/null", 2*time.Second).ExitCode == 0
	if !installed && !active {
		// Skip on non-Debian or when systemctl is missing entirely.
		if adapter.Run("command -v systemctl >/dev/null 2>&1", time.Second).ExitCode != 0 {
			return CheckResult{
				ID: "system.unattended_upgrades_inactive", Title: "Automatic security updates are enabled",
				Category: "system", Severity: Medium, Status: Skipped,
				Summary: "systemctl is not available; cannot evaluate automatic security updates.",
			}
		}
	}
	status := Failed
	if installed && active {
		status = Passed
	}
	summary := "unattended-upgrades is missing or disabled."
	if installed && active {
		summary = "unattended-upgrades package is installed and the service is enabled."
	} else if installed {
		summary = "unattended-upgrades is installed but the service is not enabled."
	}
	return CheckResult{
		ID: "system.unattended_upgrades_inactive", Title: "Automatic security updates are enabled",
		Category: "system", Severity: Medium, Status: status,
		Summary:     summary,
		Remediation: "Run 'apt install unattended-upgrades' and 'dpkg-reconfigure -plow unattended-upgrades' on Debian/Ubuntu.",
		Evidence:    map[string]any{"installed": installed, "enabled": active},
	}
}

// checkFail2ban verifies fail2ban is active.
func checkFail2ban(adapter execx.Adapter) CheckResult {
	if adapter.Run("command -v systemctl >/dev/null 2>&1", time.Second).ExitCode != 0 {
		return CheckResult{
			ID: "system.fail2ban_inactive", Title: "fail2ban is active",
			Category: "system", Severity: Medium, Status: Skipped,
			Summary: "systemctl is not available; cannot evaluate fail2ban.",
		}
	}
	active := adapter.Run("systemctl is-active fail2ban 2>/dev/null", 2*time.Second).ExitCode == 0
	status := Failed
	summary := "fail2ban is not active."
	if active {
		status = Passed
		summary = "fail2ban service is active."
	}
	return CheckResult{
		ID: "system.fail2ban_inactive", Title: "fail2ban is active",
		Category: "system", Severity: Medium, Status: status,
		Summary:     summary,
		Remediation: "Install and enable fail2ban: 'apt install fail2ban && systemctl enable --now fail2ban'.",
		Evidence:    map[string]any{"active": active},
	}
}

// checkSwap warns when no swap is configured.
func checkSwap(adapter execx.Adapter) CheckResult {
	res := adapter.Run("swapon --show=NAME --noheadings 2>/dev/null", 2*time.Second)
	if res.ExitCode != 0 {
		return CheckResult{
			ID: "system.swap_missing", Title: "Swap is configured",
			Category: "system", Severity: Low, Status: Skipped,
			Summary: "swapon is not available on this host.",
		}
	}
	enabled := strings.TrimSpace(res.Stdout) != ""
	status := Failed
	summary := "No swap device or file is configured."
	if enabled {
		status = Passed
		summary = "Swap is configured."
	}
	return CheckResult{
		ID: "system.swap_missing", Title: "Swap is configured",
		Category: "system", Severity: Low, Status: status,
		Summary:     summary,
		Remediation: "Add a swap file (e.g. 1-2 GB on small VPS) so OOM events fall back to disk before killing workloads.",
		Evidence:    map[string]any{"enabled": enabled},
	}
}

// checkTimeSync verifies a time-sync daemon is active.
func checkTimeSync(adapter execx.Adapter) CheckResult {
	if adapter.Run("command -v systemctl >/dev/null 2>&1", time.Second).ExitCode != 0 {
		return CheckResult{
			ID: "system.time_sync_inactive", Title: "Time synchronization is active",
			Category: "system", Severity: Medium, Status: Skipped,
			Summary: "systemctl is not available; cannot evaluate time sync.",
		}
	}
	candidates := []string{"systemd-timesyncd", "chrony", "chronyd", "ntp", "ntpd"}
	active := ""
	for _, name := range candidates {
		if adapter.Run("systemctl is-active "+name+" 2>/dev/null", 2*time.Second).ExitCode == 0 {
			active = name
			break
		}
	}
	status := Failed
	summary := "No time synchronization daemon is active (checked: " + strings.Join(candidates, ", ") + ")."
	if active != "" {
		status = Passed
		summary = "Time synchronization daemon active: " + active + "."
	}
	return CheckResult{
		ID: "system.time_sync_inactive", Title: "Time synchronization is active",
		Category: "system", Severity: Medium, Status: status,
		Summary:     summary,
		Remediation: "Enable systemd-timesyncd or install chrony so logs and TLS handshakes use accurate time.",
		Evidence:    map[string]any{"daemon": active},
	}
}

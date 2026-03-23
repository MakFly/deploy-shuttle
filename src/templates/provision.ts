import { shellEscape } from '../utils/shell.ts'

/**
 * Returns shell commands to create a dedicated deploy user, grant sudo access,
 * and install the provided SSH public key for key-based authentication.
 */
export function createUserCommands(username: string, publicKey: string): string[] {
	const u = shellEscape(username)
	const pk = shellEscape(publicKey)
	return [
		// Create user with a home directory and bash shell, skip if already exists
		`id -u ${u} &>/dev/null || useradd -m -s /bin/bash ${u}`,
		// Add user to the sudo group
		`usermod -aG sudo ${u}`,
		// Create the .ssh directory with correct permissions
		`mkdir -p /home/${u}/.ssh`,
		`chmod 700 /home/${u}/.ssh`,
		// Append the public key (idempotent — skips if already present)
		`grep -qxF ${pk} /home/${u}/.ssh/authorized_keys 2>/dev/null || echo ${pk} >> /home/${u}/.ssh/authorized_keys`,
		// Lock down authorized_keys
		`chmod 600 /home/${u}/.ssh/authorized_keys`,
		// Ensure the user owns the entire .ssh directory
		`chown -R ${u}:${u} /home/${u}/.ssh`,
	]
}

/**
 * Returns shell commands to install Docker CE from the official Docker apt
 * repository on a Debian/Ubuntu-based VPS.
 */
export function installDockerCommands(): string[] {
	return [
		// Remove any distro-packaged Docker variants
		'apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true',
		// Refresh package index
		'apt-get update -y',
		// Install dependencies needed to add the Docker apt repo
		'apt-get install -y ca-certificates curl gnupg lsb-release',
		// Add Docker's official GPG key
		'install -m 0755 -d /etc/apt/keyrings',
		'curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg',
		'chmod a+r /etc/apt/keyrings/docker.gpg',
		// Add the Docker apt repository
		'echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null',
		// Install Docker Engine, CLI, containerd, and compose plugin
		'apt-get update -y',
		'apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin',
		// Enable and start Docker
		'systemctl enable docker',
		'systemctl start docker',
	]
}

/**
 * Returns shell commands to run Caddy as a Docker container with host networking
 * so it can reach app containers on localhost ports, and auto-restart on reboot.
 */
export function installCaddyCommands(): string[] {
	return [
		// Pull the latest official Caddy image
		'docker pull caddy:latest',
		// Create persistent directories for Caddy config and data (TLS certs)
		'mkdir -p /etc/caddy /var/caddy/data /var/caddy/config',
		// Write an empty Caddyfile if none exists yet
		'[ -f /etc/caddy/Caddyfile ] || touch /etc/caddy/Caddyfile',
		// Remove any existing caddy container before (re-)creating
		'docker rm -f caddy 2>/dev/null || true',
		// Run Caddy with host networking, TLS data persistence, and auto-restart
		[
			'docker run -d',
			'--name caddy',
			'--restart unless-stopped',
			'--network host',
			'-v /etc/caddy/Caddyfile:/etc/caddy/Caddyfile:ro',
			'-v /var/caddy/data:/data',
			'-v /var/caddy/config:/config',
			'caddy:latest',
		].join(' '),
	]
}

/**
 * Returns shell commands to configure UFW firewall rules (SSH, HTTP, HTTPS)
 * and install fail2ban for brute-force protection.
 */
export function firewallCommands(): string[] {
	return [
		// Install UFW and fail2ban if not already present
		'apt-get install -y ufw fail2ban',
		// Reset to defaults to avoid stale rules (non-interactive)
		'ufw --force reset',
		// Set default policies
		'ufw default deny incoming',
		'ufw default allow outgoing',
		// Allow SSH (must come before enable to avoid lockout)
		"ufw allow 22/tcp comment 'SSH'",
		// Allow HTTP and HTTPS for Caddy
		"ufw allow 80/tcp comment 'HTTP'",
		"ufw allow 443/tcp comment 'HTTPS'",
		// Enable UFW non-interactively
		'ufw --force enable',
		// Enable and start fail2ban
		'systemctl enable fail2ban',
		'systemctl start fail2ban',
	]
}

/**
 * Returns shell commands to harden the SSH daemon by disabling root login
 * and password-based authentication, then restarting sshd.
 */
export function hardenSSHCommands(): string[] {
	return [
		// Disable root login over SSH
		"sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config",
		// Disable password authentication (key-only)
		"sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config",
		// Disable challenge-response auth
		"sed -i 's/^#*ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' /etc/ssh/sshd_config",
		// Validate the config before restarting
		'sshd -t',
		// Restart sshd to apply changes
		'systemctl restart sshd',
	]
}

/**
 * Returns shell commands to create the /opt/shuttle/<project>/ directory
 * structure used by Shuttle. Creates the base app dir and the workdir
 * (<base>/<project>) that RuntimeManager expects for deployments.
 */
export function setupDirectoryCommands(project: string, username: string): string[] {
	const base = `/opt/shuttle/${shellEscape(project)}`

	return [
		// Create the base project directory
		`mkdir -p ${base}`,
		// Create the workdir that RuntimeManager.getWorkDir() resolves to
		`mkdir -p ${base}/${shellEscape(project)}`,
		// Set ownership to the deploy user
		`chown -R ${shellEscape(username)}:${shellEscape(username)} /opt/shuttle 2>/dev/null || true`,
		// Lock down permissions so only the owner can read/write
		`chmod -R 750 ${base}`,
	]
}

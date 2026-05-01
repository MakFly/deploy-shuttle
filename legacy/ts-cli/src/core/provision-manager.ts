import {
	createUserCommands,
	firewallCommands,
	hardenSSHCommands,
	installCaddyCommands,
	installDockerCommands,
	setupDirectoryCommands,
} from '../templates/provision.ts'
import { ProvisionError } from '../utils/errors.ts'
import { logger } from '../utils/logger.ts'
import { type SSHManager, ssh as defaultSsh } from './ssh-manager.ts'

// ---------------------------------------------------------------------------
// ProvisionManager
// ---------------------------------------------------------------------------

export class ProvisionManager {
	constructor(private readonly ssh: SSHManager = defaultSsh) {}

	/**
	 * Full VPS bootstrap sequence executed as root.
	 *
	 * Steps:
	 *  1.  Connect as root
	 *  2.  Detect OS (Debian/Ubuntu required)
	 *  3.  Create deploy user
	 *  4.  Install Docker
	 *  5.  Install Caddy (as Docker container)
	 *  6.  Configure firewall (ufw)
	 *  7.  Harden SSH
	 *  8.  Create project directories
	 *  9.  Verify everything works
	 */
	async provision(
		host: string,
		config: {
			user: string
			publicKey: string
			project: string
			sshPort?: number
		},
	): Promise<void> {
		const TOTAL_STEPS = 9

		// ── Step 1: Connect as root ───────────────────────────────────────────────
		logger.step(1, TOTAL_STEPS, `Connecting to ${host} as root`)

		await this.ssh.connect({ host, user: 'root' })

		// ── Step 2: Detect OS ─────────────────────────────────────────────────────
		logger.step(2, TOTAL_STEPS, 'Detecting OS')

		const { stdout: osRelease, code: osCode } = await this.ssh.exec(host, 'cat /etc/os-release')

		if (osCode !== 0) {
			throw new ProvisionError(`Cannot read /etc/os-release on ${host}. Is the host reachable?`)
		}

		const isDebianFamily =
			osRelease.includes('debian') ||
			osRelease.includes('Debian') ||
			osRelease.includes('ubuntu') ||
			osRelease.includes('Ubuntu')

		if (!isDebianFamily) {
			throw new ProvisionError(
				`Unsupported OS on ${host}. Only Debian/Ubuntu is supported.\n${osRelease}`,
			)
		}

		logger.debug(`OS check passed on ${host}`)

		// ── Step 3: Create deploy user ────────────────────────────────────────────
		logger.step(3, TOTAL_STEPS, `Creating deploy user "${config.user}"`)

		for (const cmd of createUserCommands(config.user, config.publicKey)) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				throw new ProvisionError(
					`Failed to create user "${config.user}" on ${host}: ${stderr.trim()}`,
				)
			}
		}

		// ── Step 4: Install Docker ────────────────────────────────────────────────
		logger.step(4, TOTAL_STEPS, 'Installing Docker')

		for (const cmd of installDockerCommands()) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				throw new ProvisionError(
					`Docker installation failed on ${host} at command "${cmd}": ${stderr.trim()}`,
				)
			}
		}

		// ── Step 5: Install Caddy ─────────────────────────────────────────────────
		logger.step(5, TOTAL_STEPS, 'Installing Caddy container')

		for (const cmd of installCaddyCommands()) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				throw new ProvisionError(
					`Caddy setup failed on ${host} at command "${cmd}": ${stderr.trim()}`,
				)
			}
		}

		// ── Step 6: Configure firewall ────────────────────────────────────────────
		logger.step(6, TOTAL_STEPS, 'Configuring firewall')

		for (const cmd of firewallCommands()) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				// Firewall failures are warnings, not fatal — the host may already
				// have a different firewall setup.
				logger.warn(`Firewall command "${cmd}" failed on ${host}: ${stderr.trim()}`)
			}
		}

		// ── Step 7: Harden SSH ────────────────────────────────────────────────────
		logger.step(7, TOTAL_STEPS, 'Hardening SSH configuration')

		for (const cmd of hardenSSHCommands()) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				logger.warn(`SSH hardening command "${cmd}" failed on ${host}: ${stderr.trim()}`)
			}
		}

		// ── Step 8: Create project directories ───────────────────────────────────
		logger.step(8, TOTAL_STEPS, 'Creating project directories')

		for (const cmd of setupDirectoryCommands(config.project, config.user)) {
			const { code, stderr } = await this.ssh.exec(host, cmd)
			if (code !== 0) {
				throw new ProvisionError(`Directory setup failed on ${host}: ${stderr.trim()}`)
			}
		}

		// ── Step 9: Verify ────────────────────────────────────────────────────────
		logger.step(9, TOTAL_STEPS, 'Verifying provisioning')

		this.ssh.disconnect(host)

		const ok = await this.verify(host, config.user)

		if (!ok) {
			throw new ProvisionError(`Post-provision verification failed on ${host}. Check logs above.`)
		}

		logger.success(`Server ${host} provisioned successfully`)
	}

	/**
	 * Connects as the deploy user and verifies that Docker, Caddy, and the
	 * project directories are all in place.
	 *
	 * Returns true on success, false on any failure.
	 */
	async verify(host: string, user: string): Promise<boolean> {
		try {
			await this.ssh.connect({ host, user })

			// docker info — exits 0 when Docker daemon is running and accessible
			const docker = await this.ssh.exec(host, 'docker info > /dev/null 2>&1')
			if (docker.code !== 0) {
				logger.error(`Docker not accessible as "${user}" on ${host}`)
				return false
			}

			// Caddy container must be running
			const caddy = await this.ssh.exec(
				host,
				'docker inspect --format "{{.State.Running}}" caddy 2>/dev/null',
			)
			if (caddy.code !== 0 || caddy.stdout.trim() !== 'true') {
				logger.error(`Caddy container is not running on ${host}`)
				return false
			}

			// /opt/shuttle directory must exist
			const dirs = await this.ssh.exec(host, 'test -d /opt/shuttle')
			if (dirs.code !== 0) {
				logger.error(`/opt/shuttle does not exist on ${host}`)
				return false
			}

			this.ssh.disconnect(host)
			return true
		} catch (err) {
			logger.error(
				`Verification failed on ${host}: ${err instanceof Error ? err.message : String(err)}`,
			)
			return false
		}
	}
}

export const provisioner = new ProvisionManager(defaultSsh)

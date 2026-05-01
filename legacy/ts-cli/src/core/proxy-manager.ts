import type { ShuttleConfig } from '../config/schema.ts'
import { generateCaddyfile, generateUpstreamSwitch } from '../templates/caddyfile.ts'
import { ShuttleError } from '../utils/errors.ts'
import { logger } from '../utils/logger.ts'
import { type SSHManager, ssh as defaultSsh } from './ssh-manager.ts'

// ---------------------------------------------------------------------------
// ProxyManager
// ---------------------------------------------------------------------------

const CADDYFILE_REMOTE_PATH = '/opt/caddy/Caddyfile'
const CADDY_CONTAINER_NAME = 'caddy'
const CADDY_CONFIG_IN_CONTAINER = '/etc/caddy/Caddyfile'

export class ProxyManager {
	constructor(private readonly ssh: SSHManager = defaultSsh) {}

	private async readConfig(host: string): Promise<string> {
		const { stdout, code } = await this.ssh.exec(host, `cat ${CADDYFILE_REMOTE_PATH}`)
		if (code !== 0) {
			return ''
		}
		return stdout
	}

	private stripDomainBlocks(caddyfile: string, domains: string[]): string {
		const sections = caddyfile
			.split(/\n{2,}/)
			.map((section) => section.trim())
			.filter((section) => section.length > 0)

		const kept = sections.filter((section) => {
			const firstLine = section.split('\n')[0]?.trim() ?? ''
			if (firstLine === '{') {
				return true
			}

			return !domains.some((domain) => firstLine.startsWith(domain))
		})

		return kept.length > 0 ? `${kept.join('\n\n')}\n` : ''
	}

	/**
	 * Generates a complete Caddyfile from the shuttle config and a live
	 * upstream map of the form `{ serviceName: "127.0.0.1:PORT" }`.
	 */
	generateConfig(config: ShuttleConfig, activeUpstreams: Map<string, string>): string {
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]

		const sslEmail = config.proxy?.ssl?.email

		// Build one CaddyApp per upstream entry — if no services are listed,
		// fall back to a single app using all domains.
		if (activeUpstreams.size === 0) {
			return generateCaddyfile([
				{
					domains,
					upstream: 'localhost:3000',
					headers: config.proxy?.headers,
					ssl: sslEmail !== undefined ? { email: sslEmail } : undefined,
				},
			])
		}

		const apps = Array.from(activeUpstreams.entries()).map(([_service, upstream]) => ({
			domains,
			upstream,
			headers: config.proxy?.headers,
			ssl: sslEmail !== undefined ? { email: sslEmail } : undefined,
		}))

		return generateCaddyfile(apps)
	}

	/**
	 * Updates the Caddyfile on the remote host to point `app` at `newPort`,
	 * then reloads Caddy.
	 */
	async switchUpstream(host: string, app: string, newPort: number): Promise<void> {
		logger.debug(`Switching upstream for ${app} on ${host} to port ${newPort}`)

		const newUpstream = `127.0.0.1:${newPort}`
		const snippet = generateUpstreamSwitch([app], newUpstream)

		// Replace the existing block for this domain/app inside the Caddyfile.
		// We write the minimal snippet directly — the apply() method handles
		// the full-reload path.
		await this.apply(host, snippet)
	}

	/**
	 * Uploads a Caddyfile to the remote host and instructs the Caddy container
	 * to reload its configuration without downtime.
	 */
	async apply(host: string, caddyfile: string): Promise<void> {
		logger.debug(`Uploading Caddyfile to ${host}:${CADDYFILE_REMOTE_PATH}`)

		await this.ssh.uploadContent(host, caddyfile, CADDYFILE_REMOTE_PATH, 0o644)

		const reloadCmd = [
			'docker',
			'exec',
			CADDY_CONTAINER_NAME,
			'caddy',
			'reload',
			'--config',
			CADDY_CONFIG_IN_CONTAINER,
		].join(' ')

		logger.debug(`Reloading Caddy on ${host}`)

		const { code, stderr } = await this.ssh.exec(host, reloadCmd)

		if (code !== 0) {
			throw new ShuttleError(
				`Caddy reload failed on ${host} (exit ${code}): ${stderr.trim()}`,
				'PROXY_RELOAD_FAILED',
			)
		}

		logger.success(`Caddy reloaded on ${host}`)
	}

	/**
	 * Returns the JSON-formatted docker inspect output for the Caddy container.
	 */
	async getStatus(host: string): Promise<string> {
		const { stdout, code, stderr } = await this.ssh.exec(
			host,
			`docker inspect ${CADDY_CONTAINER_NAME}`,
		)

		if (code !== 0) {
			throw new ShuttleError(
				`Failed to inspect Caddy on ${host}: ${stderr.trim()}`,
				'PROXY_STATUS_FAILED',
			)
		}

		return stdout.trim()
	}

	/**
	 * Removes the configured domains for an application from the remote
	 * Caddyfile, then reloads Caddy if any config remains.
	 */
	async removeDomains(host: string, domains: string[]): Promise<void> {
		const current = await this.readConfig(host)
		if (current.length === 0) {
			return
		}

		const next = this.stripDomainBlocks(current, domains)

		await this.apply(host, next.length > 0 ? next : '# Shuttle: no configured apps\n')
	}
}

export const proxy = new ProxyManager(defaultSsh)

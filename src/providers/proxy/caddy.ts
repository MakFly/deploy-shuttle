import type { ShuttleConfig } from '../../config/schema.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { generateCaddyfile } from '../../templates/caddyfile.ts'
import { ShuttleError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import type { ProxyProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// CaddyProxyProvider
// ---------------------------------------------------------------------------

const CADDYFILE_REMOTE_PATH = '/etc/caddy/Caddyfile'
const CADDY_CONTAINER_NAME = 'caddy'
const CADDY_CONFIG_IN_CONTAINER = '/etc/caddy/Caddyfile'

export class CaddyProxyProvider implements ProxyProvider {
	constructor(private readonly ssh: SSHManager) {}

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
	async apply(host: string, config: ShuttleConfig, upstreams: Map<string, string>): Promise<void> {
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		const sslEmail = config.proxy?.ssl?.email

		let caddyfile: string

		if (upstreams.size === 0) {
			caddyfile = generateCaddyfile([
				{
					domains,
					upstream: 'localhost:3000',
					headers: config.proxy?.headers,
					ssl: sslEmail !== undefined ? { email: sslEmail } : undefined,
				},
			])
		} else {
			const apps = Array.from(upstreams.entries()).map(([_service, upstream]) => ({
				domains,
				upstream,
				headers: config.proxy?.headers,
				ssl: sslEmail !== undefined ? { email: sslEmail } : undefined,
			}))
			caddyfile = generateCaddyfile(apps)
		}

		await this._uploadAndReload(host, caddyfile)
	}

	/**
	 * Hot-switches upstream(s) during blue-green or rollback.
	 * Reads the existing Caddyfile, strips the domain blocks for the given
	 * domains, then appends a new block with reverse_proxy + headers + ssl.
	 */
	async switchUpstream(
		host: string,
		config: ShuttleConfig,
		domains: string[],
		newPort: number,
	): Promise<void> {
		logger.debug(`Switching upstream for ${domains.join(', ')} on ${host} to port ${newPort}`)

		const current = await this.readConfig(host)
		const stripped = current.length > 0 ? this.stripDomainBlocks(current, domains) : ''

		const sslEmail = config.proxy?.ssl?.email
		const newBlock = generateCaddyfile([
			{
				domains,
				upstream: `127.0.0.1:${newPort}`,
				headers: config.proxy?.headers,
				ssl: sslEmail !== undefined ? { email: sslEmail } : undefined,
			},
		])

		// Combine: keep existing non-domain blocks + append the new block
		const combined = stripped.length > 0 ? `${stripped.trimEnd()}\n\n${newBlock}` : newBlock

		await this._uploadAndReload(host, combined)
	}

	/**
	 * Removes all proxy config for the given domains, then reloads Caddy.
	 */
	async removeDomains(host: string, domains: string[]): Promise<void> {
		const current = await this.readConfig(host)
		if (current.length === 0) {
			return
		}

		const next = this.stripDomainBlocks(current, domains)
		await this._uploadAndReload(host, next.length > 0 ? next : '# Shuttle: no configured apps\n')
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

	// ---------------------------------------------------------------------------
	// Private helpers
	// ---------------------------------------------------------------------------

	private async _uploadAndReload(host: string, caddyfile: string): Promise<void> {
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
}

export function createCaddyProxyProvider(ssh: SSHManager): CaddyProxyProvider {
	return new CaddyProxyProvider(ssh)
}

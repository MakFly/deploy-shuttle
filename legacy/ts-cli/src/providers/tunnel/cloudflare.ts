import type { ShuttleConfig } from '../../config/schema.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { DeployError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import { shellEscape } from '../../utils/shell.ts'
import type { TunnelProvider } from '../types.ts'

const CONTAINER_NAME = 'shuttle-cloudflared'

export class CloudflareTunnelProvider implements TunnelProvider {
	constructor(private readonly ssh: SSHManager) {}

	async start(host: string, config: ShuttleConfig): Promise<void> {
		const tokenEnv = config.tunnel?.token_env ?? 'CLOUDFLARE_TUNNEL_TOKEN'
		const token = process.env[tokenEnv]
		if (!token) {
			throw new DeployError(
				`Cloudflare Tunnel token not found in env var ${tokenEnv}. Set it or configure tunnel.token_env in shuttle.yml`,
				'tunnel',
			)
		}
		await this.ssh.exec(host, `docker rm -f ${CONTAINER_NAME} 2>/dev/null`)
		const { code, stderr } = await this.ssh.exec(
			host,
			[
				'docker run -d',
				`--name ${CONTAINER_NAME}`,
				'--restart always --network host',
				'cloudflare/cloudflared:latest',
				'tunnel --no-autoupdate run',
				`--token ${shellEscape(token)}`,
			].join(' '),
		)
		if (code !== 0) {
			throw new DeployError(
				`Failed to start Cloudflare Tunnel on ${host}: ${stderr.trim()}`,
				'tunnel',
			)
		}
		logger.success(`Cloudflare Tunnel started on ${host}`)
	}

	async stop(host: string, _config: ShuttleConfig): Promise<void> {
		const { code } = await this.ssh.exec(host, `docker rm -f ${CONTAINER_NAME}`)
		if (code !== 0) {
			logger.warn(`Could not stop Cloudflare Tunnel container on ${host}`)
		} else {
			logger.success(`Cloudflare Tunnel stopped on ${host}`)
		}
	}

	async getStatus(host: string): Promise<string> {
		const { stdout, code } = await this.ssh.exec(
			host,
			`docker inspect ${CONTAINER_NAME} --format '{{.State.Status}}' 2>/dev/null`,
		)
		return code === 0 ? stdout.trim() : 'not running'
	}
}

import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { logger } from '../../utils/logger.ts'
import { shellEscape } from '../../utils/shell.ts'
import type { RegistryProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// GHCRRegistry
// ---------------------------------------------------------------------------

export class GHCRRegistry implements RegistryProvider {
	constructor(
		private readonly docker: DockerManager,
		private readonly ssh: SSHManager,
	) {}

	async resolve(config: ShuttleConfig, tag: string): Promise<string> {
		if (config.image) return config.image
		const url = config.registry?.url ?? 'ghcr.io'
		return `${url}/${tag}`
	}

	async distribute(host: string, imageRef: string, config: ShuttleConfig): Promise<void> {
		const username = config.registry?.username
		const passwordEnv = config.registry?.password_env

		if (username && passwordEnv) {
			const password = process.env[passwordEnv]
			if (!password) {
				throw new Error(`Environment variable ${passwordEnv} is not set (required for GHCR login)`)
			}
			const url = config.registry?.url ?? 'ghcr.io'
			logger.debug(`Logging into ${url} on ${host}`)
			const loginCmd = `echo ${shellEscape(password)} | docker login ${shellEscape(url)} -u ${shellEscape(username)} --password-stdin`
			const { code, stderr } = await this.ssh.exec(host, loginCmd)
			if (code !== 0) {
				throw new Error(`Docker login failed on ${host}: ${stderr.trim()}`)
			}
		}

		await this.docker.pull(host, imageRef)
		logger.success(`Image ${imageRef} pulled on ${host}`)
	}
}

export function createGHCRRegistry(docker: DockerManager, ssh: SSHManager): RegistryProvider {
	return new GHCRRegistry(docker, ssh)
}

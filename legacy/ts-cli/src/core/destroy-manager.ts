import type { ShuttleConfig } from '../config/schema.ts'
import { logger } from '../utils/logger.ts'
import type { DockerManager } from './docker-manager.ts'
import { docker as defaultDocker } from './docker-manager.ts'
import type { ProxyManager } from './proxy-manager.ts'
import { proxy as defaultProxy } from './proxy-manager.ts'
import type { RuntimeManager } from './runtime-manager.ts'
import { runtime as defaultRuntime } from './runtime-manager.ts'
import type { SSHManager } from './ssh-manager.ts'
import { ssh as defaultSsh } from './ssh-manager.ts'

export class DestroyManager {
	constructor(
		private readonly docker: DockerManager = defaultDocker,
		private readonly proxy: ProxyManager = defaultProxy,
		private readonly runtime: RuntimeManager = defaultRuntime,
		private readonly ssh: SSHManager = defaultSsh,
	) {}

	async destroy(config: ShuttleConfig, host: string): Promise<void> {
		const prefix = `${config.app}_`
		const containers = await this.docker.listContainers(host, prefix)

		for (const container of containers) {
			logger.info(`Removing container "${container}" on ${host}`)
			await this.docker.stop(host, container)
		}

		const images = await this.docker.listImages(host, `shuttle/${config.app}:*`)
		await this.docker.removeImages(host, images)

		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		await this.proxy.removeDomains(host, domains)

		await this.ssh.exec(host, `rm -rf ${this.runtime.getAppDir(config.app)}`)
		logger.success(`Destroyed "${config.app}" on ${host}`)
	}
}

export const destroyer = new DestroyManager(defaultDocker, defaultProxy, defaultRuntime, defaultSsh)

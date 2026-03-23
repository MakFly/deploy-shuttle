import { createHash } from 'node:crypto'
import type { AccessoryConfig, ShuttleConfig } from '../config/schema.ts'
import { logger } from '../utils/logger.ts'
import type { DockerManager } from './docker-manager.ts'
import { docker as defaultDocker } from './docker-manager.ts'

interface DockerInspectContainer {
	Config?: {
		Labels?: Record<string, string>
	}
}

function createAccessoryHash(config: AccessoryConfig): string {
	return createHash('sha256').update(JSON.stringify(config)).digest('hex')
}

export class AccessoryManager {
	constructor(private readonly docker: DockerManager = defaultDocker) {}

	private buildPortMapping(config: AccessoryConfig): string | undefined {
		if (config.port === undefined) {
			return undefined
		}

		if (typeof config.port === 'number') {
			return `127.0.0.1:${config.port}:${config.port}`
		}

		return config.port.includes(':') ? config.port : `127.0.0.1:${config.port}:${config.port}`
	}

	async ensureAccessory(
		host: string,
		app: string,
		name: string,
		config: AccessoryConfig,
	): Promise<void> {
		const containerName = `${app}_${name}`
		const desiredHash = createAccessoryHash(config)
		const existing = await this.docker.inspect<DockerInspectContainer>(host, containerName)
		const currentHash = existing?.Config?.Labels?.['shuttle.config-hash']

		if (currentHash === desiredHash) {
			logger.debug(`Accessory "${containerName}" already matches desired configuration`)
			return
		}

		await this.docker.stop(host, containerName)

		await this.docker.pull(host, config.image)

		await this.docker.run(host, {
			name: containerName,
			image: config.image,
			port: this.buildPortMapping(config),
			env: config.env,
			volumes: config.volumes,
			labels: {
				'shuttle.kind': 'accessory',
				'shuttle.config-hash': desiredHash,
			},
		})
	}

	async ensureAccessories(host: string, config: ShuttleConfig): Promise<void> {
		for (const [name, accessoryConfig] of Object.entries(config.accessories ?? {})) {
			logger.info(`Ensuring accessory "${name}" on ${host}`)
			await this.ensureAccessory(host, config.app, name, accessoryConfig)
		}
	}
}

export const accessories = new AccessoryManager(defaultDocker)

import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type { RegistryProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// ImageRefRegistry
// ---------------------------------------------------------------------------

/**
 * Provider for pre-built images declared via `image:` in the config.
 * Resolves the image reference directly from config and pulls it on the remote host.
 */
export class ImageRefRegistry implements RegistryProvider {
	constructor(private readonly docker: DockerManager) {}

	async resolve(config: ShuttleConfig, _tag: string): Promise<string> {
		if (!config.image) {
			throw new Error('ImageRefRegistry requires config.image to be set')
		}
		return config.image
	}

	async distribute(host: string, imageRef: string, _config: ShuttleConfig): Promise<void> {
		await this.docker.pull(host, imageRef)
	}
}

export function createImageRefRegistry(docker: DockerManager): RegistryProvider {
	return new ImageRefRegistry(docker)
}

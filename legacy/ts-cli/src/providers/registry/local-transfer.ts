import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type { RegistryProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// LocalTransferRegistry
// ---------------------------------------------------------------------------

export class LocalTransferRegistry implements RegistryProvider {
	constructor(private readonly docker: DockerManager) {}

	/**
	 * Returns the image tag as-is. For local transfer the tag is already the
	 * fully-qualified local reference.
	 */
	async resolve(_config: ShuttleConfig, tag: string): Promise<string> {
		return tag
	}

	/**
	 * Builds the image locally (using config.build options) then transfers it
	 * to the remote host via `docker save | ssh docker load`.
	 */
	async distribute(host: string, imageRef: string, config: ShuttleConfig): Promise<void> {
		await this.docker.build({
			dockerfile: config.build?.dockerfile,
			context: config.build?.context,
			target: config.build?.target,
			platform: config.build?.platform,
			tag: imageRef,
			args: config.build?.args,
		})

		await this.docker.transfer(imageRef, host)
	}
}

export function createLocalTransferRegistry(docker: DockerManager): LocalTransferRegistry {
	return new LocalTransferRegistry(docker)
}

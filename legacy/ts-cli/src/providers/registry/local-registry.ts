import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { DeployError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import type { RegistryProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// LocalRegistryProvider
// ---------------------------------------------------------------------------

const REGISTRY_PORT = 5080
const REGISTRY_CONTAINER = 'shuttle-registry'
const REGISTRY_IMAGE = 'registry:2'

/**
 * Ephemeral local registry + SSH tunnel for Swarm multi-node deploys
 * without requiring an external registry (GHCR, DockerHub).
 *
 * Flow:
 * 1. Start a local registry container (port 5080)
 * 2. Build and push the image to localhost:5080
 * 3. Open an SSH reverse tunnel so the remote can reach the registry
 * 4. Remote pulls the image via the tunnel
 * 5. Cleanup: close tunnel, stop registry
 */
export class LocalRegistryProvider implements RegistryProvider {
	constructor(
		private readonly docker: DockerManager,
		private readonly ssh: SSHManager,
	) {}

	async resolve(_config: ShuttleConfig, tag: string): Promise<string> {
		return `127.0.0.1:${REGISTRY_PORT}/${tag}`
	}

	async distribute(host: string, imageRef: string, config: ShuttleConfig): Promise<void> {
		const registryRef = `127.0.0.1:${REGISTRY_PORT}/${imageRef}`

		try {
			// Step 1: Start local registry
			logger.debug('Starting ephemeral local registry...')
			await this.startLocalRegistry()

			// Step 2: Build image
			logger.debug(`Building image ${imageRef}...`)
			await this.docker.build({
				dockerfile: config.build?.dockerfile,
				context: config.build?.context,
				target: config.build?.target,
				platform: config.build?.platform,
				tag: imageRef,
				args: config.build?.args,
			})

			// Step 3: Tag and push to local registry
			logger.debug(`Pushing ${registryRef} to local registry...`)
			await this.tagAndPush(imageRef, registryRef)

			// Step 4: Open SSH reverse tunnel (remote:5080 → local:5080)
			logger.debug(`Opening SSH tunnel to ${host}...`)
			await this.openTunnel(host)

			// Step 5: Pull on remote via tunnel
			logger.debug(`Pulling image on ${host} via tunnel...`)
			await this.docker.pull(host, registryRef)

			// Step 6: Re-tag on remote to the original tag (without registry prefix)
			const { code } = await this.ssh.exec(host, `docker tag ${registryRef} ${imageRef}`)
			if (code !== 0) {
				throw new DeployError(`Failed to re-tag image on ${host}`, 'deploy')
			}

			logger.success(`Image ${imageRef} distributed to ${host} via local registry`)
		} finally {
			// Cleanup: always stop registry and close tunnel
			await this.cleanup(host)
		}
	}

	private async startLocalRegistry(): Promise<void> {
		// Stop any existing registry first
		const stopResult = Bun.spawnSync(['docker', 'stop', REGISTRY_CONTAINER])
		if (stopResult.exitCode === 0) {
			Bun.spawnSync(['docker', 'rm', REGISTRY_CONTAINER])
		}

		const result = Bun.spawnSync([
			'docker',
			'run',
			'--detach',
			'--name',
			REGISTRY_CONTAINER,
			'--publish',
			`${REGISTRY_PORT}:5000`,
			'--restart',
			'no',
			REGISTRY_IMAGE,
		])

		if (result.exitCode !== 0) {
			throw new DeployError(
				`Failed to start local registry: ${result.stderr.toString().trim()}`,
				'deploy',
			)
		}

		// Wait for registry to be ready
		for (let i = 0; i < 10; i++) {
			try {
				const response = await fetch(`http://127.0.0.1:${REGISTRY_PORT}/v2/`)
				if (response.ok) return
			} catch {
				// Not ready yet
			}
			await new Promise((resolve) => setTimeout(resolve, 500))
		}

		throw new DeployError('Local registry failed to start within 5s', 'deploy')
	}

	private async tagAndPush(imageRef: string, registryRef: string): Promise<void> {
		const tagResult = Bun.spawnSync(['docker', 'tag', imageRef, registryRef])
		if (tagResult.exitCode !== 0) {
			throw new DeployError(`Failed to tag image: ${tagResult.stderr.toString().trim()}`, 'deploy')
		}

		const pushResult = Bun.spawnSync(['docker', 'push', registryRef])
		if (pushResult.exitCode !== 0) {
			throw new DeployError(
				`Failed to push to local registry: ${pushResult.stderr.toString().trim()}`,
				'deploy',
			)
		}
	}

	private async openTunnel(host: string): Promise<void> {
		// Use SSH port forwarding: -R remotePort:localhost:localPort
		// Run in background, will be killed in cleanup
		const proc = Bun.spawn([
			'ssh',
			'-o',
			'StrictHostKeyChecking=no',
			'-N',
			'-R',
			`${REGISTRY_PORT}:127.0.0.1:${REGISTRY_PORT}`,
			host,
		])

		// Store PID for cleanup
		this._tunnelPid = proc.pid

		// Wait for tunnel to be established
		await new Promise((resolve) => setTimeout(resolve, 2000))
	}

	private _tunnelPid: number | null = null

	private async cleanup(host: string): Promise<void> {
		// Kill SSH tunnel if we spawned one
		if (this._tunnelPid !== null) {
			try {
				process.kill(this._tunnelPid, 'SIGTERM')
			} catch {
				// Already dead
			}
			this._tunnelPid = null
		}

		// Remove registry-tagged images on remote to save space
		await this.ssh.exec(host, `docker rmi 127.0.0.1:${REGISTRY_PORT}/* 2>/dev/null || true`)

		// Stop local registry
		Bun.spawnSync(['docker', 'stop', REGISTRY_CONTAINER])
		Bun.spawnSync(['docker', 'rm', REGISTRY_CONTAINER])

		logger.debug('Local registry cleaned up')
	}
}

export function createLocalRegistryProvider(
	docker: DockerManager,
	ssh: SSHManager,
): LocalRegistryProvider {
	return new LocalRegistryProvider(docker, ssh)
}

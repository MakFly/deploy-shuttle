import type { ShuttleConfig } from '../config/schema.ts'
import { DeployError } from '../utils/errors.ts'
import { checkHttp, checkTcp, waitForHealth } from '../utils/health.ts'
import { logger } from '../utils/logger.ts'
import type { DeployManager, DeployState } from './deploy-manager.ts'
import { deployer as defaultDeployer } from './deploy-manager.ts'
import type { DockerManager } from './docker-manager.ts'
import { docker as defaultDocker } from './docker-manager.ts'
import type { ProxyManager } from './proxy-manager.ts'
import { proxy as defaultProxy } from './proxy-manager.ts'
import type { RuntimeManager } from './runtime-manager.ts'
import { runtime as defaultRuntime } from './runtime-manager.ts'

// ---------------------------------------------------------------------------
// RollbackManager
// ---------------------------------------------------------------------------

export class RollbackManager {
	constructor(
		private readonly docker: DockerManager = defaultDocker,
		private readonly proxy: ProxyManager = defaultProxy,
		private readonly runtime: RuntimeManager = defaultRuntime,
		private readonly deployer: DeployManager = defaultDeployer,
	) {}

	/**
	 * Lists available image tags for an app on a remote host.
	 */
	async listVersions(host: string, app: string): Promise<string[]> {
		return this.docker.listImages(host, `shuttle/${app}:deploy-*`)
	}

	/**
	 * Rolls back a deployment to a previous or explicitly specified image tag.
	 *
	 * Steps:
	 *  1. Resolve target tag from state.json or explicit argument
	 *  2. Start container using existing image (no build/transfer)
	 *  3. Wait for health check
	 *  4. Switch proxy to new container
	 *  5. Drain old container
	 *  6. Update state.json
	 */
	async rollback(config: ShuttleConfig, host: string, targetTag?: string): Promise<void> {
		const TOTAL_STEPS = 6

		// ── Step 1: Resolve target tag ────────────────────────────────────────────
		logger.step(1, TOTAL_STEPS, 'Resolving rollback target')

		const state = await this.deployer.readState(host, config.app)

		const rollbackTag = targetTag ?? state.previous_tag

		if (rollbackTag === undefined) {
			throw new DeployError(
				`No previous version available to roll back to for "${config.app}" on ${host}`,
				'rollback',
			)
		}

		logger.info(`Rolling back to ${rollbackTag}`)

		// Determine slots — put rollback on the inactive slot
		const currentSlot = state.active_slot
		const newSlot: 'blue' | 'green' = currentSlot === 'blue' ? 'green' : 'blue'

		const services = Object.entries(config.services ?? {})
		const envFilePath = `${this.runtime.getWorkDir(config.app)}/.env`
		const drainTimeout = config.deploy?.blue_green?.drain_timeout ?? 30

		// Use serviceIndex 0 for proxy switch port (first service)
		const firstServicePort = this.deployer.getServicePort(0, newSlot)
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]

		// ── Step 2: Start containers from existing image ──────────────────────────
		logger.step(2, TOTAL_STEPS, `Starting rollback containers on ${newSlot} slot`)

		for (let i = 0; i < services.length; i++) {
			const [serviceName] = services[i]
			const newPort = this.deployer.getServicePort(i, newSlot)
			const newContainerName = `${config.app}_${serviceName}_${newSlot}`
			const servicePort = config.services?.[serviceName]?.port ?? 3000

			// Remove stale container if any
			await this.docker.stop(host, newContainerName)

			await this.docker.run(host, {
				name: newContainerName,
				image: rollbackTag,
				port: `127.0.0.1:${newPort}:${servicePort}`,
				envFile: envFilePath,
				command: config.services?.[serviceName]?.command,
			})
		}

		// ── Step 3: Health check (first service) ──────────────────────────────────
		logger.step(3, TOTAL_STEPS, 'Waiting for health check')

		if (services.length > 0) {
			const [firstServiceName] = services[0]
			const firstContainerName = `${config.app}_${firstServiceName}_${newSlot}`
			const healthcheck = config.services?.[firstServiceName]?.healthcheck

			if (healthcheck !== undefined) {
				const hcTimeoutMs = (healthcheck.timeout ?? 60) * 1000

				let check: () => Promise<boolean>
				if (healthcheck.type === 'http') {
					const hcPath = healthcheck.path ?? '/'
					check = () => checkHttp(`http://127.0.0.1:${firstServicePort}${hcPath}`, hcTimeoutMs)
				} else if (healthcheck.type === 'exec') {
					check = async () => {
						const { code } = await this.docker.exec(host, firstContainerName, healthcheck.command)
						return code === 0
					}
				} else {
					check = () => checkTcp('127.0.0.1', firstServicePort, hcTimeoutMs)
				}

				await waitForHealth(check, {
					timeout: hcTimeoutMs,
					retries: healthcheck.retries ?? 5,
					interval: healthcheck.interval ?? 2000,
				})
			}
		}

		// ── Step 4: Switch proxy (once, using first service port) ─────────────────
		logger.step(4, TOTAL_STEPS, 'Switching proxy upstream')

		await this.proxy.switchUpstream(host, domains[0], firstServicePort)

		// ── Step 5: Drain old containers ──────────────────────────────────────────
		logger.step(5, TOTAL_STEPS, 'Draining old containers')

		for (const [serviceName] of services) {
			const oldContainerName = `${config.app}_${serviceName}_${currentSlot}`
			await this.docker.stop(host, oldContainerName, drainTimeout)
		}

		// ── Step 6: Update state ──────────────────────────────────────────────────
		logger.step(6, TOTAL_STEPS, 'Updating deployment state')

		const newState: DeployState = {
			active_slot: newSlot,
			active_tag: rollbackTag,
			previous_tag: state.active_tag,
			port: firstServicePort,
			deployed_at: new Date().toISOString(),
			version: state.version + 1,
		}

		await this.deployer.writeState(host, config.app, newState)

		logger.success(`Rolled back "${config.app}" on ${host} to ${rollbackTag} (slot ${newSlot})`)
	}

	/**
	 * Removes old image tags on the remote, keeping the N most recent.
	 */
	async prune(host: string, app: string, keep: number): Promise<void> {
		logger.info(`Pruning old images for "${app}" on ${host}, keeping ${keep}`)
		await this.docker.prune(host, keep, `shuttle/${app}:deploy-`)
		logger.success(`Pruned images for "${app}" on ${host}`)
	}
}

export const rollback = new RollbackManager(
	defaultDocker,
	defaultProxy,
	defaultRuntime,
	defaultDeployer,
)

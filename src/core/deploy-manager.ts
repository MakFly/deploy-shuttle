import type { ShuttleConfig } from '../config/schema.ts'
import { resolveProxy, resolveRegistry, resolveSecrets } from '../providers/resolver.ts'
import { BlueGreenStrategy } from '../providers/strategy/blue-green.ts'
import { RollingStrategy } from '../providers/strategy/rolling.ts'
import { SwarmStrategy } from '../providers/strategy/swarm.ts'
import type { DeployStrategy } from '../providers/types.ts'
import { DeployError } from '../utils/errors.ts'
import { logger } from '../utils/logger.ts'
import type { AccessoryManager } from './accessory-manager.ts'
import { accessories as defaultAccessories } from './accessory-manager.ts'
import type { DockerManager } from './docker-manager.ts'
import { docker as defaultDocker } from './docker-manager.ts'
import type { HostTask, ParallelResult } from './parallel.ts'
import { executeParallel, flattenServerGroups } from './parallel.ts'
import type { ProxyManager } from './proxy-manager.ts'
import { proxy as defaultProxy } from './proxy-manager.ts'
import type { DeployState, InFlightDeployState, RuntimeManager } from './runtime-manager.ts'
import { runtime as defaultRuntime } from './runtime-manager.ts'
import type { SecretsManager } from './secrets-manager.ts'
import { secrets as defaultSecrets } from './secrets-manager.ts'
import type { SSHManager } from './ssh-manager.ts'
import { ssh as defaultSsh } from './ssh-manager.ts'

export type { DeployState } from './runtime-manager.ts'

export interface DeployOptions {
	skipBuild?: boolean
	dryRun?: boolean
}

const BASE_PORT = 10000
const BLUE_OFFSET = 0
const GREEN_OFFSET = 1

export class DeployManager {
	constructor(
		private readonly ssh: SSHManager = defaultSsh,
		private readonly docker: DockerManager = defaultDocker,
		private readonly proxy: ProxyManager = defaultProxy,
		private readonly runtime: RuntimeManager = defaultRuntime,
		private readonly secrets: SecretsManager = defaultSecrets,
		private readonly accessories: AccessoryManager = defaultAccessories,
	) {}

	getServicePort(serviceIndex: number, slot: 'blue' | 'green'): number {
		const offset = slot === 'blue' ? BLUE_OFFSET : GREEN_OFFSET
		return BASE_PORT + serviceIndex * 2 + offset
	}

	private async createInFlightState(
		config: ShuttleConfig,
		host: string,
		stage: string,
		status: InFlightDeployState['status'],
		service?: string,
		error?: string,
	): Promise<InFlightDeployState> {
		const now = new Date().toISOString()
		let startedAt = now
		try {
			const { stdout, code } = await this.ssh.exec(
				host,
				`cat ${this.runtime.getInFlightPath(config.app)}`,
			)
			if (code === 0) {
				const existing = JSON.parse(stdout) as Partial<InFlightDeployState>
				if (typeof existing.started_at === 'string' && existing.started_at.length > 0) {
					startedAt = existing.started_at
				}
			}
		} catch {
			// Keep the new timestamp when no prior in-flight state exists.
		}

		return {
			status,
			app: config.app,
			host,
			strategy: config.deploy?.strategy ?? 'blue-green',
			service,
			stage,
			error,
			started_at: startedAt,
			updated_at: now,
		}
	}

	private async updateInFlight(
		config: ShuttleConfig,
		host: string,
		stage: string,
		status: InFlightDeployState['status'],
		service?: string,
		error?: string,
	): Promise<void> {
		await this.runtime.writeInFlight(
			host,
			config.app,
			await this.createInFlightState(config, host, stage, status, service, error),
		)
	}

	private async runPostDeployHooks(config: ShuttleConfig, host: string): Promise<void> {
		for (const hook of config.deploy?.hooks?.post_deploy ?? []) {
			try {
				await this.runtime.runHook(host, config.app, hook, 'post_deploy')
			} catch (err) {
				logger.warn(err instanceof Error ? err.message : String(err))
			}
		}
	}

	private createStrategy(name: string, config: ShuttleConfig): DeployStrategy {
		const registry = resolveRegistry(config, this.docker, this.ssh)
		const proxy = resolveProxy(config, this.ssh)
		const secrets = resolveSecrets(config)
		const deps = {
			docker: this.docker,
			ssh: this.ssh,
			runtime: this.runtime,
			secrets,
			registry,
			proxy,
		}

		switch (name) {
			case 'rolling':
				return new RollingStrategy(deps)
			case 'swarm':
				return new SwarmStrategy(deps)
			default:
				return new BlueGreenStrategy(deps)
		}
	}

	private logDryRun(
		config: ShuttleConfig,
		host: string,
		strategy: 'blue-green' | 'rolling' | 'swarm',
	): void {
		logger.info(`[dry-run] Host ${host}`)
		logger.info(`[dry-run] Strategy: ${strategy}`)
		logger.info(`[dry-run] Services: ${Object.keys(config.services ?? {}).join(', ')}`)
		if (Object.keys(config.accessories ?? {}).length > 0) {
			logger.info(`[dry-run] Accessories: ${Object.keys(config.accessories ?? {}).join(', ')}`)
		}
		if ((config.deploy?.hooks?.pre_deploy?.length ?? 0) > 0) {
			logger.info(
				`[dry-run] pre_deploy hooks: ${(config.deploy?.hooks?.pre_deploy ?? []).join(' | ')}`,
			)
		}
		if ((config.deploy?.hooks?.post_deploy?.length ?? 0) > 0) {
			logger.info(
				`[dry-run] post_deploy hooks: ${(config.deploy?.hooks?.post_deploy ?? []).join(' | ')}`,
			)
		}
	}

	private async deployToHost(
		config: ShuttleConfig,
		host: string,
		user: string,
		options: DeployOptions,
	): Promise<void> {
		const strategy = config.deploy?.strategy ?? 'blue-green'
		const services = config.services ?? {}

		await this.ssh.connect({ host, user })

		try {
			await this.runtime.acquireLock(host, config.app)
			await this.updateInFlight(config, host, 'initializing', 'running')

			await this.updateInFlight(config, host, 'ensuring accessories', 'running')
			await this.accessories.ensureAccessories(host, config)

			await this.updateInFlight(config, host, 'running pre-deploy hooks', 'running')
			for (const hook of config.deploy?.hooks?.pre_deploy ?? []) {
				await this.runtime.runHook(host, config.app, hook, 'pre_deploy')
			}

			const serviceEntries = Object.entries(services)
			for (let i = 0; i < serviceEntries.length; i++) {
				const [serviceName] = serviceEntries[i]
				logger.info(`Deploying service "${serviceName}" to ${host} (${strategy})`)
				await this.updateInFlight(config, host, `deploying ${serviceName}`, 'running', serviceName)

				const strategyImpl = this.createStrategy(strategy, config)
				await strategyImpl.execute({
					config,
					host,
					service: serviceName,
					serviceIndex: i,
					tag: '',
					options,
				})
			}

			await this.updateInFlight(config, host, 'running post-deploy hooks', 'running')
			await this.runPostDeployHooks(config, host)
			await this.runtime.clearInFlight(host, config.app)
		} catch (err) {
			await this.updateInFlight(
				config,
				host,
				'failed',
				'failed',
				undefined,
				err instanceof Error ? err.message : String(err),
			)
			throw err
		} finally {
			try {
				await this.runtime.releaseLock(host, config.app)
			} catch (err) {
				logger.warn(
					`Failed to release deploy lock for "${config.app}" on ${host}: ${
						err instanceof Error ? err.message : String(err)
					}`,
				)
			}
			this.ssh.disconnect(host)
		}
	}

	async deploy(config: ShuttleConfig, options: DeployOptions = {}): Promise<void> {
		const strategy = config.deploy?.strategy ?? 'blue-green'
		const services = config.services ?? {}

		if (Object.keys(services).length === 0) {
			throw new DeployError('No services defined in shuttle config', 'validate')
		}

		// Dry-run path: sequential, no parallelism
		if (options.dryRun) {
			for (const [, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					this.logDryRun(config, host, strategy)
				}
			}
			return
		}

		const concurrency = config.deploy?.concurrency ?? 5
		const flatHosts = flattenServerGroups(config.servers)

		const tasks: HostTask<void>[] = flatHosts.map(({ host, user, group }) => ({
			host,
			user,
			group,
			execute: () => this.deployToHost(config, host, user, options),
		}))

		const results: ParallelResult<void>[] = await executeParallel(tasks, concurrency)

		const failed = results.filter((r) => r.error !== undefined)
		if (failed.length > 0) {
			const details = failed
				.map((r) => `  - ${r.host} (${r.group}): ${r.error?.message ?? 'unknown error'}`)
				.join('\n')
			throw new DeployError(`Deploy failed on ${failed.length} host(s):\n${details}`, 'deploy')
		}
	}

	async readState(host: string, app: string): Promise<DeployState> {
		return this.runtime.readState(host, app)
	}

	async writeState(host: string, app: string, state: DeployState): Promise<void> {
		await this.runtime.writeState(host, app, state)
	}
}

export const deployer = new DeployManager(
	defaultSsh,
	defaultDocker,
	defaultProxy,
	defaultRuntime,
	defaultSecrets,
	defaultAccessories,
)

import type { ShuttleConfig } from '../config/schema.ts'
import { DeployError } from '../utils/errors.ts'
import { checkHttp, checkTcp, waitForHealth } from '../utils/health.ts'
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

	private async getShortSha(): Promise<string> {
		try {
			const proc = Bun.spawn(['git', 'rev-parse', '--short', 'HEAD'], { stdout: 'pipe' })
			return (await new Response(proc.stdout).text()).trim()
		} catch {
			return Date.now().toString(36)
		}
	}

	private async sleep(ms: number): Promise<void> {
		await new Promise((resolve) => setTimeout(resolve, ms))
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

	private buildHealthCheck(
		healthcheck: NonNullable<NonNullable<ShuttleConfig['services']>[string]['healthcheck']>,
		host: string,
		container: string,
		port: number,
		timeoutMs: number,
	): () => Promise<boolean> {
		if (healthcheck.type === 'http') {
			const hcPath = healthcheck.path ?? '/'
			return () => checkHttp(`http://127.0.0.1:${port}${hcPath}`, timeoutMs)
		}
		if (healthcheck.type === 'exec') {
			return async () => {
				const { code } = await this.docker.exec(host, container, healthcheck.command)
				return code === 0
			}
		}
		return () => checkTcp('127.0.0.1', port, timeoutMs)
	}

	private async runHealthCheck(
		config: ShuttleConfig,
		host: string,
		service: string,
		containerName: string,
		port: number,
	): Promise<void> {
		const healthcheck = config.services?.[service]?.healthcheck
		const healthTimeout = config.deploy?.timeout ?? 60

		if (healthcheck === undefined) {
			return
		}

		const readinessDelayMs = (config.deploy?.blue_green?.readiness_delay ?? 0) * 1000
		if (readinessDelayMs > 0) {
			await this.sleep(readinessDelayMs)
		}

		const hcTimeoutMs = (healthcheck.timeout ?? healthTimeout) * 1000
		const check = this.buildHealthCheck(healthcheck, host, containerName, port, hcTimeoutMs)

		const healthy = await waitForHealth(check, {
			timeout: hcTimeoutMs,
			retries: healthcheck.retries ?? 5,
			interval: healthcheck.interval ?? 2000,
		})

		if (!healthy) {
			throw new DeployError(`Health check did not pass for "${service}" on ${host}`, 'healthcheck')
		}
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

	private logDryRun(config: ShuttleConfig, host: string, strategy: 'blue-green' | 'rolling'): void {
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
				await this.updateInFlight(
					config,
					host,
					`deploying ${serviceName}`,
					'running',
					serviceName,
				)

				if (strategy === 'rolling') {
					await this.rollingDeploy(config, host, serviceName, i, options)
				} else {
					await this.blueGreenDeploy(config, host, serviceName, i, options)
				}
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
			throw new DeployError(
				`Deploy failed on ${failed.length} host(s):\n${details}`,
				'deploy',
			)
		}
	}

	async blueGreenDeploy(
		config: ShuttleConfig,
		host: string,
		service: string,
		serviceIndex: number,
		options: DeployOptions = {},
	): Promise<void> {
		const TOTAL_STEPS = 12

		logger.step(1, TOTAL_STEPS, 'Reading deployment state')

		let state: DeployState | null = null
		try {
			state = await this.runtime.readState(host, config.app)
		} catch {
			// First deploy.
		}

		const activeSlot = state?.active_slot ?? 'blue'
		const newSlot: 'blue' | 'green' = activeSlot === 'blue' ? 'green' : 'blue'
		const newPort = this.getServicePort(serviceIndex, newSlot)

		const newContainerName = `${config.app}_${service}_${newSlot}`
		const oldContainerName = state !== null ? `${config.app}_${service}_${activeSlot}` : null

		logger.step(2, TOTAL_STEPS, 'Generating image tag')

		const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
		const shortSha = await this.getShortSha()
		const tag = `shuttle/${config.app}:deploy-${date}-${shortSha}`

		if (!options.skipBuild) {
			logger.step(3, TOTAL_STEPS, `Building image ${tag}`)
			await this.docker.build({
				dockerfile: config.build?.dockerfile,
				context: config.build?.context,
				target: config.build?.target,
				platform: config.build?.platform,
				tag,
				args: config.build?.args,
			})

			logger.step(4, TOTAL_STEPS, `Transferring image to ${host}`)
			await this.docker.transfer(tag, host)
		} else {
			logger.step(3, TOTAL_STEPS, 'Skipping build (--skip-build)')
			logger.step(4, TOTAL_STEPS, 'Skipping transfer (--skip-build)')
		}

		logger.step(5, TOTAL_STEPS, 'Uploading environment')

		try {
			await this.secrets.push(host, config.app)
		} catch (err) {
			logger.warn(
				`No secrets to push or push failed — continuing: ${err instanceof Error ? err.message : String(err)}`,
			)
		}

		const envFilePath = `${this.runtime.getWorkDir(config.app)}/.env`

		logger.step(6, TOTAL_STEPS, `Starting container on ${newSlot} slot (port ${newPort})`)

		await this.docker.stop(host, newContainerName)
		await this.docker.run(host, {
			name: newContainerName,
			image: tag,
			port: `127.0.0.1:${newPort}:${config.services?.[service]?.port ?? 3000}`,
			envFile: envFilePath,
			command: config.services?.[service]?.command,
			labels: {
				'shuttle.kind': 'service',
				'shuttle.service': service,
			},
		})

		logger.step(7, TOTAL_STEPS, 'Waiting for health check')

		try {
			await this.runHealthCheck(config, host, service, newContainerName, newPort)
		} catch (err) {
			if (config.deploy?.auto_rollback ?? true) {
				await this.docker.stop(host, newContainerName)
			}
			throw DeployError.wrap(err, `Health check failed for "${service}" on ${host}`, 'healthcheck')
		}

		logger.step(8, TOTAL_STEPS, 'Switching proxy upstream')

		const domains = Array.isArray(config.domain) ? config.domain[0] : config.domain
		await this.proxy.switchUpstream(host, domains, newPort)

		logger.step(9, TOTAL_STEPS, 'Updating deployment state')

		const newState: DeployState = {
			active_slot: newSlot,
			active_tag: tag,
			previous_tag: state?.active_tag,
			port: newPort,
			deployed_at: new Date().toISOString(),
			version: (state?.version ?? 0) + 1,
		}

		await this.runtime.writeState(host, config.app, newState)

		logger.step(10, TOTAL_STEPS, 'Draining old container')

		if (oldContainerName !== null) {
			const drainTimeout = config.deploy?.blue_green?.drain_timeout ?? 30
			await this.docker.stop(host, oldContainerName, drainTimeout)
		}

		logger.step(11, TOTAL_STEPS, 'Tagging images')

		await this.docker.tag(host, tag, `shuttle/${config.app}:current`)
		if (state?.active_tag !== undefined) {
			await this.docker.tag(host, state.active_tag, `shuttle/${config.app}:previous`)
		}

		logger.step(12, TOTAL_STEPS, 'Cleaning up old images')

		const retain = config.deploy?.retain ?? 3
		await this.docker.prune(host, retain, `shuttle/${config.app}:deploy-`)

		logger.success(`Service "${service}" deployed to ${host} — slot ${newSlot} on port ${newPort}`)
	}

	async rollingDeploy(
		config: ShuttleConfig,
		host: string,
		service: string,
		serviceIndex: number,
		options: DeployOptions = {},
	): Promise<void> {
		const TOTAL_STEPS = 8
		const replicas = config.services?.[service]?.replicas ?? 1

		logger.step(1, TOTAL_STEPS, 'Generating image tag')

		let state: DeployState | null = null
		try {
			state = await this.runtime.readState(host, config.app)
		} catch {
			// First deploy.
		}

		const previousTag = state?.active_tag
		const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
		const shortSha = await this.getShortSha()
		const tag = `shuttle/${config.app}:deploy-${date}-${shortSha}`

		if (!options.skipBuild) {
			logger.step(2, TOTAL_STEPS, `Building image ${tag}`)
			await this.docker.build({
				dockerfile: config.build?.dockerfile,
				context: config.build?.context,
				target: config.build?.target,
				platform: config.build?.platform,
				tag,
				args: config.build?.args,
			})

			logger.step(3, TOTAL_STEPS, 'Transferring image')
			await this.docker.transfer(tag, host)
		} else {
			logger.step(2, TOTAL_STEPS, 'Skipping build (--skip-build)')
			logger.step(3, TOTAL_STEPS, 'Skipping transfer (--skip-build)')
		}

		logger.step(4, TOTAL_STEPS, 'Uploading environment')
		try {
			await this.secrets.push(host, config.app)
		} catch (err) {
			logger.warn(
				`No secrets to push — continuing: ${err instanceof Error ? err.message : String(err)}`,
			)
		}

		const envFilePath = `${this.runtime.getWorkDir(config.app)}/.env`
		const basePort = this.getServicePort(serviceIndex, 'blue')

		for (let i = 0; i < replicas; i++) {
			const containerName = `${config.app}_${service}_${i}`
			const servicePort = config.services?.[service]?.port ?? 3000
			const hostPort = basePort + i * 2

			logger.step(5, TOTAL_STEPS, `Replacing replica ${i + 1}/${replicas} (${containerName})`)

			await this.docker.stop(host, containerName, 30)
			await this.docker.run(host, {
				name: containerName,
				image: tag,
				port: `127.0.0.1:${hostPort}:${servicePort}`,
				envFile: envFilePath,
				command: config.services?.[service]?.command,
				labels: {
					'shuttle.kind': 'service',
					'shuttle.service': service,
				},
			})

			try {
				await this.runHealthCheck(config, host, service, containerName, hostPort)
			} catch (err) {
				if ((config.deploy?.auto_rollback ?? true) && previousTag !== undefined) {
					await this.docker.stop(host, containerName)
					await this.docker.run(host, {
						name: containerName,
						image: previousTag,
						port: `127.0.0.1:${hostPort}:${servicePort}`,
						envFile: envFilePath,
						command: config.services?.[service]?.command,
						labels: {
							'shuttle.kind': 'service',
							'shuttle.service': service,
						},
					})
					await this.runHealthCheck(config, host, service, containerName, hostPort)
				}

				throw DeployError.wrap(
					err,
					`Health check failed for rolling replica ${containerName} on ${host}`,
					'healthcheck',
				)
			}
		}

		logger.step(8, TOTAL_STEPS, 'Updating deployment state')

		const newState: DeployState = {
			active_slot: 'blue',
			active_tag: tag,
			previous_tag: previousTag,
			port: basePort,
			deployed_at: new Date().toISOString(),
			version: (state?.version ?? 0) + 1,
		}

		await this.runtime.writeState(host, config.app, newState)
		logger.success(`Rolling deploy of "${service}" complete on ${host}`)
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

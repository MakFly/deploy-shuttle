import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type {
	DeployState,
	InFlightDeployState,
	RuntimeManager,
} from '../../core/runtime-manager.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { DeployError } from '../../utils/errors.ts'
import { waitForHealth } from '../../utils/health.ts'
import { logger } from '../../utils/logger.ts'
import type {
	DeployContext,
	DeployStrategy,
	ProxyProvider,
	RegistryProvider,
	SecretsProvider,
} from '../types.ts'

// ---------------------------------------------------------------------------
// BlueGreenStrategy
// ---------------------------------------------------------------------------

interface BlueGreenDeps {
	docker: DockerManager
	ssh: SSHManager
	runtime: RuntimeManager
	secrets: SecretsProvider
	registry: RegistryProvider
	proxy: ProxyProvider
}

export class BlueGreenStrategy implements DeployStrategy {
	constructor(private readonly deps: BlueGreenDeps) {}

	async execute(ctx: DeployContext): Promise<void> {
		const { config, host, service, serviceIndex, tag, options } = ctx
		const { docker, ssh, runtime, secrets, registry, proxy } = this.deps

		const TOTAL_STEPS = 12

		logger.step(1, TOTAL_STEPS, 'Reading deployment state')

		let state: DeployState | null = null
		try {
			state = await runtime.readState(host, config.app)
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
		const imageTag = `shuttle/${config.app}:deploy-${date}-${shortSha}`

		// Use provided tag if not empty, otherwise generate one
		const finalTag = tag.length > 0 ? tag : imageTag

		if (!options.skipBuild) {
			logger.step(3, TOTAL_STEPS, `Building and transferring image ${finalTag}`)
			await registry.distribute(host, finalTag, config)
		} else {
			logger.step(3, TOTAL_STEPS, 'Skipping build (--skip-build)')
			logger.step(4, TOTAL_STEPS, 'Skipping transfer (--skip-build)')
		}

		logger.step(5, TOTAL_STEPS, 'Uploading environment')

		try {
			await secrets.push(host, config.app, ssh)
		} catch (err) {
			logger.warn(
				`No secrets to push or push failed — continuing: ${err instanceof Error ? err.message : String(err)}`,
			)
		}

		const envFilePath = `${runtime.getWorkDir(config.app)}/.env`

		logger.step(6, TOTAL_STEPS, `Starting container on ${newSlot} slot (port ${newPort})`)

		await docker.stop(host, newContainerName)
		await docker.run(host, {
			name: newContainerName,
			image: finalTag,
			port: `127.0.0.1:${newPort}:${config.services?.[service]?.port ?? 3000}`,
			env: config.env?.clear,
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
				await docker.stop(host, newContainerName)
			}
			throw DeployError.wrap(err, `Health check failed for "${service}" on ${host}`, 'healthcheck')
		}

		logger.step(8, TOTAL_STEPS, 'Switching proxy upstream')

		// Bug #6 fix: pass ALL domains, not just the first
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		await proxy.switchUpstream(host, config, domains, newPort)

		logger.step(9, TOTAL_STEPS, 'Updating deployment state')

		const newState: DeployState = {
			active_slot: newSlot,
			active_tag: finalTag,
			previous_tag: state?.active_tag,
			port: newPort,
			deployed_at: new Date().toISOString(),
			version: (state?.version ?? 0) + 1,
		}

		await runtime.writeState(host, config.app, newState)

		logger.step(10, TOTAL_STEPS, 'Draining old container')

		if (oldContainerName !== null) {
			const drainTimeout = config.deploy?.blue_green?.drain_timeout ?? 30
			await docker.stop(host, oldContainerName, drainTimeout)
		}

		logger.step(11, TOTAL_STEPS, 'Tagging images')

		await docker.tag(host, finalTag, `shuttle/${config.app}:current`)
		if (state?.active_tag !== undefined) {
			await docker.tag(host, state.active_tag, `shuttle/${config.app}:previous`)
		}

		logger.step(12, TOTAL_STEPS, 'Cleaning up old images')

		const retain = config.deploy?.retain ?? 3
		await docker.prune(host, retain, `shuttle/${config.app}:deploy-`)

		logger.success(`Service "${service}" deployed to ${host} — slot ${newSlot} on port ${newPort}`)
	}

	// ---------------------------------------------------------------------------
	// Private helpers
	// ---------------------------------------------------------------------------

	private getServicePort(serviceIndex: number, slot: 'blue' | 'green'): number {
		return 10000 + serviceIndex * 2 + (slot === 'blue' ? 0 : 1)
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

	private buildHealthCheck(
		healthcheck: NonNullable<NonNullable<ShuttleConfig['services']>[string]['healthcheck']>,
		host: string,
		container: string,
		port: number,
		timeoutMs: number,
	): () => Promise<boolean> {
		if (healthcheck.type === 'http') {
			const hcPath = healthcheck.path ?? '/'
			const timeout = Math.floor(timeoutMs / 1000)
			// Critical fix: run health check REMOTELY via SSH
			return async () => {
				const { code } = await this.deps.ssh.exec(
					host,
					`curl -sf -o /dev/null -m ${timeout} http://127.0.0.1:${port}${hcPath}`,
				)
				return code === 0
			}
		}
		if (healthcheck.type === 'exec') {
			return async () => {
				const { code } = await this.deps.docker.exec(host, container, healthcheck.command)
				return code === 0
			}
		}
		// TCP: run remotely via SSH
		const timeout = Math.floor(timeoutMs / 1000)
		return async () => {
			const { code } = await this.deps.ssh.exec(
				host,
				`timeout ${timeout} bash -c 'echo > /dev/tcp/127.0.0.1/${port}' 2>/dev/null`,
			)
			return code === 0
		}
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
			const { stdout, code } = await this.deps.ssh.exec(
				host,
				`cat ${this.deps.runtime.getInFlightPath(config.app)}`,
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
		await this.deps.runtime.writeInFlight(
			host,
			config.app,
			await this.createInFlightState(config, host, stage, status, service, error),
		)
	}
}

export function createBlueGreenStrategy(deps: BlueGreenDeps): BlueGreenStrategy {
	return new BlueGreenStrategy(deps)
}

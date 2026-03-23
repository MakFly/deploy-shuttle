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
// RollingStrategy
// ---------------------------------------------------------------------------

interface RollingDeps {
	docker: DockerManager
	ssh: SSHManager
	runtime: RuntimeManager
	secrets: SecretsProvider
	registry: RegistryProvider
	proxy: ProxyProvider
}

export class RollingStrategy implements DeployStrategy {
	constructor(private readonly deps: RollingDeps) {}

	async execute(ctx: DeployContext): Promise<void> {
		const { config, host, service, serviceIndex, tag, options } = ctx
		const { docker, ssh, runtime, secrets, registry, proxy } = this.deps

		const TOTAL_STEPS = 8
		const replicas = config.services?.[service]?.replicas ?? 1

		logger.step(1, TOTAL_STEPS, 'Generating image tag')

		let state: DeployState | null = null
		try {
			state = await runtime.readState(host, config.app)
		} catch {
			// First deploy.
		}

		const previousTag = state?.active_tag
		const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
		const shortSha = await this.getShortSha()
		const imageTag = `shuttle/${config.app}:deploy-${date}-${shortSha}`

		// Use provided tag if not empty, otherwise generate one
		const finalTag = tag.length > 0 ? tag : imageTag

		if (!options.skipBuild) {
			logger.step(2, TOTAL_STEPS, `Building image ${finalTag}`)
			await registry.distribute(host, finalTag, config)
		} else {
			logger.step(2, TOTAL_STEPS, 'Skipping build (--skip-build)')
			logger.step(3, TOTAL_STEPS, 'Skipping transfer (--skip-build)')
		}

		logger.step(4, TOTAL_STEPS, 'Uploading environment')
		try {
			await secrets.push(host, config.app, ssh)
		} catch (err) {
			logger.warn(
				`No secrets to push — continuing: ${err instanceof Error ? err.message : String(err)}`,
			)
		}

		const envFilePath = `${runtime.getWorkDir(config.app)}/.env`
		const basePort = this.getServicePort(serviceIndex, 'blue')

		for (let i = 0; i < replicas; i++) {
			const containerName = `${config.app}_${service}_${i}`
			const servicePort = config.services?.[service]?.port ?? 3000
			const hostPort = basePort + i * 2

			logger.step(5, TOTAL_STEPS, `Replacing replica ${i + 1}/${replicas} (${containerName})`)

			await docker.stop(host, containerName, 30)
			await docker.run(host, {
				name: containerName,
				image: finalTag,
				port: `127.0.0.1:${hostPort}:${servicePort}`,
				env: config.env?.clear,
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
					await docker.stop(host, containerName)
					await docker.run(host, {
						name: containerName,
						image: previousTag,
						port: `127.0.0.1:${hostPort}:${servicePort}`,
						env: config.env?.clear,
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

		// Bug #10 fix: apply proxy update after all replicas replaced
		logger.step(6, TOTAL_STEPS, 'Switching proxy upstream')
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		await proxy.switchUpstream(host, config, domains, basePort)

		logger.step(7, TOTAL_STEPS, 'Tagging images')
		await docker.tag(host, finalTag, `shuttle/${config.app}:current`)
		if (previousTag !== undefined) {
			await docker.tag(host, previousTag, `shuttle/${config.app}:previous`)
		}

		logger.step(8, TOTAL_STEPS, 'Updating deployment state')

		const newState: DeployState = {
			active_slot: 'blue',
			active_tag: finalTag,
			previous_tag: previousTag,
			port: basePort,
			deployed_at: new Date().toISOString(),
			version: (state?.version ?? 0) + 1,
		}

		await runtime.writeState(host, config.app, newState)
		logger.success(`Rolling deploy of "${service}" complete on ${host}`)
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

export function createRollingStrategy(deps: RollingDeps): RollingStrategy {
	return new RollingStrategy(deps)
}

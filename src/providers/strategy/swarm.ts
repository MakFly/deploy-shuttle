import type { ShuttleConfig } from '../../config/schema.ts'
import type { DockerManager } from '../../core/docker-manager.ts'
import type {
	DeployState,
	InFlightDeployState,
	RuntimeManager,
} from '../../core/runtime-manager.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { generateSwarmCompose } from '../../templates/docker-compose.swarm.ts'
import { DeployError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import type {
	DeployContext,
	DeployStrategy,
	ProxyProvider,
	RegistryProvider,
	SecretsProvider,
} from '../types.ts'

// ---------------------------------------------------------------------------
// SwarmStrategy
// ---------------------------------------------------------------------------

interface SwarmDeps {
	docker: DockerManager
	ssh: SSHManager
	runtime: RuntimeManager
	secrets: SecretsProvider
	registry: RegistryProvider
	proxy: ProxyProvider
}

const SWARM_COMPOSE_FILENAME = 'docker-compose.swarm.yml'

export class SwarmStrategy implements DeployStrategy {
	constructor(private readonly deps: SwarmDeps) {}

	async execute(ctx: DeployContext): Promise<void> {
		const { config, host, service, tag, options } = ctx
		const { ssh, runtime, secrets, registry, proxy } = this.deps

		const TOTAL_STEPS = 8
		const swarmConfig = this.getSwarmConfig(config)

		// Step 1: Ensure Swarm is initialized
		logger.step(1, TOTAL_STEPS, 'Ensuring Docker Swarm is initialized')
		await this.ensureSwarmInit(ssh, host)

		// Step 2: Build & distribute image
		const date = new Date().toISOString().slice(0, 10).replace(/-/g, '')
		const shortSha = await this.getShortSha()
		const imageTag = `shuttle/${config.app}:deploy-${date}-${shortSha}`
		const finalTag = tag.length > 0 ? tag : imageTag

		if (!options.skipBuild) {
			logger.step(2, TOTAL_STEPS, `Building and distributing image ${finalTag}`)
			await registry.distribute(host, finalTag, config)
		} else {
			logger.step(2, TOTAL_STEPS, 'Skipping build (--skip-build)')
		}

		// Step 3: Upload secrets
		logger.step(3, TOTAL_STEPS, 'Uploading environment')
		try {
			await secrets.push(host, config.app, ssh)
		} catch (err) {
			logger.warn(
				`No secrets to push — continuing: ${err instanceof Error ? err.message : String(err)}`,
			)
		}

		const envFilePath = `${runtime.getWorkDir(config.app)}/.env`

		// Step 4: Generate and upload Swarm compose file
		logger.step(4, TOTAL_STEPS, 'Generating Swarm compose file')

		const servicePort = config.services?.[service]?.port
		const composeContent = generateSwarmCompose({
			stackName: config.app,
			services: [
				{
					name: service,
					image: finalTag,
					port: servicePort,
					command: config.services?.[service]?.command,
					envFile: envFilePath,
					replicas: swarmConfig.replicas,
					updateParallelism: swarmConfig.update_parallelism,
					updateDelay: swarmConfig.update_delay,
					updateOrder: swarmConfig.update_order,
					monitor: swarmConfig.monitor,
				},
			],
		})

		const composePath = `${runtime.getWorkDir(config.app)}/${SWARM_COMPOSE_FILENAME}`
		await ssh.uploadContent(host, composeContent, composePath, 0o644)

		// Step 5: Deploy stack
		logger.step(5, TOTAL_STEPS, `Deploying stack ${config.app}`)

		const { code: deployCode, stderr: deployErr } = await ssh.exec(
			host,
			`docker stack deploy -c ${composePath} --with-registry-auth ${config.app}`,
		)

		if (deployCode !== 0) {
			throw new DeployError(`Stack deploy failed on ${host}: ${deployErr.trim()}`, 'deploy')
		}

		// Step 6: Wait for convergence
		logger.step(6, TOTAL_STEPS, 'Waiting for service convergence')
		const swarmServiceName = `${config.app}_${service}`
		await this.waitForConvergence(ssh, host, swarmServiceName, swarmConfig.convergence_timeout)

		// Step 7: Update proxy
		logger.step(7, TOTAL_STEPS, 'Updating proxy upstream')
		if (servicePort !== undefined) {
			const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
			await proxy.switchUpstream(host, config, domains, servicePort)
		}

		// Step 8: Write state
		logger.step(8, TOTAL_STEPS, 'Updating deployment state')

		let state: DeployState | null = null
		try {
			state = await runtime.readState(host, config.app)
		} catch {
			// First deploy
		}

		const newState: DeployState = {
			active_slot: 'blue',
			active_tag: finalTag,
			previous_tag: state?.active_tag,
			port: servicePort ?? 0,
			deployed_at: new Date().toISOString(),
			version: (state?.version ?? 0) + 1,
		}

		await runtime.writeState(host, config.app, newState)
		logger.success(`Swarm deploy of "${service}" complete on ${host}`)
	}

	// ---------------------------------------------------------------------------
	// Private helpers
	// ---------------------------------------------------------------------------

	private getSwarmConfig(config: ShuttleConfig): {
		replicas: number
		update_parallelism: number
		update_delay: string
		update_order: 'start-first' | 'stop-first'
		monitor: string
		convergence_timeout: number
	} {
		return {
			replicas: config.swarm?.replicas ?? 2,
			update_parallelism: config.swarm?.update_parallelism ?? 1,
			update_delay: config.swarm?.update_delay ?? '10s',
			update_order: config.swarm?.update_order ?? 'start-first',
			monitor: config.swarm?.monitor ?? '15s',
			convergence_timeout: config.swarm?.convergence_timeout ?? 300,
		}
	}

	private async ensureSwarmInit(ssh: SSHManager, host: string): Promise<void> {
		const { stdout } = await ssh.exec(host, "docker info --format '{{.Swarm.LocalNodeState}}'")

		if (stdout.trim() === 'active') {
			logger.debug('Swarm already initialized')
			return
		}

		logger.info('Initializing Docker Swarm...')
		const { code, stderr } = await ssh.exec(host, `docker swarm init --advertise-addr ${host}`)

		if (code !== 0) {
			throw new DeployError(
				`Failed to initialize Docker Swarm on ${host}: ${stderr.trim()}`,
				'provision',
			)
		}

		logger.success('Docker Swarm initialized')
	}

	private async waitForConvergence(
		ssh: SSHManager,
		host: string,
		serviceName: string,
		timeoutSeconds: number,
	): Promise<void> {
		const maxAttempts = Math.ceil(timeoutSeconds / 5)

		for (let attempt = 0; attempt < maxAttempts; attempt++) {
			const { stdout: updateState } = await ssh.exec(
				host,
				`docker service inspect ${serviceName} --format '{{.UpdateStatus.State}}' 2>/dev/null`,
			)

			const state = updateState.trim()

			if (state === 'completed' || state === '') {
				// Empty state means no update in progress (first deploy or already converged).
				// Verify all tasks are running.
				const { stdout: taskOutput } = await ssh.exec(
					host,
					`docker service ps --filter "desired-state=running" --format "{{.CurrentState}}" ${serviceName}`,
				)

				const tasks = taskOutput
					.trim()
					.split('\n')
					.filter((t) => t.length > 0)
				const allRunning = tasks.length > 0 && tasks.every((t) => t.startsWith('Running'))

				if (allRunning) {
					logger.success(`Service ${serviceName} converged (${tasks.length} tasks running)`)
					return
				}
			}

			if (state === 'rollback_completed' || state === 'rollback_paused') {
				throw new DeployError(
					`Swarm auto-rollback triggered for ${serviceName}: ${state}`,
					'healthcheck',
				)
			}

			if (state === 'paused') {
				throw new DeployError(
					`Swarm update paused for ${serviceName} due to failure`,
					'healthcheck',
				)
			}

			await new Promise<void>((resolve) => setTimeout(resolve, 5000))
		}

		throw new DeployError(
			`Service ${serviceName} did not converge within ${timeoutSeconds}s`,
			'healthcheck',
		)
	}

	private async getShortSha(): Promise<string> {
		try {
			const proc = Bun.spawn(['git', 'rev-parse', '--short', 'HEAD'], { stdout: 'pipe' })
			return (await new Response(proc.stdout).text()).trim()
		} catch {
			return Date.now().toString(36)
		}
	}
}

export function createSwarmStrategy(deps: SwarmDeps): SwarmStrategy {
	return new SwarmStrategy(deps)
}

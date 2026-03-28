// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, it, mock } from 'bun:test'
import { createConfig } from '../../../helpers/config-factory.ts'
import { createMockDocker } from '../../../helpers/mock-docker.ts'
import { createMockSSH } from '../../../helpers/mock-ssh.ts'

import { SwarmStrategy } from '../../../../src/providers/strategy/swarm.ts'
import type { DeployContext } from '../../../../src/providers/types.ts'
import { DeployError } from '../../../../src/utils/errors.ts'

function createMockRuntime() {
	return {
		getAppDir: mock((app: string) => `/opt/shuttle/${app}`),
		getWorkDir: mock((app: string) => `/opt/shuttle/${app}/${app}`),
		getStatePath: mock((app: string) => `/opt/shuttle/${app}/state.json`),
		getInFlightPath: mock((app: string) => `/opt/shuttle/${app}/deploying.json`),
		getLockDir: mock((app: string) => `/opt/shuttle/${app}/.deploy.lock`),
		readState: mock(() => Promise.resolve(null)),
		writeState: mock(() => Promise.resolve()),
		writeInFlight: mock(() => Promise.resolve()),
		clearInFlight: mock(() => Promise.resolve()),
		ensureAppDir: mock(() => Promise.resolve()),
		acquireLock: mock(() => Promise.resolve()),
		releaseLock: mock(() => Promise.resolve()),
		runHook: mock(() => Promise.resolve()),
		readLock: mock(() => Promise.resolve({})),
		forceReleaseLock: mock(() => Promise.resolve()),
		resolveServiceContainer: mock(() => Promise.resolve('myapp_web_blue')),
	}
}

function createMockSecrets() {
	return {
		push: mock(() => Promise.resolve()),
		set: mock(() => Promise.resolve()),
		get: mock(() => Promise.resolve(undefined)),
		list: mock(() => Promise.resolve([])),
		remove: mock(() => Promise.resolve()),
		loadAll: mock(() => Promise.resolve({})),
	}
}

function createMockRegistry() {
	return {
		distribute: mock(() => Promise.resolve()),
		resolve: mock((_, tag: string) => Promise.resolve(tag)),
	}
}

function createMockProxy() {
	return {
		switchUpstream: mock(() => Promise.resolve()),
		apply: mock(() => Promise.resolve()),
		generateConfig: mock(() => ''),
		getStatus: mock(() => Promise.resolve('')),
		removeDomains: mock(() => Promise.resolve()),
	}
}

// Helper: build an exec mock that handles swarm-related commands
function makeExecMock(opts: { swarmState?: string; converged?: boolean } = {}) {
	const swarmState = opts.swarmState ?? 'active'
	const converged = opts.converged ?? true

	return mock((host: string, cmd: string) => {
		if (cmd.includes('docker info')) {
			return Promise.resolve({ stdout: swarmState, stderr: '', code: 0 })
		}
		if (cmd.includes('docker stack deploy')) {
			return Promise.resolve({ stdout: '', stderr: '', code: 0 })
		}
		if (cmd.includes('service inspect')) {
			// '' means completed/no update in progress
			return Promise.resolve({ stdout: '', stderr: '', code: 0 })
		}
		if (cmd.includes('service ps')) {
			const tasks = converged ? 'Running since 10 seconds\nRunning since 10 seconds' : ''
			return Promise.resolve({ stdout: tasks, stderr: '', code: 0 })
		}
		return Promise.resolve({ stdout: '', stderr: '', code: 0 })
	})
}

function buildContext(overrides: Partial<DeployContext> = {}): DeployContext {
	return {
		config: createConfig({
			services: { web: { port: 3000 } as any },
		}),
		host: '1.2.3.4',
		service: 'web',
		tag: '',
		options: { skipBuild: true, dryRun: false },
		...overrides,
	}
}

describe('SwarmStrategy', () => {
	let mockSSH: ReturnType<typeof createMockSSH>
	let mockDocker: ReturnType<typeof createMockDocker>
	let mockRuntime: ReturnType<typeof createMockRuntime>
	let mockSecrets: ReturnType<typeof createMockSecrets>
	let mockRegistry: ReturnType<typeof createMockRegistry>
	let mockProxy: ReturnType<typeof createMockProxy>
	let strategy: SwarmStrategy

	beforeEach(() => {
		mockSSH = createMockSSH()
		mockDocker = createMockDocker()
		mockRuntime = createMockRuntime()
		mockSecrets = createMockSecrets()
		mockRegistry = createMockRegistry()
		mockProxy = createMockProxy()

		strategy = new SwarmStrategy({
			ssh: mockSSH as any,
			docker: mockDocker as any,
			runtime: mockRuntime as any,
			secrets: mockSecrets as any,
			registry: mockRegistry as any,
			proxy: mockProxy as any,
		})
	})

	// ---------------------------------------------------------------------------
	// execute() happy path
	// ---------------------------------------------------------------------------

	describe('execute() happy path', () => {
		it('completes all 8 steps without throwing', async () => {
			mockSSH.exec = makeExecMock({ swarmState: 'active', converged: true })

			const ctx = buildContext()
			await expect(strategy.execute(ctx)).resolves.toBeUndefined()
		})

		it('uploads compose file to the correct path', async () => {
			mockSSH.exec = makeExecMock()

			await strategy.execute(buildContext())

			expect(mockSSH.uploadContent).toHaveBeenCalledTimes(1)
			const [host, , path] = mockSSH.uploadContent.mock.calls[0]
			expect(host).toBe('1.2.3.4')
			expect(path).toContain('docker-compose.swarm.yml')
		})

		it('writes deployment state at end of deploy', async () => {
			mockSSH.exec = makeExecMock()

			await strategy.execute(buildContext())

			expect(mockRuntime.writeState).toHaveBeenCalledTimes(1)
			const state = mockRuntime.writeState.mock.calls[0][2]
			expect(state.active_slot).toBe('blue')
			expect(state.version).toBe(1)
		})

		it('calls proxy.switchUpstream when service has a port', async () => {
			mockSSH.exec = makeExecMock()

			await strategy.execute(buildContext())

			expect(mockProxy.switchUpstream).toHaveBeenCalledTimes(1)
		})

		it('does not call proxy.switchUpstream when service has no port', async () => {
			mockSSH.exec = makeExecMock()

			const config = createConfig({ services: { web: {} as any } })
			await strategy.execute(buildContext({ config }))

			expect(mockProxy.switchUpstream).not.toHaveBeenCalled()
		})

		it('increments version when prior state exists', async () => {
			mockSSH.exec = makeExecMock()
			mockRuntime.readState.mockImplementation(() =>
				Promise.resolve({
					active_slot: 'blue',
					active_tag: 'old-tag',
					port: 3000,
					deployed_at: '2024-01-01T00:00:00.000Z',
					version: 5,
				}),
			)

			await strategy.execute(buildContext())

			const state = mockRuntime.writeState.mock.calls[0][2]
			expect(state.version).toBe(6)
			expect(state.previous_tag).toBe('old-tag')
		})
	})

	// ---------------------------------------------------------------------------
	// getSwarmConfig defaults
	// ---------------------------------------------------------------------------

	describe('getSwarmConfig defaults', () => {
		it('uses default replicas=2 when swarm config is absent', async () => {
			mockSSH.exec = makeExecMock()

			await strategy.execute(buildContext())

			// Inspect compose content uploaded
			const composeContent = mockSSH.uploadContent.mock.calls[0][1] as string
			expect(composeContent).toContain('replicas: 2')
		})

		it('uses explicit replicas from config.swarm', async () => {
			mockSSH.exec = makeExecMock()

			const config = createConfig({
				services: { web: { port: 3000 } as any },
				swarm: { replicas: 5 } as any,
			})
			await strategy.execute(buildContext({ config }))

			const composeContent = mockSSH.uploadContent.mock.calls[0][1] as string
			expect(composeContent).toContain('replicas: 5')
		})
	})

	// ---------------------------------------------------------------------------
	// ensureSwarmInit
	// ---------------------------------------------------------------------------

	describe('ensureSwarmInit', () => {
		it('does not call docker swarm init when already active', async () => {
			const execCalls: string[] = []
			mockSSH.exec = mock((host: string, cmd: string) => {
				execCalls.push(cmd)
				if (cmd.includes('docker info'))
					return Promise.resolve({ stdout: 'active', stderr: '', code: 0 })
				if (cmd.includes('service inspect'))
					return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				if (cmd.includes('service ps'))
					return Promise.resolve({
						stdout: 'Running since 5s\nRunning since 5s',
						stderr: '',
						code: 0,
					})
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			await strategy.execute(buildContext())

			const initCalls = execCalls.filter((c) => c.includes('swarm init'))
			expect(initCalls).toHaveLength(0)
		})

		it('calls docker swarm init when swarm is not active', async () => {
			const execCalls: string[] = []
			mockSSH.exec = mock((host: string, cmd: string) => {
				execCalls.push(cmd)
				if (cmd.includes('docker info'))
					return Promise.resolve({ stdout: 'inactive', stderr: '', code: 0 })
				if (cmd.includes('swarm init')) return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				if (cmd.includes('service inspect'))
					return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				if (cmd.includes('service ps'))
					return Promise.resolve({
						stdout: 'Running since 5s\nRunning since 5s',
						stderr: '',
						code: 0,
					})
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			await strategy.execute(buildContext())

			const initCalls = execCalls.filter((c) => c.includes('swarm init'))
			expect(initCalls).toHaveLength(1)
		})

		it('throws DeployError when swarm init fails', async () => {
			mockSSH.exec = mock((host: string, cmd: string) => {
				if (cmd.includes('docker info'))
					return Promise.resolve({ stdout: 'inactive', stderr: '', code: 0 })
				if (cmd.includes('swarm init'))
					return Promise.resolve({ stdout: '', stderr: 'address already in use', code: 1 })
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			await expect(strategy.execute(buildContext())).rejects.toBeInstanceOf(DeployError)
		})
	})

	// ---------------------------------------------------------------------------
	// stack deploy failure
	// ---------------------------------------------------------------------------

	describe('stack deploy failure', () => {
		it('throws DeployError when docker stack deploy returns non-zero', async () => {
			mockSSH.exec = mock((host: string, cmd: string) => {
				if (cmd.includes('docker info'))
					return Promise.resolve({ stdout: 'active', stderr: '', code: 0 })
				if (cmd.includes('stack deploy'))
					return Promise.resolve({ stdout: '', stderr: 'deploy failed', code: 1 })
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			await expect(strategy.execute(buildContext())).rejects.toBeInstanceOf(DeployError)
		})
	})
})

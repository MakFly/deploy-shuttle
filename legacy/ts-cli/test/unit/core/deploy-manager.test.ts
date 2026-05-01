// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createConfig } from '../../helpers/config-factory.ts'
import { createMockDocker } from '../../helpers/mock-docker.ts'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

const mockSSH = createMockSSH()
const mockDocker = createMockDocker()
const mockProxy = {
	switchUpstream: mock(() => Promise.resolve()),
	apply: mock(() => Promise.resolve()),
	generateConfig: mock(() => ''),
	getStatus: mock(() => Promise.resolve('')),
}
const mockSecrets = {
	push: mock(() => Promise.resolve()),
	set: mock(() => Promise.resolve()),
	get: mock(() => Promise.resolve(undefined)),
	list: mock(() => Promise.resolve([])),
	remove: mock(() => Promise.resolve()),
	loadAll: mock(() => Promise.resolve({})),
}

const mockAccessories = {
	ensureAccessories: mock(() => Promise.resolve()),
	ensureAccessory: mock(() => Promise.resolve()),
}
const mockRuntime = {
	acquireLock: mock(() => Promise.resolve()),
	releaseLock: mock(() => Promise.resolve()),
	readState: mock(() => Promise.resolve(null)),
	writeState: mock(() => Promise.resolve()),
	writeInFlight: mock(() => Promise.resolve()),
	clearInFlight: mock(() => Promise.resolve()),
	ensureAppDir: mock(() => Promise.resolve()),
	getAppDir: mock((app: string) => `/opt/shuttle/${app}`),
	getWorkDir: mock((app: string) => `/opt/shuttle/${app}/${app}`),
	getStatePath: mock((app: string) => `/opt/shuttle/${app}/state.json`),
	getInFlightPath: mock((app: string) => `/opt/shuttle/${app}/deploying.json`),
	getLockDir: mock((app: string) => `/opt/shuttle/${app}/.deploy.lock`),
	runHook: mock(() => Promise.resolve()),
	readLock: mock(() => Promise.resolve({})),
	forceReleaseLock: mock(() => Promise.resolve()),
	resolveServiceContainer: mock(() => Promise.resolve('myapp_web_blue')),
}

mock.module('../../../src/core/ssh-manager.ts', () => ({ ssh: mockSSH }))
mock.module('../../../src/core/docker-manager.ts', () => ({ docker: mockDocker }))
mock.module('../../../src/core/proxy-manager.ts', () => ({ proxy: mockProxy }))
mock.module('../../../src/core/secrets-manager.ts', () => ({ secrets: mockSecrets }))
mock.module('../../../src/core/accessory-manager.ts', () => ({ accessories: mockAccessories }))
mock.module('../../../src/core/runtime-manager.ts', () => ({ runtime: mockRuntime }))

import { DeployManager } from '../../../src/core/deploy-manager.ts'
import { DeployError } from '../../../src/utils/errors.ts'

describe('DeployManager', () => {
	let deployer: DeployManager

	beforeEach(() => {
		deployer = new DeployManager(
			mockSSH as any,
			mockDocker as any,
			mockProxy as any,
			mockRuntime as any,
			mockSecrets as any,
			mockAccessories as any,
		)
		mockSSH.connect.mockReset()
		mockSSH.exec.mockReset()
		mockSSH.uploadContent.mockReset()
		mockSSH.disconnect.mockReset()
		mockDocker.build.mockReset()
		mockDocker.transfer.mockReset()
		mockDocker.run.mockReset()
		mockDocker.stop.mockReset()
		mockDocker.inspect.mockReset()
		mockDocker.tag.mockReset()
		mockDocker.prune.mockReset()
		mockDocker.listImages.mockReset()
		mockProxy.switchUpstream.mockReset()
		mockSecrets.push.mockReset()
		mockAccessories.ensureAccessories.mockReset()
		mockRuntime.acquireLock.mockReset()
		mockRuntime.releaseLock.mockReset()
		mockRuntime.writeInFlight.mockReset()
		mockRuntime.clearInFlight.mockReset()
		mockRuntime.runHook.mockReset()
		mockRuntime.readState.mockReset()
		mockRuntime.writeState.mockReset()
		mockRuntime.ensureAppDir.mockReset()

		mockSSH.connect.mockImplementation(() => Promise.resolve({}))
		mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
		mockSSH.uploadContent.mockImplementation(() => Promise.resolve())
		mockDocker.build.mockImplementation(() => Promise.resolve())
		mockDocker.transfer.mockImplementation(() => Promise.resolve())
		mockDocker.run.mockImplementation(() => Promise.resolve())
		mockDocker.stop.mockImplementation(() => Promise.resolve())
		mockDocker.inspect.mockImplementation(() => Promise.resolve(null))
		mockDocker.tag.mockImplementation(() => Promise.resolve())
		mockDocker.prune.mockImplementation(() => Promise.resolve())
		mockDocker.listImages.mockImplementation(() => Promise.resolve([]))
		mockProxy.switchUpstream.mockImplementation(() => Promise.resolve())
		mockSecrets.push.mockImplementation(() => Promise.resolve())
		mockAccessories.ensureAccessories.mockImplementation(() => Promise.resolve())
		mockRuntime.acquireLock.mockImplementation(() => Promise.resolve())
		mockRuntime.releaseLock.mockImplementation(() => Promise.resolve())
		mockRuntime.writeInFlight.mockImplementation(() => Promise.resolve())
		mockRuntime.clearInFlight.mockImplementation(() => Promise.resolve())
		mockRuntime.runHook.mockImplementation(() => Promise.resolve())
		// Default: throw to simulate first deploy (no state file)
		mockRuntime.readState.mockImplementation(() => {
			throw new Error('State file not found')
		})
		mockRuntime.writeState.mockImplementation(() => Promise.resolve())
		mockRuntime.ensureAppDir.mockImplementation(() => Promise.resolve())
	})

	describe('getServicePort', () => {
		test('returns base port for first service blue slot', () => {
			expect(deployer.getServicePort(0, 'blue')).toBe(10000)
		})

		test('returns base port + 1 for first service green slot', () => {
			expect(deployer.getServicePort(0, 'green')).toBe(10001)
		})

		test('returns correct port for second service blue slot', () => {
			expect(deployer.getServicePort(1, 'blue')).toBe(10002)
		})

		test('returns correct port for second service green slot', () => {
			expect(deployer.getServicePort(1, 'green')).toBe(10003)
		})

		test('returns correct port for third service', () => {
			expect(deployer.getServicePort(2, 'blue')).toBe(10004)
			expect(deployer.getServicePort(2, 'green')).toBe(10005)
		})
	})

	describe('deploy', () => {
		test('throws DeployError when no services are defined', async () => {
			const config = createConfig({ services: {} })
			expect(deployer.deploy(config)).rejects.toThrow(DeployError)
		})

		test('throws DeployError when services key is missing', async () => {
			const config = createConfig()
			// services is undefined
			expect(deployer.deploy(config)).rejects.toThrow(DeployError)
		})

		test('connects SSH for each host in each server group', async () => {
			const config = createConfig({
				servers: {
					default: { hosts: ['1.2.3.4', '5.6.7.8'], user: 'deploy' },
				},
				services: { web: { command: 'node server.js' } },
			})

			// readState throws by default (first deploy) — no SSH exec mock needed

			await deployer.deploy(config)

			const connectHosts = mockSSH.connect.mock.calls.map(
				(c: unknown[]) => (c[0] as { host: string }).host,
			)
			expect(connectHosts).toContain('1.2.3.4')
			expect(connectHosts).toContain('5.6.7.8')
		})

		test('calls docker.build and docker.transfer when skipBuild is false', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			// readState throws by default (first deploy)

			await deployer.deploy(config, { skipBuild: false })

			expect(mockDocker.build).toHaveBeenCalledTimes(1)
			expect(mockDocker.transfer).toHaveBeenCalledTimes(1)
		})

		test('skips build and transfer when skipBuild is true', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			// readState throws by default (first deploy)

			await deployer.deploy(config, { skipBuild: true })

			expect(mockDocker.build).not.toHaveBeenCalled()
			expect(mockDocker.transfer).not.toHaveBeenCalled()
		})

		test('runs pre_deploy hooks once per host before service deploys', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
				deploy: {
					strategy: 'blue-green',
					timeout: 120,
					retain: 5,
					auto_rollback: true,
					blue_green: { drain_timeout: 30, readiness_delay: 0 },
					hooks: { pre_deploy: ['echo pre'], post_deploy: [] },
				},
			})

			// readState throws by default (first deploy)

			await deployer.deploy(config)

			expect(mockRuntime.runHook).toHaveBeenCalledWith('1.2.3.4', 'myapp', 'echo pre', 'pre_deploy')
		})

		test('ensures accessories before service deploys', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
				accessories: {
					redis: { image: 'redis:7-alpine', port: 6379 },
				},
			})

			// readState throws by default (first deploy)

			await deployer.deploy(config)

			expect(mockAccessories.ensureAccessories).toHaveBeenCalledWith('1.2.3.4', config)
		})

		test('dry-run avoids SSH and Docker mutations', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await deployer.deploy(config, { dryRun: true })

			expect(mockSSH.connect).not.toHaveBeenCalled()
			expect(mockDocker.build).not.toHaveBeenCalled()
			expect(mockDocker.run).not.toHaveBeenCalled()
		})
	})

	// blueGreenDeploy/rollingDeploy inline methods removed — logic now in strategy classes

	describe('readState', () => {
		test('parses state JSON correctly', async () => {
			const expectedState = {
				active_slot: 'blue',
				active_tag: 'shuttle/myapp:deploy-20240101-abc',
				port: 10000,
				deployed_at: '2024-01-01T00:00:00.000Z',
				version: 1,
			}

			mockRuntime.readState.mockImplementation(() => Promise.resolve(expectedState))

			const state = await deployer.readState('1.2.3.4', 'myapp')

			expect(state.active_slot).toBe('blue')
			expect(state.active_tag).toBe('shuttle/myapp:deploy-20240101-abc')
			expect(state.port).toBe(10000)
			expect(state.version).toBe(1)
		})

		test('throws DeployError when state file does not exist (non-zero exit)', async () => {
			mockRuntime.readState.mockImplementation(() => {
				throw new DeployError('State file not found', 'read-state')
			})

			expect(deployer.readState('1.2.3.4', 'myapp')).rejects.toThrow(DeployError)
		})
	})

	describe('writeState', () => {
		test('delegates to runtime.writeState with correct arguments', async () => {
			const state = {
				active_slot: 'blue' as const,
				active_tag: 'shuttle/myapp:deploy-20240101-abc',
				port: 10000,
				deployed_at: '2024-01-01T00:00:00.000Z',
				version: 1,
			}

			await deployer.writeState('1.2.3.4', 'myapp', state)

			expect(mockRuntime.writeState).toHaveBeenCalledWith('1.2.3.4', 'myapp', state)
		})
	})
})

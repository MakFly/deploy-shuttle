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
import { RollbackManager } from '../../../src/core/rollback-manager.ts'
import { DeployError } from '../../../src/utils/errors.ts'

describe('RollbackManager', () => {
	let rollbackManager: RollbackManager
	let deployer: DeployManager

	const baseState = {
		active_slot: 'blue' as const,
		active_tag: 'shuttle/myapp:deploy-20240101-abc',
		previous_tag: 'shuttle/myapp:deploy-20231201-def',
		port: 10000,
		deployed_at: '2024-01-01T00:00:00.000Z',
		version: 2,
	}

	beforeEach(() => {
		deployer = new DeployManager(
			mockSSH as any,
			mockDocker as any,
			mockProxy as any,
			mockRuntime as any,
			mockSecrets as any,
			mockAccessories as any,
		)
		rollbackManager = new RollbackManager(
			mockDocker as any,
			mockProxy as any,
			mockRuntime as any,
			deployer as any,
		)

		mockSSH.exec.mockReset()
		mockSSH.uploadContent.mockReset()
		mockDocker.run.mockReset()
		mockDocker.stop.mockReset()
		mockDocker.listImages.mockReset()
		mockProxy.switchUpstream.mockReset()
		mockRuntime.readState.mockReset()
		mockRuntime.writeState.mockReset()

		mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
		mockSSH.uploadContent.mockImplementation(() => Promise.resolve())
		mockDocker.run.mockImplementation(() => Promise.resolve())
		mockDocker.stop.mockImplementation(() => Promise.resolve())
		mockDocker.listImages.mockImplementation(() => Promise.resolve([]))
		mockProxy.switchUpstream.mockImplementation(() => Promise.resolve())
		mockRuntime.readState.mockImplementation(() => Promise.resolve(baseState))
		mockRuntime.writeState.mockImplementation(() => Promise.resolve())
	})

	describe('rollback', () => {
		test('throws DeployError when no previous_tag is available', async () => {
			const stateWithoutPrevious = { ...baseState, previous_tag: undefined }
			mockRuntime.readState.mockImplementation(() => Promise.resolve(stateWithoutPrevious))

			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			expect(rollbackManager.rollback(config, '1.2.3.4')).rejects.toThrow(DeployError)
		})

		test('stops old container and starts new one with previous tag', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await rollbackManager.rollback(config, '1.2.3.4')

			// stop is called at least once (stale removal + drain)
			expect(mockDocker.stop).toHaveBeenCalled()
			// run should be called with the previous tag
			expect(mockDocker.run).toHaveBeenCalledTimes(1)
			const runOpts = mockDocker.run.mock.calls[0][1] as { image: string }
			expect(runOpts.image).toBe('shuttle/myapp:deploy-20231201-def')
		})

		test('switches proxy upstream to new port', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await rollbackManager.rollback(config, '1.2.3.4')

			expect(mockProxy.switchUpstream).toHaveBeenCalledTimes(1)
			const [host, domain, port] = mockProxy.switchUpstream.mock.calls[0] as [
				string,
				string,
				number,
			]
			expect(host).toBe('1.2.3.4')
			expect(domain).toBe('myapp.example.com')
			// green slot port for service index 0
			expect(port).toBe(10001)
		})

		test('writes new state after rollback', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await rollbackManager.rollback(config, '1.2.3.4')

			expect(mockRuntime.writeState).toHaveBeenCalledTimes(1)
			const [, , newState] = mockRuntime.writeState.mock.calls[0] as [
				string,
				string,
				Record<string, unknown>,
			]
			expect(newState.active_tag).toBe('shuttle/myapp:deploy-20231201-def')
			expect(newState.previous_tag).toBe('shuttle/myapp:deploy-20240101-abc')
			expect(newState.version).toBe(3)
		})

		test('uses explicit targetTag when provided', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await rollbackManager.rollback(config, '1.2.3.4', 'shuttle/myapp:deploy-20231101-xyz')

			const runOpts = mockDocker.run.mock.calls[0][1] as { image: string }
			expect(runOpts.image).toBe('shuttle/myapp:deploy-20231101-xyz')
		})

		test('uses deployer.getServicePort for port allocation', async () => {
			const config = createConfig({
				services: { web: { command: 'node server.js' } },
			})

			await rollbackManager.rollback(config, '1.2.3.4')

			// base state active_slot is blue, so rollback goes to green
			const expectedPort = deployer.getServicePort(0, 'green')
			const runOpts = mockDocker.run.mock.calls[0][1] as { port: string }
			expect(runOpts.port).toContain(String(expectedPort))
		})
	})

	describe('listVersions', () => {
		test('delegates to docker.listImages with correct prefix', async () => {
			mockDocker.listImages.mockImplementation(() =>
				Promise.resolve(['shuttle/myapp:deploy-20240101-abc', 'shuttle/myapp:deploy-20231201-def']),
			)

			const versions = await rollbackManager.listVersions('1.2.3.4', 'myapp')

			expect(mockDocker.listImages).toHaveBeenCalledWith('1.2.3.4', 'shuttle/myapp:deploy-*')
			expect(versions).toHaveLength(2)
			expect(versions).toContain('shuttle/myapp:deploy-20240101-abc')
		})
	})
})

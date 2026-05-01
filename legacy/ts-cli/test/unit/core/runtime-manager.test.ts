// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, it } from 'bun:test'
import { createConfig } from '../../helpers/config-factory.ts'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

import { RuntimeManager } from '../../../src/core/runtime-manager.ts'
import { DeployError } from '../../../src/utils/errors.ts'

describe('RuntimeManager', () => {
	let mockSSH: ReturnType<typeof createMockSSH>
	let runtime: RuntimeManager

	beforeEach(() => {
		mockSSH = createMockSSH()
		runtime = new RuntimeManager(mockSSH as any)
	})

	// ---------------------------------------------------------------------------
	// Pure path helpers
	// ---------------------------------------------------------------------------

	describe('path helpers', () => {
		it('getAppDir returns /opt/shuttle/<app>', () => {
			expect(runtime.getAppDir('myapp')).toBe('/opt/shuttle/myapp')
		})

		it('getWorkDir returns /opt/shuttle/<app>/<app>', () => {
			expect(runtime.getWorkDir('myapp')).toBe('/opt/shuttle/myapp/myapp')
		})

		it('getStatePath returns /opt/shuttle/<app>/state.json', () => {
			expect(runtime.getStatePath('myapp')).toBe('/opt/shuttle/myapp/state.json')
		})

		it('getInFlightPath returns /opt/shuttle/<app>/deploying.json', () => {
			expect(runtime.getInFlightPath('myapp')).toBe('/opt/shuttle/myapp/deploying.json')
		})

		it('getLockDir returns /opt/shuttle/<app>/.deploy.lock', () => {
			expect(runtime.getLockDir('myapp')).toBe('/opt/shuttle/myapp/.deploy.lock')
		})
	})

	// ---------------------------------------------------------------------------
	// readState
	// ---------------------------------------------------------------------------

	describe('readState', () => {
		it('returns parsed state on success', async () => {
			const state = {
				active_slot: 'blue',
				active_tag: 'shuttle/myapp:deploy-20240101-abc1234',
				port: 10000,
				deployed_at: '2024-01-01T00:00:00.000Z',
				version: 1,
			}
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: JSON.stringify(state), stderr: '', code: 0 }),
			)

			const result = await runtime.readState('1.2.3.4', 'myapp')
			expect(result).toEqual(state)
		})

		it('throws DeployError when ssh.exec returns non-zero exit code', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'No such file', code: 1 }),
			)

			await expect(runtime.readState('1.2.3.4', 'myapp')).rejects.toBeInstanceOf(DeployError)
		})

		it('throws DeployError when state.json contains invalid JSON', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: 'not-json', stderr: '', code: 0 }),
			)

			await expect(runtime.readState('1.2.3.4', 'myapp')).rejects.toBeInstanceOf(DeployError)
		})
	})

	// ---------------------------------------------------------------------------
	// writeState
	// ---------------------------------------------------------------------------

	describe('writeState', () => {
		it('calls ssh.uploadContent with serialised state', async () => {
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
			mockSSH.uploadContent.mockImplementation(() => Promise.resolve())

			const state = {
				active_slot: 'blue' as const,
				active_tag: 'shuttle/myapp:deploy-20240101-abc1234',
				port: 10000,
				deployed_at: '2024-01-01T00:00:00.000Z',
				version: 1,
			}
			await runtime.writeState('1.2.3.4', 'myapp', state)

			expect(mockSSH.uploadContent).toHaveBeenCalledTimes(1)
			const [host, content, path] = mockSSH.uploadContent.mock.calls[0]
			expect(host).toBe('1.2.3.4')
			expect(path).toBe('/opt/shuttle/myapp/state.json')
			expect(JSON.parse(content)).toEqual(state)
		})
	})

	// ---------------------------------------------------------------------------
	// acquireLock
	// ---------------------------------------------------------------------------

	describe('acquireLock', () => {
		it('succeeds when mkdir returns code 0', async () => {
			// ensureAppDir mkdir + acquireLock mkdir both return 0
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
			mockSSH.uploadContent.mockImplementation(() => Promise.resolve())

			await expect(runtime.acquireLock('1.2.3.4', 'myapp')).resolves.toBeUndefined()
		})

		it('throws DeployError when lock directory already exists', async () => {
			// ensureAppDir succeeds, then acquireLock mkdir fails
			let callCount = 0
			mockSSH.exec.mockImplementation(() => {
				callCount++
				// First call is ensureAppDir (mkdir -p), second is the lock mkdir
				const code = callCount === 1 ? 0 : 1
				return Promise.resolve({ stdout: '', stderr: '', code })
			})
			// readLock inside acquireLock will also call exec
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 1 }))

			await expect(runtime.acquireLock('1.2.3.4', 'myapp')).rejects.toBeInstanceOf(DeployError)
		})
	})

	// ---------------------------------------------------------------------------
	// releaseLock
	// ---------------------------------------------------------------------------

	describe('releaseLock', () => {
		it('executes rm -rf on the lock directory', async () => {
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))

			await runtime.releaseLock('1.2.3.4', 'myapp')

			expect(mockSSH.exec).toHaveBeenCalledTimes(1)
			const cmd = mockSSH.exec.mock.calls[0][1] as string
			expect(cmd).toContain('rm -rf')
			expect(cmd).toContain('.deploy.lock')
		})
	})

	// ---------------------------------------------------------------------------
	// runHook
	// ---------------------------------------------------------------------------

	describe('runHook', () => {
		it('resolves when hook exits with code 0', async () => {
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))

			await expect(
				runtime.runHook('1.2.3.4', 'myapp', 'echo hello', 'pre_deploy'),
			).resolves.toBeUndefined()
		})

		it('throws DeployError when hook exits non-zero', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'Script failed', code: 1 }),
			)

			await expect(
				runtime.runHook('1.2.3.4', 'myapp', 'false', 'pre_deploy'),
			).rejects.toBeInstanceOf(DeployError)
		})

		it('thrown DeployError phase matches hook phase', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'oops', code: 2 }),
			)

			try {
				await runtime.runHook('1.2.3.4', 'myapp', 'false', 'post_deploy')
				expect(true).toBe(false) // should not reach
			} catch (err) {
				expect(err).toBeInstanceOf(DeployError)
				expect((err as DeployError).phase).toBe('post_deploy')
			}
		})
	})

	// ---------------------------------------------------------------------------
	// resolveServiceContainer
	// ---------------------------------------------------------------------------

	describe('resolveServiceContainer', () => {
		it('returns <app>_<service>_0 for rolling strategy', async () => {
			const config = createConfig({ deploy: { strategy: 'rolling' } as any })
			const result = await runtime.resolveServiceContainer('1.2.3.4', config, 'web')
			expect(result).toBe('myapp_web_0')
		})

		it('returns <app>_<service>_<active_slot> for blue-green when state exists', async () => {
			const state = {
				active_slot: 'green',
				active_tag: 'shuttle/myapp:deploy-20240101-abc1234',
				port: 10001,
				deployed_at: '2024-01-01T00:00:00.000Z',
				version: 2,
			}
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: JSON.stringify(state), stderr: '', code: 0 }),
			)

			const config = createConfig()
			const result = await runtime.resolveServiceContainer('1.2.3.4', config, 'web')
			expect(result).toBe('myapp_web_green')
		})

		it('falls back to blue slot when state file is missing', async () => {
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 1 }))

			const config = createConfig()
			const result = await runtime.resolveServiceContainer('1.2.3.4', config, 'web')
			expect(result).toBe('myapp_web_blue')
		})
	})
})

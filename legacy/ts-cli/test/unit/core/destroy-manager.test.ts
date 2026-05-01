// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createConfig } from '../../helpers/config-factory.ts'
import { createMockDocker } from '../../helpers/mock-docker.ts'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

const mockDocker = createMockDocker()
const mockSSH = createMockSSH()
const mockProxy = { removeDomains: mock(() => Promise.resolve()) }

const mockRuntime = { getAppDir: mock((app: string) => `/opt/shuttle/${app}`) }

mock.module('../../../src/core/docker-manager.ts', () => ({ docker: mockDocker }))
mock.module('../../../src/core/proxy-manager.ts', () => ({ proxy: mockProxy }))
mock.module('../../../src/core/runtime-manager.ts', () => ({ runtime: mockRuntime }))
mock.module('../../../src/core/ssh-manager.ts', () => ({ ssh: mockSSH }))

import { DestroyManager } from '../../../src/core/destroy-manager.ts'

describe('DestroyManager', () => {
	let destroyer: InstanceType<typeof DestroyManager>
	beforeEach(() => {
		destroyer = new DestroyManager(
			mockDocker as any,
			mockProxy as any,
			mockRuntime as any,
			mockSSH as any,
		)
		mockDocker.listContainers.mockReset()
		mockDocker.stop.mockReset()
		mockDocker.listImages.mockReset()
		mockDocker.removeImages.mockReset()
		mockSSH.exec.mockReset()
		mockProxy.removeDomains.mockReset()

		mockDocker.listContainers.mockImplementation(() =>
			Promise.resolve(['myapp_web_blue', 'myapp_redis']),
		)
		mockDocker.stop.mockImplementation(() => Promise.resolve())
		mockDocker.listImages.mockImplementation(() => Promise.resolve(['shuttle/myapp:current']))
		mockDocker.removeImages.mockImplementation(() => Promise.resolve())
		mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
		mockProxy.removeDomains.mockImplementation(() => Promise.resolve())
	})

	test('removes app containers, images, proxy entries and app directory', async () => {
		const config = createConfig({
			services: { web: { command: 'node server.js' } },
			accessories: { redis: { image: 'redis:7-alpine', port: 6379 } },
		})

		await destroyer.destroy(config, '1.2.3.4')

		expect(mockDocker.stop).toHaveBeenCalledTimes(2)
		expect(mockDocker.removeImages).toHaveBeenCalledWith('1.2.3.4', ['shuttle/myapp:current'])
		expect(mockProxy.removeDomains).toHaveBeenCalledWith('1.2.3.4', ['myapp.example.com'])
		expect(
			mockSSH.exec.mock.calls.some((call: unknown[]) => (call[1] as string).includes('rm -rf')),
		).toBe(true)
	})
})

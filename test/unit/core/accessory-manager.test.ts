// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createMockDocker } from '../../helpers/mock-docker.ts'

const mockDocker = createMockDocker()

mock.module('../../../src/core/docker-manager.ts', () => ({ docker: mockDocker }))

import { AccessoryManager } from '../../../src/core/accessory-manager.ts'
import { createConfig } from '../../helpers/config-factory.ts'

describe('AccessoryManager', () => {
	let accessories: AccessoryManager

	beforeEach(() => {
		accessories = new AccessoryManager(mockDocker as any)
		mockDocker.inspect.mockReset()
		mockDocker.stop.mockReset()
		mockDocker.run.mockReset()
		mockDocker.pull.mockReset()
		mockDocker.inspect.mockImplementation(() => Promise.resolve(null))
		mockDocker.stop.mockImplementation(() => Promise.resolve())
		mockDocker.run.mockImplementation(() => Promise.resolve())
		mockDocker.pull.mockImplementation(() => Promise.resolve())
	})

	test('creates an accessory container when none exists', async () => {
		const config = createConfig({
			accessories: {
				redis: { image: 'redis:7-alpine', port: 6379 },
			},
		})

		await accessories.ensureAccessories('1.2.3.4', config)

		expect(mockDocker.run).toHaveBeenCalledTimes(1)
		const runOptions = mockDocker.run.mock.calls[0][1] as { name: string; port: string }
		expect(runOptions.name).toBe('myapp_redis')
		expect(runOptions.port).toBe('127.0.0.1:6379:6379')
	})

	test('skips recreation when the accessory config hash matches', async () => {
		const config = createConfig({
			accessories: {
				redis: { image: 'redis:7-alpine', port: 6379 },
			},
		})

		await accessories.ensureAccessories('1.2.3.4', config)
		const firstLabels = mockDocker.run.mock.calls[0][1].labels as Record<string, string>

		mockDocker.run.mockReset()
		mockDocker.inspect.mockImplementation(() =>
			Promise.resolve({
				Config: {
					Labels: firstLabels,
				},
			}),
		)

		await accessories.ensureAccessories('1.2.3.4', config)

		expect(mockDocker.run).not.toHaveBeenCalled()
	})
})

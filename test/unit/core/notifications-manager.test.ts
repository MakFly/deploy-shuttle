// @ts-nocheck — mock.calls types are not compatible with strict TS
import { afterEach, beforeEach, describe, expect, it, mock, spyOn } from 'bun:test'
import { createConfig } from '../../helpers/config-factory.ts'

import { NotificationsManager } from '../../../src/core/notifications-manager.ts'
import { logger } from '../../../src/utils/logger.ts'

describe('NotificationsManager', () => {
	let manager: NotificationsManager
	let warnSpy: ReturnType<typeof spyOn>

	beforeEach(() => {
		manager = new NotificationsManager()
		warnSpy = spyOn(logger, 'warn').mockImplementation(() => {})
	})

	afterEach(() => {
		warnSpy.mockRestore()
	})

	describe('notify — no webhooks', () => {
		it('resolves immediately when notifications config is absent', async () => {
			const config = createConfig()
			await expect(
				manager.notify(config, 'deploy_succeeded', { host: '1.2.3.4' }),
			).resolves.toBeUndefined()
		})

		it('resolves immediately when webhooks array is empty', async () => {
			const config = createConfig({ notifications: { webhooks: [] } })
			await expect(
				manager.notify(config, 'deploy_succeeded', { host: '1.2.3.4' }),
			).resolves.toBeUndefined()
		})
	})

	describe('notify — webhook success', () => {
		it('sends POST with JSON body to each webhook URL', async () => {
			const mockFetch = mock(() => Promise.resolve({ ok: true, status: 200 } as Response))
			const original = globalThis.fetch
			globalThis.fetch = mockFetch

			try {
				const config = createConfig({
					notifications: {
						webhooks: ['https://hooks.example.com/a', 'https://hooks.example.com/b'],
					},
				})
				await manager.notify(config, 'deploy_succeeded', { version: 3 })

				expect(mockFetch).toHaveBeenCalledTimes(2)

				const [url, init] = mockFetch.mock.calls[0]
				expect(url).toBe('https://hooks.example.com/a')
				expect(init.method).toBe('POST')
				expect(init.headers['content-type']).toBe('application/json')

				const body = JSON.parse(init.body as string)
				expect(body.event).toBe('deploy_succeeded')
				expect(body.app).toBe('myapp')
				expect(body.version).toBe(3)
				expect(body.timestamp).toBeDefined()
			} finally {
				globalThis.fetch = original
			}
		})
	})

	describe('notify — HTTP error handling', () => {
		it('warns (does not throw) when webhook returns non-ok response', async () => {
			const mockFetch = mock(() => Promise.resolve({ ok: false, status: 500 } as Response))
			const original = globalThis.fetch
			globalThis.fetch = mockFetch

			try {
				const config = createConfig({
					notifications: { webhooks: ['https://hooks.example.com/fail'] },
				})
				await expect(manager.notify(config, 'deploy_failed', {})).resolves.toBeUndefined()

				expect(warnSpy).toHaveBeenCalledTimes(1)
				const warnMessage = warnSpy.mock.calls[0][0] as string
				expect(warnMessage).toContain('500')
			} finally {
				globalThis.fetch = original
			}
		})
	})

	describe('notify — network failure handling', () => {
		it('warns (does not throw) when fetch throws a network error', async () => {
			const mockFetch = mock(() => Promise.reject(new Error('ECONNREFUSED')))
			const original = globalThis.fetch
			globalThis.fetch = mockFetch

			try {
				const config = createConfig({
					notifications: { webhooks: ['https://hooks.example.com/down'] },
				})
				await expect(manager.notify(config, 'rollback_succeeded', {})).resolves.toBeUndefined()

				expect(warnSpy).toHaveBeenCalledTimes(1)
				const warnMessage = warnSpy.mock.calls[0][0] as string
				expect(warnMessage).toContain('ECONNREFUSED')
			} finally {
				globalThis.fetch = original
			}
		})

		it('continues notifying remaining webhooks after a failure', async () => {
			let callIndex = 0
			const mockFetch = mock(() => {
				callIndex++
				if (callIndex === 1) return Promise.reject(new Error('first failed'))
				return Promise.resolve({ ok: true, status: 200 } as Response)
			})
			const original = globalThis.fetch
			globalThis.fetch = mockFetch

			try {
				const config = createConfig({
					notifications: {
						webhooks: ['https://hooks.example.com/bad', 'https://hooks.example.com/good'],
					},
				})
				await manager.notify(config, 'deploy_succeeded', {})

				// Both webhooks were attempted
				expect(mockFetch).toHaveBeenCalledTimes(2)
				// Only the first failure triggered a warning
				expect(warnSpy).toHaveBeenCalledTimes(1)
			} finally {
				globalThis.fetch = original
			}
		})
	})
})

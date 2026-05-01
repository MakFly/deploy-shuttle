import { beforeEach, describe, expect, test } from 'bun:test'
import { SSHManager } from '../../../src/core/ssh-manager.ts'
import { SSHError } from '../../../src/utils/errors.ts'

describe('SSHManager', () => {
	let manager: SSHManager

	beforeEach(() => {
		manager = new SSHManager()
	})

	describe('connections map', () => {
		test('starts with an empty connections map', () => {
			expect(manager.connections.size).toBe(0)
		})

		test('disconnect with specific host removes it from connections map', () => {
			// Manually inject a fake client into the map
			const fakeClient = { end: () => {} } as unknown as import('ssh2').Client
			manager.connections.set('1.2.3.4', fakeClient)

			expect(manager.connections.has('1.2.3.4')).toBe(true)
			manager.disconnect('1.2.3.4')
			expect(manager.connections.has('1.2.3.4')).toBe(false)
		})

		test('disconnect without argument removes all connections', () => {
			const fakeClient = { end: () => {} } as unknown as import('ssh2').Client
			manager.connections.set('1.2.3.4', fakeClient)
			manager.connections.set('5.6.7.8', fakeClient)

			expect(manager.connections.size).toBe(2)
			manager.disconnect()
			expect(manager.connections.size).toBe(0)
		})

		test('disconnect on unknown host does nothing', () => {
			// Should not throw
			expect(() => manager.disconnect('unknown.host')).not.toThrow()
		})
	})

	describe('exec without connection', () => {
		test('throws SSHError when exec is called without an active connection', async () => {
			expect(manager.exec('no-connection.host', 'echo hi')).rejects.toThrow(SSHError)
		})

		test('thrown SSHError contains the host name', async () => {
			try {
				await manager.exec('target.host', 'ls')
				expect(true).toBe(false) // should not reach here
			} catch (err) {
				expect(err).toBeInstanceOf(SSHError)
				expect((err as SSHError).host).toBe('target.host')
			}
		})
	})

	describe('shell without connection', () => {
		test('throws SSHError when shell is called without an active connection', async () => {
			expect(manager.shell('no-connection.host')).rejects.toThrow(SSHError)
		})
	})

	describe('execStream without connection', () => {
		test('throws SSHError when execStream is called without an active connection', async () => {
			expect(manager.execStream('no-connection.host', 'tail -f /var/log/app.log')).rejects.toThrow(
				SSHError,
			)
		})
	})
})

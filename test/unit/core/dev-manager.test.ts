// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, it, mock, spyOn } from 'bun:test'

// mkcert mock — always unavailable so Caddy is skipped by default
const mockIsMkcertInstalled = mock(() => Promise.resolve(false))
const mockGenerateCerts = mock(() =>
	Promise.resolve({ cert: '/tmp/cert.pem', key: '/tmp/key.pem' }),
)

mock.module('../../../src/providers/ssl/mkcert.ts', () => ({
	isMkcertInstalled: mockIsMkcertInstalled,
	generateCerts: mockGenerateCerts,
}))

import { DevManager } from '../../../src/core/dev-manager.ts'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeDefaultConfig() {
	return {
		app: 'myapp',
		domain: 'myapp.example.com',
		servers: { default: { hosts: ['1.2.3.4'], user: 'deploy' } },
		deploy: { strategy: 'blue-green' },
		services: {
			web: { port: 3000 },
		},
	}
}

describe('DevManager', () => {
	let manager: DevManager
	let writeSpy: ReturnType<typeof spyOn>
	let spawnSyncSpy: ReturnType<typeof spyOn>

	beforeEach(() => {
		manager = new DevManager('/tmp/test-shuttle-dev')

		mockIsMkcertInstalled.mockReset()
		mockIsMkcertInstalled.mockImplementation(() => Promise.resolve(false))

		mockGenerateCerts.mockReset()
		mockGenerateCerts.mockImplementation(() =>
			Promise.resolve({ cert: '/tmp/cert.pem', key: '/tmp/key.pem' }),
		)

		if (writeSpy) writeSpy.mockRestore()
		if (spawnSyncSpy) spawnSyncSpy.mockRestore()

		writeSpy = spyOn(Bun, 'write').mockImplementation(() => Promise.resolve(0))
		spawnSyncSpy = spyOn(Bun, 'spawnSync').mockImplementation(() => ({
			exitCode: 0,
			stdout: null,
			stderr: null,
		}))
	})

	// ---------------------------------------------------------------------------
	// up() — Caddyfile generation
	// ---------------------------------------------------------------------------

	describe('up() — Caddyfile generation', () => {
		it('writes Caddyfile.dev with the config domain', async () => {
			await manager.up(makeDefaultConfig() as any)

			const caddyfileCall = writeSpy.mock.calls.find(([p]) => String(p).endsWith('Caddyfile.dev'))
			expect(caddyfileCall).toBeDefined()
			const content = caddyfileCall[1] as string
			expect(content).toContain('myapp.example.com')
		})

		it('uses config.dev.domain when set', async () => {
			await manager.up({ ...makeDefaultConfig(), dev: { domain: 'myapp.local' } } as any)

			const caddyfileCall = writeSpy.mock.calls.find(([p]) => String(p).endsWith('Caddyfile.dev'))
			expect(caddyfileCall).toBeDefined()
			const content = caddyfileCall[1] as string
			expect(content).toContain('myapp.local')
			expect(content).not.toContain('myapp.example.com')
		})
	})

	// ---------------------------------------------------------------------------
	// up() — docker-compose.dev.yml generation (no SSL → no Caddy service)
	// ---------------------------------------------------------------------------

	describe('up() — compose file generation (no SSL)', () => {
		it('writes docker-compose.dev.yml', async () => {
			await manager.up(makeDefaultConfig() as any)

			const composeCall = writeSpy.mock.calls.find(([p]) =>
				String(p).endsWith('docker-compose.dev.yml'),
			)
			expect(composeCall).toBeDefined()
		})

		it('does not include Caddy ports when SSL is not available', async () => {
			await manager.up(makeDefaultConfig() as any)

			const composeCall = writeSpy.mock.calls.find(([p]) =>
				String(p).endsWith('docker-compose.dev.yml'),
			)
			const content = composeCall?.[1] as string
			expect(content).not.toContain('caddy')
		})
	})

	// ---------------------------------------------------------------------------
	// up() — docker-compose.dev.yml generation (with SSL)
	// ---------------------------------------------------------------------------

	describe('up() — compose file generation (with SSL)', () => {
		beforeEach(() => {
			mockIsMkcertInstalled.mockImplementation(() => Promise.resolve(true))
		})

		it('uses default ports 80/443 for Caddy when config.dev.ports is not set', async () => {
			await manager.up(makeDefaultConfig() as any)

			const composeCall = writeSpy.mock.calls.find(([p]) =>
				String(p).endsWith('docker-compose.dev.yml'),
			)
			const content = composeCall?.[1] as string
			expect(content).toContain('80:80')
			expect(content).toContain('443:443')
		})

		it('uses config.dev.ports.http/https when specified', async () => {
			await manager.up({
				...makeDefaultConfig(),
				dev: { ports: { http: 8080, https: 8443 } },
			} as any)

			const composeCall = writeSpy.mock.calls.find(([p]) =>
				String(p).endsWith('docker-compose.dev.yml'),
			)
			const content = composeCall?.[1] as string
			expect(content).toContain('8080:80')
			expect(content).toContain('8443:443')
		})
	})

	// ---------------------------------------------------------------------------
	// up() — docker compose command
	// ---------------------------------------------------------------------------

	describe('up() — docker compose invocation', () => {
		it('calls docker compose up with correct arguments', async () => {
			await manager.up(makeDefaultConfig() as any)

			expect(spawnSyncSpy).toHaveBeenCalledTimes(1)
			const [args] = spawnSyncSpy.mock.calls[0]
			expect(args).toContain('docker')
			expect(args).toContain('compose')
			expect(args).toContain('up')
			expect(args).toContain('-d')
			expect(args).toContain('--build')
		})

		it('throws when docker compose exits non-zero', async () => {
			spawnSyncSpy.mockImplementation(() => ({ exitCode: 1, stdout: null, stderr: null }))

			await expect(manager.up(makeDefaultConfig() as any)).rejects.toThrow(
				'Failed to start development environment',
			)
		})
	})

	// ---------------------------------------------------------------------------
	// down()
	// ---------------------------------------------------------------------------

	describe('down()', () => {
		it('calls docker compose down', async () => {
			await manager.down()

			expect(spawnSyncSpy).toHaveBeenCalledTimes(1)
			const [args] = spawnSyncSpy.mock.calls[0]
			expect(args).toContain('down')
		})

		it('throws when docker compose down exits non-zero', async () => {
			spawnSyncSpy.mockImplementation(() => ({ exitCode: 1, stdout: null, stderr: null }))
			await expect(manager.down()).rejects.toThrow('Failed to stop development environment')
		})
	})
})

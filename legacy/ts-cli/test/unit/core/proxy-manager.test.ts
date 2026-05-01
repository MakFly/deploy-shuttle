import { describe, expect, test } from 'bun:test'
import type { ShuttleConfig } from '../../../src/config/schema.ts'
import { ProxyManager } from '../../../src/core/proxy-manager.ts'

const createConfig = (overrides: Partial<ShuttleConfig> = {}): ShuttleConfig => ({
	app: 'myapp',
	domain: 'myapp.example.com',
	servers: { default: { hosts: ['1.2.3.4'], user: 'deploy' } },
	...overrides,
})

describe('ProxyManager.generateConfig', () => {
	const pm = new ProxyManager()

	test('generates basic Caddyfile with no upstreams (fallback to localhost:3000)', () => {
		const config = createConfig()
		const result = pm.generateConfig(config, new Map())

		expect(result).toContain('myapp.example.com')
		expect(result).toContain('reverse_proxy localhost:3000')
	})

	test('generates Caddyfile with custom upstream', () => {
		const config = createConfig()
		const upstreams = new Map([['web', '127.0.0.1:10001']])
		const result = pm.generateConfig(config, upstreams)

		expect(result).toContain('myapp.example.com')
		expect(result).toContain('reverse_proxy 127.0.0.1:10001')
	})

	test('includes SSL email in global block', () => {
		const config = createConfig({
			proxy: { ssl: { provider: 'letsencrypt', email: 'admin@example.com' } },
		})
		const result = pm.generateConfig(config, new Map())

		expect(result).toContain('email admin@example.com')
	})

	test('includes custom headers', () => {
		const config = createConfig({
			proxy: {
				headers: {
					'Strict-Transport-Security': 'max-age=63072000',
					'X-Content-Type-Options': 'nosniff',
				},
			},
		})
		const result = pm.generateConfig(config, new Map())

		expect(result).toContain('Strict-Transport-Security')
		expect(result).toContain('max-age=63072000')
		expect(result).toContain('X-Content-Type-Options')
		expect(result).toContain('nosniff')
	})

	test('handles multiple domains', () => {
		const config = createConfig({
			domain: ['myapp.example.com', 'www.myapp.example.com'],
		})
		const result = pm.generateConfig(config, new Map())

		expect(result).toContain('myapp.example.com')
		expect(result).toContain('www.myapp.example.com')
	})
})

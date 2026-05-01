import { describe, expect, test } from 'bun:test'
import path from 'node:path'
import { parse } from 'yaml'
import { loadConfig } from '../../../src/config/loader.ts'
import { shuttleConfigSchema } from '../../../src/config/schema.ts'

const FIXTURES = path.resolve(import.meta.dir, '../../fixtures')

describe('shuttleConfigSchema', () => {
	test('parses minimal config', () => {
		const raw = {
			app: 'myapp',
			domain: 'myapp.example.com',
			server: { host: '203.0.113.10', user: 'deploy' },
		}

		const result = shuttleConfigSchema.safeParse(raw)
		expect(result.success).toBe(true)

		if (result.success) {
			expect(result.data.app).toBe('myapp')
			expect(result.data.domain).toBe('myapp.example.com')
			expect(result.data.servers).toEqual({
				default: { hosts: ['203.0.113.10'], user: 'deploy' },
			})
		}
	})

	test('parses full config with servers map', () => {
		const raw = {
			app: 'myapp',
			domain: ['myapp.example.com', 'www.myapp.example.com'],
			servers: {
				web: { hosts: ['203.0.113.10', '203.0.113.11'], user: 'deploy' },
				worker: { hosts: ['203.0.113.12'], user: 'deploy' },
			},
			deploy: {
				strategy: 'blue-green',
				timeout: 120,
				retain: 5,
			},
			services: {
				web: {
					port: 3000,
					command: 'bun run start',
					healthcheck: { type: 'http', path: '/health', interval: 5, timeout: 3, retries: 5 },
				},
			},
		}

		const result = shuttleConfigSchema.safeParse(raw)
		expect(result.success).toBe(true)

		if (result.success) {
			expect(result.data.servers.web.hosts).toHaveLength(2)
			expect(result.data.servers.worker.hosts).toEqual(['203.0.113.12'])
			expect(result.data.deploy?.strategy).toBe('blue-green')
		}
	})

	test('normalizes server shorthand into servers map', () => {
		const raw = {
			app: 'test',
			domain: 'test.com',
			server: { host: '1.2.3.4', user: 'root' },
		}

		const result = shuttleConfigSchema.parse(raw)
		expect(result.servers).toEqual({
			default: { hosts: ['1.2.3.4'], user: 'root' },
		})
		// server shorthand should not be in the output
		expect('server' in result).toBe(false)
	})

	test('rejects config with neither server nor servers', () => {
		const raw = {
			app: 'myapp',
			domain: 'myapp.example.com',
		}

		const result = shuttleConfigSchema.safeParse(raw)
		expect(result.success).toBe(false)
	})

	test('rejects config with missing app', () => {
		const raw = {
			domain: 'myapp.example.com',
			server: { host: '1.2.3.4', user: 'deploy' },
		}

		const result = shuttleConfigSchema.safeParse(raw)
		expect(result.success).toBe(false)
	})

	test('rejects config with invalid deploy strategy', () => {
		const raw = {
			app: 'myapp',
			domain: 'myapp.example.com',
			server: { host: '1.2.3.4', user: 'deploy' },
			deploy: { strategy: 'invalid-strategy' },
		}

		const result = shuttleConfigSchema.safeParse(raw)
		expect(result.success).toBe(false)
	})

	test('accepts domain as array', () => {
		const raw = {
			app: 'myapp',
			domain: ['a.com', 'b.com'],
			server: { host: '1.2.3.4', user: 'deploy' },
		}

		const result = shuttleConfigSchema.parse(raw)
		expect(result.domain).toEqual(['a.com', 'b.com'])
	})

	test('accepts accessories', () => {
		const raw = {
			app: 'myapp',
			domain: 'myapp.com',
			server: { host: '1.2.3.4', user: 'deploy' },
			accessories: {
				postgres: {
					image: 'postgres:16-alpine',
					volumes: ['/data/pg:/var/lib/postgresql/data'],
					env: { POSTGRES_DB: 'myapp' },
				},
			},
		}

		const result = shuttleConfigSchema.parse(raw)
		expect(result.accessories?.postgres.image).toBe('postgres:16-alpine')
	})

	test('accepts build.platform and notifications.webhooks', () => {
		const raw = {
			app: 'myapp',
			domain: 'myapp.com',
			server: { host: '1.2.3.4', user: 'deploy' },
			build: {
				platform: 'linux/amd64',
			},
			notifications: {
				webhooks: ['https://example.com/webhook'],
			},
		}

		const result = shuttleConfigSchema.parse(raw)
		expect(result.build?.platform).toBe('linux/amd64')
		expect(result.notifications?.webhooks).toEqual(['https://example.com/webhook'])
	})
})

describe('loadConfig', () => {
	test('loads and validates minimal.yml fixture', async () => {
		const config = await loadConfig(path.join(FIXTURES, 'minimal.yml'))

		expect(config.app).toBe('myapp')
		expect(config.domain).toBe('myapp.example.com')
		expect(config.servers.default).toEqual({
			hosts: ['203.0.113.10'],
			user: 'deploy',
		})
		// Defaults should be applied
		expect(config.deploy?.strategy).toBe('blue-green')
		expect(config.deploy?.timeout).toBe(120)
		expect(config.deploy?.retain).toBe(5)
		expect(config.deploy?.auto_rollback).toBe(true)
	})

	test('loads and validates full.yml fixture', async () => {
		const config = await loadConfig(path.join(FIXTURES, 'full.yml'))

		expect(config.app).toBe('myapp')
		expect(Array.isArray(config.domain)).toBe(true)
		expect(config.servers.web.hosts).toContain('203.0.113.10')
		expect(config.servers.worker.hosts).toContain('203.0.113.12')
		expect(config.services?.web.port).toBe(3000)
		expect(config.services?.web.healthcheck?.type).toBe('http')
		expect(config.accessories?.postgres.image).toBe('postgres:16-alpine')
		expect(config.env?.clear?.APP_ENV).toBe('production')
		expect(config.env?.secret).toContain('DATABASE_URL')
		expect(config.proxy?.ssl?.email).toBe('admin@example.com')
	})

	test('throws on invalid config', async () => {
		expect(loadConfig(path.join(FIXTURES, 'invalid.yml'))).rejects.toThrow()
	})

	test('throws on nonexistent file', async () => {
		expect(loadConfig('/nonexistent/path/shuttle.yml')).rejects.toThrow()
	})
})

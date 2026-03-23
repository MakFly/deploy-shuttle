import { describe, expect, test } from 'bun:test'
import { parse } from 'yaml'
import { generateShuttleYml } from '../../../src/templates/shuttle.yml.ts'

describe('generateShuttleYml', () => {
	test('includes app name in output', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		expect(result).toContain('myapp')
	})

	test('includes domain in output', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		expect(result).toContain('myapp.example.com')
	})

	test('includes host in output', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		expect(result).toContain('203.0.113.1')
	})

	test('includes user in output', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		expect(result).toContain('deploy')
	})

	test('all four options appear in the output', () => {
		const result = generateShuttleYml({
			app: 'webapp',
			domain: 'webapp.io',
			host: '10.0.0.5',
			user: 'admin',
		})
		expect(result).toContain('webapp')
		expect(result).toContain('webapp.io')
		expect(result).toContain('10.0.0.5')
		expect(result).toContain('admin')
	})

	test('output is valid YAML', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		expect(() => parse(result)).not.toThrow()
	})

	test('parsed YAML has correct app value', () => {
		const result = generateShuttleYml({
			app: 'myapp',
			domain: 'myapp.example.com',
			host: '203.0.113.1',
			user: 'deploy',
		})
		const parsed = parse(result)
		expect(parsed.app).toBe('myapp')
		expect(parsed.domain).toBe('myapp.example.com')
	})
})

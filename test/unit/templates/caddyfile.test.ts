import { describe, expect, test } from 'bun:test'
import { generateCaddyfile, generateUpstreamSwitch } from '../../../src/templates/caddyfile.ts'

describe('generateCaddyfile', () => {
	test('generates config for a single domain', () => {
		const result = generateCaddyfile([{ domains: ['example.com'], upstream: 'localhost:3000' }])
		expect(result).toContain('example.com')
		expect(result).toContain('reverse_proxy localhost:3000')
	})

	test('generates config for multiple domains on one app', () => {
		const result = generateCaddyfile([
			{ domains: ['example.com', 'www.example.com'], upstream: 'localhost:3000' },
		])
		expect(result).toContain('example.com, www.example.com')
	})

	test('includes global email block when ssl.email is set', () => {
		const result = generateCaddyfile([
			{
				domains: ['example.com'],
				upstream: 'localhost:3000',
				ssl: { email: 'admin@example.com' },
			},
		])
		expect(result).toContain('email admin@example.com')
	})

	test('uses first ssl email found for the global block', () => {
		const result = generateCaddyfile([
			{
				domains: ['a.com'],
				upstream: 'localhost:3001',
				ssl: { email: 'first@example.com' },
			},
			{
				domains: ['b.com'],
				upstream: 'localhost:3002',
				ssl: { email: 'second@example.com' },
			},
		])
		expect(result).toContain('email first@example.com')
		expect(result).not.toContain('email second@example.com')
	})

	test('does not include global block when no ssl email', () => {
		const result = generateCaddyfile([{ domains: ['example.com'], upstream: 'localhost:3000' }])
		expect(result).not.toContain('email')
		// The global block has the form "{\n  email ...\n}" at the very start
		expect(result.startsWith('{')).toBe(false)
		// But does contain the app block braces
		expect(result).toContain('example.com {')
	})

	test('includes custom headers block when headers are provided', () => {
		const result = generateCaddyfile([
			{
				domains: ['example.com'],
				upstream: 'localhost:3000',
				headers: { 'X-Real-IP': '{remote_host}', 'Strict-Transport-Security': 'max-age=31536000' },
			},
		])
		expect(result).toContain('header {')
		expect(result).toContain('X-Real-IP "{remote_host}"')
		expect(result).toContain('Strict-Transport-Security "max-age=31536000"')
	})

	test('does not include header block when headers is empty object', () => {
		const result = generateCaddyfile([
			{ domains: ['example.com'], upstream: 'localhost:3000', headers: {} },
		])
		expect(result).not.toContain('header {')
	})

	test('generates config for multiple apps', () => {
		const result = generateCaddyfile([
			{ domains: ['app1.com'], upstream: 'localhost:3001' },
			{ domains: ['app2.com'], upstream: 'localhost:3002' },
		])
		expect(result).toContain('app1.com')
		expect(result).toContain('reverse_proxy localhost:3001')
		expect(result).toContain('app2.com')
		expect(result).toContain('reverse_proxy localhost:3002')
	})

	test('ends with a newline', () => {
		const result = generateCaddyfile([{ domains: ['example.com'], upstream: 'localhost:3000' }])
		expect(result.endsWith('\n')).toBe(true)
	})
})

describe('generateUpstreamSwitch', () => {
	test('generates correct format', () => {
		const result = generateUpstreamSwitch(['example.com'], 'localhost:4000')
		expect(result).toBe('example.com {\n  reverse_proxy localhost:4000\n}\n')
	})

	test('includes domain in output', () => {
		const result = generateUpstreamSwitch(['myapp.io'], 'localhost:8080')
		expect(result).toContain('myapp.io')
	})

	test('includes upstream in output', () => {
		const result = generateUpstreamSwitch(['myapp.io'], 'localhost:8080')
		expect(result).toContain('reverse_proxy localhost:8080')
	})
})

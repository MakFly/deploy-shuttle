import { afterAll, describe, expect, test } from 'bun:test'
import { mkdtempSync, rmSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { SecretsManager } from '../../../src/core/secrets-manager.ts'

const TEST_ROOT = mkdtempSync(path.join(os.tmpdir(), 'deploy-shuttle-secrets-test-'))

class TestSecretsManager extends SecretsManager {
	override readonly keyPath: string
	override readonly secretsPath: string

	constructor(testDir = mkdtempSync(path.join(TEST_ROOT, 'case-'))) {
		super()
		this.keyPath = path.join(testDir, 'key')
		this.secretsPath = path.join(testDir, 'secrets.enc')
	}
}

afterAll(() => {
	try {
		rmSync(TEST_ROOT, { recursive: true, force: true })
	} catch {}
})

describe('SecretsManager', () => {
	test('set and get a secret', async () => {
		const sm = new TestSecretsManager()
		await sm.set('DATABASE_URL', 'postgres://localhost/db')
		const value = await sm.get('DATABASE_URL')
		expect(value).toBe('postgres://localhost/db')
	})

	test('list returns all keys', async () => {
		const sm = new TestSecretsManager()
		await sm.set('KEY_A', 'value_a')
		await sm.set('KEY_B', 'value_b')
		await sm.set('KEY_C', 'value_c')

		const keys = await sm.list()
		expect(keys).toContain('KEY_A')
		expect(keys).toContain('KEY_B')
		expect(keys).toContain('KEY_C')
		expect(keys).toHaveLength(3)
	})

	test('get returns undefined for nonexistent key', async () => {
		const sm = new TestSecretsManager()
		const value = await sm.get('NONEXISTENT')
		expect(value).toBeUndefined()
	})

	test('set overwrites existing secret', async () => {
		const sm = new TestSecretsManager()
		await sm.set('KEY', 'old_value')
		await sm.set('KEY', 'new_value')
		const value = await sm.get('KEY')
		expect(value).toBe('new_value')
	})

	test('remove deletes a secret', async () => {
		const sm = new TestSecretsManager()
		await sm.set('TO_DELETE', 'value')
		await sm.remove('TO_DELETE')
		const value = await sm.get('TO_DELETE')
		expect(value).toBeUndefined()
	})

	test('remove throws on nonexistent key', async () => {
		const sm = new TestSecretsManager()
		expect(sm.remove('NONEXISTENT')).rejects.toThrow()
	})

	test('loadAll returns empty object when no secrets file', async () => {
		const sm = new TestSecretsManager()
		const all = await sm.loadAll()
		expect(all).toEqual({})
	})

	test('secrets persist across instances', async () => {
		const testDir = mkdtempSync(path.join(TEST_ROOT, 'persist-'))
		const sm1 = new TestSecretsManager(testDir)
		await sm1.set('PERSIST', 'across_instances')

		const sm2 = new TestSecretsManager(testDir)
		const value = await sm2.get('PERSIST')
		expect(value).toBe('across_instances')
	})

	test('handles special characters in values', async () => {
		const sm = new TestSecretsManager()
		const specialValue = 'p@$$w0rd!#%&*()_+-=[]{}|;:"<>?/~`'
		await sm.set('SPECIAL', specialValue)
		const value = await sm.get('SPECIAL')
		expect(value).toBe(specialValue)
	})

	test('handles unicode in values', async () => {
		const sm = new TestSecretsManager()
		await sm.set('UNICODE', 'café résumé 🚀')
		const value = await sm.get('UNICODE')
		expect(value).toBe('café résumé 🚀')
	})
})

import { afterAll, describe, expect, test } from 'bun:test'
import { existsSync, mkdirSync, rmdirSync, unlinkSync } from 'node:fs'
import path from 'node:path'
import { decrypt, encrypt, generateKey, loadOrCreateKey } from '../../../src/utils/crypto.ts'

const TEST_DIR = path.resolve(import.meta.dir, '../../.tmp-crypto-test')

afterAll(() => {
	try {
		const keyFile = path.join(TEST_DIR, 'test.key')
		if (existsSync(keyFile)) unlinkSync(keyFile)
		if (existsSync(TEST_DIR)) {
			rmdirSync(TEST_DIR)
		}
	} catch {}
})

describe('crypto', () => {
	test('generateKey returns 32-byte Buffer', () => {
		const key = generateKey()
		expect(key).toBeInstanceOf(Buffer)
		expect(key.length).toBe(32)
	})

	test('generateKey returns unique keys', () => {
		const k1 = generateKey()
		const k2 = generateKey()
		expect(k1.equals(k2)).toBe(false)
	})

	test('encrypt/decrypt round-trip with simple string', () => {
		const key = generateKey()
		const plaintext = 'hello world'
		const encrypted = encrypt(plaintext, key)
		const decrypted = decrypt(encrypted, key)
		expect(decrypted).toBe(plaintext)
	})

	test('encrypt/decrypt round-trip with JSON', () => {
		const key = generateKey()
		const data = JSON.stringify({ DATABASE_URL: 'postgres://localhost/db', SECRET: 's3cr3t!' })
		const encrypted = encrypt(data, key)
		const decrypted = decrypt(encrypted, key)
		expect(JSON.parse(decrypted)).toEqual(JSON.parse(data))
	})

	test('encrypt/decrypt round-trip with empty string', () => {
		const key = generateKey()
		const encrypted = encrypt('', key)
		const decrypted = decrypt(encrypted, key)
		expect(decrypted).toBe('')
	})

	test('encrypt/decrypt round-trip with unicode', () => {
		const key = generateKey()
		const plaintext = 'Hello 🌍 café résumé'
		const encrypted = encrypt(plaintext, key)
		expect(decrypt(encrypted, key)).toBe(plaintext)
	})

	test('encrypted output is base64', () => {
		const key = generateKey()
		const encrypted = encrypt('test', key)
		expect(() => Buffer.from(encrypted, 'base64')).not.toThrow()
		// Re-encoding should match (valid base64)
		expect(Buffer.from(encrypted, 'base64').toString('base64')).toBe(encrypted)
	})

	test('decryption with wrong key fails', () => {
		const key1 = generateKey()
		const key2 = generateKey()
		const encrypted = encrypt('secret data', key1)
		expect(() => decrypt(encrypted, key2)).toThrow()
	})

	test('decryption with tampered data fails', () => {
		const key = generateKey()
		const encrypted = encrypt('test data', key)
		const buf = Buffer.from(encrypted, 'base64')
		buf[buf.length - 1] ^= 0xff // Flip last byte
		const tampered = buf.toString('base64')
		expect(() => decrypt(tampered, key)).toThrow()
	})

	test('each encryption produces different output (random IV)', () => {
		const key = generateKey()
		const e1 = encrypt('same text', key)
		const e2 = encrypt('same text', key)
		expect(e1).not.toBe(e2)
		// But both decrypt to the same value
		expect(decrypt(e1, key)).toBe('same text')
		expect(decrypt(e2, key)).toBe('same text')
	})
})

describe('loadOrCreateKey', () => {
	test('creates a new key file when it does not exist', async () => {
		mkdirSync(TEST_DIR, { recursive: true })
		const keyPath = path.join(TEST_DIR, 'test.key')

		if (existsSync(keyPath)) unlinkSync(keyPath)

		const key = await loadOrCreateKey(keyPath)
		expect(key).toBeInstanceOf(Buffer)
		expect(key.length).toBe(32)
		expect(existsSync(keyPath)).toBe(true)
	})

	test('loads existing key file', async () => {
		mkdirSync(TEST_DIR, { recursive: true })
		const keyPath = path.join(TEST_DIR, 'test.key')

		const firstKey = await loadOrCreateKey(keyPath)
		const secondKey = await loadOrCreateKey(keyPath)
		expect(firstKey.equals(secondKey)).toBe(true)
	})
})

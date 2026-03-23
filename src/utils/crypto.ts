import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto'
import { existsSync } from 'node:fs'
import { chmod, readFile, writeFile } from 'node:fs/promises'

const ALGORITHM = 'aes-256-gcm'
const IV_LENGTH = 12
const AUTH_TAG_LENGTH = 16

export function generateKey(): Buffer {
	return randomBytes(32)
}

/**
 * Encrypts plaintext with AES-256-GCM.
 * Output format (base64-encoded): iv (12 bytes) | authTag (16 bytes) | ciphertext
 */
export function encrypt(plaintext: string, key: Buffer): string {
	const iv = randomBytes(IV_LENGTH)
	const cipher = createCipheriv(ALGORITHM, key, iv)

	const encrypted = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])

	const authTag = cipher.getAuthTag()

	const combined = Buffer.concat([iv, authTag, encrypted])
	return combined.toString('base64')
}

/**
 * Decrypts a base64-encoded AES-256-GCM blob produced by `encrypt`.
 */
export function decrypt(encrypted: string, key: Buffer): string {
	const combined = Buffer.from(encrypted, 'base64')

	const iv = combined.subarray(0, IV_LENGTH)
	const authTag = combined.subarray(IV_LENGTH, IV_LENGTH + AUTH_TAG_LENGTH)
	const ciphertext = combined.subarray(IV_LENGTH + AUTH_TAG_LENGTH)

	const decipher = createDecipheriv(ALGORITHM, key, iv)
	decipher.setAuthTag(authTag)

	const decrypted = Buffer.concat([decipher.update(ciphertext), decipher.final()])

	return decrypted.toString('utf8')
}

/**
 * Loads an encryption key from `keyPath`.
 * If the file does not exist, generates a new key, writes it (chmod 600), and returns it.
 */
export async function loadOrCreateKey(keyPath: string): Promise<Buffer> {
	if (existsSync(keyPath)) {
		const raw = await readFile(keyPath)
		return raw
	}

	const key = generateKey()
	await writeFile(keyPath, key, { mode: 0o600 })
	// Explicitly enforce permissions (writeFile mode may be masked by umask on some systems)
	await chmod(keyPath, 0o600)
	return key
}

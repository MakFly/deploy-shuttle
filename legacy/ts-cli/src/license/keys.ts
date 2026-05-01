import { generateKeyPairSync } from 'node:crypto'

/**
 * Ed25519 public key for offline JWT verification.
 *
 * In production, this is the ONLY key shipped with the CLI.
 * The private key is kept on the shuttle.dev signing server.
 *
 * To generate a real keypair for production:
 *   bun run scripts/generate-keypair.ts
 *
 * Set SHUTTLE_LICENSE_PUBLIC_KEY in the build environment to override the
 * dev keypair at runtime.
 */

// Ephemeral dev keypair — generated once per process, never persisted.
// In production the env var SHUTTLE_LICENSE_PUBLIC_KEY overrides this.
let _devKeypair: { publicKey: string; privateKey: string } | null = null

function getDevKeypair(): { publicKey: string; privateKey: string } {
	if (!_devKeypair) {
		const { publicKey, privateKey } = generateKeyPairSync('ed25519', {
			publicKeyEncoding: { type: 'spki', format: 'pem' },
			privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
		})
		_devKeypair = { publicKey, privateKey }
	}
	return _devKeypair
}

/**
 * Get the public key for JWT verification.
 *
 * Resolution order:
 *   1. SHUTTLE_LICENSE_PUBLIC_KEY env var (production build injects this)
 *   2. Ephemeral dev keypair generated in-process (for local dev / tests)
 */
export function getPublicKey(): string {
	if (process.env.SHUTTLE_LICENSE_PUBLIC_KEY) {
		return process.env.SHUTTLE_LICENSE_PUBLIC_KEY
	}
	return getDevKeypair().publicKey
}

/**
 * Get the private key for signing — DEV / TEST ONLY.
 *
 * This key is ephemeral and process-scoped. It matches the public key returned
 * by `getPublicKey()` only when SHUTTLE_LICENSE_PUBLIC_KEY is not set.
 * Used by tests to mint valid license tokens without a signing server.
 */
export function getDevPrivateKey(): string {
	return getDevKeypair().privateKey
}

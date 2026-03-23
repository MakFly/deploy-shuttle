import { createPublicKey, verify as cryptoVerify } from 'node:crypto'
import { chmodSync, existsSync, mkdirSync, readFileSync, unlinkSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { logger } from '../utils/logger.ts'
import { getPublicKey } from './keys.ts'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface LicenseInfo {
	email: string
	plan: string
	features: string[]
	expiresAt: Date
	issuedAt: Date
}

interface LicensePayload {
	sub: string // email
	iss: string // shuttle.dev
	iat: number // issued at (unix seconds)
	exp: number // expires at (unix seconds)
	features: string[]
	plan: string
}

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

const LICENSE_DIR = join(homedir(), '.shuttle')
export const LICENSE_FILE = join(LICENSE_DIR, 'license')

// ---------------------------------------------------------------------------
// Base64url helpers
// ---------------------------------------------------------------------------

function base64UrlDecode(str: string): Buffer {
	// Convert base64url → standard base64, then decode
	const padded = str.replace(/-/g, '+').replace(/_/g, '/')
	const pad = (4 - (padded.length % 4)) % 4
	return Buffer.from(padded + '='.repeat(pad), 'base64')
}

// ---------------------------------------------------------------------------
// Core validation
// ---------------------------------------------------------------------------

/**
 * Decode and verify a compact JWT signed with Ed25519.
 * Returns a `LicenseInfo` on success, or `null` if the token is malformed,
 * the signature is invalid, or required claims are missing.
 *
 * NOTE: Expiry is NOT checked here — callers decide how to handle it.
 */
export function validateLicense(token: string): LicenseInfo | null {
	try {
		const parts = token.trim().split('.')
		if (parts.length !== 3) return null

		const [headerB64, payloadB64, signatureB64] = parts as [string, string, string]

		// Decode and sanity-check the header
		const header = JSON.parse(base64UrlDecode(headerB64).toString()) as Record<string, unknown>
		if (header['alg'] !== 'EdDSA' && header['alg'] !== 'Ed25519') {
			logger.debug(`License: unexpected alg "${String(header['alg'])}"`)
			return null
		}

		// Decode payload
		const payload = JSON.parse(base64UrlDecode(payloadB64).toString()) as LicensePayload

		// Verify issuer claim
		if (payload.iss !== 'shuttle.dev') {
			logger.debug(`License: unexpected issuer "${payload.iss}"`)
			return null
		}

		// Verify required claims present
		if (!payload.sub || typeof payload.iat !== 'number' || typeof payload.exp !== 'number') {
			logger.debug('License: missing required claims')
			return null
		}

		// Verify Ed25519 signature
		const publicKeyPem = getPublicKey()
		if (!publicKeyPem) {
			logger.debug('License: no public key configured')
			return null
		}

		const key = createPublicKey(publicKeyPem)
		const data = Buffer.from(`${headerB64}.${payloadB64}`)
		const signature = base64UrlDecode(signatureB64)

		const valid = cryptoVerify(null, data, key, signature)
		if (!valid) {
			logger.debug('License: signature verification failed')
			return null
		}

		return {
			email: payload.sub,
			plan: payload.plan ?? 'pro',
			features: Array.isArray(payload.features) ? payload.features : [],
			expiresAt: new Date(payload.exp * 1000),
			issuedAt: new Date(payload.iat * 1000),
		}
	} catch (err) {
		logger.debug(`License validation error: ${err instanceof Error ? err.message : String(err)}`)
		return null
	}
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

/**
 * Load a license from the environment or the on-disk license file.
 *
 * Resolution order:
 *   1. SHUTTLE_LICENSE_KEY env var
 *   2. ~/.shuttle/license file
 *   3. null (no license)
 */
export function loadLicense(): LicenseInfo | null {
	// 1. Environment variable
	const envKey = process.env.SHUTTLE_LICENSE_KEY
	if (envKey) {
		const info = validateLicense(envKey)
		if (info) return info
		logger.debug('SHUTTLE_LICENSE_KEY is set but contains an invalid token')
	}

	// 2. On-disk license file
	if (existsSync(LICENSE_FILE)) {
		try {
			const token = readFileSync(LICENSE_FILE, 'utf-8').trim()
			const info = validateLicense(token)
			if (info) return info
			logger.debug('~/.shuttle/license exists but contains an invalid token')
		} catch (err) {
			logger.debug(`Failed to read license file: ${err instanceof Error ? err.message : String(err)}`)
		}
	}

	return null
}

/**
 * Persist a license token to ~/.shuttle/license (mode 0600).
 */
export function saveLicense(token: string): void {
	mkdirSync(LICENSE_DIR, { recursive: true })
	writeFileSync(LICENSE_FILE, token, { encoding: 'utf-8' })
	chmodSync(LICENSE_FILE, 0o600)
}

/**
 * Remove the license file if it exists. Silently ignores missing files.
 */
export function removeLicense(): void {
	try {
		unlinkSync(LICENSE_FILE)
	} catch {
		// File does not exist — nothing to do
	}
}

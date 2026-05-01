#!/usr/bin/env bun
/**
 * Sign a test Shuttle Pro license JWT.
 *
 * Usage: bun run scripts/sign-license.ts [email] [plan] [days] [org_id] [role]
 *
 * Examples:
 *   bun run scripts/sign-license.ts                                         # individual pro
 *   bun run scripts/sign-license.ts user@example.com pro 365                # individual pro
 *   bun run scripts/sign-license.ts user@corp.com team 365 acme-uuid admin  # team license
 */

import { createPrivateKey, sign } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const devKeyPath = join(import.meta.dir, '..', '.shuttle-dev', 'license.key')

if (!existsSync(devKeyPath)) {
	console.error('No private key found at .shuttle-dev/license.key')
	console.error('Run: bun run scripts/generate-keypair.ts first')
	process.exit(1)
}

const privateKeyPem = readFileSync(devKeyPath, 'utf-8')
const privateKey = createPrivateKey(privateKeyPem)

// Parse args
const email = process.argv[2] ?? 'dev@shuttle.dev'
const plan = process.argv[3] ?? 'pro'
const days = Number(process.argv[4] ?? '365')
const orgId = process.argv[5] as string | undefined
const role = process.argv[6] as 'admin' | 'deployer' | 'viewer' | undefined

const ALL_FEATURES = [
	'traefik',
	'cloudflare-tunnel',
	'doppler',
	'vault',
	'slack',
	'pagerduty',
	'canary',
]

const now = Math.floor(Date.now() / 1000)
const exp = now + days * 86400

// Build JWT
const header = {
	alg: 'EdDSA',
	typ: 'JWT',
}

const payload: Record<string, unknown> = {
	sub: email,
	iss: 'shuttle.dev',
	iat: now,
	exp,
	plan,
	features: ALL_FEATURES,
}

// Add team claims if org_id is provided
if (orgId) {
	payload.org_id = orgId
	payload.org_name = orgId // Use org_id as name for dev scripts
	payload.seats = 10
	payload.role = role ?? 'admin'
	payload.seat_id = `${orgId}-${email}`
}

function base64UrlEncode(data: string | Buffer): string {
	const buf = typeof data === 'string' ? Buffer.from(data) : data
	return buf.toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

const headerB64 = base64UrlEncode(JSON.stringify(header))
const payloadB64 = base64UrlEncode(JSON.stringify(payload))
const signingInput = `${headerB64}.${payloadB64}`

const signature = sign(null, Buffer.from(signingInput), privateKey)
const signatureB64 = base64UrlEncode(signature)

const token = `${signingInput}.${signatureB64}`

console.log('=== Shuttle Pro License Token ===\n')
console.log(`Email:    ${email}`)
console.log(`Plan:     ${plan}`)
console.log(`Features: ${ALL_FEATURES.join(', ')}`)
console.log(`Issued:   ${new Date(now * 1000).toISOString().slice(0, 10)}`)
console.log(`Expires:  ${new Date(exp * 1000).toISOString().slice(0, 10)}`)
console.log()
console.log('--- Token (copy this) ---\n')
console.log(token)
console.log()
console.log('--- Activate with ---\n')
console.log(`shuttle license activate ${token}`)
console.log()
console.log('--- Or set env var ---\n')
console.log(`export SHUTTLE_LICENSE_KEY="${token}"`)

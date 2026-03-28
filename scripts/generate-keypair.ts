#!/usr/bin/env bun
/**
 * Generate an Ed25519 keypair for Shuttle license signing.
 *
 * Usage: bun run scripts/generate-keypair.ts
 *
 * Output:
 * - Public key (PEM) — embed in src/license/keys.ts
 * - Private key (PEM) — keep SECRET, use for signing only
 */

import { generateKeyPairSync } from 'node:crypto'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const { publicKey, privateKey } = generateKeyPairSync('ed25519', {
	publicKeyEncoding: { type: 'spki', format: 'pem' },
	privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
})

// Save to .shuttle-dev/ directory (gitignored)
const devDir = join(import.meta.dir, '..', '.shuttle-dev')
mkdirSync(devDir, { recursive: true })

writeFileSync(join(devDir, 'license.pub'), publicKey, 'utf-8')
writeFileSync(join(devDir, 'license.key'), privateKey, { mode: 0o600 })

console.log('=== Ed25519 Keypair Generated ===\n')
console.log('Public key saved to: .shuttle-dev/license.pub')
console.log('Private key saved to: .shuttle-dev/license.key\n')

console.log('--- Copy this into src/license/keys.ts ---\n')
console.log(`export const LICENSE_PUBLIC_KEY = \`${publicKey.trim()}\`\n`)

console.log('--- Keep this SECRET (for signing only) ---\n')
console.log(privateKey)

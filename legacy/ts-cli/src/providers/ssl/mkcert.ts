import { logger } from '@/utils/logger.ts'

/**
 * Check if mkcert is installed and CA is set up.
 */
export async function isMkcertInstalled(): Promise<boolean> {
	try {
		const result = Bun.spawnSync(['mkcert', '-CAROOT'])
		return result.exitCode === 0
	} catch {
		return false
	}
}

/**
 * Get the mkcert CA root directory.
 */
export function getMkcertCARoot(): string | null {
	try {
		const result = Bun.spawnSync(['mkcert', '-CAROOT'])
		if (result.exitCode !== 0) return null
		return result.stdout.toString().trim()
	} catch {
		return null
	}
}

/**
 * Generate SSL certificates using mkcert.
 */
export async function generateCerts(
	certDir: string,
	domains: string[],
): Promise<{ cert: string; key: string }> {
	const { mkdir } = await import('node:fs/promises')
	await mkdir(certDir, { recursive: true })

	const certFile = `${certDir}/cert.pem`
	const keyFile = `${certDir}/key.pem`

	const allDomains = ['localhost', '127.0.0.1', '::1', ...domains]

	const args = ['mkcert', '-cert-file', certFile, '-key-file', keyFile, ...allDomains]

	const result = Bun.spawnSync(args)
	if (result.exitCode !== 0) {
		throw new Error(`mkcert failed: ${result.stderr.toString()}`)
	}

	logger.success(`SSL certificates generated for ${allDomains.join(', ')}`)
	return { cert: certFile, key: keyFile }
}

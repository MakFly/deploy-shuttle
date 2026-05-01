import type { SSHManager } from '../../core/ssh-manager.ts'
import { decrypt, encrypt, loadOrCreateKey } from '../../utils/crypto.ts'
import { SecretsError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import { formatEnvLine, shellEscape } from '../../utils/shell.ts'
import type { SecretsProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// AESSecretsProvider
// ---------------------------------------------------------------------------

export class AESSecretsProvider implements SecretsProvider {
	readonly keyPath: string = '.shuttle/key'
	readonly secretsPath: string = '.shuttle/secrets.enc'

	/**
	 * Adds or updates a secret key-value pair.
	 */
	async set(key: string, value: string): Promise<void> {
		const secrets = await this.loadAll()
		secrets[key] = value
		await this.saveAll(secrets)
		logger.debug(`Secret "${key}" saved`)
	}

	/**
	 * Returns the plaintext value for a given key, or undefined if not found.
	 */
	async get(key: string): Promise<string | undefined> {
		const secrets = await this.loadAll()
		return secrets[key]
	}

	/**
	 * Returns all secret keys (never the values).
	 */
	async list(): Promise<string[]> {
		const secrets = await this.loadAll()
		return Object.keys(secrets)
	}

	/**
	 * Removes a key from the secrets store.
	 */
	async remove(key: string): Promise<void> {
		const secrets = await this.loadAll()

		if (!(key in secrets)) {
			throw new SecretsError(`Secret "${key}" not found`)
		}

		delete secrets[key]
		await this.saveAll(secrets)
		logger.debug(`Secret "${key}" removed`)
	}

	/**
	 * Decrypts all secrets, formats them as an .env file, and uploads it to the
	 * remote host at /opt/shuttle/<app>/<app>/.env with mode 0600.
	 */
	async push(host: string, app: string, ssh: SSHManager): Promise<void> {
		const secrets = await this.loadAll()

		if (Object.keys(secrets).length === 0) {
			logger.warn('No secrets to push')
			return
		}

		const envContent = `${Object.entries(secrets)
			.map(([k, v]) => formatEnvLine(k, v))
			.join('\n')}\n`

		const remotePath = `/opt/shuttle/${app}/${app}/.env`

		// Ensure the directory exists on the remote
		await ssh.exec(host, `mkdir -p ${shellEscape(`/opt/shuttle/${app}/${app}`)}`)

		await ssh.uploadContent(host, envContent, remotePath, 0o600)

		logger.success(`Secrets pushed to ${host}:${remotePath}`)
	}

	// ---------------------------------------------------------------------------
	// Internal helpers
	// ---------------------------------------------------------------------------

	/**
	 * Loads and decrypts the full secrets map.
	 * Returns an empty object if no secrets file exists yet.
	 */
	async loadAll(): Promise<Record<string, string>> {
		const file = Bun.file(this.secretsPath)

		const exists = await file.exists()
		if (!exists) {
			return {}
		}

		try {
			const encryptedData = await file.text()
			const key = await loadOrCreateKey(this.keyPath)
			const json = decrypt(encryptedData, key)
			return JSON.parse(json) as Record<string, string>
		} catch (err) {
			throw SecretsError.wrap(err, 'Failed to load secrets')
		}
	}

	/**
	 * Encrypts and persists the full secrets map.
	 */
	async saveAll(secrets: Record<string, string>): Promise<void> {
		try {
			const key = await loadOrCreateKey(this.keyPath)
			const json = JSON.stringify(secrets, null, 2)
			const encrypted = encrypt(json, key)
			await Bun.write(this.secretsPath, encrypted)
		} catch (err) {
			throw SecretsError.wrap(err, 'Failed to save secrets')
		}
	}
}

export function createAESSecretsProvider(): AESSecretsProvider {
	return new AESSecretsProvider()
}

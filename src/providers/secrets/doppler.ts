import type { SSHManager } from '../../core/ssh-manager.ts'
import { ConfigError } from '../../utils/errors.ts'
import { logger } from '../../utils/logger.ts'
import type { SecretsProvider } from '../types.ts'

const DOPPLER_API = 'https://api.doppler.com/v3'

export class DopplerSecretsProvider implements SecretsProvider {
	private readonly token: string
	private readonly project: string
	private readonly dopplerConfig: string

	constructor() {
		const token = process.env.DOPPLER_TOKEN
		if (!token) {
			throw new ConfigError('DOPPLER_TOKEN env var is required for the Doppler secrets provider')
		}
		this.token = token
		this.project = process.env.DOPPLER_PROJECT ?? ''
		this.dopplerConfig = process.env.DOPPLER_CONFIG ?? 'production'
	}

	private async apiRequest(path: string, options: RequestInit = {}): Promise<Response> {
		return fetch(`${DOPPLER_API}${path}`, {
			...options,
			headers: {
				authorization: `Bearer ${this.token}`,
				'content-type': 'application/json',
				...(options.headers as Record<string, string>),
			},
		})
	}

	async set(key: string, value: string): Promise<void> {
		const response = await this.apiRequest('/configs/config/secrets', {
			method: 'POST',
			body: JSON.stringify({ project: this.project, config: this.dopplerConfig, secrets: { [key]: value } }),
		})
		if (!response.ok) {
			throw new ConfigError(`Doppler: failed to set secret "${key}" (${response.status})`)
		}
	}

	async get(key: string): Promise<string | undefined> {
		const secrets = await this.loadAll()
		return secrets[key]
	}

	async list(): Promise<string[]> {
		const secrets = await this.loadAll()
		return Object.keys(secrets)
	}

	async remove(key: string): Promise<void> {
		await this.set(key, '')
		logger.warn(`Doppler: secret "${key}" set to empty (Doppler API does not support single-key deletion)`)
	}

	async loadAll(): Promise<Record<string, string>> {
		const response = await this.apiRequest(
			`/configs/config/secrets/download?project=${encodeURIComponent(this.project)}&config=${encodeURIComponent(this.dopplerConfig)}&format=json`,
		)
		if (!response.ok) {
			throw new ConfigError(`Doppler: failed to fetch secrets (${response.status})`)
		}
		return (await response.json()) as Record<string, string>
	}

	async push(host: string, app: string, ssh: SSHManager): Promise<void> {
		const secrets = await this.loadAll()
		const envContent = Object.entries(secrets).map(([k, v]) => `${k}=${v}`).join('\n')
		const remotePath = `/opt/shuttle/${app}/${app}/.env`
		await ssh.uploadContent(host, envContent, remotePath, 0o600)
		logger.success(`Pushed ${Object.keys(secrets).length} secrets from Doppler to ${host}`)
	}
}

import type {
	BuildConfig,
	DeployConfig,
	HealthcheckConfig,
	NotificationsConfig,
	ProxyConfig,
	RegistryConfig,
	SecretsConfig,
} from './schema.ts'

// ---------------------------------------------------------------------------
// Build defaults
// ---------------------------------------------------------------------------

export const buildDefaults: Required<Omit<BuildConfig, 'target' | 'platform' | 'args'>> &
	Pick<BuildConfig, 'target' | 'platform' | 'args'> = {
	dockerfile: 'Dockerfile',
	context: '.',
	target: undefined,
	platform: undefined,
	args: undefined,
}

// ---------------------------------------------------------------------------
// Deploy defaults
// ---------------------------------------------------------------------------

export const deployDefaults: Required<
	Omit<DeployConfig, 'hooks' | 'concurrency'> & {
		blue_green: Required<NonNullable<DeployConfig['blue_green']>>
		hooks: Required<NonNullable<DeployConfig['hooks']>>
		concurrency: number
	}
> = {
	strategy: 'blue-green',
	timeout: 120,
	retain: 5,
	auto_rollback: true,
	concurrency: 5,
	blue_green: {
		drain_timeout: 30,
		readiness_delay: 5,
	},
	hooks: {
		pre_deploy: [],
		post_deploy: [],
	},
}

// ---------------------------------------------------------------------------
// Service healthcheck defaults
// ---------------------------------------------------------------------------

export const healthcheckDefaults: Extract<HealthcheckConfig, { type: 'http' }> & {
	interval: number
	timeout: number
	retries: number
} = {
	type: 'http',
	path: '/health',
	interval: 5,
	timeout: 3,
	retries: 5,
}

// ---------------------------------------------------------------------------
// Proxy defaults
// ---------------------------------------------------------------------------

export const registryDefaults: Required<Pick<RegistryConfig, 'driver'>> = {
	driver: 'local-transfer',
}

export const proxyDefaults: Required<Omit<ProxyConfig, 'driver'>> & Pick<ProxyConfig, 'driver'> = {
	driver: 'caddy',
	ssl: {
		provider: 'letsencrypt',
	},
	headers: {},
}

// ---------------------------------------------------------------------------
// Secrets defaults
// ---------------------------------------------------------------------------

export const secretsDefaults: Pick<Required<SecretsConfig>, 'driver'> = {
	driver: 'aes',
}

// ---------------------------------------------------------------------------
// Notifications defaults
// ---------------------------------------------------------------------------

export const notificationsDefaults: Required<NotificationsConfig> = {
	webhooks: [],
}

// ---------------------------------------------------------------------------
// Combined defaults object
// ---------------------------------------------------------------------------

export const defaults = {
	build: buildDefaults,
	deploy: deployDefaults,
	healthcheck: healthcheckDefaults,
	registry: registryDefaults,
	proxy: proxyDefaults,
	secrets: secretsDefaults,
	notifications: notificationsDefaults,
} as const

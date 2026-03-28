import type { ShuttleConfig } from '../config/schema.ts'
import type { DockerManager } from '../core/docker-manager.ts'
import type { SSHManager } from '../core/ssh-manager.ts'
import { requirePremium } from '../license/gate.ts'
import { ConfigError } from '../utils/errors.ts'
import { SlackNotificationsProvider } from './notifications/slack.ts'
import { WebhookNotificationsProvider } from './notifications/webhook.ts'
import { createCaddyProxyProvider } from './proxy/caddy.ts'
import { NoopProxyProvider } from './proxy/noop.ts'
import { TraefikProxyProvider } from './proxy/traefik.ts'
import { DockerHubRegistry } from './registry/docker-hub.ts'
import { GHCRRegistry } from './registry/ghcr.ts'
import { ImageRefRegistry } from './registry/image-ref.ts'
import { LocalRegistryProvider } from './registry/local-registry.ts'
import { LocalTransferRegistry } from './registry/local-transfer.ts'
import { AESSecretsProvider } from './secrets/aes.ts'
import { DopplerSecretsProvider } from './secrets/doppler.ts'
import { CloudflareTunnelProvider } from './tunnel/cloudflare.ts'
import type {
	NotificationsProvider,
	ProxyProvider,
	RegistryProvider,
	SecretsProvider,
	TunnelProvider,
} from './types.ts'

// ---------------------------------------------------------------------------
// Registry resolver
// ---------------------------------------------------------------------------

/**
 * Maps the registry driver from config to the appropriate RegistryProvider
 * implementation. Falls back to `local-transfer` when no driver is specified,
 * and uses `image-ref` when `image:` is set without an explicit driver.
 */
export function resolveRegistry(
	config: ShuttleConfig,
	docker: DockerManager,
	ssh: SSHManager,
): RegistryProvider {
	// When image: is set and no explicit driver is declared, pull the pre-built image directly.
	if (config.image && !config.registry?.driver) {
		return new ImageRefRegistry(docker)
	}

	const driver = config.registry?.driver ?? 'local-transfer'

	// Premium provider gates — when premium providers are added, call
	// requirePremium() before instantiating the provider so unlicensed users
	// get a clear error message instead of a runtime failure:
	//
	//   case 'traefik': requirePremium('traefik'); return new TraefikProxy(ssh)
	//   case 'doppler': requirePremium('doppler'); return new DopplerSecrets()

	switch (driver) {
		case 'ghcr':
			return new GHCRRegistry(docker, ssh)
		case 'docker-hub':
			return new DockerHubRegistry(docker, ssh)
		case 'local-transfer':
			return new LocalTransferRegistry(docker)
		case 'local-registry':
			return new LocalRegistryProvider(docker, ssh)
		case 'custom':
			// Custom registries follow the same login-then-pull flow as GHCR.
			return new GHCRRegistry(docker, ssh)
		default: {
			// TypeScript exhaustiveness guard — `driver` is `never` here if the
			// schema enum stays in sync with this switch.
			const _exhaustive: never = driver
			throw new Error(`Unknown registry driver: ${String(_exhaustive)}`)
		}
	}
}

// ---------------------------------------------------------------------------
// Proxy resolver
// ---------------------------------------------------------------------------

export function resolveProxy(config: ShuttleConfig, ssh: SSHManager): ProxyProvider {
	const driver = config.proxy?.driver ?? 'caddy'

	switch (driver) {
		case 'caddy':
			return createCaddyProxyProvider(ssh)
		case 'none':
			return new NoopProxyProvider()
		case 'traefik':
			requirePremium('traefik')
			return new TraefikProxyProvider(ssh)
		default: {
			const _exhaustive: never = driver
			throw new Error(`Unknown proxy driver: ${String(_exhaustive)}`)
		}
	}
}

// ---------------------------------------------------------------------------
// Notifications resolver
// ---------------------------------------------------------------------------

export function resolveNotifications(config: ShuttleConfig): NotificationsProvider[] {
	const providers: NotificationsProvider[] = []

	if (config.notifications?.webhooks?.length) {
		providers.push(new WebhookNotificationsProvider())
	}

	if (config.notifications?.slack) {
		requirePremium('slack')
		providers.push(new SlackNotificationsProvider(config.notifications.slack.webhook_url))
	}

	return providers
}

// ---------------------------------------------------------------------------
// Tunnel resolver
// ---------------------------------------------------------------------------

export function resolveTunnel(config: ShuttleConfig, ssh: SSHManager): TunnelProvider | null {
	const driver = config.tunnel?.driver ?? 'none'

	switch (driver) {
		case 'none':
			return null
		case 'cloudflare':
			requirePremium('cloudflare-tunnel')
			return new CloudflareTunnelProvider(ssh)
		default: {
			const _exhaustive: never = driver
			throw new Error(`Unknown tunnel driver: ${String(_exhaustive)}`)
		}
	}
}

// ---------------------------------------------------------------------------
// Secrets resolver
// ---------------------------------------------------------------------------

export function resolveSecrets(config: ShuttleConfig): SecretsProvider {
	const driver = config.secrets?.driver ?? 'aes'

	switch (driver) {
		case 'aes':
			return new AESSecretsProvider()
		case 'doppler':
			requirePremium('doppler')
			return new DopplerSecretsProvider()
		case 'vault':
			requirePremium('vault')
			throw new ConfigError('Vault secrets provider not yet implemented')
		default: {
			const _exhaustive: never = driver
			throw new Error(`Unknown secrets driver: ${String(_exhaustive)}`)
		}
	}
}

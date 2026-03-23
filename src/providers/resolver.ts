import type { ShuttleConfig } from '../config/schema.ts'
import type { DockerManager } from '../core/docker-manager.ts'
import type { SSHManager } from '../core/ssh-manager.ts'
import { requirePremium } from '../license/gate.ts'
import type { RegistryProvider } from './types.ts'
import { DockerHubRegistry } from './registry/docker-hub.ts'
import { GHCRRegistry } from './registry/ghcr.ts'
import { ImageRefRegistry } from './registry/image-ref.ts'
import { LocalTransferRegistry } from './registry/local-transfer.ts'

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

// @ts-nocheck — mock.calls types are not compatible with strict TS
import { describe, expect, it, mock } from 'bun:test'
import { createConfig } from '../../helpers/config-factory.ts'
import { createMockDocker } from '../../helpers/mock-docker.ts'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

// Mock license gate so premium features throw ConfigError in tests
mock.module('../../../src/license/gate.ts', () => ({
	requirePremium: mock((feature: string) => {
		const { ShuttleError } = require('../../../src/utils/errors.ts')
		throw new ShuttleError(`"${feature}" is a Pro feature.`, 'LICENSE_REQUIRED')
	}),
}))

import { NoopProxyProvider } from '../../../src/providers/proxy/noop.ts'
import { GHCRRegistry } from '../../../src/providers/registry/ghcr.ts'
import { ImageRefRegistry } from '../../../src/providers/registry/image-ref.ts'
import { LocalTransferRegistry } from '../../../src/providers/registry/local-transfer.ts'
import { resolveProxy, resolveRegistry, resolveSecrets } from '../../../src/providers/resolver.ts'
import { AESSecretsProvider } from '../../../src/providers/secrets/aes.ts'
import { ConfigError, ShuttleError } from '../../../src/utils/errors.ts'

describe('resolveSecrets', () => {
	it('returns AESSecretsProvider when driver is not set (default)', () => {
		const config = createConfig()
		const result = resolveSecrets(config)
		expect(result).toBeInstanceOf(AESSecretsProvider)
	})

	it('returns AESSecretsProvider when driver is explicitly "aes"', () => {
		const config = createConfig({ secrets: { driver: 'aes' } as any })
		const result = resolveSecrets(config)
		expect(result).toBeInstanceOf(AESSecretsProvider)
	})

	it('throws when driver is "doppler" (premium feature, no license)', () => {
		const config = createConfig({ secrets: { driver: 'doppler' } as any })
		expect(() => resolveSecrets(config)).toThrow(ShuttleError)
	})

	it('throws when driver is "vault" (premium feature, no license)', () => {
		const config = createConfig({ secrets: { driver: 'vault' } as any })
		expect(() => resolveSecrets(config)).toThrow(ShuttleError)
	})
})

describe('resolveProxy', () => {
	const mockSSH = createMockSSH()

	it('returns a CaddyProxyProvider when driver is not set (default)', () => {
		const config = createConfig()
		const result = resolveProxy(config, mockSSH as any)
		// CaddyProxyProvider is created via factory; verify it has the expected interface
		expect(typeof result.apply).toBe('function')
		expect(typeof result.switchUpstream).toBe('function')
		expect(typeof result.getStatus).toBe('function')
		expect(result).not.toBeInstanceOf(NoopProxyProvider)
	})

	it('returns a CaddyProxyProvider when driver is explicitly "caddy"', () => {
		const config = createConfig({ proxy: { driver: 'caddy' } as any })
		const result = resolveProxy(config, mockSSH as any)
		expect(result).not.toBeInstanceOf(NoopProxyProvider)
		expect(typeof result.apply).toBe('function')
	})

	it('returns NoopProxyProvider when driver is "none"', () => {
		const config = createConfig({ proxy: { driver: 'none' } as any })
		const result = resolveProxy(config, mockSSH as any)
		expect(result).toBeInstanceOf(NoopProxyProvider)
	})

	it('throws (ShuttleError) when driver is "traefik" (premium, no license)', () => {
		const config = createConfig({ proxy: { driver: 'traefik' } as any })
		expect(() => resolveProxy(config, mockSSH as any)).toThrow(ShuttleError)
	})
})

describe('resolveRegistry', () => {
	const mockSSH = createMockSSH()
	const mockDocker = createMockDocker()

	it('returns LocalTransferRegistry when no driver is set', () => {
		const config = createConfig()
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(LocalTransferRegistry)
	})

	it('returns LocalTransferRegistry when driver is "local-transfer"', () => {
		const config = createConfig({ registry: { driver: 'local-transfer' } as any })
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(LocalTransferRegistry)
	})

	it('returns GHCRRegistry when driver is "ghcr"', () => {
		const config = createConfig({ registry: { driver: 'ghcr' } as any })
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(GHCRRegistry)
	})

	it('returns ImageRefRegistry when image is set and driver is absent', () => {
		const config = createConfig({ image: 'ghcr.io/acme/myapp:latest' })
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(ImageRefRegistry)
	})

	it('does NOT return ImageRefRegistry when image is set but driver is also set', () => {
		const config = createConfig({
			image: 'ghcr.io/acme/myapp:latest',
			registry: { driver: 'ghcr' } as any,
		})
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(GHCRRegistry)
		expect(result).not.toBeInstanceOf(ImageRefRegistry)
	})

	it('returns GHCRRegistry for driver "custom"', () => {
		const config = createConfig({ registry: { driver: 'custom' } as any })
		const result = resolveRegistry(config, mockDocker as any, mockSSH as any)
		expect(result).toBeInstanceOf(GHCRRegistry)
	})
})

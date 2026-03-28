import type { ShuttleConfig } from '../../config/schema.ts'
import type { ProxyProvider } from '../types.ts'

// ---------------------------------------------------------------------------
// NoopProxyProvider
// ---------------------------------------------------------------------------

/**
 * No-op proxy provider for driver: 'none'.
 * All methods resolve immediately without performing any action.
 */
export class NoopProxyProvider implements ProxyProvider {
	async apply(
		_host: string,
		_config: ShuttleConfig,
		_upstreams: Map<string, string>,
	): Promise<void> {}

	async switchUpstream(
		_host: string,
		_config: ShuttleConfig,
		_domains: string[],
		_newPort: number,
	): Promise<void> {}

	async removeDomains(_host: string, _domains: string[]): Promise<void> {}

	async getStatus(_host: string): Promise<string> {
		return 'none'
	}
}

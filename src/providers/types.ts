import type { ShuttleConfig } from '../config/schema.ts'
import type { SSHManager } from '../core/ssh-manager.ts'

// ---------------------------------------------------------------------------
// Proxy Provider
// ---------------------------------------------------------------------------

export interface ProxyProvider {
	/** Generate and apply full config for initial setup or full refresh. */
	apply(host: string, config: ShuttleConfig, upstreams: Map<string, string>): Promise<void>

	/**
	 * Hot-switch upstream(s) during blue-green or rollback.
	 * Accepts all domains (not just the first) and preserves headers/ssl.
	 */
	switchUpstream(
		host: string,
		config: ShuttleConfig,
		domains: string[],
		newPort: number,
	): Promise<void>

	/** Remove all proxy config for the given domains. */
	removeDomains(host: string, domains: string[]): Promise<void>

	/** Return proxy container/process status string. */
	getStatus(host: string): Promise<string>
}

// ---------------------------------------------------------------------------
// Registry Provider
// ---------------------------------------------------------------------------

export interface RegistryProvider {
	/**
	 * Resolve the final image reference for deployment.
	 * For local-transfer: returns `shuttle/<app>:deploy-<date>-<sha>`.
	 * For GHCR/DockerHub: returns the full registry URL.
	 */
	resolve(config: ShuttleConfig, tag: string): Promise<string>

	/**
	 * Make the image available on the remote host.
	 * For local-transfer: build + docker save | ssh docker load.
	 * For registries: docker pull on remote.
	 */
	distribute(host: string, imageRef: string, config: ShuttleConfig): Promise<void>
}

// ---------------------------------------------------------------------------
// Secrets Provider
// ---------------------------------------------------------------------------

export interface SecretsProvider {
	set(key: string, value: string): Promise<void>
	get(key: string): Promise<string | undefined>
	list(): Promise<string[]>
	remove(key: string): Promise<void>
	loadAll(): Promise<Record<string, string>>

	/**
	 * Push secrets as .env file to the remote host.
	 * Uses the provided SSH manager instance for the upload.
	 */
	push(host: string, app: string, ssh: SSHManager): Promise<void>
}

// ---------------------------------------------------------------------------
// Notifications Provider
// ---------------------------------------------------------------------------

export type ShuttleEvent =
	| 'deploy_succeeded'
	| 'deploy_failed'
	| 'rollback_succeeded'
	| 'rollback_failed'

export interface NotificationsProvider {
	notify(
		config: ShuttleConfig,
		event: ShuttleEvent,
		payload: Record<string, unknown>,
	): Promise<void>
}

// ---------------------------------------------------------------------------
// Deploy Strategy
// ---------------------------------------------------------------------------

export interface DeployContext {
	config: ShuttleConfig
	host: string
	service: string
	serviceIndex: number
	tag: string
	options: DeployOptions
}

export interface DeployOptions {
	skipBuild?: boolean
	dryRun?: boolean
}

export interface DeployStrategy {
	execute(ctx: DeployContext): Promise<void>
}

// ---------------------------------------------------------------------------
// Tunnel Provider
// ---------------------------------------------------------------------------

export interface TunnelProvider {
	/** Start or ensure the tunnel container is running on the remote host. */
	start(host: string, config: ShuttleConfig): Promise<void>
	/** Stop and remove the tunnel container from the remote host. */
	stop(host: string, config: ShuttleConfig): Promise<void>
	/** Return tunnel container status string. */
	getStatus(host: string): Promise<string>
}

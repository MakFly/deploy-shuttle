import { z } from 'zod'

// ---------------------------------------------------------------------------
// Primitives
// ---------------------------------------------------------------------------

const domainSchema = z.union([z.string().min(1), z.array(z.string().min(1)).min(1)])

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

export const buildSchema = z.object({
	dockerfile: z.string().optional(),
	context: z.string().optional(),
	target: z.string().optional(),
	platform: z.string().min(1).optional(),
	args: z.record(z.string()).optional(),
})

export type BuildConfig = z.infer<typeof buildSchema>

// ---------------------------------------------------------------------------
// Deploy
// ---------------------------------------------------------------------------

export const hooksSchema = z.object({
	pre_deploy: z.array(z.string()).optional(),
	post_deploy: z.array(z.string()).optional(),
})

export type HooksConfig = z.infer<typeof hooksSchema>

export const blueGreenSchema = z.object({
	drain_timeout: z.number().int().positive().optional(),
	readiness_delay: z.number().int().nonnegative().optional(),
})

export type BlueGreenConfig = z.infer<typeof blueGreenSchema>

export const deploySchema = z.object({
	strategy: z.enum(['blue-green', 'rolling']).optional(),
	blue_green: blueGreenSchema.optional(),
	timeout: z.number().int().positive().optional(),
	retain: z.number().int().positive().optional(),
	auto_rollback: z.boolean().optional(),
	hooks: hooksSchema.optional(),
	concurrency: z.number().int().positive().optional(),
})

export type DeployConfig = z.infer<typeof deploySchema>

// ---------------------------------------------------------------------------
// Servers
// ---------------------------------------------------------------------------

/**
 * Normalised server group entry (always an array of hosts).
 */
export const serverGroupSchema = z.object({
	hosts: z.array(z.string().min(1)).min(1),
	user: z.string().min(1),
})

export type ServerGroup = z.infer<typeof serverGroupSchema>

/**
 * Shorthand single-server block (top-level `server:`).
 */
export const serverShorthandSchema = z.object({
	host: z.string().min(1),
	user: z.string().min(1),
})

export type ServerShorthand = z.infer<typeof serverShorthandSchema>

// ---------------------------------------------------------------------------
// Services
// ---------------------------------------------------------------------------

export const healthcheckSchema = z.discriminatedUnion('type', [
	z.object({
		type: z.literal('http'),
		path: z.string().optional(),
		interval: z.number().int().positive().optional(),
		timeout: z.number().int().positive().optional(),
		retries: z.number().int().positive().optional(),
	}),
	z.object({
		type: z.literal('tcp'),
		interval: z.number().int().positive().optional(),
		timeout: z.number().int().positive().optional(),
		retries: z.number().int().positive().optional(),
	}),
	z.object({
		type: z.literal('exec'),
		command: z.string().min(1),
		interval: z.number().int().positive().optional(),
		timeout: z.number().int().positive().optional(),
		retries: z.number().int().positive().optional(),
	}),
])

export type HealthcheckConfig = z.infer<typeof healthcheckSchema>

export const serviceSchema = z.object({
	port: z.number().int().positive().optional(),
	command: z.string().min(1),
	replicas: z.number().int().positive().optional(),
	healthcheck: healthcheckSchema.optional(),
})

export type ServiceConfig = z.infer<typeof serviceSchema>

// ---------------------------------------------------------------------------
// Accessories
// ---------------------------------------------------------------------------

export const accessorySchema = z.object({
	image: z.string().min(1),
	port: z.union([z.string(), z.number().int().positive()]).optional(),
	volumes: z.array(z.string()).optional(),
	env: z.record(z.string()).optional(),
})

export type AccessoryConfig = z.infer<typeof accessorySchema>

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

export const registrySchema = z.object({
	driver: z.enum(['local-transfer', 'docker-hub', 'ghcr', 'custom']).optional(),
	url: z.string().optional(),
	username: z.string().optional(),
	password_env: z.string().optional(),
})

export type RegistryConfig = z.infer<typeof registrySchema>

// ---------------------------------------------------------------------------
// Tunnel
// ---------------------------------------------------------------------------

export const tunnelSchema = z.object({
	driver: z.enum(['cloudflare', 'none']).optional(),
	token_env: z.string().optional(),
})

export type TunnelConfig = z.infer<typeof tunnelSchema>

// ---------------------------------------------------------------------------
// Secrets
// ---------------------------------------------------------------------------

export const secretsSchema = z.object({
	file: z.string().min(1),
	driver: z.enum(['aes', 'doppler', 'vault']).optional(),
})

export type SecretsConfig = z.infer<typeof secretsSchema>

// ---------------------------------------------------------------------------
// Env
// ---------------------------------------------------------------------------

export const envSchema = z.object({
	clear: z.record(z.string()).optional(),
	secret: z.array(z.string()).optional(),
})

export type EnvConfig = z.infer<typeof envSchema>

// ---------------------------------------------------------------------------
// Proxy
// ---------------------------------------------------------------------------

export const proxySslSchema = z.object({
	provider: z.literal('letsencrypt').optional(),
	email: z.string().email().optional(),
})

export type ProxySslConfig = z.infer<typeof proxySslSchema>

export const proxySchema = z.object({
	driver: z.enum(['caddy', 'traefik', 'none']).optional(),
	ssl: proxySslSchema.optional(),
	headers: z.record(z.string()).optional(),
})

export type ProxyConfig = z.infer<typeof proxySchema>

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

export const notificationsSchema = z.object({
	webhooks: z.array(z.string().url()).min(1).optional(),
})

export type NotificationsConfig = z.infer<typeof notificationsSchema>

// ---------------------------------------------------------------------------
// Raw input shape (before normalisation)
// ---------------------------------------------------------------------------

const rawConfigSchema = z
	.object({
		app: z
			.string()
			.min(1)
			.regex(/^[a-z][a-z0-9-]*$/, 'App name must match /^[a-z][a-z0-9-]*$/'),
		domain: domainSchema,
		// shorthand single-server form
		server: serverShorthandSchema.optional(),
		// multi-server groups
		servers: z.record(serverGroupSchema).optional(),
		// image source: build from Dockerfile OR pull pre-built image (mutually exclusive)
		build: buildSchema.optional(),
		image: z.string().min(1).optional(),
		// registry for image distribution
		registry: registrySchema.optional(),
		deploy: deploySchema.optional(),
		services: z.record(serviceSchema).optional(),
		accessories: z.record(accessorySchema).optional(),
		secrets: secretsSchema.optional(),
		env: envSchema.optional(),
		proxy: proxySchema.optional(),
		tunnel: tunnelSchema.optional(),
		notifications: notificationsSchema.optional(),
	})
	.refine((data) => data.server !== undefined || data.servers !== undefined, {
		message: 'Either "server" (shorthand) or "servers" must be provided.',
	})
	.refine((data) => !(data.build !== undefined && data.image !== undefined), {
		message: '"build" and "image" are mutually exclusive. Use one or the other.',
	})
	.transform((data) => {
		// Normalise `server` shorthand into `servers` map
		let servers: Record<string, ServerGroup>

		if (data.servers) {
			servers = data.servers
		} else {
			// data.server is guaranteed by the refine above
			const s = data.server as ServerShorthand
			servers = { default: { hosts: [s.host], user: s.user } }
		}

		const { server: _server, servers: _servers, ...rest } = data
		return { ...rest, servers }
	})

/**
 * Full validated and normalised Shuttle config schema.
 *
 * Accepts both the minimal shorthand form (top-level `server:`) and the full
 * `servers:` map form.  The returned value always contains a `servers` map —
 * the `server` shorthand is removed after normalisation.
 */
export const shuttleConfigSchema = rawConfigSchema

export type ShuttleConfig = z.infer<typeof shuttleConfigSchema>

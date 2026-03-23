import { z } from 'zod'
import {
	accessorySchema,
	buildSchema,
	deploySchema,
	envSchema,
	notificationsSchema,
	proxySchema,
	registrySchema,
	secretsSchema,
	serverGroupSchema,
	serverShorthandSchema,
	serviceSchema,
	tunnelSchema,
} from './schema.ts'

/**
 * Partial schema for environment overlay files (shuttle.<env>.yml).
 *
 * All fields are optional — overlay files only specify what they override.
 * No server shorthand normalisation or mutual exclusivity checks are applied
 * here; those are enforced after the merge with the base config.
 */
export const partialConfigSchema = z.object({
	app: z
		.string()
		.min(1)
		.regex(/^[a-z][a-z0-9-]*$/)
		.optional(),
	domain: z.union([z.string().min(1), z.array(z.string().min(1)).min(1)]).optional(),
	server: serverShorthandSchema.optional(),
	servers: z.record(serverGroupSchema).optional(),
	build: buildSchema.optional(),
	image: z.string().min(1).optional(),
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

export type PartialConfig = z.infer<typeof partialConfigSchema>

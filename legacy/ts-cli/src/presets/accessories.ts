import type { AccessoryConfig } from '@/config/schema.ts'

export const PRESET_NAMES = [
	'postgres',
	'mysql',
	'mariadb',
	'redis',
	'mailpit',
	'meilisearch',
] as const

export type AccessoryPresetName = (typeof PRESET_NAMES)[number]

export const ACCESSORY_PRESETS: Record<AccessoryPresetName, AccessoryConfig> = {
	postgres: {
		image: 'postgres:17',
		port: 5432,
		volumes: ['/data/postgres:/var/lib/postgresql/data'],
		env: { POSTGRES_PASSWORD: 'shuttle', POSTGRES_DB: 'app' },
	},
	mysql: {
		image: 'mysql:8',
		port: 3306,
		volumes: ['/data/mysql:/var/lib/mysql'],
		env: { MYSQL_ROOT_PASSWORD: 'shuttle', MYSQL_DATABASE: 'app' },
	},
	mariadb: {
		image: 'mariadb:11',
		port: 3306,
		volumes: ['/data/mariadb:/var/lib/mysql'],
		env: { MARIADB_ROOT_PASSWORD: 'shuttle', MARIADB_DATABASE: 'app' },
	},
	redis: {
		image: 'redis:7-alpine',
		port: 6379,
		volumes: ['/data/redis:/data'],
	},
	mailpit: {
		image: 'axllent/mailpit',
		port: '1025:1025',
		env: { MP_SMTP_AUTH_ACCEPT_ANY: '1' },
	},
	meilisearch: {
		image: 'getmeili/meilisearch:v1',
		port: 7700,
		volumes: ['/data/meilisearch:/meili_data'],
		env: { MEILI_ENV: 'production' },
	},
}

/**
 * Resolve preset-based accessories into full AccessoryConfig objects.
 * User-supplied fields (env, volumes, port) override preset defaults.
 */
export function resolveAccessoryPreset(
	preset: AccessoryPresetName,
	overrides?: Partial<AccessoryConfig>,
): AccessoryConfig {
	const base = ACCESSORY_PRESETS[preset]
	return {
		...base,
		...overrides,
		env: { ...base.env, ...overrides?.env },
		volumes: overrides?.volumes ?? base.volumes,
	}
}

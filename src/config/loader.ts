import path from 'node:path'
import { parse } from 'yaml'
import type { z } from 'zod'
import { resolveAccessoryPreset } from '../presets/accessories.ts'
import type { AccessoryPresetName } from '../presets/accessories.ts'
import { ConfigError } from '../utils/errors.ts'
import { assertSafeName } from '../utils/shell.ts'
import { defaults } from './defaults.ts'
import { deepMerge } from './merge.ts'
import { partialConfigSchema } from './partial-schema.ts'
import { shuttleConfigSchema } from './schema.ts'
import type { HealthcheckConfig, ServiceConfig, ShuttleConfig } from './schema.ts'

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const CONFIG_FILENAME = 'shuttle.yml'

/**
 * Walk from `cwd` up to the filesystem root and return the first directory
 * that contains a `shuttle.yml` file.  Returns `null` if none is found.
 */
export async function findConfigFile(cwd: string = process.cwd()): Promise<string | null> {
	let dir = path.resolve(cwd)

	while (true) {
		const candidate = path.join(dir, CONFIG_FILENAME)
		const file = Bun.file(candidate)
		const exists = await file.exists()
		if (exists) return candidate

		const parent = path.dirname(dir)
		if (parent === dir) break
		dir = parent
	}

	return null
}

/**
 * Merge service-level healthcheck with defaults, preserving any user-supplied
 * fields.
 */
function applyHealthcheckDefaults(hc: HealthcheckConfig): HealthcheckConfig {
	return {
		interval: defaults.healthcheck.interval,
		timeout: defaults.healthcheck.timeout,
		retries: defaults.healthcheck.retries,
		...(hc.type === 'http' ? { path: defaults.healthcheck.path } : {}),
		...hc,
	} as HealthcheckConfig
}

/**
 * Resolve accessory presets into full config with image, port, volumes, env.
 */
function resolvePresets(config: ShuttleConfig): ShuttleConfig {
	if (!config.accessories) return config

	const resolved = Object.fromEntries(
		Object.entries(config.accessories).map(([name, acc]) => {
			if (acc.preset) {
				const { preset, ...overrides } = acc
				return [name, resolveAccessoryPreset(preset as AccessoryPresetName, overrides)]
			}
			return [name, acc]
		}),
	)

	return { ...config, accessories: resolved }
}

/**
 * Deeply merge config with sensible defaults.  User-supplied values always
 * win; defaults only fill in missing keys.
 */
function applyDefaults(config: ShuttleConfig): ShuttleConfig {
	return {
		...config,
		build: config.build ? { ...defaults.build, ...config.build } : undefined,
		deploy: {
			...defaults.deploy,
			...config.deploy,
			blue_green: {
				...defaults.deploy.blue_green,
				...config.deploy?.blue_green,
			},
			hooks: {
				...defaults.deploy.hooks,
				...config.deploy?.hooks,
			},
		},
		services: config.services
			? Object.fromEntries(
					Object.entries(config.services).map(([name, svc]) => {
						const merged: ServiceConfig = {
							...svc,
							healthcheck: svc.healthcheck
								? applyHealthcheckDefaults(svc.healthcheck)
								: applyHealthcheckDefaults({ type: 'http' }),
						}
						return [name, merged]
					}),
				)
			: undefined,
		registry: config.registry ? { ...defaults.registry, ...config.registry } : undefined,
		secrets: config.secrets ? { ...defaults.secrets, ...config.secrets } : undefined,
		proxy: config.proxy
			? {
					...defaults.proxy,
					...config.proxy,
					ssl: {
						...defaults.proxy.ssl,
						...config.proxy.ssl,
					},
				}
			: undefined,
		notifications: config.notifications
			? {
					...defaults.notifications,
					...config.notifications,
				}
			: undefined,
	}
}

/**
 * Format a ZodError into a readable multiline message.
 */
function formatZodError(err: z.ZodError): string {
	const lines = err.errors.map((issue) => {
		const location = issue.path.length > 0 ? issue.path.join('.') : '<root>'
		return `  • ${location}: ${issue.message}`
	})
	return `Invalid shuttle.yml configuration:\n${lines.join('\n')}`
}

/**
 * Parse a YAML string into an object, throwing ConfigError on failure.
 */
function parseYaml(content: string, filename: string): Record<string, unknown> {
	let parsed: unknown
	try {
		parsed = parse(content)
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err)
		throw new ConfigError(`Failed to parse ${filename} as YAML: ${message}`)
	}

	if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
		throw new ConfigError(`${filename} must contain a YAML mapping (object) at the root level.`)
	}

	return parsed as Record<string, unknown>
}

/**
 * Validate names (app, services, accessories) after Zod parse.
 */
function validateNames(data: ShuttleConfig): void {
	assertSafeName(data.app, 'app name')
	if (data.services) {
		for (const name of Object.keys(data.services)) {
			assertSafeName(name, 'service name')
		}
	}
	if (data.accessories) {
		for (const name of Object.keys(data.accessories)) {
			assertSafeName(name, 'accessory name')
		}
	}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Load, parse, validate, and normalise a Shuttle configuration file.
 *
 * Supports multi-environment overlays: when `env` is provided, loads
 * `shuttle.<env>.yml` alongside the base `shuttle.yml` and deep-merges them.
 *
 * @param filePath  Explicit path to the config file.  When omitted, the loader
 *                  walks up from the current working directory.
 * @param env       Environment name (e.g. "lab", "production"). Loads overlay
 *                  file `shuttle.<env>.yml` from the same directory. Falls back
 *                  to `SHUTTLE_ENV` env var when omitted.
 * @returns         Validated and default-merged {@link ShuttleConfig}.
 */
export async function loadConfig(filePath?: string, env?: string): Promise<ShuttleConfig> {
	// 1. Resolve the config file path
	let resolvedPath: string

	if (filePath) {
		resolvedPath = path.resolve(filePath)
	} else {
		const found = await findConfigFile()
		if (!found) {
			throw new ConfigError(
				`No ${CONFIG_FILENAME} found. Searched from "${process.cwd()}" up to the filesystem root.`,
			)
		}
		resolvedPath = found
	}

	// 2. Read and parse the base config
	const baseFile = Bun.file(resolvedPath)
	if (!(await baseFile.exists())) {
		throw new ConfigError(`Configuration file not found: ${resolvedPath}`)
	}

	let parsed = parseYaml(await baseFile.text(), CONFIG_FILENAME)

	// 3. Load environment overlay if specified
	const resolvedEnv = env ?? process.env.SHUTTLE_ENV
	if (resolvedEnv) {
		const envFilename = `shuttle.${resolvedEnv}.yml`
		const envPath = path.join(path.dirname(resolvedPath), envFilename)
		const envFile = Bun.file(envPath)

		if (await envFile.exists()) {
			const envParsed = parseYaml(await envFile.text(), envFilename)

			// Validate overlay with partial schema
			const envResult = partialConfigSchema.safeParse(envParsed)
			if (!envResult.success) {
				throw new ConfigError(`Invalid ${envFilename}:\n${formatZodError(envResult.error)}`)
			}

			// Deep merge: overlay wins
			parsed = deepMerge(parsed, envParsed)
		}
	}

	// 4. Validate and normalise with Zod
	const result = shuttleConfigSchema.safeParse(parsed)
	if (!result.success) {
		throw new ConfigError(formatZodError(result.error))
	}

	// 5. Validate names
	validateNames(result.data)

	// 6. Resolve accessory presets
	const resolved = resolvePresets(result.data)

	// 7. Apply defaults
	return applyDefaults(resolved)
}

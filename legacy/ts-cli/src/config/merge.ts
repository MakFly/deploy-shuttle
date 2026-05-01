/**
 * Deep merge utility for shuttle config overlays.
 *
 * Rules:
 * - Objects: recursive merge (overlay wins on conflicts)
 * - Arrays: overlay replaces base entirely (same as docker-compose)
 * - Scalars: overlay wins
 * - undefined in overlay: keeps base value
 */
export function deepMerge<T extends Record<string, unknown>>(base: T, overlay: Partial<T>): T {
	const result = { ...base }

	for (const key of Object.keys(overlay) as Array<keyof T>) {
		const baseVal = base[key]
		const overVal = overlay[key]

		if (overVal === undefined) {
			continue
		}

		if (
			typeof baseVal === 'object' &&
			baseVal !== null &&
			!Array.isArray(baseVal) &&
			typeof overVal === 'object' &&
			overVal !== null &&
			!Array.isArray(overVal)
		) {
			result[key] = deepMerge(
				baseVal as Record<string, unknown>,
				overVal as Record<string, unknown>,
			) as T[keyof T]
		} else {
			result[key] = overVal as T[keyof T]
		}
	}

	return result
}

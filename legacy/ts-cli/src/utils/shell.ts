/**
 * POSIX shell escaping utilities for safe SSH command construction.
 */

/**
 * Wraps a value in POSIX single quotes, escaping any embedded single quotes.
 * `'value'` with `'` replaced by `'\''`
 */
export function shellEscape(value: string): string {
	return `'${value.replace(/'/g, "'\\''")}'`
}

/**
 * Formats a key=value pair for .env files with proper double-quote escaping.
 * Escapes `"`, `$`, `` ` ``, `\`, and newlines inside the value.
 */
export function formatEnvLine(key: string, value: string): string {
	const escaped = value
		.replace(/\\/g, '\\\\')
		.replace(/"/g, '\\"')
		.replace(/\$/g, '\\$')
		.replace(/`/g, '\\`')
		.replace(/\n/g, '\\n')
	return `${key}="${escaped}"`
}

/**
 * Validates that a name contains only lowercase alphanumeric characters,
 * hyphens, and underscores, starting with a letter.
 * Throws if the name is invalid.
 */
export function assertSafeName(value: string, label: string): void {
	if (!/^[a-z][a-z0-9_-]*$/.test(value)) {
		throw new Error(`Invalid ${label}: "${value}". Must match /^[a-z][a-z0-9_-]*$/.`)
	}
}

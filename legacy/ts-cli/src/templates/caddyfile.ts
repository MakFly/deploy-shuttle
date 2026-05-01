export interface CaddyApp {
	domains: string[]
	upstream: string
	headers?: Record<string, string>
	ssl?: { email: string }
}

/**
 * Generates a Caddyfile from an array of app configurations.
 * Includes global TLS email when any app defines ssl.email,
 * and per-app reverse_proxy + optional custom headers blocks.
 */
export function generateCaddyfile(apps: CaddyApp[]): string {
	const sections: string[] = []

	// Collect the first ssl email found across all apps for the global block
	const globalEmail = apps.find((a) => a.ssl?.email)?.ssl?.email

	if (globalEmail !== undefined) {
		sections.push(`{\n  email ${globalEmail}\n}`)
	}

	for (const app of apps) {
		const domainLine = app.domains.join(', ')
		const lines: string[] = [`${domainLine} {`]

		lines.push(`  reverse_proxy ${app.upstream}`)

		if (app.headers !== undefined && Object.keys(app.headers).length > 0) {
			lines.push('  header {')
			for (const [name, value] of Object.entries(app.headers)) {
				lines.push(`    ${name} "${value}"`)
			}
			lines.push('  }')
		}

		lines.push('}')
		sections.push(lines.join('\n'))
	}

	return `${sections.join('\n\n')}\n`
}

/**
 * Generates a full Caddyfile snippet that switches the upstream for a set of
 * domains. Useful during blue/green deployments to point traffic at the new slot.
 * Includes an optional global TLS email block and optional custom headers block.
 */
export function generateUpstreamSwitch(
	domains: string[],
	newUpstream: string,
	headers?: Record<string, string>,
	ssl?: { email: string },
): string {
	const sections: string[] = []

	if (ssl !== undefined) {
		sections.push(`{\n  email ${ssl.email}\n}`)
	}

	const domainLine = domains.join(', ')
	const lines: string[] = [`${domainLine} {`]

	lines.push(`  reverse_proxy ${newUpstream}`)

	if (headers !== undefined && Object.keys(headers).length > 0) {
		lines.push('  header {')
		for (const [name, value] of Object.entries(headers)) {
			lines.push(`    ${name} "${value}"`)
		}
		lines.push('  }')
	}

	lines.push('}')
	sections.push(lines.join('\n'))

	return `${sections.join('\n\n')}\n`
}

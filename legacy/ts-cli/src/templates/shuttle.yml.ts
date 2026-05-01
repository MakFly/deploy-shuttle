export interface ShuttleYmlOptions {
	app: string
	domain: string
	host: string
	user: string
}

/**
 * Generates a minimal shuttle.yml configuration file content as a YAML string.
 * Includes comments explaining each section and optional commented-out advanced settings.
 */
export function generateShuttleYml(options: ShuttleYmlOptions): string {
	const { app, domain, host, user } = options

	return `# Shuttle deployment configuration
app: ${app}
domain: ${domain}

server:
  host: ${host}
  user: ${user}

# Uncomment to customize build settings
# build:
#   dockerfile: Dockerfile
#   context: .

# Uncomment to customize deploy settings
# deploy:
#   strategy: blue-green
#   timeout: 120
`
}

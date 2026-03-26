export interface DevComposeService {
	name: string
	build?: { context: string; dockerfile: string }
	image?: string
	command?: string
	ports?: string[]
	volumes?: string[]
	environment?: Record<string, string>
	depends_on?: string[]
}

export interface DevComposeOptions {
	services: DevComposeService[]
	volumes?: string[]
}

export function generateDevCompose(options: DevComposeOptions): string {
	const { services, volumes } = options
	let yml = 'services:\n'

	for (const svc of services) {
		yml += `  ${svc.name}:\n`

		if (svc.build) {
			yml += '    build:\n'
			yml += `      context: ${svc.build.context}\n`
			yml += `      dockerfile: ${svc.build.dockerfile}\n`
		}

		if (svc.image) {
			yml += `    image: ${svc.image}\n`
		}

		if (svc.command) {
			yml += `    command: ${svc.command}\n`
		}

		if (svc.ports && svc.ports.length > 0) {
			yml += '    ports:\n'
			for (const port of svc.ports) {
				yml += `      - "${port}"\n`
			}
		}

		if (svc.volumes && svc.volumes.length > 0) {
			yml += '    volumes:\n'
			for (const vol of svc.volumes) {
				yml += `      - ${vol}\n`
			}
		}

		if (svc.environment && Object.keys(svc.environment).length > 0) {
			yml += '    environment:\n'
			for (const [key, value] of Object.entries(svc.environment)) {
				yml += `      ${key}: "${value}"\n`
			}
		}

		if (svc.depends_on && svc.depends_on.length > 0) {
			yml += '    depends_on:\n'
			for (const dep of svc.depends_on) {
				yml += `      - ${dep}\n`
			}
		}

		yml += '    restart: unless-stopped\n'
		yml += '\n'
	}

	if (volumes && volumes.length > 0) {
		yml += 'volumes:\n'
		for (const vol of volumes) {
			yml += `  ${vol}:\n`
		}
	}

	return yml
}

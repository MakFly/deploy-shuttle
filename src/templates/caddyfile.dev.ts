export interface DevCaddyService {
	name: string
	domain: string
	port: number
}

export interface DevCaddyOptions {
	certPath: string
	keyPath: string
	services: DevCaddyService[]
}

export function generateDevCaddyfile(options: DevCaddyOptions): string {
	const { certPath, keyPath, services } = options

	const blocks = services.map(
		(svc) => `${svc.domain} {
	tls ${certPath} ${keyPath}
	reverse_proxy ${svc.name}:${svc.port}
}`,
	)

	return `${blocks.join('\n\n')}\n`
}

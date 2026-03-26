import path from 'node:path'
import { loadConfig } from '@/config/loader.ts'
import type { ShuttleConfig } from '@/config/schema.ts'
import { generateCerts, isMkcertInstalled } from '@/providers/ssl/mkcert.ts'
import { type DevCaddyService, generateDevCaddyfile } from '@/templates/caddyfile.dev.ts'
import {
	type DevComposeOptions,
	type DevComposeService,
	generateDevCompose,
} from '@/templates/docker-compose.dev.ts'
import { logger } from '@/utils/logger.ts'

export class DevManager {
	private readonly cwd: string

	constructor(cwd: string = process.cwd()) {
		this.cwd = cwd
	}

	async up(): Promise<void> {
		const config = await loadConfig()
		const shuttleDir = path.join(this.cwd, '.shuttle')

		// Step 1: SSL setup
		const ssl = await this.setupSSL(shuttleDir, config.domain)

		// Step 2: Generate Caddyfile
		const caddyServices = this.buildCaddyServices(config)
		const caddyfile = generateDevCaddyfile({
			certPath: '/certs/cert.pem',
			keyPath: '/certs/key.pem',
			services: caddyServices,
		})
		await Bun.write(path.join(shuttleDir, 'Caddyfile.dev'), caddyfile)

		// Step 3: Generate docker-compose.dev.yml
		const compose = this.buildComposeConfig(config, ssl)
		const composeContent = generateDevCompose(compose)
		const composePath = path.join(this.cwd, 'docker-compose.dev.yml')
		await Bun.write(composePath, composeContent)

		// Step 4: Run docker compose
		logger.start('Starting development environment...')
		const result = Bun.spawnSync(
			['docker', 'compose', '-f', 'docker-compose.dev.yml', 'up', '-d', '--build'],
			{ cwd: this.cwd, stdout: 'inherit', stderr: 'inherit' },
		)

		if (result.exitCode !== 0) {
			throw new Error('Failed to start development environment')
		}

		// Step 5: Print URLs
		logger.success('Development environment started!')
		logger.info('')
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		for (const domain of domains) {
			logger.info(`  https://${domain}`)
		}
		logger.info('  https://localhost')
		if (config.accessories) {
			for (const [_name, acc] of Object.entries(config.accessories)) {
				const preset = acc.preset
				if (preset === 'mailpit') {
					logger.info('  http://localhost:8025  (Mailpit UI)')
				}
			}
		}
	}

	async down(): Promise<void> {
		logger.start('Stopping development environment...')
		const result = Bun.spawnSync(['docker', 'compose', '-f', 'docker-compose.dev.yml', 'down'], {
			cwd: this.cwd,
			stdout: 'inherit',
			stderr: 'inherit',
		})

		if (result.exitCode !== 0) {
			throw new Error('Failed to stop development environment')
		}

		logger.success('Development environment stopped.')
	}

	async restart(): Promise<void> {
		await this.down()
		await this.up()
	}

	async logs(follow = true): Promise<void> {
		const args = ['docker', 'compose', '-f', 'docker-compose.dev.yml', 'logs']
		if (follow) args.push('-f')

		const proc = Bun.spawn(args, {
			cwd: this.cwd,
			stdout: 'inherit',
			stderr: 'inherit',
		})

		await proc.exited
	}

	async status(): Promise<void> {
		const result = Bun.spawnSync(['docker', 'compose', '-f', 'docker-compose.dev.yml', 'ps'], {
			cwd: this.cwd,
			stdout: 'inherit',
			stderr: 'inherit',
		})

		if (result.exitCode !== 0) {
			throw new Error('Failed to get status')
		}
	}

	private async setupSSL(
		shuttleDir: string,
		domain: string | string[],
	): Promise<{ cert: string; key: string }> {
		const certDir = path.join(shuttleDir, 'certs')

		// Check for existing certs
		const certFile = Bun.file(path.join(certDir, 'cert.pem'))
		if (await certFile.exists()) {
			logger.debug('SSL certificates already exist, skipping generation.')
			return { cert: path.join(certDir, 'cert.pem'), key: path.join(certDir, 'key.pem') }
		}

		// Check mkcert
		if (!(await isMkcertInstalled())) {
			logger.warn('mkcert is not installed. Install it for trusted local SSL:')
			logger.info('  brew install mkcert  (macOS)')
			logger.info('  mkcert -install')
			logger.info('')
			logger.info('Continuing without SSL...')
			return { cert: '', key: '' }
		}

		const domains = Array.isArray(domain) ? domain : [domain]
		return generateCerts(certDir, domains)
	}

	private buildCaddyServices(config: ShuttleConfig): DevCaddyService[] {
		const domains = Array.isArray(config.domain) ? config.domain : [config.domain]
		const services: DevCaddyService[] = []

		if (config.services) {
			const entries = Object.entries(config.services)
			const webService = entries.find(([_name, svc]) => svc.port !== undefined)
			if (webService && webService[1].port !== undefined) {
				const port = webService[1].port
				for (const domain of domains) {
					services.push({
						name: webService[0],
						domain,
						port: typeof port === 'string' ? Number.parseInt(port, 10) : port,
					})
				}
			}
		}

		return services
	}

	private buildComposeConfig(
		config: ShuttleConfig,
		ssl: { cert: string; key: string },
	): DevComposeOptions {
		const services: DevComposeService[] = []
		const volumes: string[] = []

		// App services
		if (config.services) {
			for (const [name, svc] of Object.entries(config.services)) {
				const service: DevComposeService = {
					name,
					volumes: ['.:/var/www/html'],
				}

				if (config.build) {
					service.build = {
						context: config.build.context ?? '.',
						dockerfile: config.build.dockerfile ?? 'Dockerfile',
					}
				} else if (config.image) {
					service.image = config.image
				}

				if (svc.command) {
					service.command = svc.command
				}

				if (svc.port) {
					service.ports = [`${svc.port}:${svc.port}`]
				}

				services.push(service)
			}
		}

		// Caddy proxy (only if SSL is available)
		if (ssl.cert) {
			services.push({
				name: 'caddy',
				image: 'caddy:2-alpine',
				ports: ['80:80', '443:443'],
				volumes: ['.shuttle/certs:/certs:ro', '.shuttle/Caddyfile.dev:/etc/caddy/Caddyfile:ro'],
				depends_on: config.services ? Object.keys(config.services).slice(0, 1) : [],
			})
		}

		// Accessories
		if (config.accessories) {
			for (const [name, acc] of Object.entries(config.accessories)) {
				const service: DevComposeService = {
					name,
					image: acc.image,
					environment: acc.env,
				}

				if (acc.port !== undefined) {
					const portStr = String(acc.port)
					if (portStr.includes(':')) {
						service.ports = [portStr]
					} else {
						service.ports = [`${portStr}:${portStr}`]
					}
				}

				if (acc.volumes) {
					// Convert remote paths to named volumes for dev
					service.volumes = acc.volumes.map((v) => {
						if (v.startsWith('/data/')) {
							const volName = `${name}_data`
							if (!volumes.includes(volName)) {
								volumes.push(volName)
							}
							const parts = v.split(':')
							return `${volName}:${parts[1] ?? parts[0]}`
						}
						return v
					})
				}

				services.push(service)
			}
		}

		return { services, volumes }
	}
}

import type { ShuttleConfig } from '../../config/schema.ts'
import type { SSHManager } from '../../core/ssh-manager.ts'
import { logger } from '../../utils/logger.ts'
import { shellEscape } from '../../utils/shell.ts'
import type { ProxyProvider } from '../types.ts'

const TRAEFIK_CONFIG_DIR = '/opt/traefik'
const TRAEFIK_DYNAMIC_DIR = '/opt/traefik/dynamic'
const TRAEFIK_CONTAINER_NAME = 'traefik'

export class TraefikProxyProvider implements ProxyProvider {
	constructor(private readonly ssh: SSHManager) {}

	async apply(host: string, config: ShuttleConfig, upstreams: Map<string, string>): Promise<void> {
		await this.ssh.exec(host, `mkdir -p ${shellEscape(TRAEFIK_DYNAMIC_DIR)}`)
		const staticConfig = this.generateStaticConfig(config)
		await this.ssh.uploadContent(host, staticConfig, `${TRAEFIK_CONFIG_DIR}/traefik.yml`, 0o644)
		const dynamicConfig = this.generateDynamicConfig(config, upstreams)
		await this.ssh.uploadContent(
			host,
			dynamicConfig,
			`${TRAEFIK_DYNAMIC_DIR}/${config.app}.yml`,
			0o644,
		)
		await this.ensureTraefikRunning(host)
	}

	async switchUpstream(
		host: string,
		config: ShuttleConfig,
		domains: string[],
		newPort: number,
	): Promise<void> {
		const upstreams = new Map<string, string>()
		for (const domain of domains) {
			upstreams.set(domain, `http://host.docker.internal:${newPort}`)
		}
		const dynamicConfig = this.generateDynamicConfig(config, upstreams)
		await this.ssh.uploadContent(
			host,
			dynamicConfig,
			`${TRAEFIK_DYNAMIC_DIR}/${config.app}.yml`,
			0o644,
		)
	}

	async removeDomains(host: string, _domains: string[]): Promise<void> {
		await this.ssh.exec(host, `rm -f ${shellEscape(TRAEFIK_DYNAMIC_DIR)}/*.yml`)
	}

	async getStatus(host: string): Promise<string> {
		const { stdout, code } = await this.ssh.exec(
			host,
			`docker inspect ${TRAEFIK_CONTAINER_NAME} --format '{{.State.Status}}' 2>/dev/null`,
		)
		return code === 0 ? stdout.trim() : 'not running'
	}

	private generateStaticConfig(config: ShuttleConfig): string {
		const sslEmail = config.proxy?.ssl?.email
		let yml = `entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"

providers:
  file:
    directory: /dynamic
    watch: true

api:
  dashboard: false
`
		if (sslEmail) {
			yml += `
certificatesResolvers:
  letsencrypt:
    acme:
      email: ${sslEmail}
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
`
		}
		return yml
	}

	private generateDynamicConfig(config: ShuttleConfig, upstreams: Map<string, string>): string {
		const domains = Array.from(upstreams.keys())
		const hostRules = domains.map((d) => `Host(\`${d}\`)`).join(' || ')
		const servers = Array.from(upstreams.values())
			.map((url) => `          - url: "${url}"`)
			.join('\n')
		const hasSsl = config.proxy?.ssl !== undefined

		return `http:
  routers:
    ${config.app}:
      rule: "${hostRules}"
      entryPoints:
        - websecure
      service: "${config.app}"${
				hasSsl
					? `
      tls:
        certResolver: letsencrypt`
					: ''
			}

  services:
    ${config.app}:
      loadBalancer:
        servers:
${servers}
`
	}

	private async ensureTraefikRunning(host: string): Promise<void> {
		const { code } = await this.ssh.exec(
			host,
			`docker inspect ${TRAEFIK_CONTAINER_NAME} --format '{{.State.Status}}' 2>/dev/null`,
		)
		if (code === 0) {
			logger.debug('Traefik container already running')
			return
		}
		logger.info('Starting Traefik container...')
		const { code: runCode, stderr } = await this.ssh.exec(
			host,
			[
				'docker run -d',
				`--name ${TRAEFIK_CONTAINER_NAME}`,
				'--restart always',
				'-p 80:80 -p 443:443',
				`-v ${TRAEFIK_CONFIG_DIR}:/etc/traefik`,
				`-v ${TRAEFIK_DYNAMIC_DIR}:/dynamic`,
				'--add-host host.docker.internal:host-gateway',
				'traefik:v3',
			].join(' '),
		)
		if (runCode !== 0) {
			logger.warn(`Failed to start Traefik: ${stderr.trim()}`)
		} else {
			logger.success('Traefik started')
		}
	}
}

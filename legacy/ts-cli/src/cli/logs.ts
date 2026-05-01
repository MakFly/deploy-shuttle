import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { docker } from '../core/docker-manager.ts'
import { runtime } from '../core/runtime-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'logs',
		description: 'Stream remote container logs',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		follow: {
			type: 'boolean',
			alias: 'f',
			description: 'Follow log output (like docker logs -f)',
			default: false,
		},
		service: {
			type: 'string',
			description: 'Service name to fetch logs from',
		},
		lines: {
			type: 'string',
			description: 'Number of tail lines to show (e.g. 100)',
		},
		env: {
			type: 'string',
			alias: 'e',
			description: 'Environment (loads shuttle.<env>.yml overlay)',
		},
	},
	async run({ args }) {
		try {
			const config = await loadConfig(args.config, args.env)

			const tailLines = args.lines !== undefined ? Number(args.lines) : undefined
			if (tailLines !== undefined && (Number.isNaN(tailLines) || tailLines < 1)) {
				logger.error('--lines must be a positive integer.')
				process.exit(1)
			}

			const service = args.service ?? config.app
			logger.info(
				`Fetching logs for service "${service}"${tailLines !== undefined ? ` (last ${tailLines} lines)` : ''}${args.follow ? ' [following]' : ''}...`,
			)

			// biome-ignore lint/style/noNonNullAssertion: config always has servers
			const firstGroup = Object.values(config.servers)[0]!
			// biome-ignore lint/style/noNonNullAssertion: servers always have at least one host
			const host = firstGroup.hosts[0]!
			const serviceName = args.service ?? Object.keys(config.services ?? {})[0] ?? config.app

			await ssh.connect({ host, user: firstGroup.user })
			try {
				const containerName = await runtime.resolveServiceContainer(host, config, serviceName)
				const stream = await docker.logs(host, containerName, args.follow, tailLines)
				await new Promise<void>((resolve, reject) => {
					const readable = stream as NodeJS.ReadableStream
					let settled = false
					const finish = () => {
						if (!settled) {
							settled = true
							resolve()
						}
					}

					readable.on('data', (chunk: Buffer) => {
						process.stdout.write(chunk)
					})
					readable.on('end', finish)
					readable.on('close', finish)
					readable.on('error', (err: Error) => {
						if (!settled) {
							settled = true
							reject(err)
						}
					})
				})
			} finally {
				ssh.disconnect(host)
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

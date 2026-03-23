import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { deployer } from '../core/deploy-manager.ts'
import { docker } from '../core/docker-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'status',
		description: 'Show running container and proxy status across all servers',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		json: {
			type: 'boolean',
			description: 'Output status as JSON',
			default: false,
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

			logger.info(`Checking status for app "${config.app}"...`)

			const serverStatuses = []

			for (const [name, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					await ssh.connect({ host, user: group.user })

					let deployState: { active_slot: string; active_tag: string; version: number } | null =
						null
					try {
						deployState = await deployer.readState(host, config.app)
					} catch {
						// No deploy state yet
					}

					const images = await docker.listImages(host, `shuttle/${config.app}:*`)

					serverStatuses.push({
						group: name,
						host,
						user: group.user,
						active_slot: deployState?.active_slot ?? null,
						active_tag: deployState?.active_tag ?? null,
						version: deployState?.version ?? null,
						images,
					})

					ssh.disconnect(host)
				}
			}

			const statusResult = { app: config.app, servers: serverStatuses }

			if (args.json) {
				process.stdout.write(`${JSON.stringify(statusResult, null, 2)}\n`)
			} else {
				for (const s of serverStatuses) {
					logger.info(`[${s.group}] ${s.user}@${s.host}`)
					logger.info(`  slot:    ${s.active_slot ?? 'none'}`)
					logger.info(`  tag:     ${s.active_tag ?? 'none'}`)
					logger.info(`  version: ${s.version ?? 'none'}`)
					logger.info(`  images:  ${s.images.length > 0 ? s.images.join(', ') : 'none'}`)
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

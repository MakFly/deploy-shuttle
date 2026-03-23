import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { notifications } from '../core/notifications-manager.ts'
import { rollback } from '../core/rollback-manager.ts'
import { runtime } from '../core/runtime-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'rollback',
		description: 'Rollback the application to a previous version',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		to: {
			type: 'string',
			description: 'Target version or image tag to roll back to',
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

			if (args.to) {
				logger.info(`Rolling back app "${config.app}" to version "${args.to}"...`)
			} else {
				logger.info(`Rolling back app "${config.app}" to the previous version...`)
			}

			for (const [, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					await ssh.connect({ host, user: group.user })
					try {
						await runtime.acquireLock(host, config.app)
						await rollback.rollback(config, host, args.to)
					} finally {
						try {
							await runtime.releaseLock(host, config.app)
						} finally {
							ssh.disconnect(host)
						}
					}
				}
			}

			await notifications.notify(config, 'rollback_succeeded', {
				target: args.to ?? 'previous',
			})
		} catch (err) {
			try {
				const config = await loadConfig(args.config, args.env)
				await notifications.notify(config, 'rollback_failed', {
					target: args.to ?? 'previous',
					error: err instanceof Error ? err.message : String(err),
				})
			} catch {
				// Ignore notification failures on top of the rollback error.
			}
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

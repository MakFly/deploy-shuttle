import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { runtime } from '../core/runtime-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'lock',
		description: 'Manage deployment locks',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		action: {
			type: 'positional',
			description: 'Action: break or status',
			required: true,
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

			for (const [, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					await ssh.connect({ host, user: group.user })
					try {
						if (args.action === 'break') {
							try {
								const lock = await runtime.readLock(host, config.app)
								logger.info(
									`Breaking lock on ${host} (owner pid=${lock.pid}, created=${lock.created_at})`,
								)
							} catch {
								// No lock metadata
							}
							await runtime.forceReleaseLock(host, config.app)
							logger.success(`Lock broken for "${config.app}" on ${host}`)
						} else if (args.action === 'status') {
							try {
								const lock = await runtime.readLock(host, config.app)
								logger.info(`Lock active on ${host}: pid=${lock.pid}, created=${lock.created_at}`)
							} catch {
								logger.info(`No active lock for "${config.app}" on ${host}`)
							}
						} else {
							logger.error(`Unknown action "${args.action}". Use "break" or "status".`)
							process.exit(1)
						}
					} finally {
						ssh.disconnect(host)
					}
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

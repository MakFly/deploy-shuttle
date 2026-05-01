import { defineCommand } from 'citty'
import prompts from 'prompts'
import { loadConfig } from '../config/loader.ts'
import { destroyer } from '../core/destroy-manager.ts'
import { runtime } from '../core/runtime-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'destroy',
		description: 'Remove an app deployment from the remote host(s)',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		yes: {
			type: 'boolean',
			description: 'Skip the confirmation prompt',
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
			if (!args.yes) {
				const { confirmed } = await prompts({
					type: 'confirm',
					name: 'confirmed',
					message: `Destroy deployment "${config.app}" on all configured servers?`,
					initial: false,
				})

				if (!confirmed) {
					logger.warn('Aborted.')
					process.exit(0)
				}
			}

			for (const [, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					await ssh.connect({ host, user: group.user })
					try {
						await runtime.acquireLock(host, config.app)
						await destroyer.destroy(config, host)
					} finally {
						try {
							await runtime.releaseLock(host, config.app)
						} finally {
							ssh.disconnect(host)
						}
					}
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { ssh as sshManager } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'ssh',
		description: 'Open an interactive shell in the remote container',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		server: {
			type: 'string',
			description: 'Target server group name (defaults to first group)',
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

			const serverNames = Object.keys(config.servers)
			const targetGroup = args.server ?? serverNames[0]

			if (!targetGroup || !(targetGroup in config.servers)) {
				logger.error(
					`Server group "${targetGroup}" not found. Available: ${serverNames.join(', ')}`,
				)
				process.exit(1)
			}

			// biome-ignore lint/style/noNonNullAssertion: validated above
			const group = config.servers[targetGroup]!
			// biome-ignore lint/style/noNonNullAssertion: servers always have at least one host
			const host = group.hosts[0]!
			const user = group.user

			logger.info(`Opening SSH shell to ${user}@${host} (group: ${targetGroup})...`)

			await sshManager.connect({ host, user })
			await sshManager.shell(host)
			sshManager.disconnect(host)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

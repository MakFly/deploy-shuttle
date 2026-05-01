import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { docker } from '../core/docker-manager.ts'
import { runtime } from '../core/runtime-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'exec',
		description: 'Execute a command inside the remote container (usage: shuttle exec -- <cmd>)',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		service: {
			type: 'string',
			description: 'Service name to run command in (defaults to app name)',
		},
		env: {
			type: 'string',
			alias: 'e',
			description: 'Environment (loads shuttle.<env>.yml overlay)',
		},
	},
	async run({ args, rawArgs }) {
		try {
			const config = await loadConfig(args.config, args.env)

			// Everything after "--" is captured in rawArgs by citty
			const separatorIndex = rawArgs.indexOf('--')
			const remoteCommand = separatorIndex !== -1 ? rawArgs.slice(separatorIndex + 1) : []

			if (remoteCommand.length === 0) {
				logger.error(
					'No command provided. Usage: shuttle exec -- <command>\nExample: shuttle exec -- rails console',
				)
				process.exit(1)
			}

			const service = args.service ?? Object.keys(config.services ?? {})[0] ?? config.app
			logger.info(`Executing [${remoteCommand.join(' ')}] in service "${service}"...`)

			// biome-ignore lint/style/noNonNullAssertion: config always has servers
			const firstGroup = Object.values(config.servers)[0]!
			// biome-ignore lint/style/noNonNullAssertion: servers always have at least one host
			const host = firstGroup.hosts[0]!
			await ssh.connect({ host, user: firstGroup.user })
			let stdout = ''
			let code = 0
			try {
				const containerName = await runtime.resolveServiceContainer(host, config, service)
				;({ stdout, code } = await docker.exec(host, containerName, remoteCommand.join(' ')))
			} finally {
				ssh.disconnect(host)
			}

			if (stdout.length > 0) {
				process.stdout.write(stdout)
			}
			process.exit(code)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

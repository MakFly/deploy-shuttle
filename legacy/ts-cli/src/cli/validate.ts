import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'validate',
		description: 'Validate shuttle.yml without deploying',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		json: {
			type: 'boolean',
			description: 'Print the resolved config as JSON',
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

			if (args.json) {
				process.stdout.write(`${JSON.stringify(config, null, 2)}\n`)
				return
			}

			logger.success(`Configuration for "${config.app}" is valid.`)
			if (args.env) {
				logger.info(`Environment overlay: shuttle.${args.env}.yml`)
			}
			logger.info(`Servers: ${Object.keys(config.servers).join(', ')}`)
			logger.info(`Services: ${Object.keys(config.services ?? {}).join(', ') || 'none'}`)
			logger.info(`Accessories: ${Object.keys(config.accessories ?? {}).join(', ') || 'none'}`)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

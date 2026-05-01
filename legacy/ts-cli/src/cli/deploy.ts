import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { deployer } from '../core/deploy-manager.ts'
import { notifications } from '../core/notifications-manager.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'deploy',
		description: 'Build and deploy the application to your VPS',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		verbose: {
			type: 'boolean',
			description: 'Enable verbose output',
			default: false,
		},
		'skip-build': {
			type: 'boolean',
			description: 'Skip Docker build and use existing image',
			default: false,
		},
		'dry-run': {
			type: 'boolean',
			description: 'Show the planned deployment steps without mutating anything',
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
			if (args.verbose) {
				logger.setVerbose(true)
			}

			const config = await loadConfig(args.config, args.env)
			logger.info(
				`Deploying app "${config.app}" to domain "${Array.isArray(config.domain) ? config.domain.join(', ') : config.domain}"...`,
			)

			const serverNames = Object.keys(config.servers)
			logger.debug(`Target servers: ${serverNames.join(', ')}`)

			if (args['skip-build']) {
				logger.info('Skipping Docker build (--skip-build)')
			}

			if (args['dry-run']) {
				logger.info('Running deployment in dry-run mode')
			}

			await deployer.deploy(config, {
				skipBuild: args['skip-build'],
				dryRun: args['dry-run'],
			})

			if (!args['dry-run']) {
				await notifications.notify(config, 'deploy_succeeded', {
					skip_build: args['skip-build'],
				})
			}
		} catch (err) {
			try {
				const config = await loadConfig(args.config, args.env)
				await notifications.notify(config, 'deploy_failed', {
					error: err instanceof Error ? err.message : String(err),
				})
			} catch {
				// Ignore secondary notification failures after a primary deploy error.
			}
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

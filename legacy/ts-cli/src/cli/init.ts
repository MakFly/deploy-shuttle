import path from 'node:path'
import { defineCommand } from 'citty'
import prompts from 'prompts'
import { generateShuttleYml } from '../templates/shuttle.yml.ts'
import { logger } from '../utils/logger.ts'

export default defineCommand({
	meta: {
		name: 'init',
		description: 'Interactively generate a shuttle.yml configuration file',
	},
	args: {
		force: {
			type: 'boolean',
			description: 'Overwrite existing shuttle.yml',
			default: false,
		},
		yes: {
			type: 'boolean',
			description: 'Use defaults without prompting',
			default: false,
		},
	},
	async run({ args }) {
		try {
			const configPath = path.join(process.cwd(), 'shuttle.yml')
			const configFile = Bun.file(configPath)
			const exists = await configFile.exists()

			if (exists && !args.force) {
				logger.error('shuttle.yml already exists. Use --force to overwrite.')
				process.exit(1)
			}

			let answers: { app: string; domain: string; host: string; user: string }

			if (args.yes) {
				const dirName = path.basename(process.cwd())
				answers = {
					app: dirName,
					domain: `${dirName}.example.com`,
					host: 'your-server-ip',
					user: 'deploy',
				}
			} else {
				const response = await prompts(
					[
						{
							type: 'text',
							name: 'app',
							message: 'App name:',
							initial: path.basename(process.cwd()),
							validate: (v: string) => v.trim().length > 0 || 'App name is required',
						},
						{
							type: 'text',
							name: 'domain',
							message: 'Domain (e.g. myapp.example.com):',
							validate: (v: string) => v.trim().length > 0 || 'Domain is required',
						},
						{
							type: 'text',
							name: 'host',
							message: 'Server host (IP or hostname):',
							validate: (v: string) => v.trim().length > 0 || 'Server host is required',
						},
						{
							type: 'text',
							name: 'user',
							message: 'Server SSH user:',
							initial: 'deploy',
							validate: (v: string) => v.trim().length > 0 || 'SSH user is required',
						},
					],
					{
						onCancel: () => {
							logger.warn('Initialization cancelled.')
							process.exit(0)
						},
					},
				)

				answers = response as typeof answers
			}

			const yml = generateShuttleYml({
				app: answers.app,
				domain: answers.domain,
				host: answers.host,
				user: answers.user,
			})

			// Ensure .shuttle/ directory exists
			const shuttleDir = path.join(process.cwd(), '.shuttle')
			await Bun.write(path.join(shuttleDir, '.gitkeep'), '')

			// Write shuttle.yml
			await Bun.write(configPath, yml)

			logger.success(`shuttle.yml created at ${configPath}`)
			logger.info(`Run "shuttle provision" to bootstrap your VPS, then "shuttle deploy" to deploy.`)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

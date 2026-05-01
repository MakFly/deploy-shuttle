import { defineCommand } from 'citty'
import prompts from 'prompts'
import { loadConfig } from '../config/loader.ts'
import { secrets } from '../core/secrets-manager.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

const setCommand = defineCommand({
	meta: {
		name: 'set',
		description: 'Set an encrypted secret (prompts for value if not provided)',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		key: {
			type: 'positional',
			description: 'Secret key name',
			required: true,
		},
		value: {
			type: 'positional',
			description: 'Secret value (will prompt if omitted)',
			required: false,
		},
		env: {
			type: 'string',
			alias: 'e',
			description: 'Environment (loads shuttle.<env>.yml overlay)',
		},
	},
	async run({ args }) {
		try {
			await loadConfig(args.config, args.env)

			let secretValue = args.value as string | undefined

			if (!secretValue) {
				const { input } = await prompts(
					{
						type: 'password',
						name: 'input',
						message: `Value for "${args.key}":`,
						validate: (v: string) => v.trim().length > 0 || 'Value cannot be empty',
					},
					{
						onCancel: () => {
							logger.warn('Cancelled.')
							process.exit(0)
						},
					},
				)
				secretValue = input as string
			}

			// biome-ignore lint/style/noNonNullAssertion: validated by prompt above
			await secrets.set(args.key, secretValue!)
			logger.success(`Secret "${args.key}" set.`)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const getCommand = defineCommand({
	meta: {
		name: 'get',
		description: 'Print a decrypted secret value',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		key: {
			type: 'positional',
			description: 'Secret key name',
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
			await loadConfig(args.config, args.env)

			const value = await secrets.get(args.key)
			if (value === undefined) {
				logger.error(`Secret "${args.key}" not found.`)
				process.exit(1)
			}
			process.stdout.write(`${value}\n`)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const listCommand = defineCommand({
	meta: {
		name: 'list',
		description: 'List all secret keys',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		env: {
			type: 'string',
			alias: 'e',
			description: 'Environment (loads shuttle.<env>.yml overlay)',
		},
	},
	async run({ args }) {
		try {
			await loadConfig(args.config, args.env)

			const keys = await secrets.list()
			if (keys.length === 0) {
				logger.info('No secrets stored.')
			} else {
				for (const k of keys) {
					process.stdout.write(`${k}\n`)
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const removeCommand = defineCommand({
	meta: {
		name: 'remove',
		description: 'Remove a secret by key',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		key: {
			type: 'positional',
			description: 'Secret key name',
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
			await loadConfig(args.config, args.env)

			await secrets.remove(args.key)
			logger.success(`Secret "${args.key}" removed.`)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const pushCommand = defineCommand({
	meta: {
		name: 'push',
		description: 'Push all secrets to the remote server',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
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

			logger.info(`Pushing secrets for app "${config.app}"...`)

			for (const [, group] of Object.entries(config.servers)) {
				for (const host of group.hosts) {
					await ssh.connect({ host, user: group.user })
					await secrets.push(host, config.app)
					ssh.disconnect(host)
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

export default defineCommand({
	meta: {
		name: 'secrets',
		description: 'Manage encrypted secrets for your application',
	},
	subCommands: {
		set: setCommand,
		get: getCommand,
		list: listCommand,
		remove: removeCommand,
		push: pushCommand,
	},
})

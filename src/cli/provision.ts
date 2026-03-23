import path from 'node:path'
import { defineCommand } from 'citty'
import prompts from 'prompts'
import { loadConfig } from '../config/loader.ts'
import { provisioner } from '../core/provision-manager.ts'
import { logger } from '../utils/logger.ts'

async function readPublicKey(): Promise<string | null> {
	const home = process.env.HOME ?? ''
	const candidates = [
		path.join(home, '.ssh', 'id_ed25519.pub'),
		path.join(home, '.ssh', 'id_rsa.pub'),
	]

	for (const candidate of candidates) {
		const file = Bun.file(candidate)
		if (await file.exists()) {
			logger.debug(`Using SSH public key: ${candidate}`)
			return file.text()
		}
	}

	return null
}

export default defineCommand({
	meta: {
		name: 'provision',
		description: 'Bootstrap a VPS: install Docker, Caddy, create deploy user',
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
		user: {
			type: 'string',
			description: 'Override the SSH user from config',
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
			const serverNames = Object.keys(config.servers)

			logger.info(`Provisioning ${serverNames.length} server(s) for app "${config.app}"...`)

			const publicKey = await readPublicKey()
			if (!publicKey) {
				logger.error('No SSH public key found at ~/.ssh/id_ed25519.pub or ~/.ssh/id_rsa.pub.')
				process.exit(1)
			}

			const { confirmed } = await prompts(
				{
					type: 'confirm',
					name: 'confirmed',
					message: `This will provision the following servers: ${serverNames.join(', ')}. Continue?`,
					initial: false,
				},
				{
					onCancel: () => {
						logger.warn('Provision cancelled.')
						process.exit(0)
					},
				},
			)

			if (!confirmed) {
				logger.warn('Aborted.')
				process.exit(0)
			}

			for (const [name, group] of Object.entries(config.servers)) {
				const user = args.user ?? group.user
				for (const host of group.hosts) {
					logger.info(`Provisioning ${user}@${host} (group: ${name})...`)
					await provisioner.provision(host, {
						user,
						// biome-ignore lint/style/noNonNullAssertion: validated above with process.exit
						publicKey: publicKey!,
						project: config.app,
					})
				}
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

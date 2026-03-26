import { DevManager } from '@/core/dev-manager.ts'
import { logger } from '@/utils/logger.ts'
import { defineCommand } from 'citty'

const up = defineCommand({
	meta: {
		name: 'up',
		description: 'Start the development environment',
	},
	async run() {
		try {
			const manager = new DevManager()
			await manager.up()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const down = defineCommand({
	meta: {
		name: 'down',
		description: 'Stop the development environment',
	},
	async run() {
		try {
			const manager = new DevManager()
			await manager.down()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const restart = defineCommand({
	meta: {
		name: 'restart',
		description: 'Restart the development environment',
	},
	async run() {
		try {
			const manager = new DevManager()
			await manager.restart()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const logs = defineCommand({
	meta: {
		name: 'logs',
		description: 'Show development environment logs',
	},
	args: {
		follow: {
			type: 'boolean',
			description: 'Follow log output',
			default: true,
		},
	},
	async run({ args }) {
		try {
			const manager = new DevManager()
			await manager.logs(args.follow)
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

const status = defineCommand({
	meta: {
		name: 'status',
		description: 'Show development environment status',
	},
	async run() {
		try {
			const manager = new DevManager()
			await manager.status()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

export default defineCommand({
	meta: {
		name: 'dev',
		description: 'Local development environment management',
	},
	subCommands: {
		up,
		down,
		restart,
		logs,
		status,
	},
	async run() {
		// Default action: start the dev environment
		try {
			const manager = new DevManager()
			await manager.up()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

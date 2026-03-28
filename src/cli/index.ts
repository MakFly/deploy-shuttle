import { defineCommand, runMain } from 'citty'

const mainCommand = defineCommand({
	meta: {
		name: 'shuttle',
		version: '0.1.0',
		description: 'Intelligent local → VPS deployment CLI',
	},
	subCommands: {
		init: () => import('./init.ts').then((m) => m.default),
		provision: () => import('./provision.ts').then((m) => m.default),
		deploy: () => import('./deploy.ts').then((m) => m.default),
		validate: () => import('./validate.ts').then((m) => m.default),
		rollback: () => import('./rollback.ts').then((m) => m.default),
		destroy: () => import('./destroy.ts').then((m) => m.default),
		logs: () => import('./logs.ts').then((m) => m.default),
		ssh: () => import('./ssh.ts').then((m) => m.default),
		status: () => import('./status.ts').then((m) => m.default),
		secrets: () => import('./secrets.ts').then((m) => m.default),
		exec: () => import('./exec.ts').then((m) => m.default),
		lock: () => import('./lock.ts').then((m) => m.default),
		license: () => import('./license.ts').then((m) => m.default),
		ci: () => import('./ci.ts').then((m) => m.default),
		new: () => import('./new.ts').then((m) => m.default),
		dev: () => import('./dev.ts').then((m) => m.default),
		monitor: () => import('./monitor.ts').then((m) => m.default),
	},
})

export function main(): void {
	runMain(mainCommand)
}

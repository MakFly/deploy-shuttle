import path from 'node:path'
import { scaffold } from '@/starters/base.ts'
import type { StarterOptions } from '@/starters/base.ts'
import type { HonoStarterOptions } from '@/starters/hono.ts'
import type { LaravelStarterOptions } from '@/starters/laravel.ts'
import type { NextjsStarterOptions } from '@/starters/nextjs.ts'
import type { NodeStarterOptions } from '@/starters/node.ts'
import type { NuxtStarterOptions } from '@/starters/nuxt.ts'
import type { SymfonyStarterOptions } from '@/starters/symfony.ts'
import { logger } from '@/utils/logger.ts'
import { defineCommand } from 'citty'
import prompts from 'prompts'

export default defineCommand({
	meta: {
		name: 'new',
		description: 'Scaffold a new project with shuttle.yml, Dockerfile, and services',
	},
	args: {
		framework: {
			type: 'positional',
			description: 'Framework template (laravel, symfony, node, nextjs, nuxt, hono)',
			required: false,
		},
		name: {
			type: 'positional',
			description: 'Project directory name',
			required: false,
		},
		yes: {
			type: 'boolean',
			description: 'Use defaults without prompting',
			default: false,
		},
	},
	async run({ args }) {
		try {
			const onCancel = () => {
				logger.warn('Cancelled.')
				process.exit(0)
			}

			// Step 1: Framework selection
			let framework = args.framework as string | undefined
			if (!framework) {
				const { value } = await prompts(
					{
						type: 'select',
						name: 'value',
						message: 'Framework:',
						choices: [
							{ title: 'Laravel', value: 'laravel' },
							{ title: 'Symfony', value: 'symfony' },
							{ title: 'Node / Express', value: 'node' },
							{ title: 'Next.js', value: 'nextjs' },
							{ title: 'Nuxt', value: 'nuxt' },
							{ title: 'Hono (Bun)', value: 'hono' },
						],
					},
					{ onCancel },
				)
				framework = value as string
			}

			const validFrameworks = ['laravel', 'symfony', 'node', 'nextjs', 'nuxt', 'hono']
			if (!validFrameworks.includes(framework)) {
				logger.error(`Unknown framework: ${framework}. Use one of: ${validFrameworks.join(', ')}`)
				process.exit(1)
			}

			// Step 2: Project name
			let name = args.name as string | undefined
			if (!name) {
				const { value } = await prompts(
					{
						type: 'text',
						name: 'value',
						message: 'Project name:',
						initial: `my-${framework}-app`,
						validate: (v: string) => v.trim().length > 0 || 'Project name is required',
					},
					{ onCancel },
				)
				name = value as string
			}

			// Step 3: Base info
			let baseOptions: StarterOptions
			if (args.yes) {
				baseOptions = {
					app: name,
					domain: `${name}.example.com`,
					host: 'your-server-ip',
					user: 'deploy',
				}
			} else {
				const base = await prompts(
					[
						{
							type: 'text',
							name: 'domain',
							message: 'Domain:',
							initial: `${name}.example.com`,
						},
						{
							type: 'text',
							name: 'host',
							message: 'Server host (IP or hostname):',
							initial: 'your-server-ip',
						},
						{
							type: 'text',
							name: 'user',
							message: 'SSH user:',
							initial: 'deploy',
						},
					],
					{ onCancel },
				)
				baseOptions = { app: name, ...(base as { domain: string; host: string; user: string }) }
			}

			// Step 4: Framework-specific options
			const targetDir = path.resolve(name)

			// Check if dir already exists
			const dirFile = Bun.file(path.join(targetDir, 'shuttle.yml'))
			if (await dirFile.exists()) {
				logger.error(`${targetDir}/shuttle.yml already exists.`)
				process.exit(1)
			}

			if (framework === 'laravel') {
				await scaffoldLaravel(targetDir, baseOptions, args.yes, onCancel)
			} else if (framework === 'symfony') {
				await scaffoldSymfony(targetDir, baseOptions, args.yes, onCancel)
			} else if (framework === 'node') {
				await scaffoldNode(targetDir, baseOptions, args.yes, onCancel)
			} else if (framework === 'nextjs') {
				await scaffoldNextjs(targetDir, baseOptions, args.yes, onCancel)
			} else if (framework === 'nuxt') {
				await scaffoldNuxt(targetDir, baseOptions, args.yes, onCancel)
			} else if (framework === 'hono') {
				await scaffoldHono(targetDir, baseOptions, args.yes, onCancel)
			}

			logger.info('')
			logger.info('Next steps:')
			logger.info(`  cd ${name}`)
			logger.info('  shuttle provision')
			logger.info('  shuttle deploy')
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

async function scaffoldLaravel(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getLaravelFiles } = await import('@/starters/laravel.ts')

	let opts: LaravelStarterOptions

	if (useDefaults) {
		opts = {
			database: 'postgres',
			redis: true,
			horizon: true,
			scheduler: true,
			reverb: false,
			mailpit: true,
		}
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'MySQL', value: 'mysql' },
						{ title: 'MariaDB', value: 'mariadb' },
						{ title: 'SQLite', value: 'sqlite' },
					],
				},
				{
					type: 'confirm',
					name: 'redis',
					message: 'Redis cache?',
					initial: true,
				},
				{
					type: 'confirm',
					name: 'horizon',
					message: 'Laravel Horizon (queue dashboard)?',
					initial: true,
				},
				{
					type: 'confirm',
					name: 'scheduler',
					message: 'Task scheduler?',
					initial: true,
				},
				{
					type: 'confirm',
					name: 'reverb',
					message: 'Laravel Reverb (WebSocket)?',
					initial: false,
				},
				{
					type: 'confirm',
					name: 'mailpit',
					message: 'Mailpit (dev email)?',
					initial: true,
				},
			],
			{ onCancel },
		)
		opts = answers as LaravelStarterOptions
	}

	const files = getLaravelFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

async function scaffoldSymfony(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getSymfonyFiles } = await import('@/starters/symfony.ts')

	let opts: SymfonyStarterOptions

	if (useDefaults) {
		opts = {
			database: 'postgres',
			redis: true,
			messenger: true,
			messengerReplicas: 2,
			mailpit: true,
		}
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'MySQL', value: 'mysql' },
						{ title: 'MariaDB', value: 'mariadb' },
						{ title: 'SQLite', value: 'sqlite' },
					],
				},
				{
					type: 'confirm',
					name: 'redis',
					message: 'Redis cache?',
					initial: true,
				},
				{
					type: 'confirm',
					name: 'messenger',
					message: 'Symfony Messenger (async workers)?',
					initial: true,
				},
				{
					type: (_prev: unknown, values: Record<string, unknown>) =>
						values.messenger ? 'number' : null,
					name: 'messengerReplicas',
					message: 'Worker replicas:',
					initial: 2,
					min: 1,
					max: 10,
				},
				{
					type: 'confirm',
					name: 'mailpit',
					message: 'Mailpit (dev email)?',
					initial: true,
				},
			],
			{ onCancel },
		)
		opts = {
			...answers,
			messengerReplicas: (answers.messengerReplicas as number | undefined) ?? 2,
		} as SymfonyStarterOptions
	}

	const files = getSymfonyFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

async function scaffoldNode(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getNodeFiles } = await import('@/starters/node.ts')

	let opts: NodeStarterOptions

	if (useDefaults) {
		opts = { database: 'postgres', redis: true }
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'MySQL', value: 'mysql' },
						{ title: 'None', value: 'none' },
					],
				},
				{
					type: 'confirm',
					name: 'redis',
					message: 'Redis?',
					initial: true,
				},
			],
			{ onCancel },
		)
		opts = answers as NodeStarterOptions
	}

	const files = getNodeFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

async function scaffoldNextjs(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getNextjsFiles } = await import('@/starters/nextjs.ts')

	let opts: NextjsStarterOptions

	if (useDefaults) {
		opts = { database: 'none' }
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'None', value: 'none' },
					],
				},
			],
			{ onCancel },
		)
		opts = answers as NextjsStarterOptions
	}

	const files = getNextjsFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

async function scaffoldNuxt(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getNuxtFiles } = await import('@/starters/nuxt.ts')

	let opts: NuxtStarterOptions

	if (useDefaults) {
		opts = { database: 'none' }
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'None', value: 'none' },
					],
				},
			],
			{ onCancel },
		)
		opts = answers as NuxtStarterOptions
	}

	const files = getNuxtFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

async function scaffoldHono(
	targetDir: string,
	baseOptions: StarterOptions,
	useDefaults: boolean,
	onCancel: () => void,
): Promise<void> {
	const { getHonoFiles } = await import('@/starters/hono.ts')

	let opts: HonoStarterOptions

	if (useDefaults) {
		opts = { database: 'postgres', redis: true }
	} else {
		const answers = await prompts(
			[
				{
					type: 'select',
					name: 'database',
					message: 'Database:',
					choices: [
						{ title: 'PostgreSQL', value: 'postgres' },
						{ title: 'MySQL', value: 'mysql' },
						{ title: 'None', value: 'none' },
					],
				},
				{
					type: 'confirm',
					name: 'redis',
					message: 'Redis?',
					initial: true,
				},
			],
			{ onCancel },
		)
		opts = answers as HonoStarterOptions
	}

	const files = getHonoFiles(opts)
	await scaffold(targetDir, files, baseOptions)
}

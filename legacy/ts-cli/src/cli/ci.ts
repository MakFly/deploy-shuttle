import { mkdir } from 'node:fs/promises'
import { logger } from '@/utils/logger.ts'
import { defineCommand } from 'citty'

const generate = defineCommand({
	meta: {
		name: 'generate',
		description: 'Generate CI/CD pipeline configuration',
	},
	args: {
		provider: {
			type: 'string',
			description: 'CI provider (github or gitlab)',
			default: 'github',
		},
		registry: {
			type: 'string',
			description: 'Docker registry (ghcr or docker-hub)',
			default: 'ghcr',
		},
		branch: {
			type: 'string',
			description: 'Branch to trigger deployments',
			default: 'main',
		},
		app: {
			type: 'string',
			description: 'Application name (defaults to shuttle.yml app name)',
		},
	},
	async run({ args }) {
		const provider = args.provider as 'github' | 'gitlab'
		const app = args.app || (await detectAppName())

		if (provider === 'github') {
			const { generateGitHubActionsWorkflow } = await import('@/templates/github-actions.ts')
			const content = generateGitHubActionsWorkflow({
				app,
				registry: (args.registry as 'ghcr' | 'docker-hub') || 'ghcr',
				branch: args.branch || 'main',
			})

			const dir = '.github/workflows'
			const filePath = `${dir}/shuttle-deploy.yml`
			await mkdir(dir, { recursive: true })
			await Bun.write(filePath, content)
			logger.success(`Generated ${filePath}`)
			logRequiredSecrets('github', args.registry as string)
		} else {
			const { generateGitLabCI } = await import('@/templates/gitlab-ci.ts')
			const content = generateGitLabCI({
				app,
				branch: args.branch || 'main',
			})

			const filePath = '.gitlab-ci.yml'
			await Bun.write(filePath, content)
			logger.success(`Generated ${filePath}`)
			logRequiredSecrets('gitlab', 'gitlab')
		}
	},
})

async function detectAppName(): Promise<string> {
	try {
		const { loadConfig } = await import('@/config/loader.ts')
		const config = await loadConfig()
		return config.app
	} catch {
		return 'my-app'
	}
}

function logRequiredSecrets(provider: string, registry: string): void {
	logger.info('')
	logger.info('Required secrets:')
	if (provider === 'github') {
		logger.info('  • DEPLOY_HOST     — Server IP or hostname')
		logger.info('  • DEPLOY_USER     — SSH username')
		logger.info('  • DEPLOY_SSH_KEY  — SSH private key')
		if (registry === 'docker-hub') {
			logger.info('  • DOCKERHUB_USERNAME — Docker Hub username')
			logger.info('  • DOCKERHUB_TOKEN    — Docker Hub access token')
		}
		logger.info('')
		logger.info('GITHUB_TOKEN is provided automatically for GHCR.')
	} else {
		logger.info('  • SSH_PRIVATE_KEY — SSH private key (type: File)')
		logger.info('  • SSH_HOST_KEY    — Server SSH fingerprint')
		logger.info('  • DEPLOY_HOST     — Server IP or hostname')
		logger.info('  • DEPLOY_USER     — SSH username')
		logger.info('')
		logger.info('CI_REGISTRY_* variables are provided automatically by GitLab.')
	}
}

export default defineCommand({
	meta: {
		name: 'ci',
		description: 'CI/CD pipeline management',
	},
	subCommands: {
		generate,
	},
})

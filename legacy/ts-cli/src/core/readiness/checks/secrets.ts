import type { Check, CheckContext, CheckResult } from '../types.ts'

export const envWorldReadableCheck: Check = {
	id: 'secrets.env_world_readable',
	async run(context: CheckContext): Promise<CheckResult> {
		const exists = await context.exec.run('test -f .env', { timeoutMs: 1000 })
		if (exists.exitCode !== 0) {
			return {
				id: 'secrets.env_world_readable',
				title: '.env is not world-readable',
				category: 'secrets',
				severity: 'critical',
				status: 'skipped',
				summary: 'No .env file found in the current project.',
				autoFixAvailable: false,
			}
		}

		const mode = await context.exec.run("stat -c '%a' .env 2>/dev/null || stat -f '%Lp' .env", {
			timeoutMs: 1000,
		})
		const permissions = mode.stdout.trim()
		const lastDigit = Number.parseInt(permissions.at(-1) ?? '0', 10)
		const worldReadable = !Number.isNaN(lastDigit) && (lastDigit & 4) === 4

		return {
			id: 'secrets.env_world_readable',
			title: '.env is not world-readable',
			category: 'secrets',
			severity: 'critical',
			status: worldReadable ? 'failed' : 'passed',
			summary: `Current .env permissions: ${permissions || 'unknown'}.`,
			remediation: 'Run chmod 600 .env and keep secrets out of shared-readable files.',
			autoFixAvailable: true,
			evidence: { permissions },
		}
	},
}

export const envTrackedByGitCheck: Check = {
	id: 'secrets.env_in_git',
	async run(context: CheckContext): Promise<CheckResult> {
		const tracked = await context.exec.run('git ls-files --error-unmatch .env', {
			timeoutMs: 1000,
		})
		const isTracked = tracked.exitCode === 0

		return {
			id: 'secrets.env_in_git',
			title: '.env is not tracked by Git',
			category: 'secrets',
			severity: 'critical',
			status: isTracked ? 'failed' : 'passed',
			summary: isTracked ? '.env is tracked by Git.' : '.env is not tracked by Git.',
			remediation: 'Remove .env from Git history/tracking and keep .env in .gitignore.',
			autoFixAvailable: false,
		}
	},
}

import type { Check, CheckContext, CheckResult } from '../types.ts'

export const dockerInstalledCheck: Check = {
	id: 'docker.not_installed',
	async run(context: CheckContext): Promise<CheckResult> {
		const docker = await context.exec.run('command -v docker && docker --version', {
			timeoutMs: 3000,
		})
		const installed = docker.exitCode === 0

		return {
			id: 'docker.not_installed',
			title: 'Docker is installed',
			category: 'docker',
			severity: 'high',
			status: installed ? 'passed' : 'failed',
			summary: installed
				? (docker.stdout.trim().split('\n').at(-1) ?? 'Docker found.')
				: 'Docker is not installed or not available in PATH.',
			remediation: 'Install Docker Engine before running production Docker workloads.',
			autoFixAvailable: false,
		}
	},
}

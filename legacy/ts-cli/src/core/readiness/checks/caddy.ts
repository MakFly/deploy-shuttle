import type { Check, CheckContext, CheckResult } from '../types.ts'

export const caddyInstalledCheck: Check = {
	id: 'caddy.not_installed',
	async run(context: CheckContext): Promise<CheckResult> {
		const caddy = await context.exec.run(
			"command -v caddy >/dev/null 2>&1 || docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^caddy$|caddy'",
			{ timeoutMs: 3000 },
		)
		const installed = caddy.exitCode === 0

		return {
			id: 'caddy.not_installed',
			title: 'Caddy or Caddy container is present',
			category: 'reverse-proxy',
			severity: 'medium',
			status: installed ? 'passed' : 'failed',
			summary: installed
				? 'Caddy was detected.'
				: 'No Caddy binary or running Caddy container detected.',
			remediation: 'Install/configure Caddy or another reverse proxy before exposing the app.',
			autoFixAvailable: false,
		}
	},
}

import type { Check, CheckContext, CheckResult } from '../types.ts'

const SENSITIVE_PORTS = ['5432', '3306', '6379', '7700', '9200', '27017']

export const ufwActiveCheck: Check = {
	id: 'firewall.ufw_inactive',
	async run(context: CheckContext): Promise<CheckResult> {
		const ufw = await context.exec.run('command -v ufw >/dev/null 2>&1 && ufw status', {
			timeoutMs: 3000,
		})
		const active = ufw.exitCode === 0 && /^Status:\s+active$/im.test(ufw.stdout)

		return {
			id: 'firewall.ufw_inactive',
			title: 'UFW firewall is active',
			category: 'firewall',
			severity: 'high',
			status: active ? 'passed' : 'failed',
			summary: active ? 'UFW is active.' : 'UFW is missing or inactive.',
			remediation: 'Enable a firewall with SSH, HTTP and HTTPS explicitly allowed.',
			autoFixAvailable: true,
			evidence: { stdout: ufw.stdout.trim(), stderr: ufw.stderr.trim() },
		}
	},
}

export const databasePortPublicCheck: Check = {
	id: 'firewall.database_port_public',
	async run(context: CheckContext): Promise<CheckResult> {
		const sockets = await context.exec.run(
			'if command -v ss >/dev/null 2>&1; then ss -ltn; elif command -v netstat >/dev/null 2>&1; then netstat -ltn; else exit 127; fi',
			{ timeoutMs: 3000 },
		)

		if (sockets.exitCode !== 0) {
			return {
				id: 'firewall.database_port_public',
				title: 'Sensitive database ports are not public',
				category: 'firewall',
				severity: 'critical',
				status: 'unknown',
				summary: 'Could not inspect listening TCP sockets.',
				remediation:
					'Install ss or netstat, then verify database ports bind to localhost or private networks only.',
				autoFixAvailable: false,
				evidence: { stderr: sockets.stderr.trim() },
			}
		}

		const publicPorts = SENSITIVE_PORTS.filter((port) => {
			const pattern = new RegExp(`(?:0\\.0\\.0\\.0|\\[::\\]|::):${port}\\b`)
			return pattern.test(sockets.stdout)
		})

		return {
			id: 'firewall.database_port_public',
			title: 'Sensitive database ports are not public',
			category: 'firewall',
			severity: 'critical',
			status: publicPorts.length > 0 ? 'failed' : 'passed',
			summary:
				publicPorts.length > 0
					? `Public sensitive ports detected: ${publicPorts.join(', ')}.`
					: 'No public sensitive database ports detected.',
			remediation:
				'Bind databases to localhost/private Docker networks and close public database ports.',
			autoFixAvailable: false,
			evidence: { publicPorts },
		}
	},
}

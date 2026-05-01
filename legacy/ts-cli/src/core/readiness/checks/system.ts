import type { Check, CheckContext, CheckResult } from '../types.ts'

function result(
	overrides: Partial<CheckResult> & Pick<CheckResult, 'status' | 'summary'>,
): CheckResult {
	return {
		id: 'system.os_supported',
		title: 'Operating system is supported',
		category: 'system',
		severity: 'high',
		autoFixAvailable: false,
		...overrides,
	}
}

export const systemOsSupportedCheck: Check = {
	id: 'system.os_supported',
	async run(context: CheckContext): Promise<CheckResult> {
		const release = await context.exec.run('cat /etc/os-release', { timeoutMs: 3000 })
		if (release.exitCode !== 0) {
			return result({
				status: 'unknown',
				summary: 'Could not read /etc/os-release.',
				details: release.stderr.trim(),
			})
		}

		const id = release.stdout.match(/^ID="?([^"\n]+)"?/m)?.[1] ?? ''
		const version = release.stdout.match(/^VERSION_ID="?([^"\n]+)"?/m)?.[1] ?? ''
		const supported =
			(id === 'ubuntu' && (version === '22.04' || version === '24.04')) ||
			(id === 'debian' && version === '12')

		return result({
			status: supported ? 'passed' : 'failed',
			summary: supported
				? `${id} ${version} is supported.`
				: `${id || 'unknown'} ${version || 'unknown'} is not in the MVP support matrix.`,
			remediation: 'Use Ubuntu 22.04, Ubuntu 24.04, or Debian 12 for MVP support.',
			evidence: { id, version },
		})
	},
}

export const systemDiskSpaceLowCheck: Check = {
	id: 'system.disk_space_low',
	async run(context: CheckContext): Promise<CheckResult> {
		const disk = await context.exec.run("df -Pk / | awk 'NR==2 {print $5}'", { timeoutMs: 3000 })
		const usage = Number.parseInt(disk.stdout.trim().replace('%', ''), 10)

		if (disk.exitCode !== 0 || Number.isNaN(usage)) {
			return {
				id: 'system.disk_space_low',
				title: 'Disk space has enough free capacity',
				category: 'system',
				severity: 'medium',
				status: 'unknown',
				summary: 'Could not determine disk usage for root filesystem.',
				details: disk.stderr.trim(),
				autoFixAvailable: false,
			}
		}

		const failed = usage >= 80
		return {
			id: 'system.disk_space_low',
			title: 'Disk space has enough free capacity',
			category: 'system',
			severity: usage >= 90 ? 'critical' : 'medium',
			status: failed ? 'failed' : 'passed',
			summary: `Root filesystem usage is ${usage}%.`,
			remediation: 'Free disk space, prune unused Docker resources, or increase VPS disk size.',
			autoFixAvailable: false,
			evidence: { usagePercent: usage },
		}
	},
}

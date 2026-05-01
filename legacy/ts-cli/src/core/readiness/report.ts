import { formatReadinessLevel } from './scoring.ts'
import type { CheckResult, CheckSeverity, ReadinessReport } from './types.ts'

const SEVERITY_ORDER: CheckSeverity[] = ['critical', 'high', 'medium', 'low', 'info']

function formatCheckLine(check: CheckResult): string {
	const marker = check.status === 'passed' ? '[ok]' : check.status === 'failed' ? '[x]' : '[?]'
	return `  ${marker} ${check.title} - ${check.summary}`
}

export function formatConsoleReport(report: ReadinessReport): string {
	const lines: string[] = [
		'DeployShuttle Doctor Report',
		'',
		`Target: ${report.target}`,
		`Profile: ${report.profile.length > 0 ? report.profile.join(', ') : 'default'}`,
		`Score: ${report.score}/100 - ${formatReadinessLevel(report.level)}`,
		`Generated: ${report.generatedAt}`,
		'',
	]

	for (const severity of SEVERITY_ORDER) {
		const checks = report.checks.filter((check) => check.severity === severity)
		if (checks.length === 0) continue

		lines.push(`${severity[0].toUpperCase()}${severity.slice(1)}:`)
		for (const check of checks) {
			lines.push(formatCheckLine(check))
			if (check.status === 'failed' && check.remediation) {
				lines.push(`      Fix: ${check.remediation}`)
			}
		}
		lines.push('')
	}

	const failed = report.checks.filter((check) => check.status === 'failed')
	if (failed.length > 0) {
		lines.push('Next:')
		lines.push('  deploy-shuttle doctor --format json')
		lines.push('  deploy-shuttle harden --dry-run  # planned')
		lines.push('')
	}

	return lines.join('\n')
}

export function formatJsonReport(report: ReadinessReport): string {
	return JSON.stringify(report, null, 2)
}

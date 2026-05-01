import type { CheckResult, CheckSeverity, ReadinessLevel } from './types.ts'

const PENALTIES: Record<CheckSeverity, number> = {
	critical: 20,
	high: 10,
	medium: 5,
	low: 2,
	info: 0,
}

export function scoreChecks(checks: CheckResult[]): number {
	const penalty = checks.reduce((total, check) => {
		if (check.status !== 'failed') {
			return total
		}

		return total + PENALTIES[check.severity]
	}, 0)

	return Math.max(0, Math.min(100, 100 - penalty))
}

export function getReadinessLevel(score: number): ReadinessLevel {
	if (score >= 90) return 'production-ready'
	if (score >= 75) return 'almost-ready'
	if (score >= 50) return 'risky'
	return 'not-production-ready'
}

export function formatReadinessLevel(level: ReadinessLevel): string {
	switch (level) {
		case 'production-ready':
			return 'Production Ready'
		case 'almost-ready':
			return 'Almost Ready'
		case 'risky':
			return 'Risky'
		case 'not-production-ready':
			return 'Not Production Ready'
	}
}

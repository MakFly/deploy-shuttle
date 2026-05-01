import { describe, expect, it } from 'bun:test'
import { formatReadinessLevel, getReadinessLevel, scoreChecks } from '@/core/readiness/scoring.ts'
import type { CheckResult, CheckSeverity } from '@/core/readiness/types.ts'

function failed(id: string, severity: CheckSeverity): CheckResult {
	return {
		id,
		title: id,
		category: 'system',
		severity,
		status: 'failed',
		summary: 'failed',
		autoFixAvailable: false,
	}
}

describe('readiness scoring', () => {
	it('subtracts only failed check penalties', () => {
		const checks: CheckResult[] = [
			failed('critical', 'critical'),
			failed('high', 'high'),
			{ ...failed('medium', 'medium'), status: 'passed' },
		]

		expect(scoreChecks(checks)).toBe(70)
	})

	it('clamps the score at zero', () => {
		expect(
			scoreChecks(Array.from({ length: 10 }, (_, index) => failed(String(index), 'critical'))),
		).toBe(0)
	})

	it('maps score bands to readiness levels', () => {
		expect(getReadinessLevel(95)).toBe('production-ready')
		expect(getReadinessLevel(80)).toBe('almost-ready')
		expect(getReadinessLevel(65)).toBe('risky')
		expect(getReadinessLevel(30)).toBe('not-production-ready')
		expect(formatReadinessLevel('almost-ready')).toBe('Almost Ready')
	})
})

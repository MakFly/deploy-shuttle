import { describe, expect, it } from 'bun:test'
import { runDoctor } from '@/core/readiness/doctor.ts'
import type { Check, ExecAdapter } from '@/core/readiness/types.ts'

const passingCheck: Check = {
	id: 'system.test_pass',
	async run() {
		return {
			id: 'system.test_pass',
			title: 'Pass',
			category: 'system',
			severity: 'high',
			status: 'passed',
			summary: 'ok',
			autoFixAvailable: false,
		}
	},
}

const failingCheck: Check = {
	id: 'docker.test_fail',
	async run() {
		return {
			id: 'docker.test_fail',
			title: 'Fail',
			category: 'docker',
			severity: 'critical',
			status: 'failed',
			summary: 'bad',
			autoFixAvailable: false,
		}
	},
}

const noopExec: ExecAdapter = {
	async run() {
		return { exitCode: 0, stdout: '', stderr: '' }
	},
}

describe('runDoctor', () => {
	it('runs checks and computes a deterministic score', async () => {
		const report = await runDoctor({
			exec: noopExec,
			checks: [passingCheck, failingCheck],
			profile: ['docker'],
		})

		expect(report.target).toBe('local')
		expect(report.profile).toEqual(['docker'])
		expect(report.score).toBe(80)
		expect(report.level).toBe('almost-ready')
		expect(report.checks).toHaveLength(2)
		expect(report.generatedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/)
	})
})

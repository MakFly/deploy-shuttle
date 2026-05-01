import { defaultChecks } from './checks/index.ts'
import { getReadinessLevel, scoreChecks } from './scoring.ts'
import type { Check, CheckContext, ExecAdapter, ReadinessReport } from './types.ts'

export type DoctorOptions = {
	target?: string
	profile?: string[]
	cwd?: string
	exec: ExecAdapter
	checks?: Check[]
}

export async function runDoctor(options: DoctorOptions): Promise<ReadinessReport> {
	const target = options.target ?? 'local'
	const profile = options.profile ?? ['docker', 'caddy']
	const context: CheckContext = {
		target,
		profile,
		exec: options.exec,
		cwd: options.cwd ?? process.cwd(),
	}

	const checks = options.checks ?? defaultChecks
	const results = []

	for (const check of checks) {
		results.push(await check.run(context))
	}

	const score = scoreChecks(results)

	return {
		target,
		profile,
		score,
		level: getReadinessLevel(score),
		checks: results,
		generatedAt: new Date().toISOString(),
	}
}

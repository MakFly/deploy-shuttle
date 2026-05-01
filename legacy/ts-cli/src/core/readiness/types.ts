export type CheckSeverity = 'critical' | 'high' | 'medium' | 'low' | 'info'

export type CheckStatus = 'passed' | 'failed' | 'skipped' | 'unknown'

export type CheckCategory =
	| 'system'
	| 'ssh'
	| 'firewall'
	| 'docker'
	| 'compose'
	| 'reverse-proxy'
	| 'tls'
	| 'cloudflare'
	| 'database'
	| 'backups'
	| 'secrets'
	| 'monitoring'
	| 'app'
	| 'performance'

export type ExecResult = {
	exitCode: number
	stdout: string
	stderr: string
}

export type ExecAdapter = {
	run: (command: string, options?: { timeoutMs?: number }) => Promise<ExecResult>
}

export type CheckResult = {
	id: string
	title: string
	category: CheckCategory
	severity: CheckSeverity
	status: CheckStatus
	summary: string
	details?: string
	remediation?: string
	autoFixAvailable: boolean
	evidence?: Record<string, unknown>
}

export type CheckContext = {
	target: string
	profile: string[]
	exec: ExecAdapter
	cwd: string
}

export type Check = {
	id: string
	run: (context: CheckContext) => Promise<CheckResult>
}

export type ReadinessLevel = 'production-ready' | 'almost-ready' | 'risky' | 'not-production-ready'

export type ReadinessReport = {
	target: string
	profile: string[]
	score: number
	level: ReadinessLevel
	checks: CheckResult[]
	generatedAt: string
}

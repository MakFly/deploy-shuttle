import { LocalShellAdapter } from '@/adapters/local-shell.ts'
import { runDoctor } from '@/core/readiness/doctor.ts'
import { formatConsoleReport, formatJsonReport } from '@/core/readiness/report.ts'
import { logger } from '@/utils/logger.ts'
import { defineCommand } from 'citty'

type DoctorFormat = 'console' | 'json'

function parseFormat(value: string | undefined): DoctorFormat {
	if (value === undefined || value === 'console') return 'console'
	if (value === 'json') return 'json'
	throw new Error(`Unsupported format "${value}". Supported formats: console, json.`)
}

function parseProfile(value: string | undefined): string[] {
	if (!value) return ['docker', 'caddy']
	return value
		.split(',')
		.map((part) => part.trim())
		.filter(Boolean)
}

export default defineCommand({
	meta: {
		name: 'doctor',
		description: 'Run a VPS production readiness audit',
	},
	args: {
		target: {
			type: 'string',
			description: 'Remote SSH target user@host (planned; local only in current MVP slice)',
		},
		format: {
			type: 'string',
			description: 'Output format: console or json',
			default: 'console',
		},
		profile: {
			type: 'string',
			description: 'Comma-separated check profile labels',
		},
		'fail-below': {
			type: 'string',
			description: 'Exit with code 1 when score is below this threshold',
		},
		verbose: {
			type: 'boolean',
			description: 'Enable verbose output',
			default: false,
		},
	},
	async run({ args }) {
		try {
			if (args.verbose) {
				logger.setVerbose(true)
			}

			if (args.target) {
				throw new Error(
					'Remote doctor targets are planned but not implemented in this slice. Run local doctor without --target.',
				)
			}

			const format = parseFormat(args.format)
			const failBelow =
				args['fail-below'] !== undefined ? Number.parseInt(args['fail-below'], 10) : undefined

			if (
				failBelow !== undefined &&
				(Number.isNaN(failBelow) || failBelow < 0 || failBelow > 100)
			) {
				throw new Error('--fail-below must be a number between 0 and 100.')
			}

			const report = await runDoctor({
				exec: new LocalShellAdapter(process.cwd()),
				profile: parseProfile(args.profile),
			})

			const output = format === 'json' ? formatJsonReport(report) : formatConsoleReport(report)
			process.stdout.write(`${output}\n`)

			const hasCriticalFailure = report.checks.some(
				(check) => check.severity === 'critical' && check.status === 'failed',
			)
			if (hasCriticalFailure || (failBelow !== undefined && report.score < failBelow)) {
				process.exit(1)
			}
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

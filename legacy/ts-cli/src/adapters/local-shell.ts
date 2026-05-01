import type { ExecAdapter, ExecResult } from '@/core/readiness/types.ts'

export class LocalShellAdapter implements ExecAdapter {
	constructor(private readonly cwd: string = process.cwd()) {}

	async run(command: string, options?: { timeoutMs?: number }): Promise<ExecResult> {
		const proc = Bun.spawn(['sh', '-lc', command], {
			cwd: this.cwd,
			stdout: 'pipe',
			stderr: 'pipe',
		})

		const timeoutMs = options?.timeoutMs
		let timeout: Timer | undefined

		const timeoutPromise =
			timeoutMs !== undefined
				? new Promise<ExecResult>((resolve) => {
						timeout = setTimeout(() => {
							proc.kill()
							resolve({
								exitCode: 124,
								stdout: '',
								stderr: `Command timed out after ${timeoutMs}ms`,
							})
						}, timeoutMs)
					})
				: undefined

		const commandPromise = (async (): Promise<ExecResult> => {
			const [stdout, stderr, exitCode] = await Promise.all([
				new Response(proc.stdout).text(),
				new Response(proc.stderr).text(),
				proc.exited,
			])

			if (timeout !== undefined) {
				clearTimeout(timeout)
			}

			return {
				exitCode,
				stdout,
				stderr,
			}
		})()

		return timeoutPromise ? Promise.race([commandPromise, timeoutPromise]) : commandPromise
	}
}

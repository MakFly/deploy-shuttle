import { logger } from '../utils/logger.ts'

export interface HostTask<T> {
	host: string
	user: string
	group: string
	execute: () => Promise<T>
}

export interface ParallelResult<T> {
	host: string
	group: string
	result?: T
	error?: Error
}

/**
 * Execute tasks across hosts with configurable concurrency.
 * - concurrency=1: sequential (no overhead)
 * - concurrency>1: parallel with semaphore
 *
 * Errors don't abort other hosts — all results are collected.
 */
export async function executeParallel<T>(
	tasks: HostTask<T>[],
	concurrency = 5,
): Promise<ParallelResult<T>[]> {
	if (tasks.length === 0) return []

	// Single task or concurrency=1: run sequentially
	if (tasks.length === 1 || concurrency <= 1) {
		const results: ParallelResult<T>[] = []
		for (const task of tasks) {
			try {
				const result = await task.execute()
				results.push({ host: task.host, group: task.group, result })
			} catch (err) {
				results.push({
					host: task.host,
					group: task.group,
					error: err instanceof Error ? err : new Error(String(err)),
				})
			}
		}
		return results
	}

	// Parallel with semaphore
	let running = 0
	let index = 0
	const results: ParallelResult<T>[] = new Array(tasks.length)

	return new Promise((resolve) => {
		function next() {
			while (running < concurrency && index < tasks.length) {
				const i = index++
				const task = tasks[i]
				running++

				logger.debug(`[parallel] Starting ${task.host} (${running}/${concurrency} slots)`)

				task
					.execute()
					.then((result) => {
						results[i] = { host: task.host, group: task.group, result }
					})
					.catch((err) => {
						results[i] = {
							host: task.host,
							group: task.group,
							error: err instanceof Error ? err : new Error(String(err)),
						}
					})
					.finally(() => {
						running--
						logger.debug(`[parallel] Finished ${task.host} (${running}/${concurrency} slots)`)
						if (index >= tasks.length && running === 0) {
							resolve(results)
						} else {
							next()
						}
					})
			}
		}

		next()
	})
}

/**
 * Flatten server groups into a flat list of host tasks.
 */
export function flattenServerGroups(
	servers: Record<string, { hosts: string[]; user: string }>,
): Array<{ host: string; user: string; group: string }> {
	const result: Array<{ host: string; user: string; group: string }> = []
	for (const [group, config] of Object.entries(servers)) {
		for (const host of config.hosts) {
			result.push({ host, user: config.user, group })
		}
	}
	return result
}

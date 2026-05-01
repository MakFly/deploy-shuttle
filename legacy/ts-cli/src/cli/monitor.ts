import { defineCommand } from 'citty'
import { loadConfig } from '../config/loader.ts'
import { ssh } from '../core/ssh-manager.ts'
import { logger } from '../utils/logger.ts'

interface DockerStats {
	Name: string
	CPUPerc: string
	MemUsage: string
	MemPerc: string
	NetIO: string
	BlockIO: string
	PIDs: string
}

function padEnd(str: string, len: number): string {
	return str.length >= len ? str.slice(0, len) : str + ' '.repeat(len - str.length)
}

function formatTable(host: string, rows: DockerStats[]): string {
	const lines: string[] = []
	lines.push(`\x1b[1m\x1b[36m${host}\x1b[0m`)

	if (rows.length === 0) {
		lines.push('  (no containers running)')
		return lines.join('\n')
	}

	const header = [
		padEnd('NAME', 24),
		padEnd('CPU %', 8),
		padEnd('MEM USAGE', 18),
		padEnd('MEM %', 8),
		padEnd('NET I/O', 20),
		padEnd('BLOCK I/O', 20),
		padEnd('PIDs', 6),
	].join('  ')

	lines.push(`  \x1b[2m${header}\x1b[0m`)

	for (const row of rows) {
		const line = [
			padEnd(row.Name, 24),
			padEnd(row.CPUPerc, 8),
			padEnd(row.MemUsage, 18),
			padEnd(row.MemPerc, 8),
			padEnd(row.NetIO, 20),
			padEnd(row.BlockIO, 20),
			padEnd(row.PIDs, 6),
		].join('  ')
		lines.push(`  ${line}`)
	}

	return lines.join('\n')
}

async function fetchStats(
	host: string,
	user: string,
): Promise<{ host: string; rows: DockerStats[] }> {
	try {
		await ssh.connect({ host, user })
		const { stdout } = await ssh.exec(host, `docker stats --no-stream --format '{{json .}}'`)

		const rows: DockerStats[] = stdout
			.trim()
			.split('\n')
			.filter((line) => line.trim().length > 0)
			.map((line) => {
				try {
					return JSON.parse(line) as DockerStats
				} catch {
					return null
				}
			})
			.filter((row): row is DockerStats => row !== null)

		return { host, rows }
	} catch (err) {
		logger.debug(
			`Failed to fetch stats from ${host}: ${err instanceof Error ? err.message : String(err)}`,
		)
		return { host, rows: [] }
	}
}

async function printStats(
	servers: Record<string, { hosts: string[]; user: string }>,
	once: boolean,
): Promise<void> {
	if (!once) {
		process.stdout.write('\x1b[2J\x1b[H')
	}

	const now = new Date().toLocaleTimeString()
	console.log(`\x1b[1mshuttle monitor\x1b[0m  \x1b[2m${now}\x1b[0m\n`)

	for (const [groupName, group] of Object.entries(servers)) {
		console.log(`\x1b[33m[${groupName}]\x1b[0m`)
		for (const host of group.hosts) {
			const { rows } = await fetchStats(host, group.user)
			console.log(formatTable(host, rows))
		}
		console.log()
	}
}

export default defineCommand({
	meta: {
		name: 'monitor',
		description: 'Live Docker resource usage across all servers',
	},
	args: {
		config: {
			type: 'string',
			description: 'Path to shuttle.yml',
		},
		env: {
			type: 'string',
			description: 'Environment overlay',
		},
		once: {
			type: 'boolean',
			description: 'Show stats once and exit',
			default: false,
		},
	},
	async run({ args }) {
		try {
			const config = await loadConfig(args.config, args.env)

			if (args.once) {
				await printStats(config.servers, true)
				for (const group of Object.values(config.servers)) {
					for (const host of group.hosts) {
						ssh.disconnect(host)
					}
				}
				return
			}

			// Initial render
			await printStats(config.servers, false)

			const interval = setInterval(async () => {
				await printStats(config.servers, false)
			}, 3000)

			// Graceful Ctrl+C
			process.on('SIGINT', () => {
				clearInterval(interval)
				for (const group of Object.values(config.servers)) {
					for (const host of group.hosts) {
						ssh.disconnect(host)
					}
				}
				process.stdout.write('\x1b[?25h') // restore cursor
				console.log('\nBye.')
				process.exit(0)
			})

			// Keep process alive
			process.stdin.resume()
		} catch (err) {
			logger.error(err instanceof Error ? err : new Error(String(err)))
			process.exit(1)
		}
	},
})

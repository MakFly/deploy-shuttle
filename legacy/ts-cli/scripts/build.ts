import { createHash } from 'node:crypto'
import { existsSync } from 'node:fs'
import { mkdir, writeFile } from 'node:fs/promises'
import { readFile } from 'node:fs/promises'
import { $ } from 'bun'

const ENTRY = 'src/index.ts'
const OUT_DIR = 'dist'

const TARGETS = [
	{ target: 'bun-linux-x64', output: 'shuttle-linux-x64' },
	{ target: 'bun-darwin-arm64', output: 'shuttle-darwin-arm64' },
	{ target: 'bun-darwin-x64', output: 'shuttle-darwin-x64' },
] as const

async function sha256(filePath: string): Promise<string> {
	const data = await readFile(filePath)
	return createHash('sha256').update(data).digest('hex')
}

async function build() {
	if (!existsSync(OUT_DIR)) {
		await mkdir(OUT_DIR, { recursive: true })
		console.log(`Created ${OUT_DIR}/`)
	}

	const checksums: string[] = []

	for (const { target, output } of TARGETS) {
		const outPath = `${OUT_DIR}/${output}`
		console.log(`\nBuilding ${output} (${target})...`)

		const result =
			await $`bun build ${ENTRY} --compile --target=${target} --outfile=${outPath}`.nothrow()

		if (result.exitCode !== 0) {
			console.error(`  Failed to build ${output}:`)
			console.error(result.stderr.toString())
			process.exit(1)
		}

		const hash = await sha256(outPath)
		checksums.push(`${hash}  ${output}`)
		console.log(`  ${output} — OK (sha256: ${hash.slice(0, 16)}...)`)
	}

	const checksumsPath = `${OUT_DIR}/checksums.txt`
	await writeFile(checksumsPath, `${checksums.join('\n')}\n`)
	console.log(`\nChecksums written to ${checksumsPath}`)
	console.log('Build complete.')
}

build()

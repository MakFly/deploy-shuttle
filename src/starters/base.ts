import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { logger } from '@/utils/logger.ts'

export interface StarterFile {
	path: string
	content: string
}

export interface StarterOptions {
	app: string
	domain: string
	host: string
	user: string
}

/**
 * Write all starter files to the target directory, replacing tokens.
 */
export async function scaffold(
	targetDir: string,
	files: StarterFile[],
	options: StarterOptions,
): Promise<void> {
	await mkdir(targetDir, { recursive: true })

	for (const file of files) {
		const content = replaceTokens(file.content, options)
		const filePath = path.join(targetDir, file.path)
		const dir = path.dirname(filePath)
		await mkdir(dir, { recursive: true })
		await Bun.write(filePath, content)
	}

	// Create .shuttle directory
	const shuttleDir = path.join(targetDir, '.shuttle')
	await mkdir(shuttleDir, { recursive: true })
	await Bun.write(path.join(shuttleDir, '.gitkeep'), '')

	logger.success(`Project scaffolded at ${targetDir}`)
}

function replaceTokens(content: string, options: StarterOptions): string {
	return content
		.replaceAll('__APP_NAME__', options.app)
		.replaceAll('__DOMAIN__', options.domain)
		.replaceAll('__HOST__', options.host)
		.replaceAll('__USER__', options.user)
}

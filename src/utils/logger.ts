import { consola } from 'consola'

let verbose = false

export const logger = {
	get verbose(): boolean {
		return verbose
	},

	setVerbose(value: boolean): void {
		verbose = value
	},

	info(message: string, ...args: unknown[]): void {
		consola.info(message, ...args)
	},

	success(message: string, ...args: unknown[]): void {
		consola.success(message, ...args)
	},

	warn(message: string, ...args: unknown[]): void {
		consola.warn(message, ...args)
	},

	error(message: string | Error, ...args: unknown[]): void {
		consola.error(message, ...args)
	},

	debug(message: string, ...args: unknown[]): void {
		if (!verbose) return
		consola.debug(message, ...args)
	},

	fatal(message: string | Error, ...args: unknown[]): void {
		consola.fatal(message, ...args)
	},

	box(message: string): void {
		consola.box(message)
	},

	start(message: string, ...args: unknown[]): void {
		consola.start(message, ...args)
	},

	/**
	 * Logs a numbered deployment step, e.g. "[3/10] Starting container..."
	 */
	step(n: number, total: number, message: string): void {
		consola.info(`[${n}/${total}] ${message}`)
	},
}

import type { ShuttleConfig } from '../config/schema.ts'
import { DeployError } from '../utils/errors.ts'
import { shellEscape } from '../utils/shell.ts'
import { ssh } from './ssh-manager.ts'

const STATE_DIR = '/opt/shuttle'

export interface DeployState {
	active_slot: 'blue' | 'green'
	active_tag: string
	previous_tag?: string
	port: number
	deployed_at: string
	version: number
}

export interface InFlightDeployState {
	status: 'running' | 'failed'
	app: string
	host: string
	strategy: 'blue-green' | 'rolling'
	service?: string
	stage: string
	error?: string
	started_at: string
	updated_at: string
}

export interface DeployLock {
	app: string
	host: string
	pid: number
	created_at: string
}

export class RuntimeManager {
	getAppDir(app: string): string {
		return `${STATE_DIR}/${app}`
	}

	getWorkDir(app: string): string {
		return `${this.getAppDir(app)}/${app}`
	}

	getStatePath(app: string): string {
		return `${this.getAppDir(app)}/state.json`
	}

	getInFlightPath(app: string): string {
		return `${this.getAppDir(app)}/deploying.json`
	}

	getLockDir(app: string): string {
		return `${this.getAppDir(app)}/.deploy.lock`
	}

	async ensureAppDir(host: string, app: string): Promise<void> {
		await ssh.exec(host, `mkdir -p ${shellEscape(this.getAppDir(app))}`)
	}

	async readState(host: string, app: string): Promise<DeployState> {
		const path = this.getStatePath(app)
		const { stdout, code } = await ssh.exec(host, `cat ${shellEscape(path)}`)

		if (code !== 0) {
			throw new DeployError(`State file not found at ${path} on ${host}`, 'read-state')
		}

		try {
			return JSON.parse(stdout) as DeployState
		} catch (err) {
			throw new DeployError(`Failed to parse state.json on ${host}: ${String(err)}`, 'read-state')
		}
	}

	async writeState(host: string, app: string, state: DeployState): Promise<void> {
		await this.ensureAppDir(host, app)
		await ssh.uploadContent(host, JSON.stringify(state, null, 2), this.getStatePath(app), 0o644)
	}

	async writeInFlight(host: string, app: string, state: InFlightDeployState): Promise<void> {
		await this.ensureAppDir(host, app)
		await ssh.uploadContent(host, JSON.stringify(state, null, 2), this.getInFlightPath(app), 0o644)
	}

	async clearInFlight(host: string, app: string): Promise<void> {
		await ssh.exec(host, `rm -f ${shellEscape(this.getInFlightPath(app))}`)
	}

	async acquireLock(host: string, app: string): Promise<void> {
		await this.ensureAppDir(host, app)

		const lockDir = this.getLockDir(app)
		const { code } = await ssh.exec(host, `mkdir ${shellEscape(lockDir)}`)

		if (code !== 0) {
			let message = `Deployment lock already exists for "${app}" on ${host}`
			try {
				const existing = await this.readLock(host, app)
				message += ` (owner host=${existing.host}, pid=${existing.pid}, created_at=${existing.created_at})`
			} catch {
				// Ignore missing metadata and keep the base lock error.
			}
			throw new DeployError(message, 'lock')
		}

		const lock: DeployLock = {
			app,
			host,
			pid: process.pid,
			created_at: new Date().toISOString(),
		}

		try {
			await ssh.uploadContent(
				host,
				JSON.stringify(lock, null, 2),
				`${lockDir}/metadata.json`,
				0o644,
			)
		} catch (err) {
			await ssh.exec(host, `rm -rf ${shellEscape(lockDir)}`)
			throw DeployError.wrap(
				err,
				`Failed to persist deployment lock for "${app}" on ${host}`,
				'lock',
			)
		}
	}

	async releaseLock(host: string, app: string): Promise<void> {
		await ssh.exec(host, `rm -rf ${shellEscape(this.getLockDir(app))}`)
	}

	async readLock(host: string, app: string): Promise<DeployLock> {
		const path = `${this.getLockDir(app)}/metadata.json`
		const { stdout, code } = await ssh.exec(host, `cat ${shellEscape(path)}`)

		if (code !== 0) {
			throw new DeployError(`Deployment lock metadata not found at ${path} on ${host}`, 'lock')
		}

		try {
			return JSON.parse(stdout) as DeployLock
		} catch (err) {
			throw new DeployError(
				`Failed to parse deployment lock metadata on ${host}: ${String(err)}`,
				'lock',
			)
		}
	}

	async runHook(
		host: string,
		app: string,
		command: string,
		phase: 'pre_deploy' | 'post_deploy',
	): Promise<void> {
		const workDir = this.getWorkDir(app)
		const wrapped = `cd ${shellEscape(workDir)} && /bin/sh -lc ${shellEscape(command)}`
		const { code, stderr } = await ssh.exec(host, wrapped)

		if (code !== 0) {
			throw new DeployError(
				`${phase} hook failed on ${host} (exit ${code}): ${stderr.trim()}`,
				phase,
			)
		}
	}

	async forceReleaseLock(host: string, app: string): Promise<void> {
		const lockDir = this.getLockDir(app)
		await ssh.exec(host, `rm -rf ${shellEscape(lockDir)}`)
	}

	async resolveServiceContainer(
		host: string,
		config: ShuttleConfig,
		service: string,
	): Promise<string> {
		const strategy = config.deploy?.strategy ?? 'blue-green'

		if (strategy === 'rolling') {
			return `${config.app}_${service}_0`
		}

		try {
			const state = await this.readState(host, config.app)
			return `${config.app}_${service}_${state.active_slot}`
		} catch {
			return `${config.app}_${service}_blue`
		}
	}
}

export const runtime = new RuntimeManager()

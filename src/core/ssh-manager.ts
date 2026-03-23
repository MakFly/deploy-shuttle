import { existsSync, readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { Client, type ConnectConfig } from 'ssh2'
import { SSHError } from '../utils/errors.ts'
import { logger } from '../utils/logger.ts'

// ---------------------------------------------------------------------------
// SSHManager
// ---------------------------------------------------------------------------

export class SSHManager {
	readonly connections: Map<string, Client> = new Map()

	/**
	 * Returns an existing live connection for `host`, or establishes a new one.
	 * Key resolution order for authentication:
	 *   1. SSH agent (SSH_AUTH_SOCK)
	 *   2. Explicit `privateKey` parameter
	 *   3. ~/.ssh/id_ed25519
	 *   4. ~/.ssh/id_rsa
	 */
	async connect(config: {
		host: string
		user: string
		port?: number
		privateKey?: string
	}): Promise<Client> {
		const existing = this.connections.get(config.host)
		if (existing !== undefined) {
			return existing
		}

		const connectConfig = await this._buildConnectConfig(config)

		return new Promise<Client>((resolve, reject) => {
			const client = new Client()

			client
				.on('ready', () => {
					this.connections.set(config.host, client)
					logger.debug(`SSH connected to ${config.host}`)
					resolve(client)
				})
				.on('error', (err) => {
					reject(new SSHError(`Connection failed: ${err.message}`, config.host))
				})
				.on('close', () => {
					this.connections.delete(config.host)
					logger.debug(`SSH connection closed for ${config.host}`)
				})

			client.connect(connectConfig)
		})
	}

	/**
	 * Executes a command on a connected host and collects full output.
	 */
	async exec(
		host: string,
		command: string,
	): Promise<{ stdout: string; stderr: string; code: number }> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.exec(command, (err, stream) => {
				if (err != null) {
					return reject(new SSHError(`exec failed: ${err.message}`, host))
				}

				let stdout = ''
				let stderr = ''

				stream
					.on('data', (chunk: Buffer) => {
						stdout += chunk.toString()
					})
					.stderr.on('data', (chunk: Buffer) => {
						stderr += chunk.toString()
					})

				stream.on('close', (code: number) => {
					resolve({ stdout, stderr, code: code ?? 0 })
				})

				stream.on('error', (err: Error) => {
					reject(new SSHError(`stream error: ${err.message}`, host))
				})
			})
		})
	}

	/**
	 * Executes a command and returns a readable stream for the stdout.
	 */
	async execStream(host: string, command: string): Promise<NodeJS.ReadableStream> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.exec(command, (err, stream) => {
				if (err != null) {
					return reject(new SSHError(`execStream failed: ${err.message}`, host))
				}
				resolve(stream as unknown as NodeJS.ReadableStream)
			})
		})
	}

	/**
	 * Uploads a local file to a remote path via SFTP.
	 */
	async upload(host: string, localPath: string, remotePath: string): Promise<void> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.sftp((err, sftp) => {
				if (err != null) {
					return reject(new SSHError(`SFTP init failed: ${err.message}`, host))
				}

				sftp.fastPut(localPath, remotePath, (err) => {
					if (err != null) {
						return reject(new SSHError(`Upload failed: ${err.message}`, host))
					}
					resolve()
				})
			})
		})
	}

	/**
	 * Writes a string as a remote file via SFTP with an optional chmod mode.
	 */
	async uploadContent(
		host: string,
		content: string,
		remotePath: string,
		mode?: number,
	): Promise<void> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.sftp((err, sftp) => {
				if (err != null) {
					return reject(new SSHError(`SFTP init failed: ${err.message}`, host))
				}

				const opts: { mode?: number } = mode !== undefined ? { mode } : {}

				const writeStream = sftp.createWriteStream(remotePath, opts)

				writeStream.on('error', (err: Error) => {
					reject(new SSHError(`Write failed: ${err.message}`, host))
				})

				writeStream.on('close', () => {
					resolve()
				})

				writeStream.end(content, 'utf8')
			})
		})
	}

	/**
	 * Opens an interactive shell on the remote host, piping stdin/stdout.
	 */
	async shell(host: string): Promise<void> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.shell((err, stream) => {
				if (err != null) {
					return reject(new SSHError(`shell failed: ${err.message}`, host))
				}

				process.stdin.setRawMode?.(true)
				process.stdin.pipe(stream)
				stream.pipe(process.stdout)
				stream.stderr.pipe(process.stderr)

				stream.on('close', () => {
					process.stdin.unpipe(stream)
					process.stdin.setRawMode?.(false)
					resolve()
				})

				stream.on('error', (err: Error) => {
					reject(new SSHError(`shell stream error: ${err.message}`, host))
				})
			})
		})
	}

	/**
	 * Pipes a readable stream as stdin to a remote command and returns stdout.
	 */
	async pipe(host: string, command: string, input: NodeJS.ReadableStream): Promise<string> {
		const client = this._getClient(host)

		return new Promise((resolve, reject) => {
			client.exec(command, (err, stream) => {
				if (err != null) {
					return reject(new SSHError(`pipe exec failed: ${err.message}`, host))
				}

				let stdout = ''
				let stderr = ''

				stream
					.on('data', (chunk: Buffer) => {
						stdout += chunk.toString()
					})
					.stderr.on('data', (chunk: Buffer) => {
						stderr += chunk.toString()
					})

				stream.on('close', (code: number) => {
					if (code !== 0) {
						return reject(new SSHError(`pipe command exited ${code}: ${stderr.trim()}`, host))
					}
					resolve(stdout)
				})

				stream.on('error', (err: Error) => {
					reject(new SSHError(`pipe stream error: ${err.message}`, host))
				})

				// Pipe the input stream into the remote stdin
				;(input as NodeJS.ReadableStream & { pipe: (dest: unknown) => void }).pipe(stream)
			})
		})
	}

	/**
	 * Closes a specific connection (by host) or all connections when called
	 * without arguments.
	 */
	disconnect(host?: string): void {
		if (host !== undefined) {
			const client = this.connections.get(host)
			if (client !== undefined) {
				client.end()
				this.connections.delete(host)
			}
			return
		}

		for (const [h, client] of this.connections) {
			client.end()
			this.connections.delete(h)
		}
	}

	// ---------------------------------------------------------------------------
	// Private helpers
	// ---------------------------------------------------------------------------

	private _getClient(host: string): Client {
		const client = this.connections.get(host)
		if (client === undefined) {
			throw new SSHError(`No active connection for host "${host}". Call connect() first.`, host)
		}
		return client
	}

	private async _buildConnectConfig(config: {
		host: string
		user: string
		port?: number
		privateKey?: string
	}): Promise<ConnectConfig> {
		const base: ConnectConfig = {
			host: config.host,
			port: config.port ?? 22,
			username: config.user,
		}

		// 1. Try SSH agent
		const authSock = process.env.SSH_AUTH_SOCK
		if (authSock !== undefined && authSock.length > 0) {
			logger.debug(`Using SSH agent for ${config.host}`)
			return { ...base, agent: authSock }
		}

		// 2. Explicit privateKey parameter
		if (config.privateKey !== undefined) {
			return { ...base, privateKey: config.privateKey }
		}

		// 3. ~/.ssh/id_ed25519 then ~/.ssh/id_rsa
		const home = homedir()
		for (const name of ['id_ed25519', 'id_rsa']) {
			const keyPath = join(home, '.ssh', name)
			if (existsSync(keyPath)) {
				logger.debug(`Using key ${keyPath} for ${config.host}`)
				return { ...base, privateKey: readFileSync(keyPath) }
			}
		}

		throw new SSHError(`No SSH authentication method available for ${config.host}`, config.host)
	}
}

export const ssh = new SSHManager()

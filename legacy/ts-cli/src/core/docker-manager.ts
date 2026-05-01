import { DeployError } from '../utils/errors.ts'
import { logger } from '../utils/logger.ts'
import { shellEscape } from '../utils/shell.ts'
import { type SSHManager, ssh as defaultSsh } from './ssh-manager.ts'

// ---------------------------------------------------------------------------
// DockerManager
// ---------------------------------------------------------------------------

export class DockerManager {
	constructor(private readonly ssh: SSHManager = defaultSsh) {}

	/**
	 * Builds a Docker image locally using Bun.spawn.
	 */
	async build(options: {
		dockerfile?: string
		context?: string
		target?: string
		platform?: string
		tag: string
		args?: Record<string, string>
	}): Promise<void> {
		const args =
			options.platform !== undefined
				? [
						'docker',
						'buildx',
						'build',
						'--load',
						'--platform',
						options.platform,
						'--tag',
						options.tag,
					]
				: ['docker', 'build', '--tag', options.tag]

		if (options.dockerfile !== undefined) {
			args.push('--file', options.dockerfile)
		}

		if (options.target !== undefined) {
			args.push('--target', options.target)
		}

		for (const [key, value] of Object.entries(options.args ?? {})) {
			args.push('--build-arg', `${key}=${value}`)
		}

		args.push(options.context ?? '.')

		logger.debug(`Building image: ${args.join(' ')}`)

		const proc = Bun.spawn(args, {
			stdout: 'pipe',
			stderr: 'pipe',
		})

		const exitCode = await proc.exited

		if (exitCode !== 0) {
			const stderr = await new Response(proc.stderr).text()
			throw new DeployError(`docker build failed (exit ${exitCode}): ${stderr.trim()}`, 'build')
		}
	}

	/**
	 * Runs `docker save <tag>` and returns the stdout as a readable stream.
	 */
	save(tag: string): NodeJS.ReadableStream {
		logger.debug(`Saving image: ${tag}`)

		const proc = Bun.spawn(['docker', 'save', tag], {
			stdout: 'pipe',
			stderr: 'pipe',
		})

		return proc.stdout as unknown as NodeJS.ReadableStream
	}

	/**
	 * Loads a docker image archive from a stream on the remote host via SSH pipe.
	 */
	async loadRemote(host: string, stream: NodeJS.ReadableStream): Promise<void> {
		logger.debug(`Loading image on ${host}`)
		await this.ssh.pipe(host, 'docker load', stream)
	}

	/**
	 * Transfers an image from local to remote without writing to disk.
	 * Equivalent to: docker save <tag> | ssh <host> docker load
	 */
	async transfer(tag: string, host: string): Promise<void> {
		logger.info(`Transferring image ${tag} to ${host}`)
		const stream = this.save(tag)
		await this.loadRemote(host, stream)
		logger.success(`Image ${tag} transferred to ${host}`)
	}

	/**
	 * Pulls an image on a remote host from a registry.
	 */
	async pull(host: string, image: string): Promise<void> {
		logger.debug(`Pulling image ${image} on ${host}`)
		const { code, stderr } = await this.ssh.exec(host, `docker pull ${shellEscape(image)}`)
		if (code !== 0) {
			throw new DeployError(
				`docker pull failed on ${host} (exit ${code}): ${stderr.trim()}`,
				'pull',
			)
		}
	}

	/**
	 * Runs a container on a remote host via SSH.
	 */
	async run(
		host: string,
		options: {
			name: string
			image: string
			port?: string
			env?: Record<string, string>
			envFile?: string
			command?: string
			network?: string
			volumes?: string[]
			labels?: Record<string, string>
		},
	): Promise<void> {
		const args = ['docker', 'run', '--detach', '--restart', 'unless-stopped']

		args.push('--name', shellEscape(options.name))

		if (options.port !== undefined) {
			args.push('--publish', shellEscape(options.port))
		}

		if (options.network !== undefined) {
			args.push('--network', shellEscape(options.network))
		}

		for (const [key, value] of Object.entries(options.env ?? {})) {
			args.push('--env', shellEscape(`${key}=${value}`))
		}

		for (const [key, value] of Object.entries(options.labels ?? {})) {
			args.push('--label', shellEscape(`${key}=${value}`))
		}

		if (options.envFile !== undefined) {
			args.push('--env-file', shellEscape(options.envFile))
		}

		for (const vol of options.volumes ?? []) {
			args.push('--volume', shellEscape(vol))
		}

		args.push(shellEscape(options.image))

		if (options.command !== undefined) {
			args.push(...options.command.split(' '))
		}

		const command = args.join(' ')
		logger.debug(`docker run on ${host}: ${command}`)

		const { code, stderr } = await this.ssh.exec(host, command)

		if (code !== 0) {
			throw new DeployError(`docker run failed on ${host} (exit ${code}): ${stderr.trim()}`, 'run')
		}
	}

	/**
	 * Stops and removes a container on a remote host.
	 */
	async stop(host: string, name: string, timeout?: number): Promise<void> {
		const timeoutFlag = timeout !== undefined ? `--time ${timeout}` : '--time 30'

		// Stop — ignore error if already stopped
		await this.ssh.exec(host, `docker stop ${timeoutFlag} ${shellEscape(name)} 2>/dev/null || true`)
		// Remove — ignore error if already removed
		await this.ssh.exec(host, `docker rm ${shellEscape(name)} 2>/dev/null || true`)

		logger.debug(`Stopped and removed container ${name} on ${host}`)
	}

	/**
	 * Removes a container without waiting for a graceful stop.
	 */
	async remove(host: string, name: string, force = false): Promise<void> {
		const forceFlag = force ? '--force' : ''
		await this.ssh.exec(
			host,
			`docker rm ${forceFlag} ${shellEscape(name)} 2>/dev/null || true`.trim(),
		)
		logger.debug(`Removed container ${name} on ${host}`)
	}

	/**
	 * Returns a stream of docker logs from a remote container.
	 */
	async logs(
		host: string,
		name: string,
		follow = false,
		lines?: number,
	): Promise<NodeJS.ReadableStream> {
		const followFlag = follow ? '--follow' : ''
		const tailFlag = lines !== undefined ? `--tail ${lines}` : ''
		const command = `docker logs ${followFlag} ${tailFlag} ${shellEscape(name)}`.trim()
		return this.ssh.execStream(host, command)
	}

	/**
	 * Runs a command inside an existing remote container.
	 */
	async exec(
		host: string,
		container: string,
		command: string,
	): Promise<{ stdout: string; code: number }> {
		const { stdout, code } = await this.ssh.exec(
			host,
			`docker exec ${shellEscape(container)} ${command}`,
		)
		return { stdout, code }
	}

	/**
	 * Returns the parsed docker inspect payload for a container, or null when
	 * the container does not exist.
	 */
	async inspect<T = Record<string, unknown>>(host: string, name: string): Promise<T | null> {
		const { stdout, code } = await this.ssh.exec(host, `docker inspect ${shellEscape(name)}`)

		if (code !== 0) {
			return null
		}

		try {
			const parsed = JSON.parse(stdout) as T[]
			return parsed[0] ?? null
		} catch (err) {
			throw new DeployError(
				`Failed to parse docker inspect output for ${name} on ${host}: ${String(err)}`,
				'inspect',
			)
		}
	}

	/**
	 * Lists image tags on a remote host, optionally filtered by a prefix string.
	 */
	async listImages(host: string, filter?: string): Promise<string[]> {
		const formatFlag = `--format "{{.Repository}}:{{.Tag}}"`
		const filterFlag = filter !== undefined ? `--filter reference=${shellEscape(filter)}` : ''

		const command = `docker images ${filterFlag} ${formatFlag}`.trim()
		const { stdout, code } = await this.ssh.exec(host, command)

		if (code !== 0) {
			return []
		}

		return stdout
			.split('\n')
			.map((l) => l.trim())
			.filter((l) => l.length > 0)
	}

	/**
	 * Lists container names on a remote host, optionally filtered by a prefix.
	 */
	async listContainers(host: string, prefix?: string): Promise<string[]> {
		const { stdout, code } = await this.ssh.exec(host, 'docker ps -a --format "{{.Names}}"')

		if (code !== 0) {
			return []
		}

		return stdout
			.split('\n')
			.map((line) => line.trim())
			.filter((line) => line.length > 0 && (prefix === undefined || line.startsWith(prefix)))
	}

	/**
	 * Removes old images on a remote host, keeping the N most recent ones
	 * whose tags start with `prefix`.
	 */
	async prune(host: string, keep: number, prefix: string): Promise<void> {
		const images = await this.listImages(host, `${prefix}*`)

		// Images are returned newest-first by Docker; remove the tail
		const toRemove = images.slice(keep)

		if (toRemove.length === 0) {
			return
		}

		const removeCmd = toRemove.map((img) => `docker rmi --force ${shellEscape(img)}`).join(' && ')

		const { code, stderr } = await this.ssh.exec(host, removeCmd)

		if (code !== 0) {
			logger.warn(`Image prune had errors on ${host}: ${stderr.trim()}`)
		} else {
			logger.debug(`Pruned ${toRemove.length} image(s) on ${host}`)
		}
	}

	/**
	 * Tags an image on a remote host.
	 */
	async tag(host: string, source: string, target: string): Promise<void> {
		const { code, stderr } = await this.ssh.exec(
			host,
			`docker tag ${shellEscape(source)} ${shellEscape(target)}`,
		)

		if (code !== 0) {
			throw new DeployError(`docker tag failed on ${host} (exit ${code}): ${stderr.trim()}`, 'tag')
		}
	}

	/**
	 * Removes images on a remote host, ignoring already-missing references.
	 */
	async removeImages(host: string, images: string[]): Promise<void> {
		if (images.length === 0) {
			return
		}

		const command = images
			.map((image) => `docker rmi --force ${shellEscape(image)} 2>/dev/null || true`)
			.join(' && ')
		await this.ssh.exec(host, command)
	}
}

export const docker = new DockerManager(defaultSsh)

import { afterAll, beforeAll, describe, expect, test } from 'bun:test'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { accessories } from '../../src/core/accessory-manager.ts'
import { runtime } from '../../src/core/runtime-manager.ts'
import { ssh } from '../../src/core/ssh-manager.ts'
import { createConfig } from '../helpers/config-factory.ts'

const shouldRun = process.env.SHUTTLE_INTEGRATION === '1'
const describeIntegration = shouldRun ? describe : describe.skip

function run(command: string, args: string[], input?: string): string {
	return execFileSync(command, args, {
		input,
		stdio: ['pipe', 'pipe', 'pipe'],
		encoding: 'utf8',
	}).trim()
}

describeIntegration('remote runtime integration', () => {
	const containerName = `shuttle-it-ssh-${Date.now()}`
	const host = '127.0.0.1'
	const port = 22222
	const keyDir = mkdtempSync(path.join(tmpdir(), 'shuttle-it-'))
	const privateKeyPath = path.join(keyDir, 'id_ed25519')
	const publicKeyPath = `${privateKeyPath}.pub`

	beforeAll(async () => {
		run('ssh-keygen', ['-t', 'ed25519', '-N', '', '-f', privateKeyPath])

		run('docker', [
			'run',
			'--detach',
			'--name',
			containerName,
			'--publish',
			`${port}:22`,
			'--volume',
			'/var/run/docker.sock:/var/run/docker.sock',
			'ubuntu:24.04',
			'sleep',
			'infinity',
		])

		run('docker', ['exec', containerName, 'bash', '-lc', 'apt-get update'])
		run('docker', [
			'exec',
			containerName,
			'bash',
			'-lc',
			'DEBIAN_FRONTEND=noninteractive apt-get install -y openssh-server docker.io',
		])

		const publicKey = readFileSync(publicKeyPath, 'utf8')
		run('docker', [
			'exec',
			containerName,
			'bash',
			'-lc',
			'mkdir -p /root/.ssh && chmod 700 /root/.ssh',
		])
		run(
			'docker',
			[
				'exec',
				'-i',
				containerName,
				'bash',
				'-lc',
				'cat > /root/.ssh/authorized_keys && chmod 600 /root/.ssh/authorized_keys',
			],
			publicKey,
		)
		run('docker', ['exec', containerName, 'bash', '-lc', 'mkdir -p /run/sshd && /usr/sbin/sshd'])

		await ssh.connect({
			host,
			port,
			user: 'root',
			privateKey: readFileSync(privateKeyPath, 'utf8'),
		})
	})

	afterAll(async () => {
		try {
			await ssh.disconnect(host)
		} catch {
			// Ignore cleanup errors in opt-in integration tests.
		}

		run('docker', ['rm', '-f', containerName])
		rmSync(keyDir, { recursive: true, force: true })
	})

	test('can manage accessories and deploy locks over real SSH + Docker', async () => {
		const config = createConfig({
			accessories: {
				redis: { image: 'redis:7-alpine', port: 6379 },
			},
		})

		await accessories.ensureAccessories(host, config)

		const inspect = await ssh.exec(host, 'docker inspect --format "{{.State.Running}}" myapp_redis')
		expect(inspect.stdout.trim()).toBe('true')

		await runtime.acquireLock(host, config.app)
		const lock = await runtime.readLock(host, config.app)
		expect(lock.app).toBe('myapp')
		await runtime.releaseLock(host, config.app)

		await ssh.exec(host, 'docker rm -f myapp_redis')
	})
})

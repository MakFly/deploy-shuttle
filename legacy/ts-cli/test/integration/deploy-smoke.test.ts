import { afterAll, beforeAll, describe, expect, test } from 'bun:test'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { DockerManager } from '../../src/core/docker-manager.ts'
import { SSHManager } from '../../src/core/ssh-manager.ts'

const describeIntegration = process.env.SHUTTLE_INTEGRATION === '1' ? describe : describe.skip

describeIntegration('integration: ssh + docker smoke', () => {
	let containerId = ''
	let host = ''
	let keyDir = ''
	const sshManager = new SSHManager()
	const dockerManager = new DockerManager()

	beforeAll(async () => {
		keyDir = mkdtempSync(path.join(tmpdir(), 'shuttle-it-'))
		const privateKeyPath = path.join(keyDir, 'id_ed25519')
		const publicKeyPath = `${privateKeyPath}.pub`
		const authorizedKeysPath = path.join(keyDir, 'authorized_keys')

		const keygen = Bun.spawnSync([
			'ssh-keygen',
			'-q',
			'-t',
			'ed25519',
			'-N',
			'',
			'-f',
			privateKeyPath,
		])
		if (keygen.exitCode !== 0) {
			throw new Error(`ssh-keygen failed: ${new TextDecoder().decode(keygen.stderr)}`)
		}

		writeFileSync(authorizedKeysPath, readFileSync(publicKeyPath))

		const run = Bun.spawnSync([
			'docker',
			'run',
			'-d',
			'--rm',
			'--privileged',
			'-v',
			`${authorizedKeysPath}:/root/.ssh/authorized_keys`,
			'docker:27-dind',
			'sh',
			'-lc',
			[
				'apk add --no-cache openssh',
				'mkdir -p /root/.ssh',
				'chmod 700 /root/.ssh',
				'chmod 600 /root/.ssh/authorized_keys',
				'ssh-keygen -A',
				'/usr/sbin/sshd',
				'exec dockerd-entrypoint.sh',
			].join(' && '),
		])
		if (run.exitCode !== 0) {
			throw new Error(
				`failed to start integration container: ${new TextDecoder().decode(run.stderr)}`,
			)
		}

		containerId = new TextDecoder().decode(run.stdout).trim()

		for (let attempt = 0; attempt < 30; attempt++) {
			const inspect = Bun.spawnSync([
				'docker',
				'inspect',
				'-f',
				'{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}',
				containerId,
			])
			host = new TextDecoder().decode(inspect.stdout).trim()
			if (host.length === 0) {
				await Bun.sleep(1000)
				continue
			}

			try {
				await sshManager.connect({
					host,
					user: 'root',
					privateKey: readFileSync(privateKeyPath, 'utf8'),
				})
				return
			} catch {
				await Bun.sleep(1000)
			}
		}

		throw new Error('integration container never became reachable over SSH')
	}, 60_000)

	afterAll(async () => {
		sshManager.disconnect()

		if (containerId.length > 0) {
			Bun.spawnSync(['docker', 'rm', '-f', containerId])
		}
	})

	test('can execute docker commands through a live SSH target', async () => {
		const { stdout, code } = await sshManager.exec(
			host,
			"docker info --format '{{.ServerVersion}}'",
		)
		expect(code).toBe(0)
		expect(stdout.trim().length).toBeGreaterThan(0)
	})

	test('can run and clean up a real container through DockerManager', async () => {
		await dockerManager.run(host, {
			name: 'shuttle_integration_probe',
			image: 'alpine:3.20',
			command: 'sleep 30',
		})

		const inspect = await dockerManager.inspect<{ State?: { Running?: boolean } }>(
			host,
			'shuttle_integration_probe',
		)
		expect(inspect?.State?.Running).toBe(true)

		await dockerManager.stop(host, 'shuttle_integration_probe')
		const afterStop = await dockerManager.inspect(host, 'shuttle_integration_probe')
		expect(afterStop).toBeNull()
	})
})

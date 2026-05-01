// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

const mockSSH = createMockSSH()

mock.module('../../../src/core/ssh-manager.ts', () => ({ ssh: mockSSH }))

import { DockerManager } from '../../../src/core/docker-manager.ts'
import { DeployError } from '../../../src/utils/errors.ts'
import { shellEscape } from '../../../src/utils/shell.ts'

describe('DockerManager', () => {
	let docker: DockerManager

	beforeEach(() => {
		docker = new DockerManager()
		mockSSH.exec.mockReset()
		mockSSH.execStream.mockReset()
		mockSSH.pipe.mockReset()
		mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
		mockSSH.execStream.mockImplementation(() => Promise.resolve({ on: mock(() => {}) }))
		mockSSH.pipe.mockImplementation(() => Promise.resolve(''))
	})

	describe('run', () => {
		test('constructs correct docker run command with all options', async () => {
			await docker.run('1.2.3.4', {
				name: 'myapp_web_blue',
				image: 'shuttle/myapp:deploy-20240101-abc1234',
				port: '127.0.0.1:10000:3000',
				env: { NODE_ENV: 'production' },
				envFile: '/opt/shuttle/myapp/.env',
				volumes: ['/data:/app/data'],
				network: 'mynet',
				command: 'node server.js',
			})

			expect(mockSSH.exec).toHaveBeenCalledTimes(1)
			const [host, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(host).toBe('1.2.3.4')
			expect(cmd).toContain('docker run')
			expect(cmd).toContain('--detach')
			expect(cmd).toContain('--restart unless-stopped')
			expect(cmd).toContain(shellEscape('myapp_web_blue'))
			expect(cmd).toContain(shellEscape('127.0.0.1:10000:3000'))
			expect(cmd).toContain(shellEscape('mynet'))
			expect(cmd).toContain(shellEscape('NODE_ENV=production'))
			expect(cmd).toContain(shellEscape('/opt/shuttle/myapp/.env'))
			expect(cmd).toContain(shellEscape('/data:/app/data'))
			expect(cmd).toContain(shellEscape('shuttle/myapp:deploy-20240101-abc1234'))
			expect(cmd).toContain('node server.js')
		})

		test('throws DeployError when docker run exits with non-zero code', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'No such image', code: 1 }),
			)

			expect(docker.run('1.2.3.4', { name: 'app', image: 'missing:latest' })).rejects.toThrow(
				DeployError,
			)
		})

		test('constructs run command without optional fields when not provided', async () => {
			await docker.run('1.2.3.4', {
				name: 'myapp_web_blue',
				image: 'shuttle/myapp:latest',
			})

			const [, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(cmd).not.toContain('--publish')
			expect(cmd).not.toContain('--network')
			expect(cmd).not.toContain('--env ')
			expect(cmd).not.toContain('--env-file')
			expect(cmd).not.toContain('--volume')
		})

		test('includes labels when provided', async () => {
			await docker.run('1.2.3.4', {
				name: 'myapp_web_blue',
				image: 'shuttle/myapp:latest',
				labels: { 'shuttle.kind': 'service' },
			})

			const [, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(cmd).toContain('--label')
			expect(cmd).toContain(shellEscape('shuttle.kind=service'))
		})
	})

	describe('build', () => {
		test('uses docker buildx with --platform when provided', async () => {
			const originalSpawn = Bun.spawn
			const calls: string[][] = []
			try {
				Bun.spawn = ((args: string[]) => {
					calls.push(args)
					return {
						stdout: new ReadableStream(),
						stderr: new ReadableStream(),
						exited: Promise.resolve(0),
					} as ReturnType<typeof Bun.spawn>
				}) as typeof Bun.spawn

				await docker.build({
					tag: 'shuttle/myapp:latest',
					platform: 'linux/amd64',
				})

				const args = calls[0] as string[]
				expect(args).toEqual([
					'docker',
					'buildx',
					'build',
					'--load',
					'--platform',
					'linux/amd64',
					'--tag',
					'shuttle/myapp:latest',
					'.',
				])
			} finally {
				Bun.spawn = originalSpawn
			}
		})
	})

	describe('stop', () => {
		test('calls docker stop and docker rm with escaped name', async () => {
			await docker.stop('1.2.3.4', 'myapp_web_blue')

			expect(mockSSH.exec).toHaveBeenCalledTimes(2)
			const stopCmd = mockSSH.exec.mock.calls[0][1] as string
			const rmCmd = mockSSH.exec.mock.calls[1][1] as string
			expect(stopCmd).toContain('docker stop')
			expect(stopCmd).toContain(shellEscape('myapp_web_blue'))
			expect(rmCmd).toContain('docker rm')
			expect(rmCmd).toContain(shellEscape('myapp_web_blue'))
		})

		test('uses provided timeout value in stop command', async () => {
			await docker.stop('1.2.3.4', 'myapp_web_blue', 60)

			const stopCmd = mockSSH.exec.mock.calls[0][1] as string
			expect(stopCmd).toContain('--time 60')
		})

		test('uses default timeout of 30 when no timeout provided', async () => {
			await docker.stop('1.2.3.4', 'myapp_web_blue')

			const stopCmd = mockSSH.exec.mock.calls[0][1] as string
			expect(stopCmd).toContain('--time 30')
		})
	})

	describe('logs', () => {
		test('includes --follow flag when follow is true', async () => {
			await docker.logs('1.2.3.4', 'myapp_web_blue', true)

			const [, cmd] = mockSSH.execStream.mock.calls[0] as [string, string]
			expect(cmd).toContain('--follow')
		})

		test('includes --tail N flag when lines specified', async () => {
			await docker.logs('1.2.3.4', 'myapp_web_blue', false, 100)

			const [, cmd] = mockSSH.execStream.mock.calls[0] as [string, string]
			expect(cmd).toContain('--tail 100')
		})

		test('escapes container name in logs command', async () => {
			await docker.logs('1.2.3.4', 'myapp_web_blue')

			const [, cmd] = mockSSH.execStream.mock.calls[0] as [string, string]
			expect(cmd).toContain(shellEscape('myapp_web_blue'))
		})
	})

	describe('exec', () => {
		test('calls docker exec with escaped container name', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: 'output', stderr: '', code: 0 }),
			)

			const result = await docker.exec('1.2.3.4', 'myapp_web_blue', 'node --version')

			const [, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(cmd).toContain('docker exec')
			expect(cmd).toContain(shellEscape('myapp_web_blue'))
			expect(cmd).toContain('node --version')
			expect(result.stdout).toBe('output')
			expect(result.code).toBe(0)
		})
	})

	describe('listImages', () => {
		test('parses stdout into an array of image tags', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({
					stdout: 'shuttle/myapp:deploy-20240101-abc\nshuttle/myapp:deploy-20231201-def\n',
					stderr: '',
					code: 0,
				}),
			)

			const images = await docker.listImages('1.2.3.4')

			expect(images).toHaveLength(2)
			expect(images).toContain('shuttle/myapp:deploy-20240101-abc')
			expect(images).toContain('shuttle/myapp:deploy-20231201-def')
		})

		test('returns empty array when exec exits with non-zero code', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'Error', code: 1 }),
			)

			const images = await docker.listImages('1.2.3.4')
			expect(images).toEqual([])
		})

		test('filters empty lines from stdout', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({
					stdout: 'shuttle/myapp:v1\n\n  \nshuttle/myapp:v2\n',
					stderr: '',
					code: 0,
				}),
			)

			const images = await docker.listImages('1.2.3.4')
			expect(images).toHaveLength(2)
		})
	})

	describe('prune', () => {
		test('removes images beyond the keep count', async () => {
			mockSSH.exec.mockImplementationOnce(() =>
				Promise.resolve({
					stdout: 'shuttle/myapp:v3\nshuttle/myapp:v2\nshuttle/myapp:v1\n',
					stderr: '',
					code: 0,
				}),
			)
			mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))

			await docker.prune('1.2.3.4', 2, 'shuttle/myapp')

			// Second call should be the rmi command for v1 (oldest, beyond keep=2)
			expect(mockSSH.exec).toHaveBeenCalledTimes(2)
			const rmiCmd = mockSSH.exec.mock.calls[1][1] as string
			expect(rmiCmd).toContain('docker rmi --force')
			expect(rmiCmd).toContain(shellEscape('shuttle/myapp:v1'))
		})

		test('does nothing when image count is within keep limit', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({
					stdout: 'shuttle/myapp:v1\n',
					stderr: '',
					code: 0,
				}),
			)

			await docker.prune('1.2.3.4', 5, 'shuttle/myapp')

			// Only the listImages call should be made — no rmi
			expect(mockSSH.exec).toHaveBeenCalledTimes(1)
		})
	})

	describe('tag', () => {
		test('calls docker tag with escaped source and target', async () => {
			await docker.tag('1.2.3.4', 'shuttle/myapp:deploy-20240101-abc', 'shuttle/myapp:current')

			const [, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(cmd).toContain('docker tag')
			expect(cmd).toContain(shellEscape('shuttle/myapp:deploy-20240101-abc'))
			expect(cmd).toContain(shellEscape('shuttle/myapp:current'))
		})

		test('throws DeployError when docker tag fails', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'No such image', code: 1 }),
			)

			expect(docker.tag('1.2.3.4', 'missing:tag', 'target:tag')).rejects.toThrow(DeployError)
		})
	})

	describe('transfer', () => {
		test('calls save then loadRemote in sequence', async () => {
			const fakeStream = {}
			const saveSpy = mock(() => fakeStream)
			const loadSpy = mock(() => Promise.resolve())

			docker.save = saveSpy as unknown as typeof docker.save
			docker.loadRemote = loadSpy

			await docker.transfer('shuttle/myapp:latest', '1.2.3.4')

			expect(saveSpy).toHaveBeenCalledWith('shuttle/myapp:latest')
			expect(loadSpy).toHaveBeenCalledWith('1.2.3.4', fakeStream)
		})
	})

	describe('removeImages', () => {
		test('removes all requested images', async () => {
			await docker.removeImages('1.2.3.4', ['shuttle/myapp:v1', 'shuttle/myapp:v2'])

			const [, cmd] = mockSSH.exec.mock.calls[0] as [string, string]
			expect(cmd).toContain('docker rmi --force')
			expect(cmd).toContain(shellEscape('shuttle/myapp:v1'))
			expect(cmd).toContain(shellEscape('shuttle/myapp:v2'))
		})
	})
})

// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

const mockSSH = createMockSSH()

mock.module('../../../src/core/ssh-manager.ts', () => ({ ssh: mockSSH }))

import { ProvisionManager } from '../../../src/core/provision-manager.ts'
import { ProvisionError } from '../../../src/utils/errors.ts'

describe('ProvisionManager', () => {
	let provisioner: ProvisionManager

	beforeEach(() => {
		provisioner = new ProvisionManager()
		mockSSH.connect.mockReset()
		mockSSH.exec.mockReset()
		mockSSH.disconnect.mockReset()

		mockSSH.connect.mockImplementation(() => Promise.resolve({}))
		// Default: all commands succeed, OS is Ubuntu, verify passes
		mockSSH.exec.mockImplementation((_host: string, cmd: string) => {
			if (cmd.includes('os-release')) {
				return Promise.resolve({ stdout: 'ID=ubuntu\nNAME="Ubuntu 22.04"', stderr: '', code: 0 })
			}
			if (cmd.includes('docker inspect')) {
				return Promise.resolve({ stdout: 'true', stderr: '', code: 0 })
			}
			return Promise.resolve({ stdout: '', stderr: '', code: 0 })
		})
	})

	describe('provision', () => {
		test('connects as root to the target host', async () => {
			await provisioner.provision('1.2.3.4', {
				user: 'deploy',
				publicKey: 'ssh-ed25519 AAAA...',
				project: 'myapp',
			})

			const connectCall = mockSSH.connect.mock.calls[0][0] as { host: string; user: string }
			expect(connectCall.host).toBe('1.2.3.4')
			expect(connectCall.user).toBe('root')
		})

		test('throws ProvisionError when OS is not Debian/Ubuntu', async () => {
			mockSSH.exec.mockImplementation((host: string, cmd: string) => {
				if (cmd.includes('os-release')) {
					return Promise.resolve({
						stdout: 'ID=fedora\nNAME="Fedora Linux 39"',
						stderr: '',
						code: 0,
					})
				}
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			expect(
				provisioner.provision('1.2.3.4', {
					user: 'deploy',
					publicKey: 'ssh-ed25519 AAAA...',
					project: 'myapp',
				}),
			).rejects.toThrow(ProvisionError)
		})

		test('runs user creation commands', async () => {
			await provisioner.provision('1.2.3.4', {
				user: 'deploy',
				publicKey: 'ssh-ed25519 AAAA...',
				project: 'myapp',
			})

			const allCmds = mockSSH.exec.mock.calls.map((c: unknown[]) => c[1] as string)
			const userCmds = allCmds.filter((cmd) => cmd.includes('useradd') || cmd.includes('usermod'))
			expect(userCmds.length).toBeGreaterThan(0)
		})

		test('runs Docker install commands', async () => {
			await provisioner.provision('1.2.3.4', {
				user: 'deploy',
				publicKey: 'ssh-ed25519 AAAA...',
				project: 'myapp',
			})

			const allCmds = mockSSH.exec.mock.calls.map((c: unknown[]) => c[1] as string)
			const dockerCmds = allCmds.filter((cmd) => cmd.includes('docker'))
			expect(dockerCmds.length).toBeGreaterThan(0)
		})

		test('warns but does not throw when firewall commands fail', async () => {
			mockSSH.exec.mockImplementation((_host: string, cmd: string) => {
				if (cmd.includes('os-release')) {
					return Promise.resolve({ stdout: 'ID=ubuntu\nNAME="Ubuntu 22.04"', stderr: '', code: 0 })
				}
				if (cmd.includes('docker inspect')) {
					return Promise.resolve({ stdout: 'true', stderr: '', code: 0 })
				}
				if (cmd.includes('ufw') || cmd.includes('fail2ban')) {
					return Promise.resolve({ stdout: '', stderr: 'ufw not found', code: 1 })
				}
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			// Should not throw despite firewall failures
			await expect(
				provisioner.provision('1.2.3.4', {
					user: 'deploy',
					publicKey: 'ssh-ed25519 AAAA...',
					project: 'myapp',
				}),
			).resolves.toBeUndefined()
		})

		test('runs setupDirectory commands for the project', async () => {
			await provisioner.provision('1.2.3.4', {
				user: 'deploy',
				publicKey: 'ssh-ed25519 AAAA...',
				project: 'myapp',
			})

			const allCmds = mockSSH.exec.mock.calls.map((c: unknown[]) => c[1] as string)
			const mkdirCmds = allCmds.filter((cmd) => cmd.includes('mkdir') && cmd.includes('shuttle'))
			expect(mkdirCmds.length).toBeGreaterThan(0)
		})
	})

	describe('verify', () => {
		test('returns true when all checks pass', async () => {
			mockSSH.exec.mockImplementation((host: string, cmd: string) => {
				if (cmd.includes('docker info')) {
					return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				}
				if (cmd.includes('docker inspect')) {
					return Promise.resolve({ stdout: 'true', stderr: '', code: 0 })
				}
				if (cmd.includes('test -d')) {
					return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				}
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			const ok = await provisioner.verify('1.2.3.4', 'deploy')
			expect(ok).toBe(true)
		})

		test('returns false when Docker check fails', async () => {
			mockSSH.exec.mockImplementation((host: string, cmd: string) => {
				if (cmd.includes('docker info')) {
					return Promise.resolve({
						stdout: '',
						stderr: 'Cannot connect to the Docker daemon',
						code: 1,
					})
				}
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			const ok = await provisioner.verify('1.2.3.4', 'deploy')
			expect(ok).toBe(false)
		})

		test('returns false when Caddy container is not running', async () => {
			mockSSH.exec.mockImplementation((host: string, cmd: string) => {
				if (cmd.includes('docker info')) {
					return Promise.resolve({ stdout: '', stderr: '', code: 0 })
				}
				if (cmd.includes('docker inspect')) {
					return Promise.resolve({ stdout: 'false', stderr: '', code: 0 })
				}
				return Promise.resolve({ stdout: '', stderr: '', code: 0 })
			})

			const ok = await provisioner.verify('1.2.3.4', 'deploy')
			expect(ok).toBe(false)
		})
	})
})

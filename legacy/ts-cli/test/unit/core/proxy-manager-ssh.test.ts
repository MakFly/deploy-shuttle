// @ts-nocheck — mock.calls types are not compatible with strict TS
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createMockSSH } from '../../helpers/mock-ssh.ts'

const mockSSH = createMockSSH()

mock.module('../../../src/core/ssh-manager.ts', () => ({ ssh: mockSSH }))

import { ProxyManager } from '../../../src/core/proxy-manager.ts'
import { ShuttleError } from '../../../src/utils/errors.ts'

describe('ProxyManager (SSH-dependent)', () => {
	let pm: ProxyManager

	beforeEach(() => {
		pm = new ProxyManager()
		mockSSH.uploadContent.mockReset()
		mockSSH.exec.mockReset()
		mockSSH.uploadContent.mockImplementation(() => Promise.resolve())
		mockSSH.exec.mockImplementation(() => Promise.resolve({ stdout: '', stderr: '', code: 0 }))
	})

	describe('switchUpstream', () => {
		test('calls apply with a snippet containing the domain and new upstream', async () => {
			const applySpy = mock(() => Promise.resolve())
			pm.apply = applySpy

			await pm.switchUpstream('1.2.3.4', 'myapp.example.com', 10001)

			expect(applySpy).toHaveBeenCalledTimes(1)
			const snippet = applySpy.mock.calls[0][1] as string
			expect(snippet).toContain('myapp.example.com')
			expect(snippet).toContain('127.0.0.1:10001')
		})
	})

	describe('apply', () => {
		test('uploads Caddyfile then reloads caddy', async () => {
			await pm.apply('1.2.3.4', 'myapp.example.com {\n  reverse_proxy 127.0.0.1:10001\n}\n')

			expect(mockSSH.uploadContent).toHaveBeenCalledTimes(1)
			const [host, content, path] = mockSSH.uploadContent.mock.calls[0] as [
				string,
				string,
				string,
				number,
			]
			expect(host).toBe('1.2.3.4')
			expect(content).toContain('reverse_proxy')
			expect(path).toBe('/opt/caddy/Caddyfile')

			expect(mockSSH.exec).toHaveBeenCalledTimes(1)
			const reloadCmd = mockSSH.exec.mock.calls[0][1] as string
			expect(reloadCmd).toContain('caddy reload')
		})

		test('throws ShuttleError when caddy reload fails', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'config error', code: 1 }),
			)

			expect(pm.apply('1.2.3.4', 'bad config')).rejects.toThrow(ShuttleError)
		})
	})

	describe('getStatus', () => {
		test('returns trimmed docker inspect output', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({
					stdout: '[{"State":{"Running":true}}]\n',
					stderr: '',
					code: 0,
				}),
			)

			const status = await pm.getStatus('1.2.3.4')
			expect(status).toBe('[{"State":{"Running":true}}]')
		})

		test('throws ShuttleError when docker inspect fails', async () => {
			mockSSH.exec.mockImplementation(() =>
				Promise.resolve({ stdout: '', stderr: 'No such container: caddy', code: 1 }),
			)

			expect(pm.getStatus('1.2.3.4')).rejects.toThrow(ShuttleError)
		})
	})
})

import { describe, expect, test } from 'bun:test'
import {
	createUserCommands,
	firewallCommands,
	hardenSSHCommands,
	installCaddyCommands,
	installDockerCommands,
	setupDirectoryCommands,
} from '../../../src/templates/provision.ts'
import { shellEscape } from '../../../src/utils/shell.ts'

describe('createUserCommands', () => {
	test('returns the correct number of commands', () => {
		const cmds = createUserCommands('deploy', 'ssh-rsa AAAA...')
		expect(cmds).toHaveLength(7)
	})

	test('uses shellEscape on username', () => {
		const username = 'deploy'
		const cmds = createUserCommands(username, 'key')
		const escaped = shellEscape(username)
		expect(cmds.some((c) => c.includes(escaped))).toBe(true)
	})

	test('uses shellEscape on publicKey', () => {
		const publicKey = 'ssh-rsa AAAA test@host'
		const cmds = createUserCommands('deploy', publicKey)
		const escaped = shellEscape(publicKey)
		expect(cmds.some((c) => c.includes(escaped))).toBe(true)
	})

	test('includes useradd command for user creation', () => {
		const cmds = createUserCommands('myuser', 'key')
		expect(cmds.some((c) => c.includes('useradd'))).toBe(true)
	})

	test('includes authorized_keys in one of the commands', () => {
		const cmds = createUserCommands('deploy', 'key')
		expect(cmds.some((c) => c.includes('authorized_keys'))).toBe(true)
	})
})

describe('installDockerCommands', () => {
	test('returns an array', () => {
		const cmds = installDockerCommands()
		expect(Array.isArray(cmds)).toBe(true)
	})

	test('includes docker-ce in the commands', () => {
		const cmds = installDockerCommands()
		expect(cmds.some((c) => c.includes('docker-ce'))).toBe(true)
	})

	test('includes systemctl enable docker', () => {
		const cmds = installDockerCommands()
		expect(cmds.some((c) => c.includes('systemctl enable docker'))).toBe(true)
	})

	test('includes apt-get update', () => {
		const cmds = installDockerCommands()
		expect(cmds.some((c) => c.includes('apt-get update'))).toBe(true)
	})
})

describe('installCaddyCommands', () => {
	test('returns an array', () => {
		const cmds = installCaddyCommands()
		expect(Array.isArray(cmds)).toBe(true)
	})

	test('includes caddy:latest', () => {
		const cmds = installCaddyCommands()
		expect(cmds.some((c) => c.includes('caddy:latest'))).toBe(true)
	})

	test('includes docker pull', () => {
		const cmds = installCaddyCommands()
		expect(cmds.some((c) => c.includes('docker pull'))).toBe(true)
	})
})

describe('firewallCommands', () => {
	test('returns an array', () => {
		const cmds = firewallCommands()
		expect(Array.isArray(cmds)).toBe(true)
	})

	test('includes ufw', () => {
		const cmds = firewallCommands()
		expect(cmds.some((c) => c.includes('ufw'))).toBe(true)
	})

	test('allows SSH port 22', () => {
		const cmds = firewallCommands()
		expect(cmds.some((c) => c.includes('22/tcp'))).toBe(true)
	})

	test('allows HTTP port 80', () => {
		const cmds = firewallCommands()
		expect(cmds.some((c) => c.includes('80/tcp'))).toBe(true)
	})

	test('allows HTTPS port 443', () => {
		const cmds = firewallCommands()
		expect(cmds.some((c) => c.includes('443/tcp'))).toBe(true)
	})
})

describe('hardenSSHCommands', () => {
	test('returns an array', () => {
		const cmds = hardenSSHCommands()
		expect(Array.isArray(cmds)).toBe(true)
	})

	test('includes sshd_config', () => {
		const cmds = hardenSSHCommands()
		expect(cmds.some((c) => c.includes('sshd_config'))).toBe(true)
	})

	test('disables PermitRootLogin', () => {
		const cmds = hardenSSHCommands()
		expect(cmds.some((c) => c.includes('PermitRootLogin'))).toBe(true)
	})

	test('disables PasswordAuthentication', () => {
		const cmds = hardenSSHCommands()
		expect(cmds.some((c) => c.includes('PasswordAuthentication'))).toBe(true)
	})

	test('restarts sshd', () => {
		const cmds = hardenSSHCommands()
		expect(cmds.some((c) => c.includes('systemctl restart sshd'))).toBe(true)
	})
})

describe('setupDirectoryCommands', () => {
	test('returns an array', () => {
		const cmds = setupDirectoryCommands('myproject', 'deploy')
		expect(Array.isArray(cmds)).toBe(true)
	})

	test('uses project name in paths', () => {
		const cmds = setupDirectoryCommands('myproject', 'deploy')
		expect(cmds.some((c) => c.includes('myproject'))).toBe(true)
	})

	test('uses username param in chown command (not hardcoded)', () => {
		const cmds = setupDirectoryCommands('myproject', 'customuser')
		const escaped = shellEscape('customuser')
		expect(cmds.some((c) => c.includes(escaped) && c.includes('chown'))).toBe(true)
	})

	test('uses shellEscape on project name', () => {
		const project = 'my-project'
		const cmds = setupDirectoryCommands(project, 'deploy')
		const escaped = shellEscape(project)
		expect(cmds.some((c) => c.includes(escaped))).toBe(true)
	})

	test('creates /opt/shuttle base path', () => {
		const cmds = setupDirectoryCommands('app', 'deploy')
		expect(cmds.some((c) => c.includes('/opt/shuttle'))).toBe(true)
	})

	test('creates workdir subdirectory matching project name', () => {
		const cmds = setupDirectoryCommands('app', 'deploy')
		expect(cmds.some((c) => c.includes("'app'/'app'") || c.includes('app/app'))).toBe(true)
	})
})

import { describe, expect, it } from 'bun:test'
import { caddyInstalledCheck } from '@/core/readiness/checks/caddy.ts'
import { dockerInstalledCheck } from '@/core/readiness/checks/docker.ts'
import { databasePortPublicCheck, ufwActiveCheck } from '@/core/readiness/checks/firewall.ts'
import { envTrackedByGitCheck, envWorldReadableCheck } from '@/core/readiness/checks/secrets.ts'
import { systemDiskSpaceLowCheck, systemOsSupportedCheck } from '@/core/readiness/checks/system.ts'
import type { CheckContext, ExecAdapter, ExecResult } from '@/core/readiness/types.ts'

class FakeExec implements ExecAdapter {
	constructor(private readonly handler: (command: string) => ExecResult) {}

	async run(command: string): Promise<ExecResult> {
		return this.handler(command)
	}
}

function context(exec: ExecAdapter): CheckContext {
	return {
		target: 'local',
		profile: ['docker'],
		exec,
		cwd: process.cwd(),
	}
}

function ok(stdout = ''): ExecResult {
	return { exitCode: 0, stdout, stderr: '' }
}

function fail(stderr = ''): ExecResult {
	return { exitCode: 1, stdout: '', stderr }
}

describe('readiness checks', () => {
	it('passes supported OS versions', async () => {
		const result = await systemOsSupportedCheck.run(
			context(new FakeExec(() => ok('ID=ubuntu\nVERSION_ID="24.04"\n'))),
		)

		expect(result.status).toBe('passed')
	})

	it('fails unsupported OS versions', async () => {
		const result = await systemOsSupportedCheck.run(
			context(new FakeExec(() => ok('ID=fedora\nVERSION_ID="40"\n'))),
		)

		expect(result.status).toBe('failed')
	})

	it('fails disk usage at 80 percent or above', async () => {
		const result = await systemDiskSpaceLowCheck.run(context(new FakeExec(() => ok('91%\n'))))

		expect(result.status).toBe('failed')
		expect(result.severity).toBe('critical')
	})

	it('passes when Docker is available', async () => {
		const result = await dockerInstalledCheck.run(
			context(new FakeExec(() => ok('/usr/bin/docker\nDocker version 27\n'))),
		)

		expect(result.status).toBe('passed')
	})

	it('fails when UFW is inactive', async () => {
		const result = await ufwActiveCheck.run(context(new FakeExec(() => ok('Status: inactive\n'))))

		expect(result.status).toBe('failed')
	})

	it('detects public sensitive database ports', async () => {
		const result = await databasePortPublicCheck.run(
			context(new FakeExec(() => ok('LISTEN 0 4096 0.0.0.0:5432 0.0.0.0:*\n'))),
		)

		expect(result.status).toBe('failed')
		expect(result.evidence).toEqual({ publicPorts: ['5432'] })
	})

	it('fails world-readable .env files', async () => {
		const exec = new FakeExec((command) => {
			if (command === 'test -f .env') return ok()
			return ok('644\n')
		})

		const result = await envWorldReadableCheck.run(context(exec))

		expect(result.status).toBe('failed')
	})

	it('passes when .env is not tracked by Git', async () => {
		const result = await envTrackedByGitCheck.run(context(new FakeExec(() => fail())))

		expect(result.status).toBe('passed')
	})

	it('fails when Caddy is missing', async () => {
		const result = await caddyInstalledCheck.run(context(new FakeExec(() => fail())))

		expect(result.status).toBe('failed')
	})
})

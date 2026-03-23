import { describe, expect, test } from 'bun:test'
import {
	ConfigError,
	DeployError,
	ProvisionError,
	SSHError,
	SecretsError,
	ShuttleError,
} from '../../../src/utils/errors.ts'

describe('ShuttleError', () => {
	test('has correct name', () => {
		const err = new ShuttleError('message', 'CODE')
		expect(err.name).toBe('ShuttleError')
	})

	test('has correct code', () => {
		const err = new ShuttleError('message', 'MY_CODE')
		expect(err.code).toBe('MY_CODE')
	})

	test('has correct message', () => {
		const err = new ShuttleError('something went wrong', 'ERR')
		expect(err.message).toBe('something went wrong')
	})

	test('is instanceof Error', () => {
		const err = new ShuttleError('msg', 'CODE')
		expect(err instanceof Error).toBe(true)
	})

	describe('wrap()', () => {
		test('wraps a plain Error', () => {
			const cause = new Error('original')
			const wrapped = ShuttleError.wrap(cause, 'context')
			expect(wrapped).toBeInstanceOf(ShuttleError)
			expect(wrapped.message).toBe('context: original')
		})

		test('wraps a string', () => {
			const wrapped = ShuttleError.wrap('raw string')
			expect(wrapped).toBeInstanceOf(ShuttleError)
			expect(wrapped.message).toBe('raw string')
		})

		test('returns existing ShuttleError instance as-is', () => {
			const original = new ShuttleError('original', 'ORIG')
			const wrapped = ShuttleError.wrap(original, 'context')
			expect(wrapped).toBe(original)
		})

		test('uses cause message without prefix when no message given', () => {
			const cause = new Error('bare cause')
			const wrapped = ShuttleError.wrap(cause)
			expect(wrapped.message).toBe('bare cause')
		})
	})
})

describe('ConfigError', () => {
	test('has correct name', () => {
		const err = new ConfigError('bad config')
		expect(err.name).toBe('ConfigError')
	})

	test('has code CONFIG_ERROR', () => {
		const err = new ConfigError('bad config')
		expect(err.code).toBe('CONFIG_ERROR')
	})

	test('is instanceof ShuttleError', () => {
		const err = new ConfigError('bad config')
		expect(err instanceof ShuttleError).toBe(true)
	})

	test('wrap returns existing ConfigError as-is', () => {
		const original = new ConfigError('already config')
		const wrapped = ConfigError.wrap(original)
		expect(wrapped).toBe(original)
	})
})

describe('SSHError', () => {
	test('has correct name', () => {
		const err = new SSHError('connection refused')
		expect(err.name).toBe('SSHError')
	})

	test('has code SSH_ERROR', () => {
		const err = new SSHError('connection refused')
		expect(err.code).toBe('SSH_ERROR')
	})

	test('stores host when provided', () => {
		const err = new SSHError('timeout', '192.168.1.1')
		expect(err.host).toBe('192.168.1.1')
	})

	test('host is undefined when not provided', () => {
		const err = new SSHError('timeout')
		expect(err.host).toBeUndefined()
	})

	test('wrap passes host through', () => {
		const cause = new Error('ECONNREFUSED')
		const wrapped = SSHError.wrap(cause, 'failed to connect', '10.0.0.1')
		expect(wrapped.host).toBe('10.0.0.1')
	})
})

describe('DeployError', () => {
	test('has correct name', () => {
		const err = new DeployError('deploy failed')
		expect(err.name).toBe('DeployError')
	})

	test('has code DEPLOY_ERROR', () => {
		const err = new DeployError('deploy failed')
		expect(err.code).toBe('DEPLOY_ERROR')
	})

	test('wrap returns existing DeployError as-is', () => {
		const original = new DeployError('already deploy')
		const wrapped = DeployError.wrap(original)
		expect(wrapped).toBe(original)
	})
})

describe('SecretsError', () => {
	test('has correct name', () => {
		const err = new SecretsError('decryption failed')
		expect(err.name).toBe('SecretsError')
	})

	test('has code SECRETS_ERROR', () => {
		const err = new SecretsError('decryption failed')
		expect(err.code).toBe('SECRETS_ERROR')
	})
})

describe('ProvisionError', () => {
	test('has correct name', () => {
		const err = new ProvisionError('provision failed')
		expect(err.name).toBe('ProvisionError')
	})

	test('has code PROVISION_ERROR', () => {
		const err = new ProvisionError('provision failed')
		expect(err.code).toBe('PROVISION_ERROR')
	})

	test('wrap wraps plain Error', () => {
		const cause = new Error('apt failed')
		const wrapped = ProvisionError.wrap(cause, 'install step')
		expect(wrapped).toBeInstanceOf(ProvisionError)
		expect(wrapped.message).toBe('install step: apt failed')
	})
})

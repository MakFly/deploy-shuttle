import { describe, expect, test } from 'bun:test'
import {
	buildDefaults,
	defaults,
	deployDefaults,
	healthcheckDefaults,
	proxyDefaults,
	secretsDefaults,
} from '../../../src/config/defaults.ts'

describe('buildDefaults', () => {
	test('has dockerfile field', () => {
		expect(buildDefaults).toHaveProperty('dockerfile')
		expect(buildDefaults.dockerfile).toBe('Dockerfile')
	})

	test('has context field', () => {
		expect(buildDefaults).toHaveProperty('context')
		expect(buildDefaults.context).toBe('.')
	})

	test('has platform field', () => {
		expect(buildDefaults).toHaveProperty('platform')
		expect(buildDefaults.platform).toBeUndefined()
	})
})

describe('deployDefaults', () => {
	test('has strategy field', () => {
		expect(deployDefaults).toHaveProperty('strategy')
		expect(deployDefaults.strategy).toBe('blue-green')
	})

	test('has timeout field', () => {
		expect(deployDefaults).toHaveProperty('timeout')
		expect(typeof deployDefaults.timeout).toBe('number')
	})

	test('has retain field', () => {
		expect(deployDefaults).toHaveProperty('retain')
		expect(typeof deployDefaults.retain).toBe('number')
	})

	test('has auto_rollback field', () => {
		expect(deployDefaults).toHaveProperty('auto_rollback')
		expect(typeof deployDefaults.auto_rollback).toBe('boolean')
	})

	test('has blue_green field', () => {
		expect(deployDefaults).toHaveProperty('blue_green')
		expect(deployDefaults.blue_green).toHaveProperty('drain_timeout')
		expect(deployDefaults.blue_green).toHaveProperty('readiness_delay')
	})

	test('has hooks field with arrays', () => {
		expect(deployDefaults).toHaveProperty('hooks')
		expect(Array.isArray(deployDefaults.hooks.pre_deploy)).toBe(true)
		expect(Array.isArray(deployDefaults.hooks.post_deploy)).toBe(true)
	})
})

describe('healthcheckDefaults', () => {
	test('has type field', () => {
		expect(healthcheckDefaults).toHaveProperty('type')
		expect(healthcheckDefaults.type).toBe('http')
	})

	test('has path field', () => {
		expect(healthcheckDefaults).toHaveProperty('path')
		expect(typeof healthcheckDefaults.path).toBe('string')
	})

	test('has interval field', () => {
		expect(healthcheckDefaults).toHaveProperty('interval')
		expect(typeof healthcheckDefaults.interval).toBe('number')
	})

	test('has timeout field', () => {
		expect(healthcheckDefaults).toHaveProperty('timeout')
		expect(typeof healthcheckDefaults.timeout).toBe('number')
	})

	test('has retries field', () => {
		expect(healthcheckDefaults).toHaveProperty('retries')
		expect(typeof healthcheckDefaults.retries).toBe('number')
	})
})

describe('proxyDefaults', () => {
	test('has ssl field', () => {
		expect(proxyDefaults).toHaveProperty('ssl')
		expect(proxyDefaults.ssl).toBeDefined()
	})

	test('has headers field', () => {
		expect(proxyDefaults).toHaveProperty('headers')
		expect(typeof proxyDefaults.headers).toBe('object')
	})
})

describe('secretsDefaults', () => {
	test("has driver field set to 'aes'", () => {
		expect(secretsDefaults).toHaveProperty('driver')
		expect(secretsDefaults.driver).toBe('aes')
	})
})

describe('defaults (combined object)', () => {
	test('has build section', () => {
		expect(defaults).toHaveProperty('build')
		expect(defaults.build).toBe(buildDefaults)
	})

	test('has deploy section', () => {
		expect(defaults).toHaveProperty('deploy')
		expect(defaults.deploy).toBe(deployDefaults)
	})

	test('has healthcheck section', () => {
		expect(defaults).toHaveProperty('healthcheck')
		expect(defaults.healthcheck).toBe(healthcheckDefaults)
	})

	test('has proxy section', () => {
		expect(defaults).toHaveProperty('proxy')
		expect(defaults.proxy).toBe(proxyDefaults)
	})

	test('has secrets section', () => {
		expect(defaults).toHaveProperty('secrets')
		expect(defaults.secrets).toBe(secretsDefaults)
	})

	test('has notifications section', () => {
		expect(defaults).toHaveProperty('notifications')
		expect(defaults.notifications.webhooks).toEqual([])
	})
})

// @ts-nocheck — consola mock types are incompatible with strict TS
import { beforeEach, describe, expect, spyOn, test } from 'bun:test'
import { consola } from 'consola'
import { logger } from '../../../src/utils/logger.ts'

beforeEach(() => {
	// Reset verbose state before each test
	logger.setVerbose(false)
})

describe('logger API surface', () => {
	test('has info method', () => {
		expect(typeof logger.info).toBe('function')
	})

	test('has success method', () => {
		expect(typeof logger.success).toBe('function')
	})

	test('has warn method', () => {
		expect(typeof logger.warn).toBe('function')
	})

	test('has error method', () => {
		expect(typeof logger.error).toBe('function')
	})

	test('has debug method', () => {
		expect(typeof logger.debug).toBe('function')
	})

	test('has fatal method', () => {
		expect(typeof logger.fatal).toBe('function')
	})

	test('has box method', () => {
		expect(typeof logger.box).toBe('function')
	})

	test('has start method', () => {
		expect(typeof logger.start).toBe('function')
	})

	test('has step method', () => {
		expect(typeof logger.step).toBe('function')
	})
})

describe('setVerbose / verbose getter', () => {
	test('verbose is false by default', () => {
		expect(logger.verbose).toBe(false)
	})

	test('setVerbose(true) makes verbose return true', () => {
		logger.setVerbose(true)
		expect(logger.verbose).toBe(true)
	})

	test('setVerbose(false) makes verbose return false', () => {
		logger.setVerbose(true)
		logger.setVerbose(false)
		expect(logger.verbose).toBe(false)
	})
})

describe('debug suppression', () => {
	test('debug does not call consola.debug when verbose is false', () => {
		const spy = spyOn(consola, 'debug').mockImplementation(() => {})
		logger.setVerbose(false)
		logger.debug('hidden message')
		expect(spy).not.toHaveBeenCalled()
		spy.mockRestore()
	})

	test('debug calls consola.debug when verbose is true', () => {
		const spy = spyOn(consola, 'debug').mockImplementation(() => {})
		logger.setVerbose(true)
		logger.debug('visible message')
		expect(spy).toHaveBeenCalledWith('visible message')
		spy.mockRestore()
	})
})

describe('step formatting', () => {
	test('step formats as [n/total] message', () => {
		const spy = spyOn(consola, 'info').mockImplementation(() => {})
		logger.step(3, 10, 'Starting container')
		expect(spy).toHaveBeenCalledWith('[3/10] Starting container')
		spy.mockRestore()
	})

	test('step formats correctly with different numbers', () => {
		const spy = spyOn(consola, 'info').mockImplementation(() => {})
		logger.step(1, 5, 'First step')
		expect(spy).toHaveBeenCalledWith('[1/5] First step')
		spy.mockRestore()
	})
})

describe('delegating to consola', () => {
	test('info delegates to consola.info', () => {
		const spy = spyOn(consola, 'info').mockImplementation(() => {})
		logger.info('test message')
		expect(spy).toHaveBeenCalledWith('test message')
		spy.mockRestore()
	})

	test('warn delegates to consola.warn', () => {
		const spy = spyOn(consola, 'warn').mockImplementation(() => {})
		logger.warn('watch out')
		expect(spy).toHaveBeenCalledWith('watch out')
		spy.mockRestore()
	})

	test('error delegates to consola.error', () => {
		const spy = spyOn(consola, 'error').mockImplementation(() => {})
		logger.error('something broke')
		expect(spy).toHaveBeenCalledWith('something broke')
		spy.mockRestore()
	})
})

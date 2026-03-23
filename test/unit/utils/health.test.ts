// @ts-nocheck — globalThis.fetch mock types are incompatible with strict TS
import { afterEach, beforeEach, describe, expect, mock, test } from 'bun:test'
import { checkHttp, checkTcp, waitForHealth } from '../../../src/utils/health.ts'

// ---------------------------------------------------------------------------
// checkHttp
// ---------------------------------------------------------------------------

describe('checkHttp', () => {
	let originalFetch: typeof fetch

	beforeEach(() => {
		originalFetch = globalThis.fetch
	})

	afterEach(() => {
		globalThis.fetch = originalFetch
	})

	test('returns true on 200 response', async () => {
		globalThis.fetch = mock(async () => new Response(null, { status: 200 }))
		const result = await checkHttp('http://localhost:3000/health', 1000)
		expect(result).toBe(true)
	})

	test('returns true on 201 response', async () => {
		globalThis.fetch = mock(async () => new Response(null, { status: 201 }))
		const result = await checkHttp('http://localhost:3000/health', 1000)
		expect(result).toBe(true)
	})

	test('returns false on 404 response', async () => {
		globalThis.fetch = mock(async () => new Response(null, { status: 404 }))
		const result = await checkHttp('http://localhost:3000/health', 1000)
		expect(result).toBe(false)
	})

	test('returns false on 500 response', async () => {
		globalThis.fetch = mock(async () => new Response(null, { status: 500 }))
		const result = await checkHttp('http://localhost:3000/health', 1000)
		expect(result).toBe(false)
	})

	test('returns false on network error', async () => {
		globalThis.fetch = mock(async () => {
			throw new Error('ECONNREFUSED')
		})
		const result = await checkHttp('http://localhost:9999/health', 1000)
		expect(result).toBe(false)
	})

	test('returns false when request is aborted due to timeout', async () => {
		globalThis.fetch = mock(
			() =>
				new Promise<Response>((_, reject) =>
					setTimeout(() => reject(new DOMException('AbortError', 'AbortError')), 200),
				),
		)
		const result = await checkHttp('http://localhost:3000/health', 50)
		expect(result).toBe(false)
	})
})

// ---------------------------------------------------------------------------
// checkTcp
// ---------------------------------------------------------------------------

describe('checkTcp', () => {
	test('returns false when connection is refused', async () => {
		// Port 1 is almost always closed/refused
		const result = await checkTcp('127.0.0.1', 1, 500)
		expect(result).toBe(false)
	})

	test('returns false when host is unreachable within timeout', async () => {
		// 192.0.2.1 is a TEST-NET address (RFC 5737) — should not be reachable
		const result = await checkTcp('192.0.2.1', 80, 200)
		expect(result).toBe(false)
	})
})

// ---------------------------------------------------------------------------
// waitForHealth
// ---------------------------------------------------------------------------

describe('waitForHealth', () => {
	test('returns true on first successful check', async () => {
		const check = mock(async () => true)
		const result = await waitForHealth(check, { interval: 10, timeout: 1000, retries: 3 })
		expect(result).toBe(true)
		expect(check).toHaveBeenCalledTimes(1)
	})

	test('returns true after initial failures then success', async () => {
		let callCount = 0
		const check = mock(async () => {
			callCount++
			return callCount >= 3
		})
		const result = await waitForHealth(check, { interval: 10, timeout: 5000, retries: 5 })
		expect(result).toBe(true)
	})

	test('returns false after all retries fail', async () => {
		const check = mock(async () => false)
		const result = await waitForHealth(check, { interval: 10, timeout: 5000, retries: 3 })
		expect(result).toBe(false)
		expect(check).toHaveBeenCalledTimes(3)
	})

	test('retries the correct number of times', async () => {
		const check = mock(async () => false)
		await waitForHealth(check, { interval: 10, timeout: 5000, retries: 4 })
		expect(check).toHaveBeenCalledTimes(4)
	})

	test('returns false when timeout is exceeded before retries', async () => {
		let callCount = 0
		const check = mock(async () => {
			callCount++
			// Simulate slow check — but we can't actually sleep here without
			// affecting overall test time, so we rely on the deadline check
			return false
		})
		const result = await waitForHealth(check, { interval: 1000, timeout: 0, retries: 10 })
		// deadline is already passed on first iteration
		expect(result).toBe(false)
	})

	test('returns false immediately when retries is 0', async () => {
		const check = mock(async () => true)
		const result = await waitForHealth(check, { interval: 10, timeout: 5000, retries: 0 })
		expect(result).toBe(false)
		expect(check).toHaveBeenCalledTimes(0)
	})
})

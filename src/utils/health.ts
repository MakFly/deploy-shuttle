import { createConnection } from 'node:net'
import type { SSHManager } from '../core/ssh-manager.ts'

/**
 * Performs an HTTP GET request and returns true if the response status is 2xx.
 */
export async function checkHttp(url: string, timeout: number): Promise<boolean> {
	const controller = new AbortController()
	const timer = setTimeout(() => controller.abort(), timeout)

	try {
		const response = await fetch(url, { signal: controller.signal })
		return response.status >= 200 && response.status < 300
	} catch {
		return false
	} finally {
		clearTimeout(timer)
	}
}

/**
 * Attempts a TCP connection to host:port and returns true if it succeeds within `timeout` ms.
 */
export function checkTcp(host: string, port: number, timeout: number): Promise<boolean> {
	return new Promise((resolve) => {
		const socket = createConnection({ host, port })

		const timer = setTimeout(() => {
			socket.destroy()
			resolve(false)
		}, timeout)

		socket.once('connect', () => {
			clearTimeout(timer)
			socket.destroy()
			resolve(true)
		})

		socket.once('error', () => {
			clearTimeout(timer)
			resolve(false)
		})
	})
}

/**
 * Runs an HTTP check on a remote host via SSH, hitting 127.0.0.1:<port><path>
 * from within the remote machine. Returns true if curl exits with code 0.
 */
export async function checkRemoteHttp(
	ssh: SSHManager,
	host: string,
	port: number,
	path: string,
	timeout: number,
): Promise<boolean> {
	try {
		const { code } = await ssh.exec(
			host,
			`curl -sf -o /dev/null -m ${Math.ceil(timeout / 1000)} http://127.0.0.1:${port}${path}`,
		)
		return code === 0
	} catch {
		return false
	}
}

/**
 * Checks TCP reachability on a remote host via SSH, connecting to
 * 127.0.0.1:<port> from within the remote machine using /dev/tcp.
 * Returns true if the connection succeeds within `timeout` ms.
 */
export async function checkRemoteTcp(
	ssh: SSHManager,
	host: string,
	port: number,
	timeout: number,
): Promise<boolean> {
	try {
		const { code } = await ssh.exec(
			host,
			`timeout ${Math.ceil(timeout / 1000)} bash -c 'echo > /dev/tcp/127.0.0.1/${port}' 2>/dev/null`,
		)
		return code === 0
	} catch {
		return false
	}
}

export interface WaitForHealthOptions {
	interval: number
	timeout: number
	retries: number
}

/**
 * Retries `check` until it returns true, `retries` is exhausted, or `timeout` ms elapses.
 * Returns true on the first successful check, false otherwise.
 */
export async function waitForHealth(
	check: () => Promise<boolean>,
	options: WaitForHealthOptions,
): Promise<boolean> {
	const { interval, timeout, retries } = options
	const deadline = Date.now() + timeout

	for (let attempt = 0; attempt < retries; attempt++) {
		if (Date.now() >= deadline) return false

		const ok = await check()
		if (ok) return true

		if (attempt < retries - 1) {
			const remaining = deadline - Date.now()
			if (remaining <= 0) return false
			await new Promise<void>((resolve) => setTimeout(resolve, Math.min(interval, remaining)))
		}
	}

	return false
}

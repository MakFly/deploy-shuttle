import { mock } from 'bun:test'

export function createMockSSH() {
	return {
		connect: mock(() => Promise.resolve({})),
		exec: mock(() => Promise.resolve({ stdout: '', stderr: '', code: 0 })),
		execStream: mock(() => Promise.resolve({ on: mock(() => {}) })),
		disconnect: mock(() => {}),
		upload: mock(() => Promise.resolve()),
		uploadContent: mock(() => Promise.resolve()),
		shell: mock(() => Promise.resolve()),
		pipe: mock(() => Promise.resolve('')),
		connections: new Map(),
	}
}

import { mock } from 'bun:test'

export function createMockDocker() {
	return {
		build: mock(() => Promise.resolve()),
		save: mock(() => ({})),
		loadRemote: mock(() => Promise.resolve()),
		transfer: mock(() => Promise.resolve()),
		run: mock(() => Promise.resolve()),
		stop: mock(() => Promise.resolve()),
		remove: mock(() => Promise.resolve()),
		inspect: mock(() => Promise.resolve(null)),
		logs: mock(() => Promise.resolve({ on: mock(() => {}) })),
		exec: mock(() => Promise.resolve({ stdout: '', code: 0 })),
		listContainers: mock(() => Promise.resolve([])),
		listImages: mock(() => Promise.resolve([])),
		prune: mock(() => Promise.resolve()),
		tag: mock(() => Promise.resolve()),
		removeImages: mock(() => Promise.resolve()),
		pull: mock(() => Promise.resolve()),
	}
}

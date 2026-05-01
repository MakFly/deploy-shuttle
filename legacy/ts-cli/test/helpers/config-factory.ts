import type { ShuttleConfig } from '../../src/config/schema.ts'

export function createConfig(overrides: Partial<ShuttleConfig> = {}): ShuttleConfig {
	return {
		app: 'myapp',
		domain: 'myapp.example.com',
		servers: { default: { hosts: ['1.2.3.4'], user: 'deploy' } },
		deploy: {
			strategy: 'blue-green',
			timeout: 120,
			retain: 5,
			auto_rollback: true,
			blue_green: { drain_timeout: 30, readiness_delay: 0 },
			hooks: { pre_deploy: [], post_deploy: [] },
		},
		...overrides,
	}
}

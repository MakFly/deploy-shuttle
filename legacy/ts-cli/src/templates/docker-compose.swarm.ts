export interface SwarmComposeService {
	name: string
	image: string
	port?: number
	command?: string
	envFile?: string
	replicas: number
	updateParallelism: number
	updateDelay: string
	updateOrder: 'start-first' | 'stop-first'
	monitor: string
}

export interface SwarmComposeOptions {
	stackName: string
	services: SwarmComposeService[]
}

export function generateSwarmCompose(options: SwarmComposeOptions): string {
	const { services } = options
	let yml = 'version: "3.8"\n\nservices:\n'

	for (const svc of services) {
		yml += `  ${svc.name}:\n`
		yml += `    image: ${svc.image}\n`

		if (svc.command) {
			yml += `    command: ${svc.command}\n`
		}

		if (svc.port) {
			yml += '    ports:\n'
			yml += `      - "${svc.port}:${svc.port}"\n`
		}

		if (svc.envFile) {
			yml += '    env_file:\n'
			yml += `      - ${svc.envFile}\n`
		}

		yml += '    deploy:\n'
		yml += `      replicas: ${svc.replicas}\n`
		yml += '      update_config:\n'
		yml += `        parallelism: ${svc.updateParallelism}\n`
		yml += `        delay: ${svc.updateDelay}\n`
		yml += `        order: ${svc.updateOrder}\n`
		yml += '        failure_action: rollback\n'
		yml += `        monitor: ${svc.monitor}\n`
		yml += '      rollback_config:\n'
		yml += '        parallelism: 1\n'
		yml += '        order: stop-first\n'
		yml += '      restart_policy:\n'
		yml += '        condition: on-failure\n'
		yml += '        delay: 5s\n'
		yml += '        max_attempts: 3\n'
		yml += '\n'
	}

	return yml
}

import { AccessoryManager } from './core/accessory-manager.ts'
import { DeployManager } from './core/deploy-manager.ts'
import { DestroyManager } from './core/destroy-manager.ts'
import { DockerManager } from './core/docker-manager.ts'
import { NotificationsManager } from './core/notifications-manager.ts'
import { ProvisionManager } from './core/provision-manager.ts'
import { ProxyManager } from './core/proxy-manager.ts'
import { RollbackManager } from './core/rollback-manager.ts'
import { type RuntimeManager, runtime } from './core/runtime-manager.ts'
import { SecretsManager } from './core/secrets-manager.ts'
import { SSHManager } from './core/ssh-manager.ts'

export interface ShuttleContainer {
	ssh: SSHManager
	docker: DockerManager
	deploy: DeployManager
	runtime: RuntimeManager
	rollback: RollbackManager
	accessories: AccessoryManager
	provisioner: ProvisionManager
	destroyer: DestroyManager
	proxy: ProxyManager
	secrets: SecretsManager
	notifications: NotificationsManager
}

/**
 * Creates a fully wired container with all managers connected via DI.
 * Accepts optional overrides for testing or custom configurations.
 *
 * Note: RuntimeManager uses its own module-level ssh singleton internally.
 * It will be refactored to accept an SSHManager in a future phase.
 */
export function createContainer(overrides?: Partial<ShuttleContainer>): ShuttleContainer {
	const ssh = overrides?.ssh ?? new SSHManager()
	const docker = overrides?.docker ?? new DockerManager(ssh)
	const proxy = overrides?.proxy ?? new ProxyManager(ssh)
	const runtimeManager = overrides?.runtime ?? runtime
	const secrets = overrides?.secrets ?? new SecretsManager(ssh)
	const notifications = overrides?.notifications ?? new NotificationsManager()
	const accessories = overrides?.accessories ?? new AccessoryManager(docker)
	const deploy =
		overrides?.deploy ?? new DeployManager(ssh, docker, proxy, runtimeManager, secrets, accessories)
	const rollback = overrides?.rollback ?? new RollbackManager(docker, proxy, runtimeManager, deploy)
	const provisioner = overrides?.provisioner ?? new ProvisionManager(ssh)
	const destroyer = overrides?.destroyer ?? new DestroyManager(docker, proxy, runtimeManager, ssh)

	return {
		ssh,
		docker,
		deploy,
		runtime: runtimeManager,
		rollback,
		accessories,
		provisioner,
		destroyer,
		proxy,
		secrets,
		notifications,
	}
}

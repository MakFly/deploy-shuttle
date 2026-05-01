import { caddyInstalledCheck } from './caddy.ts'
import { dockerInstalledCheck } from './docker.ts'
import { databasePortPublicCheck, ufwActiveCheck } from './firewall.ts'
import { envTrackedByGitCheck, envWorldReadableCheck } from './secrets.ts'
import { systemDiskSpaceLowCheck, systemOsSupportedCheck } from './system.ts'

export const defaultChecks = [
	systemOsSupportedCheck,
	systemDiskSpaceLowCheck,
	dockerInstalledCheck,
	ufwActiveCheck,
	databasePortPublicCheck,
	envWorldReadableCheck,
	envTrackedByGitCheck,
	caddyInstalledCheck,
]

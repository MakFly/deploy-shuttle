import { ShuttleError } from '../utils/errors.ts'
import type { LicenseRole } from './validate.ts'
import { loadLicense } from './validate.ts'

const ROLE_HIERARCHY: Record<LicenseRole, number> = {
	viewer: 0,
	deployer: 1,
	admin: 2,
}

/**
 * Assert that the current environment holds a valid, non-expired license that
 * includes `feature`. Throws a `ShuttleError` otherwise so the CLI can surface
 * a human-readable message without an ugly stack trace.
 *
 * Usage:
 *   requirePremium('traefik')   // before instantiating TraefikProxy
 *   requirePremium('doppler')   // before instantiating DopplerSecrets
 */
export function requirePremium(feature: string): void {
	const license = loadLicense()

	if (!license) {
		throw new ShuttleError(
			`"${feature}" is a Pro feature. Get a license at https://shuttle.dev/pricing`,
			'LICENSE_REQUIRED',
		)
	}

	if (license.expiresAt < new Date()) {
		throw new ShuttleError(
			'Your Shuttle Pro license has expired. Renew at https://shuttle.dev/pricing',
			'LICENSE_EXPIRED',
		)
	}

	if (!license.features.includes(feature)) {
		throw new ShuttleError(
			`"${feature}" is not included in your ${license.plan} license. Upgrade at https://shuttle.dev/pricing`,
			'LICENSE_FEATURE_MISSING',
		)
	}
}

/**
 * Assert that the current license holder has at least the specified role.
 * Role hierarchy: viewer < deployer < admin.
 *
 * When no role claim is present in the JWT (individual Pro licenses),
 * the user is treated as admin by default.
 */
export function requireRole(minRole: LicenseRole): void {
	const license = loadLicense()

	if (!license) {
		throw new ShuttleError(
			'A team license is required for role-based access. Get one at https://shuttle.dev/pricing',
			'LICENSE_REQUIRED',
		)
	}

	if (license.expiresAt < new Date()) {
		throw new ShuttleError(
			'Your Shuttle license has expired. Renew at https://shuttle.dev/pricing',
			'LICENSE_EXPIRED',
		)
	}

	// Individual licenses (no role claim) are treated as admin
	const userRole: LicenseRole = license.role ?? 'admin'
	const userLevel = ROLE_HIERARCHY[userRole]
	const requiredLevel = ROLE_HIERARCHY[minRole]

	if (userLevel < requiredLevel) {
		throw new ShuttleError(
			`This action requires "${minRole}" role. Your role is "${userRole}". Contact your org admin.`,
			'LICENSE_ROLE_INSUFFICIENT',
		)
	}
}

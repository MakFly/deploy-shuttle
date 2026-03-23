import { ShuttleError } from '../utils/errors.ts'
import { loadLicense } from './validate.ts'

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
			`Your Shuttle Pro license has expired. Renew at https://shuttle.dev/pricing`,
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

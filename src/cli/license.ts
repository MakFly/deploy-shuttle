import { defineCommand } from 'citty'
import { LICENSE_FILE, loadLicense, removeLicense, saveLicense, validateLicense } from '../license/validate.ts'
import { logger } from '../utils/logger.ts'

// ---------------------------------------------------------------------------
// shuttle license activate <key>
// ---------------------------------------------------------------------------

const activateCommand = defineCommand({
	meta: {
		name: 'activate',
		description: 'Activate a Shuttle Pro license',
	},
	args: {
		key: {
			type: 'positional',
			description: 'License key (JWT token)',
			required: true,
		},
	},
	async run({ args }) {
		const info = validateLicense(args.key)

		if (!info) {
			logger.error('Invalid license key. Please verify the key and try again.')
			process.exit(1)
		}

		if (info.expiresAt < new Date()) {
			logger.error(`This license expired on ${info.expiresAt.toISOString().slice(0, 10)}.`)
			logger.error('Renew at https://shuttle.dev/pricing')
			process.exit(1)
		}

		saveLicense(args.key)

		logger.success(`License activated for ${info.email} (plan: ${info.plan})`)
		logger.info(`Expires:  ${info.expiresAt.toISOString().slice(0, 10)}`)
		logger.info(`Features: ${info.features.join(', ') || '(none)'}`)
		logger.info(`Saved to  ${LICENSE_FILE}`)
	},
})

// ---------------------------------------------------------------------------
// shuttle license status
// ---------------------------------------------------------------------------

const statusCommand = defineCommand({
	meta: {
		name: 'status',
		description: 'Show the current license status',
	},
	async run() {
		const info = loadLicense()

		if (!info) {
			logger.info('No active license found. OSS features only.')
			logger.info('Get Shuttle Pro at https://shuttle.dev/pricing')
			return
		}

		const expired = info.expiresAt < new Date()

		logger.info(`Plan:     ${info.plan}${expired ? ' (EXPIRED)' : ''}`)
		logger.info(`Email:    ${info.email}`)
		logger.info(`Expires:  ${info.expiresAt.toISOString().slice(0, 10)}`)
		logger.info(`Features: ${info.features.join(', ') || '(none)'}`)

		if (expired) {
			logger.warn('Your license has expired. Renew at https://shuttle.dev/pricing')
		}
	},
})

// ---------------------------------------------------------------------------
// shuttle license deactivate
// ---------------------------------------------------------------------------

const deactivateCommand = defineCommand({
	meta: {
		name: 'deactivate',
		description: 'Remove the license from this machine',
	},
	async run() {
		removeLicense()
		logger.success('License removed. Running in OSS mode.')
	},
})

// ---------------------------------------------------------------------------
// shuttle license (root)
// ---------------------------------------------------------------------------

export default defineCommand({
	meta: {
		name: 'license',
		description: 'Manage your Shuttle Pro license',
	},
	subCommands: {
		activate: activateCommand,
		status: statusCommand,
		deactivate: deactivateCommand,
	},
})

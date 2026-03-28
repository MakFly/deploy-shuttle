import type { ShuttleConfig } from '../config/schema.ts'
import { resolveNotifications } from '../providers/resolver.ts'
import type { ShuttleEvent } from '../providers/types.ts'
import { logger } from '../utils/logger.ts'

export class NotificationsManager {
	async notify(
		config: ShuttleConfig,
		event: ShuttleEvent,
		payload: Record<string, unknown>,
	): Promise<void> {
		const providers = resolveNotifications(config)

		for (const provider of providers) {
			try {
				await provider.notify(config, event, payload)
			} catch (err) {
				logger.warn(`Notification failed: ${err instanceof Error ? err.message : String(err)}`)
			}
		}
	}
}

export type { ShuttleEvent } from '../providers/types.ts'
export const notifications = new NotificationsManager()

import type { ShuttleConfig } from '../config/schema.ts'
import { logger } from '../utils/logger.ts'

export type ShuttleEvent =
	| 'deploy_succeeded'
	| 'deploy_failed'
	| 'rollback_succeeded'
	| 'rollback_failed'

export class NotificationsManager {
	async notify(
		config: ShuttleConfig,
		event: ShuttleEvent,
		payload: Record<string, unknown>,
	): Promise<void> {
		for (const url of config.notifications?.webhooks ?? []) {
			try {
				const response = await fetch(url, {
					method: 'POST',
					headers: {
						'content-type': 'application/json',
					},
					body: JSON.stringify({
						event,
						app: config.app,
						timestamp: new Date().toISOString(),
						...payload,
					}),
				})

				if (!response.ok) {
					logger.warn(`Notification webhook ${url} returned ${response.status}`)
				}
			} catch (err) {
				logger.warn(
					`Failed to notify webhook ${url}: ${err instanceof Error ? err.message : String(err)}`,
				)
			}
		}
	}
}

export const notifications = new NotificationsManager()

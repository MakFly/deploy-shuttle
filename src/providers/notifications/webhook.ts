import type { ShuttleConfig } from '../../config/schema.ts'
import { logger } from '../../utils/logger.ts'
import type { NotificationsProvider, ShuttleEvent } from '../types.ts'

// ---------------------------------------------------------------------------
// WebhookNotificationsProvider
// ---------------------------------------------------------------------------

export class WebhookNotificationsProvider implements NotificationsProvider {
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

export function createWebhookNotificationsProvider(): WebhookNotificationsProvider {
	return new WebhookNotificationsProvider()
}

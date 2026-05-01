import type { ShuttleConfig } from '../../config/schema.ts'
import { logger } from '../../utils/logger.ts'
import type { NotificationsProvider, ShuttleEvent } from '../types.ts'

export class SlackNotificationsProvider implements NotificationsProvider {
	constructor(private readonly webhookUrl: string) {}

	async notify(
		config: ShuttleConfig,
		event: ShuttleEvent,
		payload: Record<string, unknown>,
	): Promise<void> {
		const emoji = event.includes('succeeded') ? ':white_check_mark:' : ':x:'
		const fields = Object.entries(payload)
			.slice(0, 10)
			.map(([k, v]) => ({
				type: 'mrkdwn' as const,
				text: `*${k}:*\n${String(v)}`,
			}))

		const blocks: Record<string, unknown>[] = [
			{
				type: 'section',
				text: { type: 'mrkdwn', text: `${emoji} *${config.app}* — \`${event}\`` },
			},
		]
		if (fields.length > 0) {
			blocks.push({ type: 'section', fields })
		}
		blocks.push({
			type: 'context',
			elements: [{ type: 'mrkdwn', text: `Shuttle • ${new Date().toISOString()}` }],
		})

		const response = await fetch(this.webhookUrl, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ blocks }),
		})
		if (!response.ok) {
			logger.warn(`Slack notification returned ${response.status}`)
		}
	}
}

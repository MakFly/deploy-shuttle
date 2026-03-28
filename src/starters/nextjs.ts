import type { StarterFile } from './base.ts'

export interface NextjsStarterOptions {
	database: 'postgres' | 'none'
}

export function getNextjsFiles(options: NextjsStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateNextjsShuttleYml(options) },
		{ path: 'Dockerfile', content: nextjsDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateNextjsShuttleYml(options: NextjsStarterOptions): string {
	const services: string[] = [
		`  web:
    port: 3000
    command: ""
    healthcheck:
      type: http
      path: /`,
	]

	const accessories: string[] = []

	if (options.database === 'postgres') {
		accessories.push(`  db:
    preset: postgres`)
	}

	let yml = `app: __APP_NAME__
domain: __DOMAIN__

server:
  host: __HOST__
  user: __USER__

build:
  dockerfile: Dockerfile
  context: .

services:
${services.join('\n\n')}
`

	if (accessories.length > 0) {
		yml += `
accessories:
${accessories.join('\n\n')}
`
	}

	yml += `
deploy:
  strategy: blue-green
`

	return yml
}

function nextjsDockerfile(): string {
	return `FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS production
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
EXPOSE 3000
CMD ["node", "server.js"]
`
}

function dockerignore(): string {
	return `.git
.env
.shuttle
node_modules
.next
docker-compose*.yml
`
}

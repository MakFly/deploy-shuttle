import type { StarterFile } from './base.ts'

export interface NuxtStarterOptions {
	database: 'postgres' | 'none'
}

export function getNuxtFiles(options: NuxtStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateNuxtShuttleYml(options) },
		{ path: 'Dockerfile', content: nuxtDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateNuxtShuttleYml(options: NuxtStarterOptions): string {
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

function nuxtDockerfile(): string {
	return `FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine AS production
WORKDIR /app
COPY --from=builder /app/.output ./.output
ENV NODE_ENV=production
ENV NITRO_PORT=3000
ENV NITRO_HOST=0.0.0.0
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
`
}

function dockerignore(): string {
	return `.git
.env
.shuttle
node_modules
.nuxt
.output
docker-compose*.yml
`
}

import type { StarterFile } from './base.ts'

export interface HonoStarterOptions {
	database: 'postgres' | 'none'
	redis: boolean
}

export function getHonoFiles(options: HonoStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateHonoShuttleYml(options) },
		{ path: 'Dockerfile', content: honoDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateHonoShuttleYml(options: HonoStarterOptions): string {
	const services: string[] = [
		`  web:
    port: 3000
    command: ""
    healthcheck:
      type: http
      path: /health`,
	]

	const accessories: string[] = []

	if (options.database === 'postgres') {
		accessories.push(`  db:
    preset: postgres`)
	}

	if (options.redis) {
		accessories.push(`  cache:
    preset: redis`)
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

function honoDockerfile(): string {
	return `FROM oven/bun:1-alpine AS deps
WORKDIR /app
COPY package.json bun.lock* ./
RUN bun install --frozen-lockfile --production

FROM oven/bun:1-alpine AS production
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NODE_ENV=production
EXPOSE 3000
CMD ["bun", "run", "src/index.ts"]
`
}

function dockerignore(): string {
	return `.git
.env
.shuttle
node_modules
docker-compose*.yml
`
}

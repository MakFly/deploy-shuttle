import type { StarterFile } from './base.ts'

export interface NodeStarterOptions {
	database: 'postgres' | 'mysql' | 'none'
	redis: boolean
}

export function getNodeFiles(options: NodeStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateNodeShuttleYml(options) },
		{ path: 'Dockerfile', content: nodeDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateNodeShuttleYml(options: NodeStarterOptions): string {
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
	} else if (options.database === 'mysql') {
		accessories.push(`  db:
    preset: mysql`)
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

function nodeDockerfile(): string {
	return `FROM node:22-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --production

FROM node:22-alpine AS production
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NODE_ENV=production
EXPOSE 3000
CMD ["node", "src/index.js"]
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

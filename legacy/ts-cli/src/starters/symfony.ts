import type { StarterFile } from './base.ts'

export interface SymfonyStarterOptions {
	database: 'postgres' | 'mysql' | 'mariadb' | 'sqlite'
	redis: boolean
	messenger: boolean
	messengerReplicas: number
	mailpit: boolean
}

export function getSymfonyFiles(options: SymfonyStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateSymfonyShuttleYml(options) },
		{ path: 'Dockerfile', content: symfonyDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateSymfonyShuttleYml(options: SymfonyStarterOptions): string {
	const services: string[] = [
		`  web:
    port: 8080
    command: ""
    healthcheck:
      type: http
      path: /healthz`,
	]

	if (options.messenger) {
		services.push(`  messenger:
    command: "php bin/console messenger:consume async --time-limit=3600"
    replicas: ${options.messengerReplicas}`)
	}

	const accessories: string[] = []

	if (options.database !== 'sqlite') {
		accessories.push(`  db:
    preset: ${options.database}`)
	}

	if (options.redis) {
		accessories.push(`  cache:
    preset: redis`)
	}

	if (options.mailpit) {
		accessories.push(`  mail:
    preset: mailpit`)
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
  hooks:
    post_deploy:
      - "php bin/console doctrine:migrations:migrate --no-interaction"
      - "php bin/console cache:clear"
`

	return yml
}

function symfonyDockerfile(): string {
	return `FROM serversideup/php:8.4-fpm-nginx AS base

USER root

RUN install-php-extensions bcmath gd intl opcache pdo_pgsql pdo_mysql redis amqp

COPY --chown=www-data:www-data . /var/www/html

USER www-data

RUN composer install --no-dev --optimize-autoloader --no-interaction

FROM base AS production

ENV APP_ENV=prod
ENV APP_DEBUG=0
ENV PHP_OPCACHE_ENABLE=1

EXPOSE 8080
`
}

function dockerignore(): string {
	return `.git
.env
.env.local
.shuttle
node_modules
vendor
var/cache/*
var/log/*
docker-compose*.yml
`
}

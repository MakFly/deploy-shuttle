import type { StarterFile } from './base.ts'

export interface LaravelStarterOptions {
	database: 'postgres' | 'mysql' | 'mariadb' | 'sqlite'
	redis: boolean
	horizon: boolean
	scheduler: boolean
	reverb: boolean
	mailpit: boolean
}

export function getLaravelFiles(options: LaravelStarterOptions): StarterFile[] {
	return [
		{ path: 'shuttle.yml', content: generateLaravelShuttleYml(options) },
		{ path: 'Dockerfile', content: laravelDockerfile() },
		{ path: '.dockerignore', content: dockerignore() },
	]
}

function generateLaravelShuttleYml(options: LaravelStarterOptions): string {
	const services: string[] = [
		`  web:
    port: 8080
    command: ""
    healthcheck:
      type: http
      path: /up`,
	]

	if (options.horizon) {
		services.push(`  horizon:
    command: "php artisan horizon"`)
	}

	if (options.scheduler) {
		services.push(`  scheduler:
    command: "php artisan schedule:work"`)
	}

	if (options.reverb) {
		services.push(`  reverb:
    command: "php artisan reverb:start --host=0.0.0.0"
    port: 8080`)
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
      - "php artisan migrate --force"
      - "php artisan config:cache"
      - "php artisan route:cache"
      - "php artisan view:cache"
`

	return yml
}

function laravelDockerfile(): string {
	return `FROM serversideup/php:8.4-fpm-nginx AS base

USER root

RUN install-php-extensions bcmath gd intl opcache pdo_pgsql pdo_mysql redis

COPY --chown=www-data:www-data . /var/www/html

USER www-data

RUN composer install --no-dev --optimize-autoloader --no-interaction

FROM base AS production

ENV APP_ENV=production
ENV APP_DEBUG=false
ENV PHP_OPCACHE_ENABLE=1

EXPOSE 8080
`
}

function dockerignore(): string {
	return `.git
.env
.shuttle
node_modules
vendor
storage/logs/*
storage/framework/cache/*
storage/framework/sessions/*
storage/framework/views/*
bootstrap/cache/*
.docker
docker-compose*.yml
`
}

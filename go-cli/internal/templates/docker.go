package templates

// DockerfileLaravel returns a production-ready Dockerfile for Laravel
// using FrankenPHP on Alpine.
func DockerfileLaravel() string {
	return `FROM dunglas/frankenphp:php8.4-alpine

RUN install-php-extensions \
    pdo_mysql \
    pdo_pgsql \
    pdo_sqlite \
    mbstring \
    zip \
    bcmath \
    intl \
    gd \
    opcache \
    redis

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

COPY composer.json composer.lock ./
RUN composer install --no-dev --no-scripts --no-autoloader --prefer-dist

COPY . .
RUN composer dump-autoload --optimize \
    && php artisan route:cache \
    && php artisan view:cache \
    && mkdir -p database storage/logs storage/framework/{cache,sessions,views} \
    && chown -R www-data:www-data storage bootstrap/cache database

ENV SERVER_NAME=":8080"
ENV SERVER_ROOT=/app/public

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:8080/up || exit 1
`
}

// DockerfileSymfony returns a production-ready Dockerfile for Symfony
// using FrankenPHP on Alpine.
func DockerfileSymfony() string {
	return `FROM dunglas/frankenphp:php8.4-alpine

RUN install-php-extensions \
    pdo_mysql \
    pdo_pgsql \
    pdo_sqlite \
    mbstring \
    zip \
    intl \
    opcache \
    apcu \
    redis

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

COPY composer.json composer.lock symfony.lock ./
RUN composer install --no-dev --no-scripts --no-autoloader --prefer-dist

COPY . .
RUN composer dump-autoload --optimize --classmap-authoritative \
    && php bin/console cache:clear --env=prod \
    && php bin/console assets:install --env=prod \
    && mkdir -p var/cache var/log \
    && chown -R www-data:www-data var

ENV SERVER_NAME=":8080"
ENV SERVER_ROOT=/app/public
ENV APP_ENV=prod
ENV APP_DEBUG=0

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:8080/ || exit 1
`
}

// DockerfileNextJS returns a production-ready multi-stage Dockerfile
// for Next.js with standalone output.
func DockerfileNextJS() string {
	return `FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* yarn.lock* pnpm-lock.yaml* bun.lock* ./
RUN if [ -f bun.lock ]; then npx --yes bun install --frozen-lockfile; \
    elif [ -f pnpm-lock.yaml ]; then npx --yes pnpm install --frozen-lockfile; \
    elif [ -f yarn.lock ]; then yarn install --frozen-lockfile; \
    else npm ci; fi

FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

EXPOSE 3000
ENV PORT=3000
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:3000/ || exit 1
CMD ["node", "server.js"]
`
}

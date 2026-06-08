package templates

// DockerfileLaravel returns a production-ready Dockerfile for Laravel
// using FrankenPHP + Octane with worker mode.
// Based on FrankenPHP official docs + Laravel Octane best practices.
// Uses Debian (not Alpine) to avoid musl libc issues.
func DockerfileLaravel() string {
	return `#syntax=docker/dockerfile:1
FROM dunglas/frankenphp:1-php8.4 AS base

WORKDIR /app

RUN <<EOF
apt-get update
apt-get install -y --no-install-recommends \
    git \
    unzip
install-php-extensions \
    @composer \
    bcmath \
    intl \
    opcache \
    pcntl \
    pdo_mysql \
    pdo_pgsql \
    redis \
    zip \
    gd
rm -rf /var/lib/apt/lists/*
EOF

ENV COMPOSER_ALLOW_SUPERUSER=1

# ── Build stage ───────────────────────────────────────────────
FROM base AS builder

COPY --link composer.json composer.lock ./
RUN composer install --no-cache --prefer-dist --no-dev --no-autoloader --no-scripts --no-progress

COPY --link . .

RUN <<EOF
composer dump-autoload --classmap-authoritative --no-dev
php artisan route:cache
php artisan view:cache
php artisan event:cache
chmod +x artisan
EOF

# ── Production ────────────────────────────────────────────────
FROM base AS prod

RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

COPY --link docker/opcache.ini $PHP_INI_DIR/conf.d/opcache.ini
COPY --from=builder /app /app

RUN <<EOF
mkdir -p \
    storage/app/public \
    storage/framework/cache \
    storage/framework/sessions \
    storage/framework/views \
    storage/logs \
    bootstrap/cache \
    database
chown -R www-data:www-data storage bootstrap/cache database
chmod -R ug+rwx storage bootstrap/cache
EOF

USER www-data

EXPOSE 8000

HEALTHCHECK --start-period=30s --interval=10s --timeout=5s --retries=3 \
    CMD php -r 'exit(false === @file_get_contents("http://localhost:8000/up", context: stream_context_create(["http" => ["timeout" => 3]])) ? 1 : 0);'

COPY --link docker/docker-secrets-entrypoint.sh /usr/local/bin/docker-secrets-entrypoint
RUN chmod +x /usr/local/bin/docker-secrets-entrypoint

ENTRYPOINT ["docker-secrets-entrypoint", "php", "artisan", "octane:frankenphp"]
CMD ["--host=0.0.0.0", "--port=8000", "--max-requests=500", "--workers=auto"]
`
}

// LaravelOpcacheIni returns the optimized OPcache config for Laravel production.
func LaravelOpcacheIni() string {
	return `; OPcache — production tuning for Laravel + FrankenPHP
opcache.enable = 1
opcache.memory_consumption = 256
opcache.max_accelerated_files = 32531
opcache.interned_strings_buffer = 16
opcache.validate_timestamps = 0
opcache.enable_file_override = 1

; Realpath cache
realpath_cache_size = 4096K
realpath_cache_ttl = 600

; Security
expose_php = 0
`
}

// DockerfileSymfony returns a production-ready Dockerfile for Symfony
// using FrankenPHP with native worker mode.
// Based on dunglas/symfony-docker reference implementation.
func DockerfileSymfony() string {
	return `#syntax=docker/dockerfile:1
FROM dunglas/frankenphp:1-php8.4 AS base

WORKDIR /app

RUN <<EOF
apt-get update
apt-get install -y --no-install-recommends \
    git \
    unzip \
    file
install-php-extensions \
    @composer \
    apcu \
    intl \
    opcache \
    pdo_mysql \
    pdo_pgsql \
    redis \
    zip
rm -rf /var/lib/apt/lists/*
EOF

ENV COMPOSER_ALLOW_SUPERUSER=1
ENV PHP_INI_SCAN_DIR=":$PHP_INI_DIR/app.conf.d"

COPY --link docker/Caddyfile /etc/frankenphp/Caddyfile
COPY --link docker/conf.d/10-app.ini $PHP_INI_DIR/app.conf.d/

HEALTHCHECK --start-period=60s --interval=10s --timeout=5s --retries=3 \
    CMD php -r 'exit(false === @file_get_contents("http://localhost:2019/metrics", context: stream_context_create(["http" => ["timeout" => 5]])) ? 1 : 0);'

# ── Build stage ───────────────────────────────────────────────
FROM base AS builder

ENV APP_ENV=prod

RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

COPY --link docker/conf.d/20-app.prod.ini $PHP_INI_DIR/app.conf.d/

COPY --link composer.json composer.lock symfony.lock ./
RUN composer install --no-cache --prefer-dist --no-dev --no-autoloader --no-scripts --no-progress

COPY --link . .

RUN <<EOF
composer dump-autoload --classmap-authoritative --no-dev
composer dump-env prod
composer run-script --no-dev post-install-cmd || true
if [ -f importmap.php ]; then
    php bin/console asset-map:compile
fi
chmod +x bin/console
mkdir -p var/cache var/log
chmod -R g=u var
sync
EOF

# ── Production ────────────────────────────────────────────────
FROM base AS prod

ENV APP_ENV=prod

RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

COPY --link docker/conf.d/20-app.prod.ini $PHP_INI_DIR/app.conf.d/
COPY --from=builder /app /app

RUN <<EOF
chown -R www-data:www-data var || true
chmod -R g=u var || true
EOF

USER www-data

EXPOSE 8080

COPY --link docker/docker-secrets-entrypoint.sh /usr/local/bin/docker-secrets-entrypoint
RUN chmod +x /usr/local/bin/docker-secrets-entrypoint
ENTRYPOINT ["docker-secrets-entrypoint"]
CMD ["frankenphp", "run", "--config", "/etc/frankenphp/Caddyfile"]
`
}

// SymfonyCaddyfile returns the production Caddyfile for Symfony with worker mode.
func SymfonyCaddyfile() string {
	return `{
    {$CADDY_GLOBAL_OPTIONS}

    frankenphp {
        {$FRANKENPHP_CONFIG}
    }
}

{$SERVER_NAME:localhost} {
    root /app/public
    encode zstd br gzip

    header {
        X-Content-Type-Options    nosniff
        X-Frame-Options           DENY
        Referrer-Policy            strict-origin-when-cross-origin
        ?Permissions-Policy       "browsing-topics=()"
        -Server
        -X-Powered-By
    }

    @phpRoute {
        not file {path}
    }
    rewrite @phpRoute index.php

    @frontController path index.php
    php @frontController {
        worker {
            file ./public/index.php
            {$FRANKENPHP_WORKER_CONFIG}
        }
    }

    file_server {
        hide *.php
        hide .env*
        hide .git*
    }
}
`
}

// SymfonyBaseIni returns the shared PHP config (dev + prod).
func SymfonyBaseIni() string {
	return `; Security
expose_php = 0

; Timezone
date.timezone = UTC

; APCu
apc.enable_cli = 1

; Session
session.use_strict_mode = 1

; Performance
zend.detect_unicode = 0
realpath_cache_size = 4096K
realpath_cache_ttl = 600

; OPcache
opcache.interned_strings_buffer = 16
opcache.max_accelerated_files = 32531
opcache.memory_consumption = 256
opcache.enable_file_override = 1
`
}

// SymfonyProdIni returns the production-only PHP config.
func SymfonyProdIni() string {
	return `; OPcache — production only
opcache.preload_user = www-data
opcache.preload = /app/config/preload.php
opcache.validate_timestamps = 0
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

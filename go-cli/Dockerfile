#syntax=docker/dockerfile:1
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

ENTRYPOINT ["php", "artisan", "octane:frankenphp"]
CMD ["--host=0.0.0.0", "--port=8000", "--max-requests=500", "--workers=auto"]

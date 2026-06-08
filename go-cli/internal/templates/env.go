package templates

import "strings"

// EnvExample returns a commented .env.example for the given preset.
func EnvExample(preset string) string {
	var b strings.Builder
	b.WriteString("# Environment variables for production\n")
	b.WriteString("# Copy to .env and fill in the values\n\n")

	switch preset {
	case "laravel":
		b.WriteString("APP_NAME=MyApp\n")
		b.WriteString("APP_ENV=production\n")
		b.WriteString("APP_KEY=\n")
		b.WriteString("APP_DEBUG=false\n")
		b.WriteString("APP_URL=https://example.com\n\n")
		b.WriteString("# Database\n")
		b.WriteString("DB_CONNECTION=pgsql\n")
		b.WriteString("DB_HOST=127.0.0.1\n")
		b.WriteString("DB_PORT=5432\n")
		b.WriteString("DB_DATABASE=app\n")
		b.WriteString("DB_USERNAME=app\n")
		b.WriteString("DB_PASSWORD=\n\n")
		b.WriteString("# Cache & Queue\n")
		b.WriteString("CACHE_STORE=file\n")
		b.WriteString("QUEUE_CONNECTION=sync\n\n")
		b.WriteString("# Mail\n")
		b.WriteString("MAIL_MAILER=smtp\n")
		b.WriteString("MAIL_HOST=127.0.0.1\n")
		b.WriteString("MAIL_PORT=1025\n")
		b.WriteString("MAIL_USERNAME=null\n")
		b.WriteString("MAIL_PASSWORD=null\n")

	case "symfony":
		b.WriteString("APP_ENV=prod\n")
		b.WriteString("APP_SECRET=\n\n")
		b.WriteString("# Database (Doctrine)\n")
		b.WriteString("DATABASE_URL=\"postgresql://app:password@127.0.0.1:5432/app?serverVersion=16&charset=utf8\"\n\n")
		b.WriteString("# Messenger\n")
		b.WriteString("MESSENGER_TRANSPORT_DSN=doctrine://default?auto_setup=0\n\n")
		b.WriteString("# Mailer\n")
		b.WriteString("MAILER_DSN=smtp://127.0.0.1:1025\n")

	case "nextjs":
		b.WriteString("NODE_ENV=production\n")
		b.WriteString("NEXT_PUBLIC_API_URL=https://api.example.com\n\n")
		b.WriteString("# Database (if applicable)\n")
		b.WriteString("DATABASE_URL=\n")

	case "node-api":
		b.WriteString("NODE_ENV=production\n")
		b.WriteString("PORT=3000\n\n")
		b.WriteString("# Database (if applicable)\n")
		b.WriteString("DATABASE_URL=\n")

	default:
		b.WriteString("# Add your environment variables here\n")
	}

	return b.String()
}

// EnvExamplePro returns an enriched .env.example with service connection strings.
func EnvExamplePro(preset, db string) string {
	var b strings.Builder
	b.WriteString("# Environment variables for production (Pro stack)\n")
	b.WriteString("# Copy to .env and fill in the values\n\n")

	dbHost := "postgres"
	dbPort := "5432"
	dbDriver := "pgsql"
	if db == "mysql" {
		dbHost = "mysql"
		dbPort = "3306"
		dbDriver = "mysql"
	}

	switch preset {
	case "laravel":
		b.WriteString("APP_NAME=MyApp\n")
		b.WriteString("APP_ENV=production\n")
		b.WriteString("APP_KEY=\n")
		b.WriteString("APP_DEBUG=false\n")
		b.WriteString("APP_URL=https://example.com\n\n")
		b.WriteString("# Database\n")
		b.WriteString("DB_CONNECTION=" + dbDriver + "\n")
		b.WriteString("DB_HOST=" + dbHost + "\n")
		b.WriteString("DB_PORT=" + dbPort + "\n")
		b.WriteString("DB_DATABASE=app\n")
		b.WriteString("DB_USERNAME=app\n")
		b.WriteString("DB_PASSWORD=secret\n\n")
		b.WriteString("# Redis\n")
		b.WriteString("REDIS_HOST=redis\n")
		b.WriteString("REDIS_PORT=6379\n")
		b.WriteString("REDIS_PASSWORD=null\n\n")
		b.WriteString("# Cache & Queue (wired to Redis)\n")
		b.WriteString("CACHE_STORE=redis\n")
		b.WriteString("QUEUE_CONNECTION=redis\n")
		b.WriteString("SESSION_DRIVER=redis\n\n")
		b.WriteString("# Mail (Mailpit in dev, SMTP in prod)\n")
		b.WriteString("MAIL_MAILER=smtp\n")
		b.WriteString("MAIL_HOST=mailpit\n")
		b.WriteString("MAIL_PORT=1025\n")
		b.WriteString("MAIL_USERNAME=null\n")
		b.WriteString("MAIL_PASSWORD=null\n")

	case "symfony":
		b.WriteString("APP_ENV=prod\n")
		b.WriteString("APP_SECRET=\n\n")
		b.WriteString("# Database (Doctrine)\n")
		if db == "mysql" {
			b.WriteString("DATABASE_URL=\"mysql://app:secret@mysql:3306/app?serverVersion=8.4&charset=utf8mb4\"\n\n")
		} else {
			b.WriteString("DATABASE_URL=\"postgresql://app:secret@postgres:5432/app?serverVersion=16&charset=utf8\"\n\n")
		}
		b.WriteString("# Redis\n")
		b.WriteString("REDIS_URL=redis://redis:6379\n\n")
		b.WriteString("# Messenger (wired to Redis)\n")
		b.WriteString("MESSENGER_TRANSPORT_DSN=redis://redis:6379/messages\n\n")
		b.WriteString("# Mailer (Mailpit in dev, SMTP in prod)\n")
		b.WriteString("MAILER_DSN=smtp://mailpit:1025\n")

	default:
		b.WriteString("NODE_ENV=production\n")
		b.WriteString("PORT=3000\n\n")
		if db != "" {
			b.WriteString("# Database\n")
			if db == "mysql" {
				b.WriteString("DATABASE_URL=mysql://app:secret@mysql:3306/app\n\n")
			} else {
				b.WriteString("DATABASE_URL=postgresql://app:secret@postgres:5432/app\n\n")
			}
		}
		b.WriteString("# Redis\n")
		b.WriteString("REDIS_URL=redis://redis:6379\n")
	}

	return b.String()
}

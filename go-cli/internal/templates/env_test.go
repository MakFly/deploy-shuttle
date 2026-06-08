package templates

import (
	"strings"
	"testing"
)

func TestEnvExampleLaravel(t *testing.T) {
	out := EnvExample("laravel")
	for _, key := range []string{"APP_NAME", "APP_KEY", "DB_CONNECTION", "DB_HOST", "QUEUE_CONNECTION", "MAIL_MAILER"} {
		if !strings.Contains(out, key) {
			t.Errorf("laravel env missing %s", key)
		}
	}
}

func TestEnvExampleSymfony(t *testing.T) {
	out := EnvExample("symfony")
	for _, key := range []string{"APP_ENV", "APP_SECRET", "DATABASE_URL", "MESSENGER_TRANSPORT_DSN", "MAILER_DSN"} {
		if !strings.Contains(out, key) {
			t.Errorf("symfony env missing %s", key)
		}
	}
}

func TestEnvExampleNextjs(t *testing.T) {
	out := EnvExample("nextjs")
	if !strings.Contains(out, "NODE_ENV") {
		t.Error("nextjs env missing NODE_ENV")
	}
	if !strings.Contains(out, "NEXT_PUBLIC_API_URL") {
		t.Error("nextjs env missing NEXT_PUBLIC_API_URL")
	}
}

func TestEnvExampleNodeAPI(t *testing.T) {
	out := EnvExample("node-api")
	if !strings.Contains(out, "PORT=3000") {
		t.Error("node-api env missing PORT=3000")
	}
}

func TestEnvExampleUnknownPreset(t *testing.T) {
	out := EnvExample("unknown")
	if out == "" {
		t.Error("unknown preset should still return a default")
	}
}

func TestEnvExampleProLaravelPostgres(t *testing.T) {
	out := EnvExamplePro("laravel", "postgres")
	for _, key := range []string{"DB_HOST=postgres", "REDIS_HOST=redis", "CACHE_STORE=redis", "QUEUE_CONNECTION=redis", "MAIL_HOST=mailpit"} {
		if !strings.Contains(out, key) {
			t.Errorf("laravel pro env missing %s", key)
		}
	}
}

func TestEnvExampleProLaravelMySQL(t *testing.T) {
	out := EnvExamplePro("laravel", "mysql")
	if !strings.Contains(out, "DB_HOST=mysql") {
		t.Error("laravel pro mysql env should have DB_HOST=mysql")
	}
	if !strings.Contains(out, "DB_CONNECTION=mysql") {
		t.Error("laravel pro mysql env should have DB_CONNECTION=mysql")
	}
}

func TestEnvExampleProSymfonyPostgres(t *testing.T) {
	out := EnvExamplePro("symfony", "postgres")
	if !strings.Contains(out, "postgresql://app:secret@postgres:5432") {
		t.Error("symfony pro env should have postgres DSN")
	}
	if !strings.Contains(out, "REDIS_URL=redis://redis:6379") {
		t.Error("symfony pro env missing REDIS_URL")
	}
}

func TestEnvExampleProSymfonyMySQL(t *testing.T) {
	out := EnvExamplePro("symfony", "mysql")
	if !strings.Contains(out, "mysql://app:secret@mysql:3306") {
		t.Error("symfony pro env should have mysql DSN")
	}
}

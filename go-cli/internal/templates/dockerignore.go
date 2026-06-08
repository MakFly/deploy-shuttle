package templates

// Dockerignore returns a .dockerignore tailored to the given preset.
// It combines a shared base with preset-specific additions.
func Dockerignore(preset string) string {
	base := `.git
.gitignore
docker-compose*.yml
shuttle.yml
.shuttle/
README.md
.env
.env.*
`

	switch preset {
	case "laravel":
		return base + `vendor/
node_modules/
tests/
phpunit.xml
storage/logs/*
storage/framework/cache/*
storage/framework/sessions/*
storage/framework/views/*
`
	case "symfony":
		return base + `vendor/
node_modules/
tests/
phpunit.xml.dist
var/cache/*
var/log/*
`
	case "nextjs":
		return base + `node_modules/
.next/
out/
`
	default:
		return base
	}
}

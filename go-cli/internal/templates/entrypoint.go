package templates

// SecretsEntrypoint returns a shell script that loads Docker Secrets
// (files under /run/secrets/) as environment variables at container startup.
// Each file becomes an env var with the filename as key and file contents as value.
func SecretsEntrypoint() string {
	return `#!/bin/sh
# Load Docker Secrets as environment variables
# Each file in /run/secrets/ becomes an env var with the filename as key
for secret_file in /run/secrets/*; do
    if [ -f "$secret_file" ]; then
        key=$(basename "$secret_file")
        export "$key"="$(cat "$secret_file")"
    fi
done
exec "$@"
`
}

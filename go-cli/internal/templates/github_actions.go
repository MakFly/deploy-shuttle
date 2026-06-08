package templates

import "fmt"

// CIWorkflow returns a GitHub Actions workflow YAML for shuttle doctor.
func CIWorkflow(preset string) string {
	steps := ""
	switch preset {
	case "laravel", "symfony":
		steps = fmt.Sprintf(`      - uses: actions/checkout@v6

      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: "8.4"
          tools: composer:v2

      - name: Install dependencies
        run: composer install --no-interaction --prefer-dist --no-progress

      - name: Install shuttle
        run: curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

      - name: Run readiness scan
        run: shuttle doctor --fail-below 75
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
`)
	case "nextjs", "node-api":
		steps = fmt.Sprintf(`      - uses: actions/checkout@v6

      - name: Setup Node
        uses: actions/setup-node@v6
        with:
          node-version: "lts/*"

      - name: Install dependencies
        run: npm ci

      - name: Install shuttle
        run: curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

      - name: Run readiness scan
        run: shuttle doctor --fail-below 75
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
`)
	default:
		steps = `      - uses: actions/checkout@v6

      - name: Install shuttle
        run: curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

      - name: Run readiness scan
        run: shuttle doctor --fail-below 75
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
`
	}

	return fmt.Sprintf(`name: DeployShuttle Readiness
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

jobs:
  doctor:
    runs-on: ubuntu-latest
    steps:
%s`, steps)
}

// CIWorkflowPro returns a CI workflow with service containers for testing.
func CIWorkflowPro(preset, db string) string {
	services := ""
	dbEnv := ""
	switch db {
	case "postgres":
		services = `    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: app_test
          POSTGRES_USER: app
          POSTGRES_PASSWORD: secret
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -U app"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
`
		if preset == "laravel" {
			dbEnv = `          DB_CONNECTION: pgsql
          DB_HOST: 127.0.0.1
          DB_PORT: 5432
          DB_DATABASE: app_test
          DB_USERNAME: app
          DB_PASSWORD: secret`
		} else if preset == "symfony" {
			dbEnv = `          DATABASE_URL: "postgresql://app:secret@127.0.0.1:5432/app_test?serverVersion=16&charset=utf8"`
		}
	case "mysql":
		services = `    services:
      mysql:
        image: mysql:8.4
        env:
          MYSQL_DATABASE: app_test
          MYSQL_USER: app
          MYSQL_PASSWORD: secret
          MYSQL_ROOT_PASSWORD: secret
        ports:
          - 3306:3306
        options: >-
          --health-cmd "mysqladmin ping -h 127.0.0.1"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
`
		if preset == "laravel" {
			dbEnv = `          DB_CONNECTION: mysql
          DB_HOST: 127.0.0.1
          DB_PORT: 3306
          DB_DATABASE: app_test
          DB_USERNAME: app
          DB_PASSWORD: secret`
		} else if preset == "symfony" {
			dbEnv = `          DATABASE_URL: "mysql://app:secret@127.0.0.1:3306/app_test?serverVersion=8.4&charset=utf8mb4"`
		}
	}

	testStep := ""
	switch preset {
	case "laravel":
		testStep = `      - name: Run tests
        run: php artisan test
        env:
` + dbEnv + "\n"
	case "symfony":
		testStep = `      - name: Run tests
        run: php bin/phpunit
        env:
` + dbEnv + "\n"
	}

	setupStep := ""
	switch preset {
	case "laravel", "symfony":
		setupStep = `      - name: Setup PHP
        uses: shivammathur/setup-php@v2
        with:
          php-version: "8.4"
          tools: composer:v2

      - name: Install dependencies
        run: composer install --no-interaction --prefer-dist --no-progress

`
	case "nextjs", "node-api":
		setupStep = `      - name: Setup Node
        uses: actions/setup-node@v6
        with:
          node-version: "lts/*"

      - name: Install dependencies
        run: npm ci

`
	}

	return fmt.Sprintf(`name: DeployShuttle CI
on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

jobs:
  test:
    runs-on: ubuntu-latest
%s    steps:
      - uses: actions/checkout@v6

%s%s      - name: Install shuttle
        run: curl -fsSL https://raw.githubusercontent.com/MakFly/deploy-shuttle/main/scripts/install.sh | sh

      - name: Run readiness scan
        run: shuttle doctor --fail-below 75
        env:
          SSH_PRIVATE_KEY: ${{ secrets.SSH_PRIVATE_KEY }}
`, services, setupStep, testStep)
}

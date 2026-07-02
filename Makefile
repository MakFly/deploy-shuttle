# DeployShuttle — dev shortcuts. Run `make help` for the list.

.PHONY: help site site-build site-preview test build stripe-mock e2e-license e2e-stripe-test

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

site: ## Start the docs site dev server (http://localhost:4321)
	cd docs-site && bun install && bun run dev

site-build: ## Production build of the docs site (set PUBLIC_STRIPE_PAYMENT_LINK to enable the buy button)
	cd docs-site && bun install && bun run build

site-preview: site-build ## Serve the production build locally
	cd docs-site && bun run preview

test: ## CI parity: gofmt + go vet + go test, then license-server tests
	cd go-cli && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...
	cd license-server && bun install && bun run typecheck && bun test

build: ## Build the shuttle binary (dist/)
	sh scripts/build-go.sh

stripe-mock: ## Start the dev-only fake Stripe (http://localhost:4242/pay)
	bun run stripe-mock/server.ts

e2e-license: ## Full purchase→email→activate→refund E2E against infra-postgres/mailpit
	bash scripts/e2e-license.sh

e2e-stripe-test: ## Same E2E against REAL Stripe test mode (stripe listen + Payment Link, human pays with the 4242 card)
	bash scripts/e2e-stripe-test.sh

# DeployShuttle Plans

This folder captures the product pivot plan for DeployShuttle.

DeployShuttle moves from a generic VPS deployment CLI to a CLI-first production readiness product:

```txt
Audit, harden and deploy Docker apps on VPS.
```

Main hook:

```bash
shuttle doctor --target root@server
```

## Documents

- [01-product-prd.md](./01-product-prd.md) - product direction, positioning, customers, jobs to be done, and strategy.
- [02-mvp-scope.md](./02-mvp-scope.md) - MVP commands, command behavior, acceptance criteria, and roadmap.
- [03-check-catalog.md](./03-check-catalog.md) - initial production readiness check catalog.
- [04-scoring-config-reports.md](./04-scoring-config-reports.md) - scoring model, config file, reports, and CI/CD use case.
- [05-cloud-pricing-launch.md](./05-cloud-pricing-launch.md) - optional cloud scope, pricing hypothesis, differentiation, metrics, and launch plan.
- [06-architecture-security.md](./06-architecture-security.md) - recommended technical architecture, check interface, exec adapter, and security requirements.
- [07-docs-and-implementation-prompt.md](./07-docs-and-implementation-prompt.md) - documentation structure, landing copy, and implementation prompt for coding agents.

## Implementation Priority

Do not start with the dashboard.

Recommended order:

1. Align docs and README around the PRD.
2. Implement `shuttle doctor`.
3. Add report output formats.
4. Add conservative `harden --dry-run`.
5. Treat `deploy` as the continuation after readiness checks.

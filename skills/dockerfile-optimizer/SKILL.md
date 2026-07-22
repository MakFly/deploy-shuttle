---
name: dockerfile-optimizer
description: Audit and optimize Dockerfiles and production OCI images using measured build duration, image size, layer invalidation, runtime dependencies, build context, cache, security, and deployment-transfer evidence. Use whenever an AI coding agent creates or edits a Dockerfile, container build, Compose build service, CI image pipeline, or investigates slow Docker builds, large images, slow pushes or pulls, cold caches, or source-to-production latency.
---

# Dockerfile Optimizer

Treat image size, rebuild cost, and transfer cost as acceptance criteria, not cleanup.

## Workflow

1. Inspect the Dockerfile, `.dockerignore`, build context, manifests, lockfiles, Compose targets, runtime command, and deployment transport.
2. Record a baseline with exact image size, largest layers, build duration, context size, and runtime smoke test. Use `scripts/audit-image.sh IMAGE [MAX_MB]` after an image exists.
3. Identify invalidation boundaries. Keep dependency installation before frequently changing source copies. Separate build dependencies from runtime dependencies.
4. Build only the required runtime closure. In monorepos, filter workspaces or generate a pruned runtime manifest; never copy a root-wide `node_modules` blindly.
5. Use multi-stage builds. Keep compilers, package managers, caches, tests, source maps, and unrelated workspace artifacts out of the final stage unless the runtime requires them.
6. Pin reproducible inputs and use the repository's required package manager and lockfile. Use BuildKit cache mounts for package caches, never as runtime layers.
7. Rebuild and compare cold and warm behavior. Change one ordinary source file and prove dependency layers stay cached.
8. Smoke-test the final image using its real runtime user, command, required files, health endpoint when feasible, and representative dynamic imports.
9. Report before/after evidence and remaining dominant layers. Do not claim deployment speed from image size alone; measure build, push/pull, readiness, and traffic continuity separately.

## Guardrails

- Preserve application behavior; replace a heavyweight runtime dependency only with functional tests or representative fixtures.
- Do not expose secrets through `docker history`, build arguments, logs, or copied env files.
- Do not use mutable tags as the sole production identity; prefer a digest or commit-qualified tag.
- Do not call a rollout zero-downtime unless an external probe observed no failed requests during the switch.
- Distinguish source-to-image time from image-to-healthy-production promotion time.
- For a sub-30-second promotion SLO, prebuild and preposition immutable images; an arbitrary cold application build cannot carry that guarantee.

## Evidence checklist

- Docker build exit status and elapsed time
- `docker image inspect` content size plus the engine's displayed size
- largest `docker history` layers
- runtime dependency footprint inside the final image
- warm rebuild cache hits after a source-only change
- runtime/health smoke result
- push/pull bytes or elapsed time when deployment latency is in scope
- external availability probe and exact promotion duration for zero-downtime claims

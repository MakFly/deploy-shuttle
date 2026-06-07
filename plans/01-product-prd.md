# DeployShuttle - PRD v2
## VPS Production Readiness CLI + Optional Cloud

**Status:** Draft  
**Owner:** MakFly  
**Product type:** Developer tool / DevOps automation  
**Primary surface:** CLI-first  
**Secondary surface:** Cloud dashboard, optional after traction  
**Target release:** MVP v0.1

## 1. Executive Summary

DeployShuttle pivots from a generic VPS deployment CLI into a **production readiness tool for Docker apps on VPS**.

The new core promise:

> Make your VPS production-ready before deploying client or production workloads.

Instead of competing directly with full PaaS tools like Coolify, Dokploy, CapRover, Ploi, Laravel Forge, Railway, Render or Fly.io, DeployShuttle focuses on a sharper pain:

> Developers can deploy to a VPS, but they do not know if the VPS is safe, correctly configured, monitored, backed up and production-ready.

DeployShuttle starts as a CLI that audits a server, detects production risks, generates a score, explains issues, and optionally applies safe fixes.

Deployment remains part of the roadmap, but the acquisition hook is now:

```bash
shuttle doctor --target root@server
```

## 2. Product Positioning

### Old positioning

> Deploy Docker apps to your VPS.

### New positioning

> Audit, harden and deploy Docker apps on VPS.

### One-liner

```txt
DeployShuttle scans your VPS, finds production risks, and fixes the boring DevOps before you deploy.
```

### French one-liner

```txt
DeployShuttle audite ton VPS, détecte les risques de prod et corrige la partie DevOps avant déploiement.
```

### Core tagline

```txt
Your VPS. Your Docker. Production-ready in 10 minutes.
```

### Anti-positioning

DeployShuttle is **not**:

- a Kubernetes platform;
- a full hosting provider;
- a mandatory cloud PaaS;
- a replacement for Docker;
- a replacement for Cloudflare;
- another generic deployment wrapper.

DeployShuttle is:

- a CLI-first production readiness tool;
- a DevOps doctor for VPS-based Docker apps;
- a hardening assistant for freelancers, agencies and indie hackers;
- eventually, a lightweight cockpit for deployment history, monitoring and alerts.

## 3. Problem Statement

Many developers use low-cost VPS providers such as OVH, Hetzner, Scaleway, DigitalOcean or Contabo to host apps.

They can usually get an app running, but production quality is inconsistent.

Common issues:

- Docker containers run as root.
- No restart policy.
- No healthcheck.
- Database ports exposed publicly.
- No backups.
- No rollback.
- `.env` files stored with weak permissions.
- Caddy admin API exposed.
- Caddy/Cloudflare TLS misconfigured.
- Cloudflare SSL mode set to Flexible.
- No firewall.
- No fail2ban or SSH hardening.
- No monitoring.
- No uptime checks.
- No audit trail.
- No deploy history.
- No clear production readiness score.

This creates a gap between:

```txt
"It works"
```

and:

```txt
"It is production-ready"
```

DeployShuttle owns that gap.

## 4. Target Customers

### Primary ICP

Freelance and agency developers deploying client apps on VPS.

They typically use:

- Laravel;
- Symfony;
- Node.js;
- Next.js;
- Hono/Fastify;
- Docker Compose;
- Caddy or Nginx;
- Cloudflare DNS/proxy.

They do not want Kubernetes.

They want:

- fast setup;
- low hosting cost;
- confidence before delivery;
- repeatable deployment;
- fewer production mistakes;
- a professional report they can show to clients.

### Secondary ICP

Indie hackers and solo SaaS founders.

They want:

- cheap hosting;
- clean production setup;
- basic monitoring;
- rollback;
- backups;
- no complex platform.

### Later ICP

Small agencies managing several client VPS.

They need:

- multi-server inventory;
- scheduled audits;
- client-facing reports;
- drift detection;
- white-label status reports;
- team permissions.

## 5. Core Jobs To Be Done

### JTBD 1 - Audit

When I configure a VPS manually, I want to scan it automatically so I know what is unsafe or missing.

### JTBD 2 - Explain

When a risk is detected, I want a clear explanation and recommended fix so I understand what to do.

### JTBD 3 - Fix

When a safe fix is available, I want the CLI to apply it or generate the required file so I do not waste time.

### JTBD 4 - Report

When I deliver a client project, I want to generate a clean readiness report so the client sees the deployment is professional.

### JTBD 5 - Deploy

When my server is ready, I want to deploy my Docker app with a standard, repeatable workflow.

### JTBD 6 - Monitor

After deployment, I want scheduled checks and alerts so I know if the server drifts or goes down.

## 6. Product Strategy

DeployShuttle should follow a **CLI-first, Cloud-optional** model.

### Phase 1 - CLI-only MVP

No account required.

Main commands:

- `doctor`
- `report`
- `harden`
- `init`
- `deploy` later or minimal

Goal:

- adoption;
- trust;
- shareable reports;
- fast feedback;
- low friction.

### Phase 2 - Optional Cloud

Account required only for:

- audit history;
- scheduled scans;
- alerting;
- multi-server dashboard;
- team access;
- GitHub auto-deploy;
- encrypted remote secrets;
- billing.

### Phase 3 - Deployment cockpit

Add:

- deploy history;
- rollback UI;
- logs;
- server inventory;
- app inventory;
- backup status;
- Cloudflare automation.

## 7. Final Product Direction

DeployShuttle should become:

```txt
The production readiness and deployment assistant for Docker apps on VPS.
```

The CLI earns trust with audits.

The hardening engine creates immediate value.

The cloud dashboard monetizes recurring monitoring, history, teams and agencies.

The deployment workflow becomes the natural continuation once the server is ready.

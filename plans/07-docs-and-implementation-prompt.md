# Docs, Landing Copy, and Implementation Prompt

## 1. Landing Page Copy

### Hero

```txt
Make your VPS production-ready before your clients find out it is not.
```

### Subhero

```txt
DeployShuttle scans your Docker, Caddy, firewall, database, backups and Cloudflare setup, then gives you a clear production readiness score with actionable fixes.
```

### CTA

```txt
Run your first scan
```

### Install

```bash
curl -fsSL https://deployshuttle.dev/install.sh | bash
shuttle doctor --target root@your-server
```

### Feature blocks

```txt
Production Readiness Score
Know if your VPS is safe enough to run client workloads.

Docker & Caddy Checks
Detect missing healthchecks, restart policies, unsafe ports and proxy mistakes.

Cloudflare Guardrails
Catch Flexible SSL, missing proxy mode, DNS issues and unsafe edge settings.

Reports for Clients
Generate a clean handoff report that proves your deployment is production-ready.

Safe Auto-Fixes
Apply conservative hardening fixes with dry-run and confirmation.
```

## 2. Documentation Structure

```txt
docs/
  prd/
    deployshuttle-prod-readiness-prd.md
  guides/
    quickstart.md
    first-vps-scan.md
    hardening-with-deployshuttle.md
    github-actions.md
    cloudflare-checks.md
  reference/
    config.md
    checks.md
    commands.md
```

## 3. Prompt for Claude Code / Cursor

```md
You are working on the `deploy-shuttle` repository.

Goal: update the project direction from a generic deploy CLI to a CLI-first VPS production readiness product.

Create a new PRD file:

`docs/prd/deployshuttle-prod-readiness-prd.md`

Use the PRD content below as the source of truth.

Then update the project documentation and landing/docs copy to reflect the new positioning:

Old positioning:
- Deploy Docker apps to your VPS.

New positioning:
- Audit, harden and deploy Docker apps on VPS.
- CLI-first production readiness tool.
- Main hook: `shuttle doctor --target root@server`.

Implementation guidance:
1. Do not remove existing deploy-related code.
2. Reframe deploy as phase 2/continuation after readiness checks.
3. Add docs structure if missing:
   - `docs/prd/`
   - `docs/guides/`
   - `docs/reference/`
4. Add or update README with:
   - new tagline;
   - problem statement;
   - install command placeholder;
   - command examples;
   - MVP roadmap.
5. Add command design docs for:
   - `doctor`
   - `report`
   - `harden`
   - `init`
   - `deploy`
6. Add a check catalog document with initial checks:
   - system
   - ssh
   - firewall
   - docker
   - compose
   - caddy
   - tls
   - cloudflare
   - database
   - backups
   - secrets
   - monitoring
7. Preserve existing formatting/lint rules.
8. Do not invent implemented features. Clearly label future features as planned.
9. Keep the tone technical, direct, and product-focused.

Expected output:
- new PRD markdown file;
- updated README or docs landing page;
- docs/reference/checks.md;
- docs/reference/commands.md;
- docs/guides/quickstart.md.
```

## 4. Product Guidance

Do not start by coding the dashboard.

Update the project around this PRD, then implement `doctor` first. It is the hook that makes DeployShuttle less niche.

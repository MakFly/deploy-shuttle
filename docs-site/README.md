# DeployShuttle Docs Site

Static documentation site for DeployShuttle. Built with [Astro](https://astro.build),
[shadcn](https://ui.shadcn.com) (`b0` / `radix-nova` preset), Tailwind v4,
[`@astrojs/mdx`](https://docs.astro.build/en/guides/integrations-guide/mdx/),
and [`astro-expressive-code`](https://expressive-code.com/) for syntax highlighting.

UI/UX inspired by [nextjs.org/docs](https://nextjs.org/docs): dark mode by default,
sticky top nav, left sidebar, optional right TOC, MDX-driven content.

## Develop

```bash
bun install
bun run dev          # http://localhost:4321
```

## Build

```bash
bun run build        # outputs to ./dist
bun run preview      # serves ./dist locally
```

## Deploy

The site is fully static. Any static host works:

- **Cloudflare Pages**: connect this repo, build command
  `cd docs-site && bun install && bun run build`, output `docs-site/dist`.
- **Vercel / Netlify**: same idea, set the project root to `docs-site/`.
- **GitHub Pages**: push `docs-site/dist` to a `gh-pages` branch.

Override the canonical URL at build time:

```bash
SITE_URL=https://deployshuttle.dev bun run build
```

## Structure

```
src/
  components/      TopNav, Sidebar, MobileSidebar, ThemeToggle, PrevNext, Footer
  layouts/
    Root.astro     Top nav + footer + theme bootstrap. Landing + pricing.
    Docs.astro     Root + sidebar + MDX TOC. Every /docs MDX page.
  lib/
    nav.ts         Single source of truth for top nav and docs sidebar.
  pages/
    index.astro    Landing.
    pricing.astro  Pricing tiers + early-bird + audit one-shot.
    docs/
      index.mdx, install.mdx, quickstart.mdx, checks.mdx,
      config.mdx, github-action.mdx
      commands/
        index.mdx, doctor.mdx, harden.mdx, report.mdx, init.mdx, license.mdx
  styles/
    global.css     Tailwind + shadcn tokens + typography plugin.
```

## Editing content

Each `.mdx` page declares a layout and frontmatter:

```mdx
---
layout: "@/layouts/Docs.astro"
title: "Quickstart"
description: "..."
---

Body in standard markdown + JSX.
```

Sidebar order is driven by `src/lib/nav.ts`. When you add a new MDX page,
add an entry in the matching section there. The Prev/Next navigation at
the bottom of MDX pages is computed from the same list.

## Pricing CTAs

`src/pages/pricing.astro` intentionally does not publish paid checkout
links until real Stripe Payment Links exist. Add links for:

- Pro 29 EUR / month
- Agency 99 EUR / month
- Lifetime Early Bird 199 EUR
- Production Readiness Audit 99 EUR

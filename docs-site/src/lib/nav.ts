// Single source of truth for top nav and docs sidebar.
//
// Order in `docsNav` controls the sidebar order. Each section's `items`
// list also drives prev/next pagination at the bottom of MDX pages.

export type NavLink = {
  title: string;
  href: string;
  external?: boolean;
};

export type DocsSection = {
  title: string;
  items: NavLink[];
};

export const topNav: NavLink[] = [
  { title: "Docs", href: "/docs/quickstart" },
  { title: "Pricing", href: "/pricing" },
  {
    title: "GitHub",
    href: "https://github.com/MakFly/deploy-shuttle",
    external: true,
  },
];

export const docsNav: DocsSection[] = [
  {
    title: "Getting Started",
    items: [
      { title: "Introduction", href: "/docs" },
      { title: "Install", href: "/docs/install" },
      { title: "Quickstart", href: "/docs/quickstart" },
    ],
  },
  {
    title: "Commands",
    items: [
      { title: "Overview", href: "/docs/commands" },
      { title: "init", href: "/docs/commands/init" },
      { title: "provision", href: "/docs/commands/provision" },
      { title: "deploy", href: "/docs/commands/deploy" },
      { title: "rollback", href: "/docs/commands/rollback" },
      { title: "secrets", href: "/docs/commands/secrets" },
      { title: "doctor", href: "/docs/commands/doctor" },
      { title: "harden", href: "/docs/commands/harden" },
      { title: "report", href: "/docs/commands/report" },
      { title: "license", href: "/docs/commands/license" },
    ],
  },
  {
    title: "Reference",
    items: [
      { title: "Check catalog", href: "/docs/checks" },
      { title: "Configuration", href: "/docs/config" },
      { title: "GitHub Action", href: "/docs/github-action" },
    ],
  },
];

// Flatten docsNav into an ordered list so MDX pages can compute prev/next.
export function flatDocsLinks(): NavLink[] {
  return docsNav.flatMap((section) => section.items);
}

export function findAdjacent(href: string): {
  prev?: NavLink;
  next?: NavLink;
} {
  const all = flatDocsLinks();
  const idx = all.findIndex((link) => link.href === href);
  if (idx === -1) return {};
  return {
    prev: idx > 0 ? all[idx - 1] : undefined,
    next: idx < all.length - 1 ? all[idx + 1] : undefined,
  };
}

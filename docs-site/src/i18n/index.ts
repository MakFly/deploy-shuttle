// Minimal i18n helpers. EN is the default locale (no prefix), FR lives
// under /fr/. Docs pages are EN-only by design (technical reference —
// a stale FR copy would be worse than none).

export type Locale = "en" | "fr";

export const locales: Locale[] = ["en", "fr"];

export function getLocale(pathname: string): Locale {
  return pathname === "/fr" || pathname.startsWith("/fr/") ? "fr" : "en";
}

// Map a path to its equivalent in the target locale. Docs paths always
// resolve to the EN docs.
export function localizePath(path: string, locale: Locale): string {
  const bare = path.replace(/^\/fr(?=\/|$)/, "") || "/";
  if (locale === "en") return bare;
  if (bare.startsWith("/docs")) return bare;
  return bare === "/" ? "/fr/" : `/fr${bare}`;
}

// Shared chrome strings (nav + footer). Page copy lives in each page
// component next to its markup.
export const chrome = {
  en: {
    docs: "Docs",
    pricing: "Pricing",
    github: "GitHub",
    star: "Star on GitHub",
    tagline: "VPS production-readiness CLI for Docker apps.",
    resources: "Resources",
    quickstart: "Quickstart",
    checks: "Check catalog",
    action: "GitHub Action",
    project: "Project",
    releases: "Releases",
    terms: "Terms of Sale",
    legal: "Legal Notice",
    openMenu: "Open menu",
  },
  fr: {
    docs: "Docs",
    pricing: "Tarifs",
    github: "GitHub",
    star: "Star sur GitHub",
    tagline: "Le CLI de production-readiness VPS pour apps Docker.",
    resources: "Ressources",
    quickstart: "Démarrage rapide",
    checks: "Catalogue des checks",
    action: "GitHub Action",
    project: "Projet",
    releases: "Releases",
    terms: "CGV",
    legal: "Mentions légales",
    openMenu: "Ouvrir le menu",
  },
} as const;

import type { APIRoute } from "astro";

type SitemapUrl = {
  path: string;
  changefreq: "weekly" | "monthly";
  priority: string;
  alternates?: {
    en: string;
    fr: string;
    default: string;
  };
};

const docs = [
  "/docs/",
  "/docs/quickstart/",
  "/docs/install/",
  "/docs/github-action/",
  "/docs/checks/",
  "/docs/config/",
  "/docs/commands/",
  "/docs/commands/init/",
  "/docs/commands/provision/",
  "/docs/commands/deploy/",
  "/docs/commands/rollback/",
  "/docs/commands/secrets/",
  "/docs/commands/doctor/",
  "/docs/commands/harden/",
  "/docs/commands/report/",
  "/docs/commands/license/",
];

const localized = [
  ["/", "/fr/"],
  ["/pricing/", "/fr/pricing/"],
  ["/legal/", "/fr/legal/"],
  ["/terms/", "/fr/terms/"],
] as const;

function buildUrls(): SitemapUrl[] {
  const urls: SitemapUrl[] = [];

  for (const [en, fr] of localized) {
    const alternates = { en, fr, default: en };
    urls.push({ path: en, changefreq: "weekly", priority: en === "/" ? "1.0" : "0.7", alternates });
    urls.push({ path: fr, changefreq: "weekly", priority: fr === "/fr/" ? "0.9" : "0.7", alternates });
  }

  for (const path of docs) {
    urls.push({ path, changefreq: "weekly", priority: path === "/docs/" ? "0.9" : "0.8" });
  }

  return urls;
}

function xmlEscape(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function absolute(path: string, origin: URL): string {
  return new URL(path, origin).toString();
}

export const GET: APIRoute = ({ site }) => {
  const origin = site ?? new URL("https://deployshuttle.dev");
  const urls = buildUrls()
    .map((url) => {
      const alternates = url.alternates
        ? [
            `<xhtml:link rel="alternate" hreflang="en" href="${xmlEscape(absolute(url.alternates.en, origin))}" />`,
            `<xhtml:link rel="alternate" hreflang="fr" href="${xmlEscape(absolute(url.alternates.fr, origin))}" />`,
            `<xhtml:link rel="alternate" hreflang="x-default" href="${xmlEscape(absolute(url.alternates.default, origin))}" />`,
          ].join("")
        : "";

      return [
        "<url>",
        `<loc>${xmlEscape(absolute(url.path, origin))}</loc>`,
        alternates,
        `<changefreq>${url.changefreq}</changefreq>`,
        `<priority>${url.priority}</priority>`,
        "</url>",
      ].join("");
    })
    .join("");

  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">${urls}</urlset>`,
    {
      headers: {
        "Content-Type": "application/xml; charset=utf-8",
      },
    },
  );
};

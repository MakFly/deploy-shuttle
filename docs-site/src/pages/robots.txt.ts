import type { APIRoute } from "astro";

export const GET: APIRoute = ({ site }) => {
  const origin = site ?? new URL("https://deployshuttle.dev");

  return new Response(
    [
      "User-agent: *",
      "Allow: /",
      "Disallow: /thank-you",
      "Disallow: /fr/thank-you",
      `Sitemap: ${new URL("/sitemap.xml", origin).toString()}`,
      "",
    ].join("\n"),
    {
      headers: {
        "Content-Type": "text/plain; charset=utf-8",
      },
    },
  );
};

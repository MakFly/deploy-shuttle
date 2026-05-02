// @ts-check

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import mdx from "@astrojs/mdx";
import expressiveCode from "astro-expressive-code";

const SITE = process.env.SITE_URL || "https://deployshuttle.dev";

// https://astro.build/config
export default defineConfig({
  site: SITE,
  vite: {
    plugins: [tailwindcss()],
  },
  // expressiveCode before mdx so MDX picks it up.
  integrations: [
    expressiveCode({
      themes: ["github-dark-default", "github-light"],
      themeCssSelector: (theme) => `.${theme.type}`,
      styleOverrides: {
        borderRadius: "0.5rem",
        codeFontSize: "0.85rem",
        frames: {
          shadowColor: "transparent",
        },
      },
      defaultProps: {
        wrap: true,
      },
    }),
    mdx(),
    react(),
  ],
});

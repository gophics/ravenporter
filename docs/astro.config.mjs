import { defineConfig } from "astro/config";
import mermaid from "astro-mermaid";
import starlight from "@astrojs/starlight";
import starlightCodeblockFullscreen from "starlight-codeblock-fullscreen";
import starlightLinksValidator from "starlight-links-validator";
import starlightThemeNext from "starlight-theme-next";

const isProduction = process.env.NODE_ENV === "production";
const site = process.env.DOCS_SITE_URL ?? (isProduction ? "https://gophics.github.io" : undefined);
const base = process.env.DOCS_BASE_PATH ?? (isProduction ? "/ravenporter" : undefined);

export default defineConfig({
  ...(site ? { site } : {}),
  ...(base ? { base } : {}),
  vite: {
    build: {
      // The Mermaid + ELK renderer is intentionally large but only used by the docs pages with diagrams.
      chunkSizeWarningLimit: 1600,
    },
  },
  markdown: {
    syntaxHighlight: {
      type: "shiki",
      excludeLangs: ["mermaid", "math"],
    },
  },
  integrations: [
    mermaid(),
    starlight({
      title: "RavenPorter Docs",
      description:
        "Documentation for RavenPorter, the pure-Go asset ingest and runtime-cooking library for games, tools, and asset pipelines.",
      favicon: "/favicon.svg",
      logo: {
        light: "./src/assets/logo-light.svg",
        dark: "./src/assets/logo-dark.svg",
        alt: "RavenPorter",
        replacesTitle: true,
      },
      editLink: {
        baseUrl: "https://github.com/gophics/ravenporter/edit/master/docs/",
      },
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/gophics/ravenporter",
        },
      ],
      lastUpdated: true,
      disable404Route: true,
      tableOfContents: {
        minHeadingLevel: 2,
        maxHeadingLevel: 3,
      },
      components: {
        Footer: "./src/components/DocsFooter.astro",
      },
      sidebar: [
        {
          label: "Overview",
          items: [
            { label: "Home", link: "/" },
            { slug: "overview" },
            { slug: "why-ravenporter" },
            { slug: "quick-start" },
            { slug: "supported-formats" },
          ],
        },
        {
          label: "Library Guides",
          items: [
            { slug: "import-workflows" },
            { slug: "profiles-and-presets" },
            { slug: "reports-and-validation" },
            { slug: "runtime-cache" },
            { slug: "json-ir-and-emitters" },
            { slug: "custom-detection-and-processing" },
          ],
        },
        {
          label: "Examples",
          items: [
            { slug: "examples" },
            { slug: "examples/quickstart-import" },
            { slug: "examples/input-sources" },
            { slug: "examples/batch-import" },
            { slug: "examples/asset-pipeline" },
            { slug: "examples/profiles-and-overrides" },
            { slug: "examples/inspect-and-export" },
            { slug: "examples/cache-roundtrip" },
          ],
        },
        {
          label: "CLI",
          collapsed: true,
          items: [{ slug: "cli-guide" }],
        },
        {
          label: "IR & Architecture",
          collapsed: true,
          items: [
            { slug: "ir-overview" },
            { slug: "scene-graph-and-indexing" },
            { slug: "geometry-and-materials" },
            { slug: "media-representation" },
            { slug: "animation-and-skeletons" },
            { slug: "import-pipeline-architecture" },
            { slug: "cache-format-model" },
            { slug: "error-and-issue-reference" },
          ],
        },
        {
          label: "Reference",
          collapsed: true,
          items: [
            { slug: "ravenporter-package" },
            { slug: "cache-package" },
            { slug: "detect-package" },
            { slug: "process-package" },
            { slug: "emit-json-package" },
            { slug: "validate-package" },
            { slug: "ir-asset-and-graph" },
            { slug: "ir-models-materials" },
            { slug: "ir-media-animation" },
          ],
        },
      ],
      plugins: [
        starlightThemeNext(),
        starlightLinksValidator({ errorOnRelativeLinks: false }),
        starlightCodeblockFullscreen(),
      ],
    }),
  ],
});

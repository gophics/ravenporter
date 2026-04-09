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
        {
          label: "Decoder Reference",
          collapsed: true,
          items: [
            { slug: "decoder-reference" },
            {
              label: "Models",
              collapsed: true,
              items: [
                { slug: "decoder-reference/models/alembic" },
                { slug: "decoder-reference/models/bvh" },
                { slug: "decoder-reference/models/collada" },
                { slug: "decoder-reference/models/fbx" },
                { slug: "decoder-reference/models/gltf" },
                { slug: "decoder-reference/models/obj" },
                { slug: "decoder-reference/models/ply" },
                { slug: "decoder-reference/models/stl" },
                { slug: "decoder-reference/models/three-d-studio" },
                { slug: "decoder-reference/models/three-mf" },
                { slug: "decoder-reference/models/usd" },
              ],
            },
            {
              label: "Images",
              collapsed: true,
              items: [
                { slug: "decoder-reference/images/bmp" },
                { slug: "decoder-reference/images/dds" },
                { slug: "decoder-reference/images/exr" },
                { slug: "decoder-reference/images/hdr" },
                { slug: "decoder-reference/images/jpeg" },
                { slug: "decoder-reference/images/ktx" },
                { slug: "decoder-reference/images/png" },
                { slug: "decoder-reference/images/psd" },
                { slug: "decoder-reference/images/tga" },
                { slug: "decoder-reference/images/tiff" },
                { slug: "decoder-reference/images/webp" },
              ],
            },
            {
              label: "Audio",
              collapsed: true,
              items: [
                { slug: "decoder-reference/audio/aiff" },
                { slug: "decoder-reference/audio/flac" },
                { slug: "decoder-reference/audio/mp3" },
                { slug: "decoder-reference/audio/ogg" },
                { slug: "decoder-reference/audio/opus" },
                { slug: "decoder-reference/audio/wav" },
              ],
            },
            {
              label: "Fonts",
              collapsed: true,
              items: [
                { slug: "decoder-reference/fonts/otf" },
                { slug: "decoder-reference/fonts/ttf" },
                { slug: "decoder-reference/fonts/woff" },
                { slug: "decoder-reference/fonts/woff2" },
              ],
            },
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

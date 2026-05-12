import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://northstar.example.com",
  integrations: [
    starlight({
      title: "Northstar",
      description:
        "Self-hosted iOS life dashboard — Actual finance + native goals + WHOOP biometrics with a Claude tool-use chat layer.",
      social: [
        { icon: "github", label: "GitHub", href: "https://github.com/builtbybrayden/northstar" },
      ],
      editLink: {
        baseUrl: "https://github.com/builtbybrayden/northstar/edit/main/docs-site/",
      },
      sidebar: [
        {
          label: "Getting started",
          items: [
            { label: "What is Northstar?", slug: "getting-started/overview" },
            { label: "Quickstart (Docker)", slug: "getting-started/quickstart" },
            { label: "One-shot installer", slug: "getting-started/installer" },
            { label: "Pair your iPhone", slug: "getting-started/pairing" },
          ],
        },
        {
          label: "The three pillars",
          items: [
            { label: "Finance", slug: "pillars/finance" },
            { label: "Goals", slug: "pillars/goals" },
            { label: "Health", slug: "pillars/health" },
            { label: "Ask Claude", slug: "pillars/ask" },
          ],
        },
        {
          label: "Configuration",
          items: [
            { label: "Environment variables", slug: "configuration/env" },
            { label: "Sidecars (Actual + WHOOP)", slug: "configuration/sidecars" },
            { label: "Notifications", slug: "configuration/notifications" },
            { label: "Backups (Litestream)", slug: "configuration/backups" },
          ],
        },
        {
          label: "API reference",
          items: [
            { label: "Authentication & pairing", slug: "api/auth" },
            { label: "Finance", slug: "api/finance" },
            { label: "Goals", slug: "api/goals" },
            { label: "Health", slug: "api/health" },
            { label: "Notifications", slug: "api/notifications" },
            { label: "AI (chat + tools)", slug: "api/ai" },
          ],
        },
        {
          label: "Operating",
          items: [
            { label: "Upgrading", slug: "operating/upgrades" },
            { label: "Troubleshooting", slug: "operating/troubleshooting" },
            { label: "Threat model", slug: "operating/threat-model" },
          ],
        },
        {
          label: "Contributing",
          items: [
            { label: "Repo layout", slug: "contributing/layout" },
            { label: "Dev environment", slug: "contributing/dev" },
            { label: "Style guide", slug: "contributing/style" },
          ],
        },
      ],
      customCss: ["./src/styles/custom.css"],
    }),
  ],
});

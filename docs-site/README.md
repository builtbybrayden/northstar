# northstar-docs

Astro + Starlight documentation site for Northstar.

## Develop

```sh
npm install
npm run dev
# open http://localhost:4321/
```

## Build

```sh
npm run build
npm run preview
```

Output is a static site in `dist/` — host on Cloudflare Pages, Netlify,
GitHub Pages, or anywhere that serves static HTML.

## Layout

```
src/
├── content/
│   └── docs/                Markdown / MDX pages
│       ├── getting-started/
│       ├── pillars/
│       ├── configuration/
│       ├── api/
│       ├── operating/
│       └── contributing/
├── styles/                  custom CSS (accent color overrides)
└── content.config.ts        Starlight schema wiring
astro.config.mjs             Starlight integration + sidebar tree
```

## Editing

Every page links back to its source in GitHub via Starlight's `editLink`
config. Update `astro.config.mjs` if you fork to a different repo.

# web

pnpm workspace for the demo UIs. Not initialised yet.

Plan:

- `packages/` holds shared UI: `brand` (LocalStack colors, logos and the Exo typeface, which every demo UI imports) and, later, components and a vendor "look inside" panel.
- `apps/<demo>/` holds one UI per demo, for example `apps/acme-commerce-lenders`.
- Each app is built as a **static SPA** (TanStack Router in SPA mode, not an SSR server) and its output is copied into the matching Go app's `internal/ui/assets/`, where `go:embed` packs it into the binary. Node is a build-time tool only. The demo binaries need no Node and no network at run time.

Do not commit build output. `.gitignore` tracks only `placeholder.html` in each `assets/` directory.

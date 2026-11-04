# web

pnpm workspace for the demo UIs, set up the same way as `wondertwin-web`.

- Node 22 (`.node-version`) and pnpm 10.33, pinned by `packageManager`. Run `corepack enable` and pnpm picks the right version.
- `pnpm-workspace.yaml` carries the supply-chain policy: a seven-day wait before a new release is installed, no trust downgrades, and no dependencies from outside the registry.
- Biome formats and lints (`pnpm lint`, `pnpm format`), with the same rules as `wondertwin-web`.
- Each UI is React 18, Vite, TanStack Router and Query, and Tailwind 4, with Vitest for tests.

| Path | What it holds |
|---|---|
| `packages/brand` | LocalStack's colors, logos, typeface and the `cva` button and badge variants, which every demo UI imports |
| `apps/<demo>` | One demo's UI, such as `apps/acme-commerce-lenders` |

## Commands

```sh
pnpm install            # once
pnpm dev                # the UI on Vite; the API must be running on :8080 (make kit)
pnpm lint               # Biome
pnpm type-check
pnpm test
pnpm build              # builds every UI to its dist/
```

To put a built UI into its Go binary, run `make web APP=<demo>` from the repo root. It builds the UI and copies it into the binary's embedded assets, which git ignores.

## Rules

- Every UI is a static single-page app. Node is a build-time tool only, and the binary needs no Node and no network at run time.
- Use the brand package's tokens and variants. Do not add colors that are not in it.
- Mint is the one action color.
- A new dependency has to pass the supply-chain policy. If it does not, prefer removing it to excepting it.

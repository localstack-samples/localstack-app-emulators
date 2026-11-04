# LocalStack for SaaS Apps

Demos of LocalStack for SaaS Apps. It lets you build and test against the third-party APIs your application depends on, such as Stripe, Plaid and Xero, using app emulators that run locally. An app emulator is a local stand-in for a third-party API, so you need no vendor sandbox and no real account.

## Demos

### ACME Commerce Lenders

Status: In Development

ACME Commerce Lenders is a fictional working-capital lender for ecommerce companies. A merchant applies for working capital, and the lender reads their sales from Stripe and their bank data from Plaid to make an offer. When the merchant accepts, the lender funds the advance through Stripe Connect into the bank account the merchant verified with Plaid, and the settlement appears in the merchant's Xero books. The lender's backend runs on LocalStack's AWS emulation and every third-party system is an app emulator, so the whole flow runs locally, resets in one click, and needs no vendor sandbox or real account.

Code and docs: [apps/acme-commerce-lenders](apps/acme-commerce-lenders)

#### Demo Scenarios

**Apply, offer, accept and settle.** A merchant applies for working capital and receives an offer. They accept it, the advance is funded, and the settlement appears in their Xero books.

## How demos run

Each demo is one Go binary, `cmd/kit`, that serves the demo's UI and runs its logic. It runs beside LocalStack, which hosts the app emulators and serves each one on its own host name through LocalStack's gateway, so you start LocalStack with `lstk` first. The demo calls each vendor through a local recording proxy (`shared/wire`), which is how the inspector shows what the demo sends. Each vendor's app emulator is hosted by LocalStack or run by you (`<VENDOR>_EMULATOR_URL`), so you can build against an app emulator locally and move it to LocalStack once LocalStack includes it.

Demos use no AWS services yet. Each demo's `core` package depends only on small interfaces (`Store`, `Publisher`), which is where AWS services plug in later.

## Repo layout

Each demo is a Go app under `apps/`. The repo is a Go workspace (`go.work`), so every app and the shared module build together. Adding a demo adds a directory under `apps/` and one line to `go.work`.

| Path | What it holds |
|---|---|
| `apps/<demo>/` | One demo: its Go code, its docs and its embedded UI |
| `shared/` | Code every demo uses: the recording proxy for vendor calls, vendor base URL config, and a launcher for running app emulators without LocalStack |
| `web/` | pnpm workspace for the demo UIs, built as static assets that the Go binaries embed |
| `templates/demo-docs/` | The three docs every demo carries: Usage Guide, Demo Scenarios, Troubleshooting Guide |

## Working in the repo

```sh
make vet                        # go vet for every module
make test                       # go test for every module
make web APP=<demo>            # builds that demo's UI into its binary (needs Node 22 and pnpm)
make build                      # builds every apps/<demo>/cmd/<name> to bin/<demo>-<name>
make kit APP=<demo>             # builds, then runs that demo's binary
make check-kits                 # fails if a demo has no cmd/kit
```

`go test -tags live ./wire` in `shared/` also runs a check against a LocalStack you have running.

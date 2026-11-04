# localstack-app-emulators

Demos of LocalStack app emulators. An app emulator is a local stand-in for a third-party API such as Stripe, Plaid or Xero, so you can build and test against it without a vendor sandbox or a real account.

## Demos

### ACME Commerce Lenders

Status: In Development

ACME Commerce Lenders is a fictional working-capital lender for ecommerce companies. A merchant applies for working capital, and the lender reads their sales from Stripe and their bank data from Plaid to make an offer. When the merchant accepts, the lender funds the advance through Stripe Connect into the bank account the merchant verified with Plaid, and the settlement appears in the merchant's Xero books. The lender's backend runs on LocalStack's AWS emulation and every third-party system is an app emulator, so the whole flow runs locally, resets in one click, and needs no vendor sandbox or real account.

Code and docs: [apps/acme-commerce-lenders](apps/acme-commerce-lenders)

#### Demo Scenarios

**Apply, offer, accept and settle.** A merchant applies for working capital and receives an offer. They accept it, the advance is funded, and the settlement appears in their Xero books.

## Repo layout

Each demo is a Go app under `apps/`. The repo is a Go workspace (`go.work`), so every app and the shared module build together. Adding a demo adds a directory under `apps/` and one line to `go.work`.

| Path | What it holds |
|---|---|
| `apps/<demo>/` | One demo: its Go code, its docs and its embedded UI |
| `shared/` | Code every demo uses: starting app emulators and waiting for them to be healthy, and vendor base URL config |
| `templates/demo-docs/` | The three docs every demo carries: Usage Guide, Demo Scenarios, Troubleshooting Guide |

## Every demo has an offline kit

Each demo ships `cmd/kit`: one binary that serves the UI, runs the demo logic and starts the app emulators itself, with no network and no LocalStack. A demo may also have a LocalStack mode (`cmd/lambda`), which adds LocalStack's AWS emulation. `make check-kits` fails a demo that has no `cmd/kit`.

## Working in the repo

```sh
make vet                        # go vet for every module
make test                       # go test for every module
make build                      # builds every apps/<demo>/cmd/<name> to bin/<demo>-<name>
make kit APP=<demo>             # builds, then runs that demo's offline kit
make check-kits                 # fails if a demo has no cmd/kit
```

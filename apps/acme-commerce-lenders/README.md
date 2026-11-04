# ACME Commerce Lenders

Status: In Development

ACME Commerce Lenders is a fictional working-capital lender for ecommerce companies. A merchant applies for working capital, and the lender reads their sales from Stripe and their bank data from Plaid to make an offer. When the merchant accepts, the lender funds the advance through Stripe Connect into the bank account the merchant verified with Plaid, and the settlement appears in the merchant's Xero books. The lender's backend runs on LocalStack's AWS emulation and every third-party system is an app emulator, so the whole flow runs locally, resets in one click, and needs no vendor sandbox or real account.

This describes the planned flow. Only the scaffold is built so far.

## Two run modes

| | Offline kit (default) | LocalStack mode |
|---|---|---|
| Entry point | `cmd/kit` | `cmd/lambda` |
| Runs | One binary: the UI, the lender logic and the three app emulators | The lender logic as Lambdas on LocalStack, with the app emulators reached over the network |
| Needs | No network and no LocalStack | LocalStack, plus a prior activation if the network is down |

Both modes run the same lender logic in `core/`, which depends only on small interfaces (`Store`, `Publisher`).

## Docs

- [Usage Guide](docs/usage.md)
- [Demo Scenarios](docs/scenarios.md)
- [Troubleshooting Guide](docs/troubleshooting.md)

New demos start from `templates/demo-docs/`.

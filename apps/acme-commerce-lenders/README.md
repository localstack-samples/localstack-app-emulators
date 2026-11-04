# ACME Commerce Lenders

Status: In Development

ACME Commerce Lenders is a fictional working-capital lender for ecommerce companies. A merchant applies for working capital, and the lender reads their sales from Stripe and their bank data from Plaid to make an offer. When the merchant accepts, the lender funds the advance through Stripe Connect into the bank account the merchant verified with Plaid, and the settlement appears in the merchant's Xero books. Every third-party system is an app emulator, so the whole flow runs locally, resets in one click, and needs no vendor sandbox or real account.

This describes the planned flow. Only the scaffold is built so far.

## How it runs

`cmd/kit` is the demo: one binary that serves the UI and runs the lender logic. It runs beside LocalStack, which hosts the Stripe, Plaid and Xero app emulators. See [Getting started](docs/usage.md#getting-started).

## Docs

- [Usage Guide](docs/usage.md)
- [Demo Scenarios](docs/scenarios.md)
- [Troubleshooting Guide](docs/troubleshooting.md)

New demos start from `templates/demo-docs/`.

# localstack-app-emulators

Demos of LocalStack app emulators. An app emulator is a local stand-in for a third-party API such as Stripe, Plaid or Xero, so you can build and test against it without a vendor sandbox or a real account.

## Demos

### ACME Commerce Lenders

Status: In Development

ACME Commerce Lenders is a fictional working-capital lender for ecommerce companies. A merchant applies for working capital, and the lender reads their sales from Stripe and their bank data from Plaid to make an offer. When the merchant accepts, the lender funds the advance through Stripe Connect into the bank account the merchant verified with Plaid, and the settlement appears in the merchant's Xero books. The lender's backend runs on LocalStack's AWS emulation and every third-party system is an app emulator, so the whole flow runs locally, resets in one click, and needs no vendor sandbox or real account.

#### Demo Scenarios

**Apply, offer, accept and settle.** A merchant applies for working capital and receives an offer. They accept it, the advance is funded, and the settlement appears in their Xero books.

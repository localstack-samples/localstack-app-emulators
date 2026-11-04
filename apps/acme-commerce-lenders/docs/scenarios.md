# Demo Scenarios

Status: Ready to rehearse

There is one scenario. It is the whole demo.

## A working-capital advance, from application to the books

**For.** LocalStack DevRel and engineers showing "LocalStack for SaaS Apps", and design partners who build software on Stripe, Plaid and Xero.

**The situation.** ACME Commerce Lenders is a fictional lender that advances working capital to ecommerce companies. Harbor & Pine Outfitters, an online outdoor-gear merchant, applies. The lender has no paperwork to read. It asks the merchant's own systems: Stripe for card sales and payouts, Plaid for the bank account, Xero for the books.

**What the audience sees.**

1. **Apply for working capital.** One click. The lender connects to Stripe, Plaid and Xero, reads what each holds and prices an offer. Each step shows as it finishes. The offer is an advance of $30,000 for a flat 8% fee, so $32,400 to repay. Every number in it names the vendor call it came from.
2. **Accept the offer and fund it.** One click. The lender registers the merchant's bank account in Stripe using the numbers Plaid holds, moves the advance to the merchant's Stripe account and pays it out to their bank. It then records the advance in the merchant's Xero books as money received against a loan.
3. **Reset.** One click puts everything back at the start, ready for the next audience.

**The second tab.** Open the inspector beside the demo (see [the inspector](usage.md#the-inspector)). Each click fills it with the calls behind it, in plain language: "Read the merchant's card sales from Stripe", "Fetched the bank account and routing numbers from Plaid", "Recorded the advance in the merchant's books". Open a row to show the request and the response. The badge on each row says whether LocalStack or a process on the laptop served it.

**What it proves.** An app that depends on three SaaS products can be built, run and shown end to end on a laptop, against app emulators that behave like the real vendors, with no vendor accounts and no sandbox quotas. Two clicks do real work across all three systems, and the seeded world is the same every run, so the demo never surprises the presenter.

**What it does not show.** Repayment, and any AWS service. The demo runs beside LocalStack and uses none of its AWS emulation.

### App emulators and builds

| App emulator | Served by | Build |
|---|---|---|
| Stripe | LocalStack | The LocalStack release's Stripe app emulator |
| Plaid | A process you run | `WonderTwin-AI/wondertwin` `twin-plaid`, verified at `8e4a7235` |
| Xero | A process you run | `WonderTwin-AI/wondertwin` `twin-xero`, verified at `8e4a7235` |

The Xero app emulator must be new enough to create accounts and bank transactions, which is what settling needs. See [Troubleshooting](troubleshooting.md). Plaid and Xero move to LocalStack when a release includes them. Unset `PLAID_EMULATOR_URL` and `XERO_EMULATOR_URL` and nothing else changes.

### Before each showing

1. `lstk` is running, and the app emulators are healthy (`make kit APP=acme-commerce-lenders ARGS=ready`).
2. The UI shows all three app emulators ready.
3. Press **Reset**, so the demo starts at **Apply for working capital**.

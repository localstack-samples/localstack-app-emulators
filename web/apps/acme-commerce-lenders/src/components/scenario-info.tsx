import { Info } from 'lucide-react'

/** The scenario and story, in a panel that opens on hover or keyboard focus. */
export function ScenarioInfo() {
  return (
    <div className="group relative">
      <button
        type="button"
        aria-label="About this demo"
        aria-describedby="scenario-panel"
        className="rounded-full p-1 text-muted-foreground hover:text-foreground focus-visible:text-foreground"
      >
        <Info className="size-4" />
      </button>
      <div
        id="scenario-panel"
        role="tooltip"
        className="invisible absolute left-0 top-full z-10 mt-2 w-96 rounded-lg border bg-popover p-4 text-sm text-popover-foreground opacity-0 shadow-lg transition-opacity group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100"
      >
        <p className="font-medium">
          A working-capital advance, from application to the books
        </p>
        <p className="mt-2 text-popover-foreground/80">
          ACME Commerce Lenders is a fictional lender. Harbor &amp; Pine
          Outfitters, an online outdoor-gear merchant, applies. The lender has
          no paperwork to read, so it asks the merchant&apos;s own systems:
          Stripe for card sales and payouts, Plaid for the bank account, Xero
          for the books.
        </p>
        <ol className="mt-2 list-decimal space-y-1 pl-5 text-popover-foreground/80">
          <li>Apply: the lender reads all three and prices an offer.</li>
          <li>
            Accept and fund: the advance is paid to the merchant&apos;s bank and
            recorded in their Xero books.
          </li>
          <li>Reset: back to the start for the next audience.</li>
        </ol>
        <p className="mt-2 text-popover-foreground/80">
          An app that depends on three SaaS products is built, run and shown end
          to end on a laptop, with no vendor accounts and no sandbox quotas.
        </p>
      </div>
    </div>
  )
}

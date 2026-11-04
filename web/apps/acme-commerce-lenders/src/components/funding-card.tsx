import type { Funding } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Card, CardDescription, CardTitle } from '@/components/ui/card'
import { money } from '@/lib/format'

function Row({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b py-2 last:border-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right">{children}</dd>
    </div>
  )
}

/** How the advance reached the merchant. */
export function FundingCard({ funding }: { funding: Funding }) {
  return (
    <Card data-testid="funding">
      <div className="flex items-center justify-between">
        <CardTitle>The advance reached the merchant</CardTitle>
        <Badge variant="success">{money(funding.amount_cents)}</Badge>
      </div>
      <dl className="text-sm">
        <Row label="Bank account, verified with Plaid">
          Checking ending {funding.bank_last4}
          <span className="ml-2 font-mono text-xs text-muted-foreground">
            routing {funding.routing_number}
          </span>
        </Row>
        <Row label="Transfer to the merchant's Stripe account">
          <span className="font-mono text-xs">{funding.transfer_id}</span>
        </Row>
        <Row label="Payout to their bank">
          <span className="font-mono text-xs">{funding.payout_id}</span>
          <Badge variant="outline" className="ml-2">
            {funding.payout_status}
          </Badge>
        </Row>
      </dl>
      <CardDescription>
        The settlement in the merchant's Xero books is not built yet.
      </CardDescription>
    </Card>
  )
}

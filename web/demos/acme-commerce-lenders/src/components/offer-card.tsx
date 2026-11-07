import type { Offer, OfferInput } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Card, CardDescription, CardTitle } from '@/components/ui/card'
import { money } from '@/lib/format'

const vendorNames: Record<OfferInput['source'], string> = {
  stripe: 'Stripe',
  plaid: 'Plaid',
  xero: 'Xero',
}

function Figure({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="font-heading text-3xl font-bold">{value}</div>
    </div>
  )
}

/** The offer, and every fact it rests on with the vendor call it came from. */
export function OfferCard({ offer }: { offer: Offer }) {
  return (
    <Card data-testid="offer">
      <div className="flex items-center justify-between">
        <CardTitle>{offer.approved ? 'The offer' : 'Declined'}</CardTitle>
        <Badge variant={offer.approved ? 'success' : 'destructive'}>
          {offer.approved ? 'Approved' : 'Declined'}
        </Badge>
      </div>

      {offer.approved ? (
        <div className="grid grid-cols-3 gap-6">
          <div>
            <div className="text-sm text-muted-foreground">Advance</div>
            <div className="font-heading text-5xl font-bold text-primary">
              {money(offer.advance_cents)}
            </div>
          </div>
          <Figure
            label={`Fee (${offer.fee_percent}%)`}
            value={money(offer.fee_cents)}
          />
          <Figure label="To repay" value={money(offer.total_repay_cents)} />
        </div>
      ) : null}

      <CardDescription>{offer.reason}</CardDescription>

      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-muted-foreground">
            <th className="py-2 pr-3 font-medium">From</th>
            <th className="py-2 pr-3 font-medium">What</th>
            <th className="py-2 pr-3 text-right font-medium">Value</th>
            <th className="py-2 pr-3 font-medium">Call</th>
            <th className="py-2 font-medium">In the offer</th>
          </tr>
        </thead>
        <tbody>
          {offer.inputs.map((i) => (
            <tr
              key={`${i.source}-${i.label}`}
              className="border-b last:border-0"
            >
              <td className="py-2 pr-3">
                <Badge variant="info">{vendorNames[i.source]}</Badge>
              </td>
              <td className="py-2 pr-3">{i.label}</td>
              <td className="py-2 pr-3 text-right font-mono">{i.value}</td>
              <td className="py-2 pr-3 font-mono text-xs text-muted-foreground">
                {i.call}
              </td>
              <td className="py-2">
                {i.used ? (
                  <Badge variant="success">Used</Badge>
                ) : (
                  <Badge variant="outline">Shown only</Badge>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  )
}

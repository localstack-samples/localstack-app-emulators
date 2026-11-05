import { useState } from 'react'
import { AppHeader } from '@/components/app-header'
import { ModeBanner } from '@/components/mode-banner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { CallRow, vendorName } from './call-row'
import { useEvents } from './use-events'

const vendors = ['stripe', 'plaid', 'xero']

/**
 * Every call the demo makes to Stripe, Plaid and Xero, as it happens. It is
 * meant to sit in a second browser tab beside the demo.
 */
export function InspectorPage() {
  const { events, error } = useEvents()
  const [vendor, setVendor] = useState<string | null>(null)

  const shown = events.filter((e) => vendor === null || e.vendor === vendor)

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6 px-6 py-8">
      <AppHeader
        view="inspector"
        subtitle="ACME Commerce Lenders"
        title={
          <h1 className="font-heading text-xl font-bold leading-tight">
            Demo Inspector
          </h1>
        }
      />
      <ModeBanner />

      {error && (
        <Card role="alert">
          <p className="text-destructive">
            Cannot read the calls: {error}. Is the demo still running?
          </p>
        </Card>
      )}

      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant={vendor === null ? 'default' : 'outline'}
          aria-pressed={vendor === null}
          onClick={() => setVendor(null)}
        >
          All
        </Button>
        {vendors.map((v) => (
          <Button
            key={v}
            size="sm"
            variant={vendor === v ? 'default' : 'outline'}
            aria-pressed={vendor === v}
            onClick={() => setVendor(v)}
          >
            {vendorName(v)}
          </Button>
        ))}
        <span className="ml-auto text-sm text-muted-foreground">
          {shown.length} {shown.length === 1 ? 'call' : 'calls'}
        </span>
      </div>

      <Card className="p-0">
        {shown.length === 0 ? (
          <p className="px-4 py-10 text-center text-sm text-muted-foreground">
            No calls yet. Run the demo in the other tab and they appear here.
          </p>
        ) : (
          <ul>
            {[...shown].reverse().map((e) => (
              <CallRow key={e.id} event={e} />
            ))}
          </ul>
        )}
      </Card>
    </div>
  )
}

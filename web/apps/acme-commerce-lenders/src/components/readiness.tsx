import { useQuery } from '@tanstack/react-query'
import { statusQueries } from '@/api/queries'
import type { EmulatorStatus } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

const vendorNames: Record<string, string> = {
  stripe: 'Stripe',
  plaid: 'Plaid',
  xero: 'Xero',
}

const sourceNames: Record<EmulatorStatus['source'], string> = {
  localstack: 'LocalStack',
  local: 'Local',
}

function Pill({ e }: { e: EmulatorStatus }) {
  const ok = e.state === 'healthy'
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3"
      data-testid={`emulator-${e.vendor}`}
    >
      <div>
        <div className="font-medium">{vendorNames[e.vendor] ?? e.vendor}</div>
        <div className="text-xs text-muted-foreground">
          {sourceNames[e.source]}
        </div>
      </div>
      <Badge variant={ok ? 'success' : 'destructive'}>
        {ok ? 'Ready' : e.state}
      </Badge>
    </div>
  )
}

/** Whether the three app emulators are up and the world is seeded. */
export function Readiness() {
  const { data, error, isPending } = useQuery(statusQueries.current())

  if (isPending) {
    return (
      <Card>
        <p className="text-sm text-muted-foreground">
          Checking the app emulators…
        </p>
      </Card>
    )
  }
  if (error || !data) {
    return (
      <Card role="alert">
        <p className="text-destructive">
          The demo's API did not answer
          {error instanceof Error ? `: ${error.message}` : '.'}
        </p>
      </Card>
    )
  }

  const problems = data.emulators.filter((e) => e.state !== 'healthy')
  return (
    <Card>
      <div className="grid grid-cols-3 gap-3">
        {data.emulators.map((e) => (
          <Pill key={e.vendor} e={e} />
        ))}
      </div>
      {problems.map((e) => (
        <p key={e.vendor} className="text-sm text-destructive" role="alert">
          {vendorNames[e.vendor] ?? e.vendor}: {e.hint ?? e.state}
        </p>
      ))}
      {problems.length === 0 && !data.seeded && (
        <p className="text-sm text-muted-foreground" role="status">
          The world is not seeded yet. Press Reset to seed it.
        </p>
      )}
      {data.ready && data.facts && (
        <p className="text-sm text-muted-foreground">
          {data.facts.merchant} is seeded, with sales up to {data.facts.as_of}.
        </p>
      )}
    </Card>
  )
}

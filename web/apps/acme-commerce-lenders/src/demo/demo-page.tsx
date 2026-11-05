import { useQuery } from '@tanstack/react-query'
import { statusQueries, targetQueries } from '@/api/queries'
import { FundingCard } from '@/components/funding-card'
import { Header } from '@/components/header'
import { ModeBanner } from '@/components/mode-banner'
import { OfferCard } from '@/components/offer-card'
import { Readiness } from '@/components/readiness'
import { StepList } from '@/components/step-list'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardTitle } from '@/components/ui/card'
import { acceptSteps, applySteps, useDemoFlow } from './use-demo-flow'

/**
 * The demo, in two clicks: apply, then accept. Reset puts it back at the start.
 */
export function DemoPage() {
  const flow = useDemoFlow()
  const status = useQuery(statusQueries.current())
  const targets = useQuery(targetQueries.current())
  const production = targets.data?.mode === 'production'
  const badUrl = targets.data?.lanes.some((l) => l.error) === true
  // In production mode the app emulators and the seeded world are not what the
  // run reads, so only a URL that cannot be called holds it back.
  const ready = production ? !badUrl : status.data?.ready === true
  const merchant = status.data?.facts?.merchant ?? 'The merchant'

  const app = flow.application
  const offer = app?.offer
  const applied = flow.steps.apply.status !== 'pending'
  const acceptStarted = flow.steps.accept.status !== 'pending'
  const canAccept =
    app?.status === 'offered' && offer?.approved === true && !acceptStarted

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6 px-6 py-8">
      <Header onReset={flow.reset} busy={flow.busy} />
      <ModeBanner />

      {flow.resetError && (
        <Card role="alert">
          <p className="text-destructive">Reset failed: {flow.resetError}</p>
        </Card>
      )}

      {!production && <Readiness />}

      <Card>
        <CardTitle>{merchant}</CardTitle>
        <CardDescription>
          An ecommerce company that applies to ACME Commerce Lenders for working
          capital.
        </CardDescription>
        {applied ? (
          <StepList
            keys={applySteps}
            steps={flow.steps}
            onRetry={flow.retry}
            busy={flow.busy}
          />
        ) : (
          <div className="flex items-center gap-4">
            <Button
              size="lg"
              onClick={flow.apply}
              disabled={!ready || flow.busy}
            >
              Apply for working capital
            </Button>
            {!ready && (
              <span className="text-sm text-muted-foreground">
                {badUrl
                  ? 'Fix the base URL above to run the demo.'
                  : 'Waiting for the app emulators and the seeded world.'}
              </span>
            )}
          </div>
        )}
      </Card>

      {offer && <OfferCard offer={offer} />}

      {(canAccept || acceptStarted) && (
        <Card>
          <CardTitle>Accept the offer</CardTitle>
          {acceptStarted ? (
            <StepList
              keys={acceptSteps}
              steps={flow.steps}
              onRetry={flow.retry}
              busy={flow.busy}
            />
          ) : (
            <div>
              <Button size="lg" onClick={flow.accept} disabled={flow.busy}>
                Accept the offer and fund it
              </Button>
            </div>
          )}
        </Card>
      )}

      {app?.funding &&
        (app.status === 'funded' || app.status === 'settled') && (
          <FundingCard funding={app.funding} settlement={app.settlement} />
        )}
    </div>
  )
}

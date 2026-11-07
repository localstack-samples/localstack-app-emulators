import { AlertTriangle, Check, Circle, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { type StepKey, type StepState, stepLabels } from '@/demo/use-demo-flow'
import { duration } from '@/lib/format'
import { cn } from '@/lib/utils'

function Icon({ status }: { status: StepState['status'] }) {
  switch (status) {
    case 'done':
      return <Check className="size-5 text-primary" aria-label="done" />
    case 'running':
      return (
        <Loader2
          className="size-5 animate-spin text-accent"
          aria-label="running"
        />
      )
    case 'error':
      return (
        <AlertTriangle
          className="size-5 text-destructive"
          aria-label="failed"
        />
      )
    default:
      return (
        <Circle
          className="size-5 text-muted-foreground"
          aria-label="not started"
        />
      )
  }
}

/** The steps one click runs, with where each has got to. */
export function StepList({
  keys,
  steps,
  onRetry,
  busy,
}: {
  keys: StepKey[]
  steps: Record<StepKey, StepState>
  onRetry: () => void
  busy: boolean
}) {
  return (
    <ol className="flex flex-col gap-3">
      {keys.map((key) => {
        const s = steps[key]
        return (
          <li key={key} className="flex flex-col gap-1" data-step={key}>
            <div className="flex items-center gap-3">
              <Icon status={s.status} />
              <span
                className={cn(
                  s.status === 'pending' && 'text-muted-foreground',
                  s.status === 'error' && 'text-destructive',
                )}
              >
                {stepLabels[key]}
              </span>
              {s.ms !== undefined && (
                <span className="ml-auto font-mono text-xs text-muted-foreground">
                  {duration(s.ms)}
                </span>
              )}
            </div>
            {s.status === 'error' && (
              <div className="ml-8 flex items-center gap-3">
                <p className="text-sm text-destructive" role="alert">
                  {s.error}
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={onRetry}
                  disabled={busy}
                >
                  Retry
                </Button>
              </div>
            )}
          </li>
        )
      })}
    </ol>
  )
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Settings } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import { targetQueries } from '@/api/queries'
import type { RunStyle, Targets } from '@/api/types'
import { TargetsDialog } from '@/components/targets-dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const styleNote: Record<RunStyle, string> = {
  dry: 'Dry run: each request is built and shown in the inspector, and nothing is sent.',
  live: 'Live: each request is sent to the vendor with no credential, so expect the vendor to refuse it.',
}

/**
 * Says which mode the demo is in, and cannot be dismissed. Demo mode calls the
 * app emulators. Production mode addresses the vendors' own services, to show
 * that only the base URL changed.
 */
export function ModeBanner() {
  const { data } = useQuery(targetQueries.current())
  const [open, setOpen] = useState(false)
  const queryClient = useQueryClient()
  const setStyle = useMutation({
    mutationFn: (style: RunStyle) => api.setTargets({ style }),
    onSuccess: (t: Targets) => queryClient.setQueryData(['targets'], t),
  })

  if (!data) return null
  const production = data.mode === 'production'
  const problems = data.lanes.filter((l) => l.error)

  return (
    <section
      aria-label="Mode"
      data-mode={data.mode}
      className={cn(
        'flex flex-col gap-2 rounded-xl border-2 px-4 py-3',
        production
          ? 'border-destructive bg-destructive/15'
          : 'border-primary/50 bg-primary/10',
      )}
    >
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <span
            className={cn(
              'rounded-full px-3 py-1 text-sm font-bold uppercase tracking-wide',
              production
                ? 'bg-destructive text-destructive-foreground'
                : 'bg-primary text-primary-foreground',
            )}
          >
            {production ? 'Production Mode' : 'Demo Mode'}
          </span>
          <span className="text-sm">
            {production
              ? 'Calls are addressed to the vendors’ own services.'
              : 'Every call goes to an app emulator.'}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {production && (
            <div
              role="radiogroup"
              aria-label="Run style"
              className="flex gap-1"
            >
              {(['dry', 'live'] as const).map((s) => (
                <Button
                  key={s}
                  role="radio"
                  aria-checked={data.style === s}
                  size="sm"
                  variant={data.style === s ? 'default' : 'outline'}
                  onClick={() => setStyle.mutate(s)}
                  disabled={setStyle.isPending}
                >
                  {s === 'dry' ? 'Dry run' : 'Live'}
                </Button>
              ))}
            </div>
          )}
          <Button
            variant="outline"
            className="px-2.5"
            aria-label="Change base URLs"
            onClick={() => setOpen(true)}
          >
            <Settings />
          </Button>
        </div>
      </div>
      {production && (
        <p className="text-sm" role="status">
          {styleNote[data.style]}
        </p>
      )}
      {problems.map((l) => (
        <p key={l.vendor} role="alert" className="text-sm text-destructive">
          {l.name}: {l.error}
        </p>
      ))}
      {open && <TargetsDialog targets={data} onClose={() => setOpen(false)} />}
    </section>
  )
}

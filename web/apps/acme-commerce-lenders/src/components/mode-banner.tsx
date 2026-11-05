import { useQuery } from '@tanstack/react-query'
import { Settings, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { targetQueries } from '@/api/queries'
import { TargetsDialog } from '@/components/targets-dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/**
 * Says which mode the demo is in, and cannot be dismissed. Demo mode calls the
 * app emulators at their own URLs. Production mode means a vendor has been
 * pointed at its own service's URL, to show that only the base URL changes. The
 * app emulators still answer, and the inspector shows where each call would
 * have gone.
 */
export function ModeBanner() {
  const { data } = useQuery(targetQueries.current())
  const [open, setOpen] = useState(false)

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
          ? 'border-warning bg-warning/10'
          : 'border-primary/50 bg-primary/10',
      )}
    >
      <div className="flex items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <span
            className={cn(
              'flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3 py-1 text-sm font-bold',
              production
                ? 'bg-warning text-warning-foreground'
                : 'bg-primary text-primary-foreground',
            )}
          >
            {production && <TriangleAlert className="size-4" />}
            {production ? 'Production Mode' : 'Demo Mode'}
          </span>
          <span className="text-sm">
            {production
              ? 'Base URLs point at the vendors’ own services. Nothing is sent to them. The app emulators still answer, and the inspector shows where each call would have gone.'
              : 'Every call goes to an app emulator.'}
          </span>
        </div>
        <Button
          variant="outline"
          className="px-2.5"
          aria-label="Change base URLs"
          onClick={() => setOpen(true)}
        >
          <Settings />
        </Button>
      </div>
      {problems.map((l) => (
        <p key={l.vendor} role="alert" className="text-sm text-destructive">
          {l.name}: {l.error}
        </p>
      ))}
      {open && <TargetsDialog targets={data} onClose={() => setOpen(false)} />}
    </section>
  )
}

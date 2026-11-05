import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import type { Targets } from '@/api/types'
import { Button } from '@/components/ui/button'

/**
 * The base URL each vendor is called at. Use emulator and Use service fill the
 * field, so a deleted URL is easy to put back. Saving never needs a credential.
 */
export function TargetsDialog({
  targets,
  onClose,
}: {
  targets: Targets
  onClose: () => void
}) {
  const [urls, setUrls] = useState(() =>
    Object.fromEntries(targets.lanes.map((l) => [l.vendor, l.url])),
  )
  const queryClient = useQueryClient()
  const save = useMutation({
    mutationFn: () => api.setTargets({ urls }),
    onSuccess: (t) => {
      queryClient.setQueryData(['targets'], t)
      onClose()
    },
  })

  return (
    <div className="fixed inset-0 z-20 flex items-center justify-center bg-black/70 p-6">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="targets-title"
        className="flex w-full max-w-2xl flex-col gap-5 rounded-xl border bg-popover p-6 text-popover-foreground shadow-xl"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id="targets-title" className="font-heading text-lg font-bold">
              Base URLs
            </h2>
            <p className="text-sm text-popover-foreground/80">
              The lender calls each vendor at the URL below. Change it and the
              next call goes there. Nothing else in the app changes.
            </p>
          </div>
          <Button
            variant="ghost"
            className="px-2.5"
            aria-label="Close"
            onClick={onClose}
          >
            <X />
          </Button>
        </div>

        {targets.lanes.map((l) => (
          <div key={l.vendor} className="flex flex-col gap-2">
            <label htmlFor={`url-${l.vendor}`} className="font-medium">
              {l.name}
            </label>
            <div className="flex gap-2">
              <input
                id={`url-${l.vendor}`}
                value={urls[l.vendor] ?? ''}
                onChange={(e) =>
                  setUrls({ ...urls, [l.vendor]: e.target.value })
                }
                spellCheck={false}
                className="min-w-0 flex-1 rounded-md border bg-background px-3 py-2 font-mono text-sm"
              />
              <Button
                variant="outline"
                onClick={() => setUrls({ ...urls, [l.vendor]: l.emulator_url })}
              >
                Use emulator
              </Button>
              <Button
                variant="outline"
                onClick={() => setUrls({ ...urls, [l.vendor]: l.service_url })}
              >
                Use service
              </Button>
            </div>
          </div>
        ))}

        {save.error && (
          <p role="alert" className="text-sm text-destructive">
            {save.error.message}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => save.mutate()} disabled={save.isPending}>
            Save
          </Button>
        </div>
      </div>
    </div>
  )
}

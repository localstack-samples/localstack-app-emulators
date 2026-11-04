import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useRef, useState } from 'react'
import { api } from '@/api/client'
import type { Application } from '@/api/types'

export type StepKey =
  | 'apply'
  | 'connect'
  | 'review'
  | 'offer'
  | 'accept'
  | 'fund'
  | 'settle'

export const stepLabels: Record<StepKey, string> = {
  apply: 'The merchant applies',
  connect: 'Connect to Stripe, Plaid and Xero',
  review: 'Read what the three accounts hold',
  offer: 'Price an offer',
  accept: 'The merchant accepts',
  fund: 'Pay the advance to their bank',
  settle: 'Record it in their Xero books',
}

/** The first click runs these. */
export const applySteps: StepKey[] = ['apply', 'connect', 'review', 'offer']
/** The second click runs these. */
export const acceptSteps: StepKey[] = ['accept', 'fund', 'settle']

export interface StepState {
  status: 'pending' | 'running' | 'done' | 'error'
  ms?: number
  error?: string
}

const fresh = (): Record<StepKey, StepState> => ({
  apply: { status: 'pending' },
  connect: { status: 'pending' },
  review: { status: 'pending' },
  offer: { status: 'pending' },
  accept: { status: 'pending' },
  fund: { status: 'pending' },
  settle: { status: 'pending' },
})

/**
 * The demo's flow: two clicks that run the lender's steps in order, a retry
 * that picks up at the step that failed, and a reset.
 */
export function useDemoFlow() {
  const queryClient = useQueryClient()
  const [application, setApplication] = useState<Application>()
  const [steps, setSteps] = useState(fresh)
  const [busy, setBusy] = useState(false)
  const [resetError, setResetError] = useState<string>()
  // The step functions need the application's id before React has re-rendered.
  const current = useRef<Application>()

  const run = useCallback(async (keys: StepKey[]) => {
    setBusy(true)
    for (const key of keys) {
      setSteps((s) => mark(s, key, 'running'))
      const started = performance.now()
      try {
        const next =
          key === 'apply' ? await api.apply() : await call(key, current.current)
        current.current = next
        setApplication(next)
        setSteps((s) =>
          mark(s, key, 'done', { ms: performance.now() - started }),
        )
      } catch (e) {
        setSteps((s) =>
          mark(s, key, 'error', {
            error: e instanceof Error ? e.message : String(e),
          }),
        )
        setBusy(false)
        return
      }
    }
    setBusy(false)
  }, [])

  const reset = useCallback(async () => {
    setBusy(true)
    setResetError(undefined)
    try {
      await api.reset()
      current.current = undefined
      setApplication(undefined)
      setSteps(fresh())
    } catch (e) {
      setResetError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
      await queryClient.invalidateQueries({ queryKey: ['status'] })
    }
  }, [queryClient])

  const failed = (Object.keys(steps) as StepKey[]).find(
    (k) => steps[k].status === 'error',
  )

  return {
    application,
    steps,
    busy,
    resetError,
    failed,
    apply: () => run(applySteps),
    accept: () => run(acceptSteps),
    // Retry the step that failed and carry on to the end of its click.
    retry: () => {
      if (!failed) return
      const keys = applySteps.includes(failed) ? applySteps : acceptSteps
      return run(keys.slice(keys.indexOf(failed)))
    },
    reset,
  }
}

function mark(
  steps: Record<StepKey, StepState>,
  key: StepKey,
  status: StepState['status'],
  extra: Partial<StepState> = {},
): Record<StepKey, StepState> {
  return { ...steps, [key]: { status, ...extra } }
}

function call(key: StepKey, app: Application | undefined) {
  if (!app) throw new Error('There is no application yet.')
  switch (key) {
    case 'connect':
      return api.connect(app.id)
    case 'review':
      return api.review(app.id)
    case 'offer':
      return api.offer(app.id)
    case 'accept':
      return api.accept(app.id)
    case 'fund':
      return api.fund(app.id)
    case 'settle':
      return api.settle(app.id)
    default:
      throw new Error(`Unknown step ${key}`)
  }
}

import { useEffect, useRef, useState } from 'react'
import { api } from '@/api/client'
import type { WireEvent } from '@/api/types'

const pollMs = 700

/**
 * The calls the demo has made, oldest first. It polls for what is new, and
 * starts again from nothing when a reset clears the recording.
 */
export function useEvents(): { events: WireEvent[]; error: string | null } {
  const [events, setEvents] = useState<WireEvent[]>([])
  const [error, setError] = useState<string | null>(null)
  const cursor = useRef({ last: 0, generation: null as number | null })

  useEffect(() => {
    let stopped = false
    let timer: ReturnType<typeof setTimeout>

    const poll = async () => {
      try {
        const known = cursor.current
        const res = await api.getEvents(known.last)
        if (stopped) return
        if (known.generation !== null && res.generation !== known.generation) {
          // The recording was cleared. What we hold is gone, so ask again
          // from the start rather than keep a stale list.
          cursor.current = { last: 0, generation: res.generation }
          setEvents([])
        } else {
          cursor.current = { last: res.last, generation: res.generation }
          if (res.events.length > 0) {
            setEvents((held) => [...held, ...res.events])
          }
        }
        setError(null)
      } catch (e) {
        if (stopped) return
        setError(e instanceof Error ? e.message : 'Could not read the calls')
      }
      if (!stopped) timer = setTimeout(poll, pollMs)
    }
    poll()

    return () => {
      stopped = true
      clearTimeout(timer)
    }
  }, [])

  return { events, error }
}

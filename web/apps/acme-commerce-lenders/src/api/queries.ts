import { queryOptions } from '@tanstack/react-query'
import { api } from './client'

export const statusQueries = {
  // Polled, so a presenter sees an app emulator go down before the demo does.
  current: () =>
    queryOptions({
      queryKey: ['status'],
      queryFn: api.getStatus,
      refetchInterval: 5000,
    }),
}

export const targetQueries = {
  // Polled, so a second tab or a restart shows the current mode.
  current: () =>
    queryOptions({
      queryKey: ['targets'],
      queryFn: api.getTargets,
      refetchInterval: 5000,
    }),
}

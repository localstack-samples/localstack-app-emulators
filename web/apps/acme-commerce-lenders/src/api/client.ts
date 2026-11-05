import type {
  Application,
  EventsResponse,
  Facts,
  Status,
  Targets,
  TargetsUpdate,
} from './types'

/** An error answer from the demo's API. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(
  method: 'GET' | 'POST' | 'PUT',
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    ...(body === undefined
      ? {}
      : {
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        }),
  })
  const answer: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const message =
      answer && typeof answer === 'object' && 'error' in answer
        ? String((answer as { error: unknown }).error)
        : res.statusText
    throw new ApiError(message || `HTTP ${res.status}`, res.status)
  }
  return answer as T
}

const step = (name: string) => (id: string) =>
  request<Application>('POST', `/applications/${id}/${name}`)

export const api = {
  getStatus: () => request<Status>('GET', '/status'),
  getTargets: () => request<Targets>('GET', '/targets'),
  setTargets: (update: TargetsUpdate) =>
    request<Targets>('PUT', '/targets', update),
  getEvents: (since: number) =>
    request<EventsResponse>('GET', `/events?since=${since}`),
  reset: () => request<Facts>('POST', '/reset'),
  apply: () => request<Application>('POST', '/applications'),
  getApplication: (id: string) =>
    request<Application>('GET', `/applications/${id}`),
  connect: step('connect'),
  review: step('review'),
  offer: step('offer'),
  accept: step('accept'),
  fund: step('fund'),
  settle: step('settle'),
}

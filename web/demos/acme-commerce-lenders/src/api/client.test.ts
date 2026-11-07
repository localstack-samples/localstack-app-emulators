import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api } from './client'

function respond(status: number, body: unknown) {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      statusText: status === 409 ? 'Conflict' : 'Bad Gateway',
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  it('calls the right method and path for each step', async () => {
    const fetchMock = respond(200, { id: 'app-0001' })
    vi.stubGlobal('fetch', fetchMock)

    await api.apply()
    await api.connect('app-0001')
    await api.review('app-0001')
    await api.offer('app-0001')
    await api.accept('app-0001')
    await api.fund('app-0001')
    await api.getApplication('app-0001')
    await api.reset()
    await api.getStatus()

    expect(
      fetchMock.mock.calls.map(([url, init]) => `${init.method} ${url}`),
    ).toEqual([
      'POST /api/applications',
      'POST /api/applications/app-0001/connect',
      'POST /api/applications/app-0001/review',
      'POST /api/applications/app-0001/offer',
      'POST /api/applications/app-0001/accept',
      'POST /api/applications/app-0001/fund',
      'GET /api/applications/app-0001',
      'POST /api/reset',
      'GET /api/status',
    ])
  })

  it('throws the server error message with its status', async () => {
    vi.stubGlobal(
      'fetch',
      respond(409, { error: 'cannot offer an application that is applied' }),
    )
    const err = await api.offer('app-0001').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err.message).toBe('cannot offer an application that is applied')
    expect(err.status).toBe(409)
  })

  it('falls back to the status text when the body is not JSON', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          new Response('<html>', { status: 502, statusText: 'Bad Gateway' }),
        ),
    )
    const err = await api.getStatus().catch((e) => e)
    expect(err.message).toBe('Bad Gateway')
    expect(err.status).toBe(502)
  })
})

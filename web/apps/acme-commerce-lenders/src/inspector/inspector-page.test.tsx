import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { EventsResponse, WireEvent } from '@/api/types'
import { readable } from './call-row'
import { InspectorPage } from './inspector-page'

const mocks = vi.hoisted(() => ({ getEvents: vi.fn() }))

vi.mock('@/api/client', async () => {
  const actual =
    await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { ...mocks } }
})

const body = (content = '') => ({
  content,
  size: content.length,
  truncated: false,
})

function event(id: number, over: Partial<WireEvent> = {}): WireEvent {
  return {
    id,
    time: '2026-11-04T10:00:00Z',
    vendor: 'stripe',
    source: 'localstack',
    method: 'GET',
    path: '/v1/charges?limit=100',
    status: 200,
    duration_ns: 12_000_000,
    request: body(),
    response: body('{"data":[]}'),
    ...over,
  }
}

const answer = (
  events: WireEvent[],
  generation = 0,
  last = events.at(-1)?.id ?? 0,
): EventsResponse => ({ events, last, generation })

// Lets the next poll run, and the state it sets settle.
const nextPoll = () => act(() => vi.advanceTimersByTimeAsync(800))

describe('InspectorPage', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    mocks.getEvents.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('says so when the demo has made no calls', async () => {
    mocks.getEvents.mockResolvedValue(answer([]))
    render(<InspectorPage />)
    expect(await screen.findByText(/No calls yet/)).toBeTruthy()
  })

  it('shows each call with a caption, newest first', async () => {
    mocks.getEvents.mockResolvedValue(
      answer([
        event(1),
        event(2, {
          vendor: 'xero',
          source: 'local',
          method: 'PUT',
          path: '/api.xro/2.0/BankTransactions',
        }),
      ]),
    )
    render(<InspectorPage />)
    const rows = await screen.findAllByTestId('call')
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getByText(/Recorded the advance/)).toBeTruthy()
    expect(within(rows[0]).getByText('Local')).toBeTruthy()
    expect(within(rows[1]).getByText(/card sales from Stripe/)).toBeTruthy()
    expect(within(rows[1]).getByText('LocalStack')).toBeTruthy()
  })

  it('asks only for what is new, and adds it to the list', async () => {
    mocks.getEvents
      .mockResolvedValueOnce(answer([event(1)]))
      .mockResolvedValue(answer([event(2)]))
    render(<InspectorPage />)
    await screen.findAllByTestId('call')
    await nextPoll()
    await waitFor(() => expect(screen.getAllByTestId('call')).toHaveLength(2))
    expect(mocks.getEvents).toHaveBeenNthCalledWith(1, 0)
    expect(mocks.getEvents).toHaveBeenNthCalledWith(2, 1)
  })

  it('starts again when a reset clears the recording', async () => {
    mocks.getEvents
      .mockResolvedValueOnce(answer([event(1), event(2)], 0))
      .mockResolvedValueOnce(answer([], 1, 2))
      .mockResolvedValue(answer([event(3)], 1))
    render(<InspectorPage />)
    await waitFor(() => expect(screen.getAllByTestId('call')).toHaveLength(2))
    await nextPoll()
    await screen.findByText(/No calls yet/)
    // After the clear it asks from the start, not from the old cursor.
    await nextPoll()
    await waitFor(() => expect(screen.getAllByTestId('call')).toHaveLength(1))
    expect(mocks.getEvents).toHaveBeenNthCalledWith(3, 0)
  })

  it('filters by vendor', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    mocks.getEvents.mockResolvedValue(
      answer([
        event(1),
        event(2, { vendor: 'plaid', method: 'POST', path: '/auth/get' }),
      ]),
    )
    render(<InspectorPage />)
    await screen.findAllByTestId('call')
    await user.click(screen.getByRole('button', { name: 'Plaid' }))
    const rows = screen.getAllByTestId('call')
    expect(rows).toHaveLength(1)
    expect(within(rows[0]).getByText(/routing numbers from Plaid/)).toBeTruthy()
  })

  it('opens a call to show its request and response', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    mocks.getEvents.mockResolvedValue(answer([event(1)]))
    render(<InspectorPage />)
    const row = await screen.findByTestId('call')
    expect(screen.queryByText(/"data"/)).toBeNull()
    await user.click(within(row).getByRole('button'))
    expect(screen.getByText(/"data": \[\]/)).toBeTruthy()
  })

  it('lays JSON out for reading and leaves other bodies alone', () => {
    expect(readable('{"a":1}')).toBe('{\n  "a": 1\n}')
    expect(readable('amount=100&currency=usd')).toBe('amount=100&currency=usd')
    // A body cut short is not valid JSON, so it is shown as recorded.
    expect(readable('{"a":')).toBe('{"a":')
  })

  it('shows a call that got no answer as one', async () => {
    mocks.getEvents.mockResolvedValue(
      answer([event(1, { status: 0, error: 'connection refused' })]),
    )
    render(<InspectorPage />)
    expect(await screen.findByText('No answer')).toBeTruthy()
  })

  it('says when it cannot reach the demo, and recovers', async () => {
    mocks.getEvents
      .mockRejectedValueOnce(new Error('HTTP 502'))
      .mockResolvedValue(answer([]))
    render(<InspectorPage />)
    expect(await screen.findByRole('alert')).toBeTruthy()
    await nextPoll()
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
  })
})

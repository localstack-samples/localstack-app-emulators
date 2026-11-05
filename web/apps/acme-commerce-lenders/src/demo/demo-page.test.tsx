import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Application, Offer, Status } from '@/api/types'
import { DemoPage } from './demo-page'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  getTargets: vi.fn(),
  reset: vi.fn(),
  apply: vi.fn(),
  connect: vi.fn(),
  review: vi.fn(),
  offer: vi.fn(),
  accept: vi.fn(),
  fund: vi.fn(),
  settle: vi.fn(),
  calls: [] as string[],
}))

vi.mock('@/api/client', async () => {
  const actual =
    await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { ...mocks } }
})

const healthy = (vendor: string, source: 'localstack' | 'local' = 'local') => ({
  vendor,
  source,
  url: `http://${vendor}`,
  state: 'healthy' as const,
})

const readyStatus: Status = {
  ready: true,
  seeded: true,
  emulators: [
    healthy('stripe', 'localstack'),
    healthy('plaid'),
    healthy('xero'),
  ],
  facts: { merchant: 'Harbor & Pine Outfitters', as_of: '2027-02-02' },
}

const approved: Offer = {
  approved: true,
  reason: 'An advance of the last 30 days of card sales.',
  advance_cents: 3000000,
  fee_percent: 8,
  fee_cents: 240000,
  total_repay_cents: 3240000,
  growth_percent: 14.8,
  inputs: [
    {
      source: 'stripe',
      label: 'Card sales, last 30 days',
      value: '$30,423.72',
      call: 'stripe GET /v1/charges',
      used: true,
    },
    {
      source: 'plaid',
      label: 'Checking balance',
      value: '$110.00',
      call: 'plaid POST /accounts/get',
      used: false,
    },
  ],
}

const fundedFunding = {
  amount_cents: 3000000,
  plaid_account_id: 'chk',
  bank_account_id: 'ba_1',
  bank_last4: '0000',
  routing_number: '021000021',
  transfer_id: 'tr_advance',
  payout_id: 'po_advance',
  payout_status: 'pending',
}

const app = (status: Application['status'], extra: Partial<Application> = {}) =>
  ({
    id: 'app-0001',
    merchant: 'Harbor & Pine Outfitters',
    status,
    ...extra,
  }) as Application

function track<T>(name: string, value: T) {
  return vi.fn(async () => {
    mocks.calls.push(name)
    return value
  })
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <DemoPage />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  // Calls and one-off rejections from an earlier test must not leak into this one.
  vi.resetAllMocks()
  mocks.calls.length = 0
  mocks.getStatus.mockResolvedValue(readyStatus)
  mocks.reset.mockImplementation(
    track('reset', {
      merchant: 'Harbor & Pine Outfitters',
      as_of: '2027-02-02',
    }),
  )
  mocks.apply.mockImplementation(track('apply', app('applied')))
  mocks.connect.mockImplementation(track('connect', app('connected')))
  mocks.review.mockImplementation(track('review', app('reviewed')))
  mocks.offer.mockImplementation(
    track('offer', app('offered', { offer: approved })),
  )
  mocks.accept.mockImplementation(
    track('accept', app('accepted', { offer: approved })),
  )
  mocks.fund.mockImplementation(
    track(
      'fund',
      app('funded', {
        offer: approved,
        funding: {
          amount_cents: 3000000,
          plaid_account_id: 'chk',
          bank_account_id: 'ba_1',
          bank_last4: '0000',
          routing_number: '021000021',
          transfer_id: 'tr_advance',
          payout_id: 'po_advance',
          payout_status: 'pending',
        },
      }),
    ),
  )
  mocks.settle.mockImplementation(
    track(
      'settle',
      app('settled', {
        offer: approved,
        funding: fundedFunding,
        settlement: {
          amount_cents: 3000000,
          bank_transaction_id: 'bt_advance',
          bank_account_code: '090',
          liability_code: '810',
          liability_name: 'Working capital advance',
          reference: 'app-0001',
          date: '2027-02-02',
        },
      }),
    ),
  )
})

describe('DemoPage', () => {
  it('shows each app emulator and who serves it', async () => {
    renderPage()
    expect(await screen.findByTestId('emulator-stripe')).toHaveTextContent(
      'LocalStack',
    )
    expect(screen.getByTestId('emulator-plaid')).toHaveTextContent('Local')
    expect(
      screen.getByRole('button', { name: /apply for working capital/i }),
    ).toBeEnabled()
  })

  it('waits, and says what to do, when an app emulator is not ready', async () => {
    mocks.getStatus.mockResolvedValue({
      ...readyStatus,
      ready: false,
      emulators: [
        healthy('stripe', 'localstack'),
        {
          vendor: 'plaid',
          source: 'localstack',
          url: 'http://127.0.0.1:4566',
          state: 'not-included',
          hint: 'this LocalStack release does not include plaid. Run it yourself and set PLAID_EMULATOR_URL.',
        },
        healthy('xero'),
      ],
    } satisfies Status)
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'PLAID_EMULATOR_URL',
    )
    expect(
      screen.getByRole('button', { name: /apply for working capital/i }),
    ).toBeDisabled()
  })

  it('asks for a reset when the world is not seeded', async () => {
    mocks.getStatus.mockResolvedValue({
      ...readyStatus,
      ready: false,
      seeded: false,
      facts: undefined,
    })
    renderPage()
    expect(await screen.findByRole('status')).toHaveTextContent('Press Reset')
  })

  it('runs apply, connect, review and offer on the first click, in order', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )

    const offer = await screen.findByTestId('offer')
    expect(mocks.calls).toEqual(['apply', 'connect', 'review', 'offer'])
    expect(offer).toHaveTextContent('$30,000.00')
    expect(offer).toHaveTextContent('$2,400.00')
    expect(offer).toHaveTextContent('$32,400.00')
    expect(offer).toHaveTextContent('stripe GET /v1/charges')
    expect(offer).toHaveTextContent('Shown only')
  })

  it('runs accept, fund and settle on the second click and shows how the money moved', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )
    await user.click(
      await screen.findByRole('button', {
        name: /accept the offer and fund it/i,
      }),
    )

    const funding = await screen.findByTestId('funding')
    expect(mocks.calls).toEqual([
      'apply',
      'connect',
      'review',
      'offer',
      'accept',
      'fund',
      'settle',
    ])
    expect(funding).toHaveTextContent('Checking ending 0000')
    expect(funding).toHaveTextContent('routing 021000021')
    expect(funding).toHaveTextContent('tr_advance')
    expect(funding).toHaveTextContent('po_advance')
    expect(funding).toHaveTextContent('$30,000.00')

    // The books show where the advance was booked.
    const settlement = await screen.findByTestId('settlement')
    expect(settlement).toHaveTextContent('bank account 090')
    expect(settlement).toHaveTextContent('Working capital advance')
    expect(settlement).toHaveTextContent('810')
    expect(settlement).toHaveTextContent('bt_advance')
  })

  it('retries a failed settlement without paying the advance out again', async () => {
    mocks.settle.mockRejectedValueOnce(
      new Error('xero PUT /BankTransactions: HTTP 500'),
    )
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )
    await user.click(
      await screen.findByRole('button', {
        name: /accept the offer and fund it/i,
      }),
    )

    expect(
      await screen.findByText(/xero PUT \/BankTransactions: HTTP 500/),
    ).toBeInTheDocument()
    // The money has moved and the page says so, and the books are still waiting.
    expect(await screen.findByTestId('funding')).toHaveTextContent('po_advance')
    expect(screen.queryByTestId('settlement')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /retry/i }))
    await screen.findByTestId('settlement')
    expect(mocks.fund).toHaveBeenCalledTimes(1)
    expect(mocks.settle).toHaveBeenCalledTimes(2)
  })

  it('does not offer to accept a declined offer', async () => {
    mocks.offer.mockImplementation(
      track(
        'offer',
        app('offered', {
          offer: {
            ...approved,
            approved: false,
            advance_cents: 0,
            reason: 'The bank account could not be verified.',
          },
        }),
      ),
    )
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )

    expect(await screen.findByTestId('offer')).toHaveTextContent(
      'The bank account could not be verified.',
    )
    expect(
      screen.queryByRole('button', { name: /accept the offer/i }),
    ).not.toBeInTheDocument()
  })

  it('shows a failed step and retries from that step, not from the start', async () => {
    mocks.connect.mockRejectedValueOnce(
      new Error('plaid POST /item/get: HTTP 502'),
    )
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )

    const alert = await screen.findByText(/plaid POST \/item\/get: HTTP 502/)
    expect(alert).toBeInTheDocument()
    expect(mocks.calls).toEqual(['apply']) // review and offer did not run

    await user.click(screen.getByRole('button', { name: /retry/i }))
    await screen.findByTestId('offer')
    expect(mocks.calls).toEqual(['apply', 'connect', 'review', 'offer'])
    expect(mocks.apply).toHaveBeenCalledTimes(1) // the retry did not apply twice
    expect(screen.queryByText(/HTTP 502/)).not.toBeInTheDocument()
  })

  it('puts the demo back at the start on reset', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    )
    await screen.findByTestId('offer')

    await user.click(screen.getByRole('button', { name: /reset/i }))
    await waitFor(() => expect(mocks.reset).toHaveBeenCalledTimes(1))
    expect(
      await screen.findByRole('button', { name: /apply for working capital/i }),
    ).toBeEnabled()
    expect(screen.queryByTestId('offer')).not.toBeInTheDocument()
  })

  it('reports a reset that failed', async () => {
    mocks.reset.mockRejectedValueOnce(new Error('reset failed for plaid'))
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: /reset/i }))
    expect(
      await screen.findByText(/Reset failed: reset failed for plaid/),
    ).toBeInTheDocument()
  })
})

describe('DemoPage in production mode', () => {
  const lanes = (error?: string) => [
    {
      vendor: 'stripe',
      name: 'Stripe',
      url: error ? 'stripe' : 'https://api.stripe.com',
      emulator_url: 'http://stripe.emulator',
      service_url: 'https://api.stripe.com',
      service: true,
      ...(error ? { error } : {}),
    },
  ]

  it('shows the app emulators, which still answer in production mode', async () => {
    mocks.getTargets.mockResolvedValue({ mode: 'production', lanes: lanes() })
    renderPage()
    expect(await screen.findByTestId('emulator-stripe')).toBeInTheDocument()
    const apply = await screen.findByRole('button', {
      name: 'Apply for working capital',
    })
    await waitFor(() => expect(apply).toBeEnabled())
  })

  it('holds Apply back, and says to fix the URL, when a base URL cannot be called', async () => {
    mocks.getTargets.mockResolvedValue({
      mode: 'production',
      lanes: lanes('"stripe" is not a URL the demo can use.'),
    })
    renderPage()
    expect(
      await screen.findByText('Fix the base URL above to run the demo.'),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Apply for working capital' }),
    ).toBeDisabled()
    expect(
      screen.getByText('Fix the base URL above to run the demo.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('is not a URL')
  })
})

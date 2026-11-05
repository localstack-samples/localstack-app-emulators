import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Targets } from '@/api/types'
import { ModeBanner } from './mode-banner'

const mocks = vi.hoisted(() => ({ getTargets: vi.fn(), setTargets: vi.fn() }))

vi.mock('@/api/client', async () => {
  const actual =
    await vi.importActual<typeof import('@/api/client')>('@/api/client')
  return { ...actual, api: { ...mocks } }
})

const lane = (vendor: string, name: string, url: string, service = false) => ({
  vendor,
  name,
  url,
  emulator_url: `http://${vendor}.emulator`,
  service_url: `https://api.${vendor}.com`,
  service,
})

const demo: Targets = {
  mode: 'demo',
  style: 'dry',
  lanes: [
    lane('stripe', 'Stripe', 'http://stripe.emulator'),
    lane('xero', 'Xero', 'http://xero.emulator'),
  ],
}

const production: Targets = {
  mode: 'production',
  style: 'dry',
  lanes: [
    lane('stripe', 'Stripe', 'https://api.stripe.com', true),
    lane('xero', 'Xero', 'http://xero.emulator'),
  ],
}

function renderBanner() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ModeBanner />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('ModeBanner', () => {
  it('says Demo Mode when every vendor is at its app emulator', async () => {
    mocks.getTargets.mockResolvedValue(demo)
    renderBanner()
    expect(await screen.findByText('Demo Mode')).toBeInTheDocument()
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
  })

  it('says Production Mode, with the run style, when a vendor is at its service', async () => {
    mocks.getTargets.mockResolvedValue(production)
    renderBanner()
    expect(await screen.findByText('Production Mode')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Dry run' })).toBeChecked()
    expect(screen.getByRole('status')).toHaveTextContent(/nothing is sent/i)
  })

  it('switches to Live and says requests are sent without a credential', async () => {
    mocks.getTargets.mockResolvedValue(production)
    mocks.setTargets.mockResolvedValue({ ...production, style: 'live' })
    renderBanner()
    await userEvent.click(await screen.findByRole('radio', { name: 'Live' }))
    expect(mocks.setTargets).toHaveBeenCalledWith({ style: 'live' })
    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent(/no credential/i),
    )
  })

  it('cannot be dismissed', async () => {
    mocks.getTargets.mockResolvedValue(demo)
    renderBanner()
    await screen.findByText('Demo Mode')
    expect(
      screen.queryByRole('button', { name: /dismiss|close/i }),
    ).not.toBeInTheDocument()
  })

  it('shows why a base URL cannot be called', async () => {
    mocks.getTargets.mockResolvedValue({
      ...production,
      lanes: [
        {
          ...production.lanes[0],
          url: 'stripe',
          error: '"stripe" is not a URL the demo can call.',
        },
        production.lanes[1],
      ],
    })
    renderBanner()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Stripe: "stripe" is not a URL the demo can call.',
    )
  })
})

describe('the base URL overlay', () => {
  async function open() {
    mocks.getTargets.mockResolvedValue(production)
    renderBanner()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Change base URLs' }),
    )
    return screen.getByRole('dialog', { name: 'Base URLs' })
  }

  it('fills a field from Use emulator and Use service', async () => {
    const dialog = await open()
    const stripe = within(dialog).getByLabelText('Stripe')
    expect(stripe).toHaveValue('https://api.stripe.com')

    await userEvent.clear(stripe)
    expect(stripe).toHaveValue('')

    await userEvent.click(
      within(dialog).getAllByRole('button', { name: 'Use emulator' })[0],
    )
    expect(stripe).toHaveValue('http://stripe.emulator')

    await userEvent.click(
      within(dialog).getAllByRole('button', { name: 'Use service' })[0],
    )
    expect(stripe).toHaveValue('https://api.stripe.com')
  })

  it('saves every field and closes', async () => {
    const dialog = await open()
    mocks.setTargets.mockResolvedValue(demo)
    await userEvent.click(
      within(dialog).getAllByRole('button', { name: 'Use emulator' })[0],
    )
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(mocks.setTargets).toHaveBeenCalledWith({
        urls: {
          stripe: 'http://stripe.emulator',
          xero: 'http://xero.emulator',
        },
      }),
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    )
  })

  it('says what the API refused', async () => {
    const dialog = await open()
    mocks.setTargets.mockRejectedValue(new Error('no vendor named "nope"'))
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'no vendor named "nope"',
    )
  })
})

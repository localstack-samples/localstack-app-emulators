import { describe, expect, it } from 'vitest'
import { caption } from './captions'

describe('caption', () => {
  it('describes the calls the lender makes', () => {
    expect(
      caption({ vendor: 'stripe', method: 'POST', path: '/v1/transfers' }),
    ).toBe("Moved the advance to the merchant's Stripe balance")
    expect(
      caption({
        vendor: 'xero',
        method: 'PUT',
        path: '/api.xro/2.0/BankTransactions',
      }),
    ).toBe("Recorded the advance in the merchant's books")
  })

  it('ignores the query string and the id in the path', () => {
    expect(
      caption({
        vendor: 'stripe',
        method: 'GET',
        path: '/v1/charges?limit=100&created[gte]=1',
      }),
    ).toBe("Read the merchant's card sales from Stripe")
    expect(
      caption({
        vendor: 'stripe',
        method: 'POST',
        path: '/v1/accounts/acct_123/external_accounts',
      }),
    ).toBe("Registered the merchant's bank account in Stripe")
  })

  it('tells reads from writes on the same Xero path', () => {
    const path = '/api.xro/2.0/Accounts'
    expect(caption({ vendor: 'xero', method: 'GET', path })).toMatch(/Checked/)
    expect(caption({ vendor: 'xero', method: 'PUT', path })).toMatch(/Added/)
  })

  it('falls back to the call itself for one it does not know', () => {
    expect(
      caption({ vendor: 'stripe', method: 'GET', path: '/v1/refunds?x=1' }),
    ).toBe('GET /v1/refunds')
  })
})

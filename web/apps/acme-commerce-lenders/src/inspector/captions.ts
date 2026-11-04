// Plain-language captions for the calls the demo makes. A caption says what the
// lender is doing, so someone watching can follow the feed without reading
// vendor paths.

interface Rule {
  vendor: string
  method: string
  path: RegExp
  caption: string
}

const rules: Rule[] = [
  {
    vendor: 'stripe',
    method: 'GET',
    path: /^\/v1\/accounts\/[^/]+$/,
    caption: "Looked up the merchant's Stripe account",
  },
  {
    vendor: 'stripe',
    method: 'GET',
    path: /^\/v1\/charges$/,
    caption: "Read the merchant's card sales from Stripe",
  },
  {
    vendor: 'stripe',
    method: 'GET',
    path: /^\/v1\/payouts$/,
    caption: "Read the merchant's payouts from Stripe",
  },
  {
    vendor: 'stripe',
    method: 'POST',
    path: /^\/v1\/accounts\/[^/]+\/external_accounts$/,
    caption: "Registered the merchant's bank account in Stripe",
  },
  {
    vendor: 'stripe',
    method: 'POST',
    path: /^\/v1\/transfers$/,
    caption: "Moved the advance to the merchant's Stripe balance",
  },
  {
    vendor: 'stripe',
    method: 'POST',
    path: /^\/v1\/payouts$/,
    caption: "Paid the advance out to the merchant's bank",
  },
  {
    vendor: 'plaid',
    method: 'POST',
    path: /^\/item\/get$/,
    caption: 'Checked the bank connection is healthy',
  },
  {
    vendor: 'plaid',
    method: 'POST',
    path: /^\/accounts\/get$/,
    caption: 'Read the bank account balances from Plaid',
  },
  {
    vendor: 'plaid',
    method: 'POST',
    path: /^\/transactions\/get$/,
    caption: 'Read recent bank transactions from Plaid',
  },
  {
    vendor: 'plaid',
    method: 'POST',
    path: /^\/auth\/get$/,
    caption: 'Fetched the bank account and routing numbers from Plaid',
  },
  {
    vendor: 'xero',
    method: 'GET',
    path: /^\/api\.xro\/2\.0\/Organisation$/,
    caption: "Looked up the merchant's company in Xero",
  },
  {
    vendor: 'xero',
    method: 'GET',
    path: /^\/api\.xro\/2\.0\/Invoices$/,
    caption: "Read the merchant's invoices from Xero",
  },
  {
    vendor: 'xero',
    method: 'GET',
    path: /^\/api\.xro\/2\.0\/Accounts$/,
    caption: "Checked the merchant's chart of accounts in Xero",
  },
  {
    vendor: 'xero',
    method: 'PUT',
    path: /^\/api\.xro\/2\.0\/Accounts$/,
    caption: 'Added a working capital advance account to the books',
  },
  {
    vendor: 'xero',
    method: 'GET',
    path: /^\/api\.xro\/2\.0\/Contacts$/,
    caption: 'Checked whether the lender is a contact in Xero',
  },
  {
    vendor: 'xero',
    method: 'PUT',
    path: /^\/api\.xro\/2\.0\/Contacts$/,
    caption: 'Added the lender as a contact in Xero',
  },
  {
    vendor: 'xero',
    method: 'GET',
    path: /^\/api\.xro\/2\.0\/BankTransactions$/,
    caption: 'Checked whether the advance is already in the books',
  },
  {
    vendor: 'xero',
    method: 'PUT',
    path: /^\/api\.xro\/2\.0\/BankTransactions$/,
    caption: "Recorded the advance in the merchant's books",
  },
]

/** Says what a call to a vendor was for, in a sentence. */
export function caption(call: {
  vendor: string
  method: string
  path: string
}): string {
  const path = call.path.split('?')[0]
  const rule = rules.find(
    (r) =>
      r.vendor === call.vendor && r.method === call.method && r.path.test(path),
  )
  return rule?.caption ?? `${call.method} ${path}`
}

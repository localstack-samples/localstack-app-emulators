// These mirror the JSON the Go API returns (apps/acme-commerce-lenders/lender
// and internal/api). Amounts are US cents.

export type ApplicationStatus =
  | 'applied'
  | 'connected'
  | 'reviewed'
  | 'offered'
  | 'accepted'
  | 'funded'

export interface Connection {
  vendor: string
  detail: string
}

export interface OfferInput {
  source: 'stripe' | 'plaid' | 'xero'
  label: string
  value: string
  call: string
  used: boolean
}

export interface Offer {
  approved: boolean
  reason: string
  advance_cents: number
  fee_percent: number
  fee_cents: number
  total_repay_cents: number
  growth_percent: number
  inputs: OfferInput[]
}

export interface Funding {
  amount_cents: number
  plaid_account_id: string
  bank_account_id?: string
  bank_last4?: string
  routing_number?: string
  transfer_id?: string
  payout_id?: string
  payout_status?: string
}

export interface Application {
  id: string
  merchant: string
  status: ApplicationStatus
  connections?: Connection[]
  offer?: Offer
  funding?: Funding
}

export type EmulatorState =
  | 'healthy'
  | 'unreachable'
  | 'not-included'
  | 'unhealthy'

export interface EmulatorStatus {
  vendor: string
  /** Who serves the app emulator: LocalStack's gateway, or a process you run. */
  source: 'localstack' | 'local'
  url: string
  state: EmulatorState
  detail?: string
  /** What to do about a state that is not healthy. */
  hint?: string
}

export interface Facts {
  merchant: string
  as_of: string
}

export interface Status {
  ready: boolean
  seeded: boolean
  emulators: EmulatorStatus[]
  facts?: Facts
}

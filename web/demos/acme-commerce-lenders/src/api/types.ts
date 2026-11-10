// These mirror the JSON the Go API returns (demos/acme-commerce-lenders/lender
// and internal/api). Amounts are US cents.

export type ApplicationStatus =
  | 'applied'
  | 'connected'
  | 'reviewed'
  | 'offered'
  | 'accepted'
  | 'funded'
  | 'settled'

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

export interface Settlement {
  amount_cents: number
  bank_transaction_id: string
  bank_account_code: string
  liability_code: string
  liability_name: string
  reference: string
  date: string
}

export interface Application {
  id: string
  merchant: string
  status: ApplicationStatus
  connections?: Connection[]
  offer?: Offer
  funding?: Funding
  settlement?: Settlement
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
  /**
   * Each app emulator that is healthy but no longer holds the seeded world,
   * such as one that restarted. Reset puts it right.
   */
  stale?: { vendor: string; reason: string }[]
}

export interface WireBody {
  content: string
  size: number
  truncated: boolean
}

/** One call the demo made to a vendor, as the recorder kept it. */
export interface WireEvent {
  id: number
  time: string
  vendor: string
  source: 'localstack' | 'local'
  /** Who produced the response. */
  origin: 'emulator' | 'vendor'
  /** Set when the demo was pointed at the vendor's own URL: where the call
   * would have gone. The app emulator answered it, and nothing was sent. */
  would_send_to?: string
  method: string
  path: string
  status: number
  duration_ns: number
  request: WireBody
  response: WireBody
  error?: string
}

export interface EventsResponse {
  events: WireEvent[]
  last: number
  /** Changes when a reset clears the recording. */
  generation: number
}

/** One vendor, as the switch holds it. */
export interface Lane {
  vendor: string
  name: string
  url: string
  emulator_url: string
  service_url: string
  /** The URL is not the app emulator's. */
  service: boolean
  /** Why the URL cannot be used. */
  error?: string
}

export interface Targets {
  mode: 'demo' | 'production'
  lanes: Lane[]
}

export interface TargetsUpdate {
  urls: Record<string, string>
}

import { ChevronRight } from 'lucide-react'
import { useState } from 'react'
import type { WireBody, WireEvent } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { caption } from './captions'

const vendorNames: Record<string, string> = {
  stripe: 'Stripe',
  plaid: 'Plaid',
  xero: 'Xero',
}

const sourceName: Record<WireEvent['source'], string> = {
  localstack: 'LocalStack',
  local: 'Local',
  service: 'Vendor service',
}

export const vendorName = (vendor: string) => vendorNames[vendor] ?? vendor

function ms(ns: number) {
  const n = ns / 1e6
  return n < 10 ? `${n.toFixed(1)} ms` : `${Math.round(n)} ms`
}

/** JSON laid out for reading; anything else, such as a form body, as it is. */
export function readable(content: string) {
  try {
    return JSON.stringify(JSON.parse(content), null, 2)
  } catch {
    return content
  }
}

function Body({ title, body }: { title: string; body: WireBody }) {
  return (
    <div className="min-w-0 flex-1">
      <div className="mb-1 text-xs font-medium text-muted-foreground">
        {title}
        {body.truncated && ` (first part of ${body.size} bytes)`}
      </div>
      <pre className="max-h-72 overflow-auto rounded-md bg-muted p-3 text-xs">
        {body.size === 0 ? 'No body' : readable(body.content)}
      </pre>
    </div>
  )
}

/** One call to a vendor: what it was for, then the call, then its bodies. */
export function CallRow({ event }: { event: WireEvent }) {
  const [open, setOpen] = useState(false)
  const blocked = event.origin === 'blocked'
  const failed = !blocked && (event.error !== undefined || event.status >= 400)
  const when = new Date(event.time).toLocaleTimeString()

  return (
    <li className="border-b last:border-b-0" data-testid="call">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/50"
      >
        <ChevronRight
          className={cn(
            'size-4 shrink-0 transition-transform',
            open && 'rotate-90',
          )}
        />
        <div className="min-w-0 flex-1">
          <div className="font-medium">{caption(event)}</div>
          <div className="truncate font-mono text-xs text-muted-foreground">
            {event.method} {event.upstream ?? ''}
            {event.path}
          </div>
        </div>
        <Badge variant="default">{vendorName(event.vendor)}</Badge>
        <Badge variant={blocked ? 'destructive' : 'info'}>
          {blocked ? 'Dry run, not sent' : sourceName[event.source]}
        </Badge>
        <Badge variant={failed ? 'destructive' : 'success'}>
          {blocked ? 'Not sent' : event.error ? 'No answer' : event.status}
        </Badge>
        <span className="w-16 text-right text-xs text-muted-foreground">
          {ms(event.duration_ns)}
        </span>
        <span className="w-20 text-right text-xs text-muted-foreground">
          {when}
        </span>
      </button>
      {open && (
        <div className="flex flex-col gap-3 px-11 pb-4">
          {event.error && (
            <p role="alert" className="text-sm text-destructive">
              {event.error}
            </p>
          )}
          <div className="flex gap-4">
            <Body title="Request" body={event.request} />
            <Body title="Response" body={event.response} />
          </div>
        </div>
      )}
    </li>
  )
}
